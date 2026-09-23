package apilibrary

import (
	"encoding/json"
	"net/http"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
)

// registerDraftRoutes wires the collect/list/delete draft endpoints onto mux.
// Split out from Handle (handler.go) because drafts are a self-contained
// concern layered on top of the channel store, not core library CRUD.
func registerDraftRoutes(mux *http.ServeMux, mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) {
	mux.HandleFunc("POST /api/library/{id}/channels/{slug}/drafts", collectDraft(mgr, chStore, draftStore))
	mux.HandleFunc("GET /api/channels/{slug}/drafts", listDrafts(draftStore))
	mux.HandleFunc("DELETE /api/channels/{slug}/drafts/{draftID}", deleteDraft(mgr, draftStore))
	mux.HandleFunc("DELETE /api/channels/{slug}/drafts/{draftID}/photos/{libID}/{photoID}", removeDraftPhoto(mgr, draftStore))
}

func collectDraft(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil || draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		libID := r.PathValue("id")
		slug := r.PathValue("slug")

		var body struct {
			PhotoIDs []string `json:"photoIDs"`
			DraftID  string   `json:"draftID,omitempty"`
			PostID   string   `json:"postID,omitempty"`
			Title    string   `json:"title,omitempty"`
			Unlisted bool     `json:"unlisted,omitempty"`
			Account  string   `json:"account,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if len(body.PhotoIDs) == 0 {
			http.Error(w, "photoIDs required", http.StatusBadRequest)
			return
		}
		if _, err := mgr.OpenStore(libID); err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		ch, chErr := chStore.Get(slug)
		if chErr != nil {
			http.Error(w, "channel not found: "+chErr.Error(), http.StatusBadRequest)
			return
		}

		photos := make([]channels.DraftPhoto, len(body.PhotoIDs))
		for i, id := range body.PhotoIDs {
			photos[i] = channels.DraftPhoto{LibraryID: libID, PhotoID: id}
		}

		var (
			draft *channels.Draft
			err   error
		)
		if body.DraftID != "" {
			draft, err = draftStore.AppendPhotos(slug, body.DraftID, photos)
		} else {
			draft, err = draftStore.Create(slug, channels.DraftTarget{
				PostID: body.PostID, Title: body.Title, Unlisted: body.Unlisted, Account: body.Account,
			}, photos)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if store, openErr := mgr.OpenStore(libID); openErr == nil {
			defer store.Close()
			writePendingMarkers(store, body.PhotoIDs, slug, draft, draftAlbumTitle(ch, chStore, draft))
		}

		writeJSON(w, draft)
	}
}

// draftAlbumTitle is the album name to show for a pending photo: the new
// album's title, or — when the draft appends to an already-published album —
// that album's stored title.
func draftAlbumTitle(ch *channels.Channel, chStore *channels.Store, draft *channels.Draft) string {
	if draft.Target.Title != "" || draft.Target.PostID == "" {
		return draft.Target.Title
	}
	items, err := collectGalleryItems(ch, chStore.OutputDir(ch.Slug))
	if err != nil {
		return ""
	}
	for _, it := range items {
		if it.PostID == draft.Target.PostID {
			return it.Title
		}
	}
	return ""
}

// writePendingMarkers records that these photos are collected but not yet
// published. Two keys per photo: an unqualified per-channel marker (kept for
// the meta-key list and existing consumers) and a per-draft key carrying the
// album title — a photo can be pending in several albums of one channel at
// once, and the unqualified key alone can only remember the most recent.
func writePendingMarkers(store *lib.Store, photoIDs []string, slug string, draft *channels.Draft, albumTitle string) {
	for _, id := range photoIDs {
		store.UpsertMeta(id, "pending:"+slug, draft.ID)                //nolint:errcheck
		store.UpsertMeta(id, "pending:"+slug+":"+draft.ID, albumTitle) //nolint:errcheck
	}
}

func listDrafts(draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		drafts, err := draftStore.List(r.PathValue("slug"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, drafts)
	}
}

// clearDraftPending drops the pending markers a draft's photos carry, so the
// library stops claiming a photo is collected into a gallery that no longer
// holds it. Photos of a draft can come from several libraries, so each
// library's store is opened once.
func clearDraftPending(mgr *lib.Manager, slug, draftID string, photos []channels.DraftPhoto) {
	if mgr == nil {
		return
	}
	stores := map[string]*lib.Store{}
	defer func() {
		for _, s := range stores {
			s.Close() //nolint:errcheck
		}
	}()
	for _, p := range photos {
		s, ok := stores[p.LibraryID]
		if !ok {
			opened, err := mgr.OpenStore(p.LibraryID)
			if err != nil {
				continue // a library that is gone cannot hold a stale marker either
			}
			s = opened
			stores[p.LibraryID] = s
		}
		clearPendingMarkers(s, p.PhotoID, slug, draftID)
	}
}

func deleteDraft(mgr *lib.Manager, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug, draftID := r.PathValue("slug"), r.PathValue("draftID")

		// Read the draft before deleting it: afterwards nothing records which
		// photos carried this draft's pending markers.
		var photos []channels.DraftPhoto
		if drafts, err := draftStore.List(slug); err == nil {
			for _, d := range drafts {
				if d.ID == draftID {
					photos = d.Photos
					break
				}
			}
		}

		if err := draftStore.Delete(slug, draftID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		clearDraftPending(mgr, slug, draftID, photos)
		w.WriteHeader(http.StatusNoContent)
	}
}

func removeDraftPhoto(mgr *lib.Manager, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug, draftID := r.PathValue("slug"), r.PathValue("draftID")
		libID, photoID := r.PathValue("libID"), r.PathValue("photoID")
		draft, err := draftStore.RemovePhoto(slug, draftID, libID, photoID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		clearDraftPending(mgr, slug, draftID, []channels.DraftPhoto{{LibraryID: libID, PhotoID: photoID}})
		if draft == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, draft)
	}
}
