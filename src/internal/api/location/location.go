package location

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

type locationResult struct {
	File    string `json:"file"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type locationResponse struct {
	Results []locationResult `json:"results"`
}

type removeLocationRequest struct {
	Files []string `json:"files"`
}

type setLocationRequest struct {
	Files     []string `json:"files"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
}

// Handle registers all location routes on mux.
func Handle(mux *http.ServeMux, root string, cache *media.ScanCache) {
	mux.HandleFunc("/api/set-location", handleSetLocation(root, cache))
	mux.HandleFunc("/api/remove-location", handleRemoveLocation(root, cache))
}

func handleRemoveLocation(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !media.CheckExiftool() {
			http.Error(w, "exiftool is not available", http.StatusServiceUnavailable)
			return
		}

		var req removeLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if len(req.Files) == 0 {
			http.Error(w, "No files specified", http.StatusBadRequest)
			return
		}

		results := applyToFiles(root, req.Files, func(filePath string) error {
			return media.RemoveGPSLocation(filePath)
		})

		invalidateSuccessful(root, results, cache)
		writeJSON(w, locationResponse{Results: results})
	}
}

func handleSetLocation(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !media.CheckExiftool() {
			http.Error(w, "exiftool is not available", http.StatusServiceUnavailable)
			return
		}

		var req setLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if len(req.Files) == 0 {
			http.Error(w, "No files specified", http.StatusBadRequest)
			return
		}
		if req.Latitude < -90 || req.Latitude > 90 {
			http.Error(w, "Latitude must be between -90 and 90", http.StatusBadRequest)
			return
		}
		if req.Longitude < -180 || req.Longitude > 180 {
			http.Error(w, "Longitude must be between -180 and 180", http.StatusBadRequest)
			return
		}

		results := applyToFiles(root, req.Files, func(filePath string) error {
			return media.WriteGPSLocation(filePath, req.Latitude, req.Longitude)
		})

		invalidateSuccessful(root, results, cache)
		writeJSON(w, locationResponse{Results: results})
	}
}

func applyToFiles(root string, files []string, fn func(string) error) []locationResult {
	var results []locationResult
	for _, file := range files {
		filePath, ok := pathguard.SafePath(root, file)
		if !ok {
			results = append(results, locationResult{File: file, Error: "invalid path"})
			continue
		}
		if err := fn(filePath); err != nil {
			results = append(results, locationResult{File: file, Error: err.Error()})
		} else {
			results = append(results, locationResult{File: file, Success: true})
		}
	}
	return results
}

func invalidateSuccessful(root string, results []locationResult, cache *media.ScanCache) {
	dirs := make(map[string]struct{})
	for _, r := range results {
		if r.Success {
			if filePath, ok := pathguard.SafePath(root, r.File); ok {
				dirs[filepath.Dir(filePath)] = struct{}{}
			}
		}
	}
	for dir := range dirs {
		cache.Invalidate(dir)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
