package apilibrary

import (
	"net/http"
	"os"

	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// --- Duplicates (ADR-0059) ---

// libraryDuplicates serves the duplicates in all libraries.
func libraryDuplicates(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := mgr.Duplicates(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, report)
	}
}

// deleteLibraryFile deletes one file of a library, named by its absolute
// path, with its sidecar unless another file shares it. Unlike deleting by
// photo ID it takes the copy meant, not the one the index names first;
// another copy of the same content stays in the index. Only a file the
// library holds is deleted.
func deleteLibraryFile(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		store, err := mgr.OpenStore(r.PathValue("id"))
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		if _, _, _, found, err := store.GetPathCache(path); err != nil || !found {
			http.Error(w, "the library holds no file at that path", http.StatusNotFound)
			return
		}
		if err := os.Remove(path); err != nil {
			writeJSON(w, map[string]any{"file": path, "success": false, "error": err.Error()})
			return
		}
		media.RemoveSidecar(path) //nolint:errcheck
		mgr.ForgetFiles([]string{path})
		writeJSON(w, map[string]any{"file": path, "success": true})
	}
}
