package api

import (
	"net/http"
	"strconv"

	"huepattl.de/unterlumen/internal/media"
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

		absPath, ok := safePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Parse optional size parameter (max dimension in pixels)
		size := thumbnailMaxDim
		if s := r.URL.Query().Get("size"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 50 && n <= 1024 {
				size = n
			}
		}

		// For HEIF files, extract embedded preview (fast) or convert
		if media.IsHEIF(absPath) {
			jpegData, err := media.ExtractHEIFPreview(absPath)
			if err != nil {
				http.Error(w, "Failed to convert HEIF", http.StatusInternalServerError)
				return
			}
			thumb, err := media.ResizeJPEGBytes(jpegData, size)
			if err != nil {
				thumb = jpegData
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(thumb)
			return
		}

		orientation := media.ExtractOrientation(absPath)

		// Try EXIF thumbnail first (fast path) — only when requested
		// size fits within typical EXIF thumbnail dimensions (≤ 300px)
		if size <= thumbnailMaxDim {
			thumb, ct, err := media.ExtractThumbnail(absPath, orientation)
			if err == nil {
				w.Header().Set("Content-Type", ct)
				w.Header().Set("Cache-Control", "no-cache")
				w.Write(thumb)
				return
			}
		}

		// Fallback: generate a resized thumbnail
		thumb, ct, err := media.GenerateThumbnail(absPath, size, orientation)
		if err != nil {
			http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(thumb)
	}
}
