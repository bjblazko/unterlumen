package browse

import (
	"net/http"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/api/download"
	"huepattl.de/unterlumen/internal/api/heifjpeg"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

func handleImage(root string, imgCache *media.ImageCache) http.HandlerFunc {
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

		if download.Requested(r) {
			download.Original(w, r, absPath)
			return
		}

		if media.IsHEIF(absPath) {
			heifjpeg.Serve(w, r, absPath, imgCache)
			return
		}

		if ct := contentTypeByExt(absPath); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		http.ServeFile(w, r, absPath)
	}
}

func contentTypeByExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return ""
}
