// Package apimemories serves the Memories place (ADR-0064): the overview of
// a day and each memory of it.
package apimemories

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	apilibrary "huepattl.de/unterlumen/internal/api/library"
	"huepattl.de/unterlumen/internal/birthdays"
	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/memories"
)

type handler struct {
	mgr   *library.Manager
	bdays *birthdays.Store // nil: no birthdays

	mu       sync.Mutex
	lastKey  string
	lastView memories.Overview
}

func Handle(mux *http.ServeMux, mgr *library.Manager, bdays *birthdays.Store) {
	h := &handler{mgr: mgr, bdays: bdays}
	mux.HandleFunc("GET /api/memories", h.overview)
	mux.HandleFunc("GET /api/memories/detail", h.detail)
}

// request is what every memories request names: the browser's day, the
// libraries and the shared filter without its months and stars.
type request struct {
	day    time.Time
	ids    []string
	filter library.Filter
}

func readRequest(r *http.Request) (request, bool) {
	day, err := time.Parse("2006-01-02", r.URL.Query().Get("day"))
	if err != nil {
		return request{}, false
	}
	f := apilibrary.ScopeFilter(r)
	f.From, f.Until, f.Rating = "", "", ""
	var ids []string
	if v := r.URL.Query().Get("ids"); v != "" {
		ids = strings.Split(v, ",")
	}
	return request{day: day, ids: ids, filter: f}, true
}

// photos reads what the memories are chosen from, and a key that changes
// with it.
func (h *handler) photos(r *http.Request, req request) ([]library.MemoryPhoto, map[string]string, string, error) {
	photos, version, err := h.mgr.MemoryPhotos(r.Context(), req.ids, req.filter)
	if err != nil {
		return nil, nil, "", err
	}
	births := map[string]string{}
	if h.bdays != nil {
		if all, err := h.bdays.All(); err == nil {
			births = all
		}
	}
	return photos, births, fmt.Sprintf("%s|%s|%v|%s", version, req.day.Format("2006-01-02"), req.ids, birthsKey(births)), nil
}

func birthsKey(births map[string]string) string {
	parts := make([]string, 0, len(births))
	for name, day := range births {
		parts = append(parts, name+"="+day)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (h *handler) overview(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(r)
	if !ok {
		http.Error(w, "day must be a date as YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	photos, births, key, err := h.photos(r, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if key != h.lastKey {
		h.lastView, h.lastKey = memories.Build(req.day, photos, births), key
	}
	writeJSON(w, h.lastView)
}

func (h *handler) detail(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(r)
	if !ok {
		http.Error(w, "day must be a date as YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	photos, births, _, err := h.photos(r, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d, ok := memories.BuildDetail(req.day, r.URL.Query().Get("kind"), r.URL.Query().Get("key"), photos, births)
	if !ok {
		http.Error(w, "no such memory today", http.StatusNotFound)
		return
	}
	writeJSON(w, d)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
