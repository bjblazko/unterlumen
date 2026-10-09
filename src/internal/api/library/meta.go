package apilibrary

import (
	"encoding/json"
	"net/http"
	"strings"

	"huepattl.de/unterlumen/internal/api/publish"
	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// --- Metadata ---

func getMeta(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		// The sidecar is where notes live; another installation may have
		// changed it since the last scan.
		if path, err := store.GetPhotoPathHint(photoID); err == nil && path != "" {
			if notes, err := media.ReadNotes(path); err == nil {
				store.ApplyNotes(photoID, notes, store.NotesInSidecars()) //nolint:errcheck
			}
		}
		entries, err := store.GetMeta(photoID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, normalizeBuiltMetaKeys(entries))
	}
}

// normalizeBuiltMetaKeys rewrites legacy "published:"-prefixed meta keys to the
// current "built:" prefix for API responses, so the frontend only ever has to deal
// with the current name. If a photo already has a "built:" entry with the same
// suffix (e.g. after a rebuild), the legacy entry is dropped instead of renamed, to
// avoid presenting duplicate/conflicting entries for the same channel.
func normalizeBuiltMetaKeys(entries []lib.MetaEntry) []lib.MetaEntry {
	existing := make(map[string]bool, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "built:") {
			existing[e.Key] = true
		}
	}
	out := make([]lib.MetaEntry, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "published:") {
			builtKey := "built:" + strings.TrimPrefix(e.Key, "published:")
			if existing[builtKey] {
				continue // superseded by a current built: entry
			}
			e.Key = builtKey
		}
		out = append(out, e)
	}
	return out
}

func upsertMeta(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		var body struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
			http.Error(w, "key and value required", http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		if err := writeMeta(store, photoID, body.Key, body.Value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteMeta(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "key query param required", http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		if strings.HasPrefix(key, publish.PendingPrefix) && draftStore != nil {
			publish.DeletePendingMeta(store, draftStore, id, photoID, key)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(key, publish.BuildPrefix) && chStore != nil {
			handled, rmErr := publish.DeleteBuiltMeta(store, chStore, photoID, key)
			if rmErr != nil {
				http.Error(w, "remove from gallery: "+rmErr.Error(), http.StatusInternalServerError)
				return
			}
			if handled {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		if err := writeMeta(store, photoID, key, ""); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// writeMeta writes a note through the sidecar (ADR-0048); any other key is
// the index's own. An empty value removes the key.
func writeMeta(store *lib.Store, photoID, key, value string) error {
	if lib.IsNoteKey(key) {
		return store.WriteNote(photoID, key, value)
	}
	if value == "" {
		return store.DeleteMeta(photoID, key)
	}
	return store.UpsertMeta(photoID, key, value)
}
