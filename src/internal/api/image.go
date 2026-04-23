package api

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/media"
)

func handleImage(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relPath := r.URL.Query().Get("path")
		if relPath == "" {
			http.Error(w, "Missing path parameter", http.StatusBadRequest)
			return
		}

		absPath, ok := safePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// For HEIF files, convert to JPEG on-the-fly
		if media.IsHEIF(absPath) {
			jpegData, err := media.ConvertHEIFToJPEG(absPath)
			if err != nil {
				http.Error(w, "Failed to convert HEIF: "+err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeContent(w, r, "image.jpg", time.Time{}, bytes.NewReader(jpegData))
			return
		}

		// Set content type based on extension
		ext := strings.ToLower(filepath.Ext(absPath))
		switch ext {
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".gif":
			w.Header().Set("Content-Type", "image/gif")
		case ".webp":
			w.Header().Set("Content-Type", "image/webp")
		}

		http.ServeFile(w, r, absPath)
	}
}
