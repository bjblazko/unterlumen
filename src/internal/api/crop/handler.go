package crop

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

type cropRequest struct {
	Path   string  `json:"path"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Handle registers the /api/crop route on mux.
func Handle(mux *http.ServeMux, root string, cache *media.ScanCache) {
	mux.HandleFunc("/api/crop", handleCrop(root, cache))
}

func handleCrop(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req cropRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Path == "" {
			http.Error(w, "path is required", http.StatusBadRequest)
			return
		}
		if req.Width <= 0 || req.Height <= 0 {
			http.Error(w, "width and height must be positive", http.StatusBadRequest)
			return
		}
		if req.X < 0 || req.Y < 0 || req.X+req.Width > 1.001 || req.Y+req.Height > 1.001 {
			http.Error(w, "crop region out of bounds", http.StatusBadRequest)
			return
		}

		absPath, ok := pathguard.SafePath(root, req.Path)
		if !ok {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		if !media.IsSupportedImage(absPath) {
			http.Error(w, "unsupported image format", http.StatusBadRequest)
			return
		}

		if err := media.CropImage(absPath, req.X, req.Y, req.Width, req.Height); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		media.EvictFile(absPath)
		cache.Invalidate(filepath.Dir(absPath))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}
