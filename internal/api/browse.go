package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"huepattl.de/unterlumen/internal/media"
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

func handleBrowse(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relPath := r.URL.Query().Get("path")
		absPath, ok := safePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		dirInfo, err := os.Stat(absPath)
		if err != nil {
			http.Error(w, "Failed to read directory: "+err.Error(), http.StatusInternalServerError)
			return
		}
		dirModTime := dirInfo.ModTime()

		// Check cache
		cached := cache.Get(absPath, dirModTime)
		if cached == nil {
			entries, err := media.ScanDirectoryFast(absPath)
			if err != nil {
				http.Error(w, "Failed to read directory: "+err.Error(), http.StatusInternalServerError)
				return
			}
			cached = cache.Put(absPath, entries, dirModTime)
			go extractExifBackground(absPath, cached)
		}

		// Clone entries so sorting doesn't mutate the cache
		entries := make([]media.Entry, len(cached.Entries))
		copy(entries, cached.Entries)

		// Apply EXIF dates if extraction is done
		if cached.ExifDone() {
			for i := range entries {
				if entries[i].Type != media.EntryImage {
					continue
				}
				if exifDate, ok := cached.ExifDates[entries[i].Name]; ok {
					entries[i].Date = exifDate
				}
			}
		}

		// Apply sorting
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
			Path:    relPath,
			Entries: entries,
		}

		// Check if directory contains HEIF files and warn if ffmpeg can't handle them
		hasHEIF := false
		for _, e := range entries {
			if e.Type == media.EntryImage && media.IsHEIF(e.Name) {
				hasHEIF = true
				break
			}
		}
		if hasHEIF {
			status := media.CheckFFmpeg()
			if !status.Available || !status.HEIFSupport {
				resp.Warnings = append(resp.Warnings, browseWarning{
					Type:    "heif_unsupported",
					Message: status.ErrorMessage,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

type browseDatesResponse struct {
	Ready bool                 `json:"ready"`
	Dates map[string]time.Time `json:"dates,omitempty"`
}

func handleBrowseDates(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relPath := r.URL.Query().Get("path")
		absPath, ok := safePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		dirInfo, err := os.Stat(absPath)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(browseDatesResponse{Ready: false})
			return
		}

		cached := cache.Get(absPath, dirInfo.ModTime())
		if cached == nil || !cached.ExifDone() {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(browseDatesResponse{Ready: false})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(browseDatesResponse{
			Ready: true,
			Dates: cached.ExifDates,
		})
	}
}

// extractExifBackground extracts EXIF dates for all image entries in the
// background, storing results in the cached scan.
func extractExifBackground(absPath string, cached *media.CachedScan) {
	for _, entry := range cached.Entries {
		if entry.Type != media.EntryImage {
			continue
		}
		fullPath := filepath.Join(absPath, entry.Name)
		exifDate, err := media.ExtractDateTaken(fullPath)
		if err != nil {
			continue
		}
		// Only store if EXIF date differs from the mod-time
		if !exifDate.Equal(entry.Date) {
			cached.SetExifDate(entry.Name, exifDate)
		}
	}
	cached.MarkExifDone()
}
