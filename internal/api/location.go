package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"huepattl.de/unterlumen/internal/media"
)

type setLocationRequest struct {
	Files     []string `json:"files"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
}

func handleSetLocation(root string) http.HandlerFunc {
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

		var results []fileOpResult
		for _, file := range req.Files {
			filePath, ok := safePath(root, file)
			if !ok {
				results = append(results, fileOpResult{
					File:  file,
					Error: "invalid path",
				})
				continue
			}

			if err := media.WriteGPSLocation(filePath, req.Latitude, req.Longitude); err != nil {
				results = append(results, fileOpResult{
					File:  file,
					Error: err.Error(),
				})
			} else {
				results = append(results, fileOpResult{
					File:    file,
					Success: true,
				})
			}
		}

		// Invalidate scan cache for affected directories
		dirsToInvalidate := make(map[string]struct{})
		for _, r := range results {
			if r.Success {
				filePath, ok := safePath(root, r.File)
				if ok {
					dirsToInvalidate[filepath.Dir(filePath)] = struct{}{}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fileOpResponse{Results: results})
	}
}
