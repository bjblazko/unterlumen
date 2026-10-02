// Package apitimeline serves the timeline (ADR-0040): the skeleton the
// browser lays it out from, and the details of the photos in view.
package apitimeline

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	apilibrary "huepattl.de/unterlumen/internal/api/library"
	"huepattl.de/unterlumen/internal/timeline"
)

// maxPage caps one details request; the browser asks for 500 at a time.
const maxPage = 500

func Handle(mux *http.ServeMux, b *timeline.Builder) {
	mux.HandleFunc("GET /api/timeline", skeleton(b))
	mux.HandleFunc("GET /api/timeline/photos", photos(b))
}

func skeleton(b *timeline.Builder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := b.Current(scopeOf(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, s.Skeleton())
	}
}

// photos answers [libraryID, photoID, filename, taken] for an index range.
// Another version than the current one is a 409: the browser's indexes
// point at other photos now.
func photos(b *timeline.Builder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, count, ok := pageRange(r)
		if !ok {
			http.Error(w, "from and count must be whole numbers, from at least 0 and count at least 1", http.StatusBadRequest)
			return
		}
		s, err := b.Current(scopeOf(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if r.URL.Query().Get("v") != s.Version {
			http.Error(w, "the timeline changed; load it again", http.StatusConflict)
			return
		}
		page := s.Page(from, min(count, maxPage))
		rows := make([][4]string, len(page))
		for i, p := range page {
			rows[i] = [4]string{p.LibraryID, p.ID, p.Filename, p.Taken}
		}
		writeJSON(w, map[string]any{"photos": rows})
	}
}

// scopeOf reads the libraries and the shared filter (ADR-0050). Time is the
// Timeline's own axis, so the filter's months do not narrow it.
func scopeOf(r *http.Request) timeline.Scope {
	f := apilibrary.ScopeFilter(r)
	f.From, f.Until = "", ""
	var ids []string
	if v := r.URL.Query().Get("ids"); v != "" {
		ids = strings.Split(v, ",")
	}
	return timeline.Scope{IDs: ids, Filter: f}
}

func pageRange(r *http.Request) (from, count int, ok bool) {
	from, errFrom := strconv.Atoi(r.URL.Query().Get("from"))
	count, errCount := strconv.Atoi(r.URL.Query().Get("count"))
	return from, count, errFrom == nil && errCount == nil && from >= 0 && count >= 1
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
