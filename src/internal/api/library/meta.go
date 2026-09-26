package apilibrary

import (
	"encoding/json"
	"net/http"
	"strings"

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

		if err := store.UpsertMeta(photoID, body.Key, body.Value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if body.Key == "title" {
			if pathHint, phErr := store.GetPhotoPathHint(photoID); phErr == nil && pathHint != "" {
				media.WriteTitle(pathHint, body.Value) //nolint:errcheck
			}
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

		if strings.HasPrefix(key, PendingPrefix) && draftStore != nil {
			DeletePendingMeta(store, draftStore, id, photoID, key)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(key, BuildPrefix) && chStore != nil {
			handled, rmErr := DeleteBuiltMeta(store, chStore, photoID, key)
			if rmErr != nil {
				http.Error(w, "remove from site: "+rmErr.Error(), http.StatusInternalServerError)
				return
			}
			if handled {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		if key == "title" {
			if pathHint, phErr := store.GetPhotoPathHint(photoID); phErr == nil && pathHint != "" {
				media.WriteTitle(pathHint, "") //nolint:errcheck
			}
		}
		if err := store.DeleteMeta(photoID, key); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
