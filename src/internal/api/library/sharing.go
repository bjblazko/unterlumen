package apilibrary

import (
	"net/http"
	"strings"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/pathguard"
)

// --- Sharing a library between installations (ADR-0047) ---

// sharedMarker answers whether the folder at path is a library another
// installation shares, so New library can show its name before adding it:
// {"marker": {...}} or {"marker": null}.
func sharedMarker(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		absPath, ok := pathguard.SafePath(root, strings.TrimPrefix(r.URL.Query().Get("path"), "/"))
		if !ok {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		mk, _ := lib.ReadMarker(absPath) // an unreadable marker is no offer
		writeJSON(w, struct {
			Marker *lib.Marker `json:"marker"`
		}{mk})
	}
}

func shareLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return libraryAction(mgr, root, mgr.ShareLibrary)
}

func unshareLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return libraryAction(mgr, root, mgr.UnshareLibrary)
}

// joinSharedLibrary makes the library the shared library its folder is marked
// as and points this installation's drafts at the new ID. The response is the
// library under that ID.
func joinSharedLibrary(mgr *lib.Manager, root string, drafts *channels.DraftStore) http.HandlerFunc {
	return libraryAction(mgr, root, func(id string) (*lib.Library, error) {
		joined, err := mgr.JoinShared(id)
		if err != nil {
			return nil, err
		}
		if drafts != nil {
			if err := drafts.RekeyLibrary(id, joined.ID); err != nil {
				return nil, err
			}
		}
		return joined, nil
	})
}

// libraryAction runs act on the library in the path and answers with the
// library as it is afterwards; a refusal is a 409 with act's sentence.
func libraryAction(mgr *lib.Manager, root string, act func(id string) (*lib.Library, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l, err := act(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		mgr.Annotate(l)
		writeJSON(w, toLibraryJSON(l, root, mgr.IsScanning(l.ID)))
	}
}
