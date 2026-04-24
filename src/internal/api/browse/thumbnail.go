package browse

import (
	"net/http"
	"strconv"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

const thumbnailMaxDim = 300

func handleThumbnail(root string) http.HandlerFunc {
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

		absPath, ok := pathguard.SafePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		size := parseThumbnailSize(r.URL.Query().Get("size"))

		if media.IsHEIF(absPath) {
			serveHEIFThumbnail(w, absPath, size)
			return
		}

		orientation := media.ExtractOrientation(absPath)

		if size <= thumbnailMaxDim {
			thumb, ct, err := media.ExtractThumbnail(absPath, orientation)
			if err == nil {
				serveThumbnail(w, thumb, ct)
				return
			}
		}

		thumb, ct, err := media.GenerateThumbnail(absPath, size, orientation)
		if err != nil {
			http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
			return
		}
		serveThumbnail(w, thumb, ct)
	}
}

func parseThumbnailSize(s string) int {
	if s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 50 && n <= 1024 {
			return n
		}
	}
	return thumbnailMaxDim
}

func serveHEIFThumbnail(w http.ResponseWriter, absPath string, size int) {
	jpegData, err := media.ExtractHEIFPreview(absPath)
	if err != nil {
		http.Error(w, "Failed to convert HEIF", http.StatusInternalServerError)
		return
	}
	thumb, err := media.ResizeJPEGBytes(jpegData, size)
	if err != nil {
		thumb = jpegData
	}
	serveThumbnail(w, thumb, "image/jpeg")
}

func serveThumbnail(w http.ResponseWriter, data []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}
