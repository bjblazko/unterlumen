// Package apithings serves things: installing what finds them, the things
// in a photo and in the libraries (ADR-0054).
package apithings

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"huepattl.de/unterlumen/internal/jobs"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/things"
	"huepattl.de/unterlumen/internal/toolinstall"
)

// Handle registers the things routes.
func Handle(mux *http.ServeMux, mgr *lib.Manager, reg *jobs.Registry) {
	mux.HandleFunc("GET /api/things", status(mgr))
	mux.HandleFunc("POST /api/things/install", install(mgr, reg))
	mux.HandleFunc("GET /api/library/things", totals(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}/things", photoThings(mgr))
	mux.HandleFunc("DELETE /api/library/{id}/photo/{photoID}/things/{thing}", removeThing(mgr))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func status(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, struct {
			Installed bool                  `json:"installed"`
			Outdated  bool                  `json:"outdated"` // an earlier model is there, which the download replaces
			Plan      toolinstall.ModelPlan `json:"plan"`
		}{mgr.ThingsInstalled(), mgr.ThingsOutdated(), toolinstall.PlanThings(mgr.RuntimeDir())})
	}
}

// install downloads ONNX Runtime, unless it is there, and the things
// models, as a job in the status line that goes on when the page is left;
// the request waits for it. Every library is looked at once it is done.
func install(mgr *lib.Manager, reg *jobs.Registry) http.HandlerFunc {
	var busy sync.Mutex
	return func(w http.ResponseWriter, r *http.Request) {
		if !busy.TryLock() {
			http.Error(w, "Things are being installed already.", http.StatusConflict)
			return
		}
		job := reg.Start("tools", "Installing things", "statistics")
		done := make(chan error, 1)
		go func() {
			defer busy.Unlock()
			err := toolinstall.InstallThings(context.Background(), job, mgr.RuntimeDir(), mgr.ThingsDir())
			job.Finish(err)
			done <- err
			if err == nil {
				mgr.FindAllThingsMissing()
			}
		}()
		select {
		case err := <-done:
			if err != nil {
				http.Error(w, "Things were not installed: "+err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		}
	}
}

// totals lists the things in the libraries ids (comma-separated; all when
// empty) with their photos.
func totals(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		for _, id := range strings.Split(r.URL.Query().Get("ids"), ",") {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		list, err := mgr.ThingTotals(ids)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	}
}

func photoThings(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, err := mgr.OpenStore(r.PathValue("id"))
		if err != nil {
			http.Error(w, "The library was not found.", http.StatusNotFound)
			return
		}
		found, err := store.ThingsOf(store.CurrentPhotoID(r.PathValue("photoID")))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if found == nil {
			found = []things.Found{}
		}
		writeJSON(w, found)
	}
}

func removeThing(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, err := mgr.OpenStore(r.PathValue("id"))
		if err != nil {
			http.Error(w, "The library was not found.", http.StatusNotFound)
			return
		}
		if err := store.RemoveThing(store.CurrentPhotoID(r.PathValue("photoID")), r.PathValue("thing")); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		mgr.InvalidateStatsCache(r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	}
}
