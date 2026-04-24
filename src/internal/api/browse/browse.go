package browse

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

type browseWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type browseResponse struct {
	Path     string          `json:"path"`
	Entries  []media.Entry   `json:"entries"`
	Warnings []browseWarning `json:"warnings,omitempty"`
}

type browseDatesResponse struct {
	Ready bool                 `json:"ready"`
	Dates map[string]time.Time `json:"dates,omitempty"`
}

type browseMetaResponse struct {
	Ready bool                        `json:"ready"`
	Meta  map[string]*media.EntryMeta `json:"meta,omitempty"`
}

// Handle registers all /api/browse/* routes on mux.
func Handle(mux *http.ServeMux, root string, cache *media.ScanCache) {
	mux.HandleFunc("/api/browse", handleBrowse(root, cache))
	mux.HandleFunc("/api/browse/dates", handleBrowseDates(root, cache))
	mux.HandleFunc("/api/browse/meta", handleBrowseMeta(root, cache))
	mux.HandleFunc("/api/thumbnail", handleThumbnail(root))
	mux.HandleFunc("/api/image", handleImage(root))
	mux.HandleFunc("/api/info", handleInfo(root))
}

func handleBrowse(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relPath := r.URL.Query().Get("path")
		entries, cached, err := loadEntries(root, relPath, cache)
		if err != nil {
			http.Error(w, "Failed to read directory: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if cached == nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		if cached.ExifDone() {
			applyExifDates(entries, cached)
		}

		sortField := media.SortField(r.URL.Query().Get("sort"))
		sortOrder := media.SortOrder(r.URL.Query().Get("order"))
		if sortField == "" {
			sortField = media.SortByName
		}
		if sortOrder == "" {
			sortOrder = media.OrderAsc
		}
		media.SortEntries(entries, sortField, sortOrder)

		resp := browseResponse{
			Path:     relPath,
			Entries:  entries,
			Warnings: checkHEIFWarnings(entries),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleBrowseDates(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relPath := r.URL.Query().Get("path")
		absPath, ok := pathguard.SafePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		dirInfo, err := os.Stat(absPath)
		if err != nil {
			writeJSON(w, browseDatesResponse{Ready: false})
			return
		}

		cached := cache.Get(absPath, dirInfo.ModTime())
		if cached == nil || !cached.ExifDone() {
			writeJSON(w, browseDatesResponse{Ready: false})
			return
		}

		writeJSON(w, browseDatesResponse{Ready: true, Dates: cached.ExifDates})
	}
}

func handleBrowseMeta(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relPath := r.URL.Query().Get("path")
		absPath, ok := pathguard.SafePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		dirInfo, err := os.Stat(absPath)
		if err != nil {
			writeJSON(w, browseMetaResponse{Ready: false})
			return
		}

		cached := cache.Get(absPath, dirInfo.ModTime())
		if cached == nil || !cached.ExifDone() {
			writeJSON(w, browseMetaResponse{Ready: false})
			return
		}

		writeJSON(w, browseMetaResponse{Ready: true, Meta: cached.ExifMeta})
	}
}

// loadEntries returns a cloned entry slice and the cached scan for absPath.
// Returns (nil, nil, err) on path error, (nil, nil, nil) on invalid path.
func loadEntries(root, relPath string, cache *media.ScanCache) ([]media.Entry, *media.CachedScan, error) {
	absPath, ok := pathguard.SafePath(root, relPath)
	if !ok {
		return nil, nil, nil
	}

	dirInfo, err := os.Stat(absPath)
	if err != nil {
		return nil, nil, err
	}

	cached := cache.Get(absPath, dirInfo.ModTime())
	if cached == nil {
		entries, err := media.ScanDirectoryFast(absPath)
		if err != nil {
			return nil, nil, err
		}
		cached = cache.Put(absPath, entries, dirInfo.ModTime())
		go extractExifBackground(absPath, cached)
	}

	entries := make([]media.Entry, len(cached.Entries))
	copy(entries, cached.Entries)
	return entries, cached, nil
}

func applyExifDates(entries []media.Entry, cached *media.CachedScan) {
	for i := range entries {
		if entries[i].Type != media.EntryImage {
			continue
		}
		if exifDate, ok := cached.ExifDates[entries[i].Name]; ok {
			entries[i].ExifDate = &exifDate
		}
	}
}

func checkHEIFWarnings(entries []media.Entry) []browseWarning {
	for _, e := range entries {
		if e.Type == media.EntryImage && media.IsHEIF(e.Name) {
			status := media.CheckFFmpeg()
			if !status.Available || !status.HEIFSupport {
				return []browseWarning{{Type: "heif_unsupported", Message: status.ErrorMessage}}
			}
			return nil
		}
	}
	return nil
}

func extractExifBackground(absPath string, cached *media.CachedScan) {
	for _, entry := range cached.Entries {
		if entry.Type != media.EntryImage {
			continue
		}
		fullPath := filepath.Join(absPath, entry.Name)
		exifDate, meta, err := media.ExtractDateAndMeta(fullPath)
		if err != nil {
			continue
		}
		cached.SetExifDate(entry.Name, exifDate)
		if meta != nil && (meta.HasGPS || meta.FilmSimulation != "" || meta.AspectRatio != "") {
			cached.SetExifMeta(entry.Name, meta)
		}
	}
	cached.MarkExifDone()
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
