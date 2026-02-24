package api

import (
	"encoding/json"
	"net/http"

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

func handleBrowse(root string) http.HandlerFunc {
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

		entries, err := media.ScanDirectory(absPath)
		if err != nil {
			http.Error(w, "Failed to read directory: "+err.Error(), http.StatusInternalServerError)
			return
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
