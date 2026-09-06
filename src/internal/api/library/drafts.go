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
	mux.HandleFunc("DELETE /api/channels/{slug}/drafts/{draftID}", deleteDraft(draftStore))
	mux.HandleFunc("DELETE /api/channels/{slug}/drafts/{draftID}/photos/{libID}/{photoID}", removeDraftPhoto(draftStore))
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
		if _, err := chStore.Get(slug); err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
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
			for _, id := range body.PhotoIDs {
				store.UpsertMeta(id, "pending:"+slug, draft.ID) //nolint:errcheck
			}
		}

		writeJSON(w, draft)
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

func deleteDraft(draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		if err := draftStore.Delete(r.PathValue("slug"), r.PathValue("draftID")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func removeDraftPhoto(draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		draft, err := draftStore.RemovePhoto(
			r.PathValue("slug"), r.PathValue("draftID"), r.PathValue("libID"), r.PathValue("photoID"),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if draft == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, draft)
	}
}
