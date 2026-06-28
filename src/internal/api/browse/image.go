package browse

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

		if media.IsHEIF(absPath) {
			var info os.FileInfo
			var key string
			if fi, err := os.Stat(absPath); err == nil {
				info = fi
				key = absPath + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
			}

			// serveHEIF is only called when key != "" (i.e. info is non-nil).
			serveHEIF := func(data []byte) {
				h := sha256.Sum256([]byte(absPath))
				etag := fmt.Sprintf(`"%x-%d"`, h[:4], info.ModTime().Unix())
				w.Header().Set("Cache-Control", "private, max-age=3600")
				w.Header().Set("ETag", etag)
				if r.Header.Get("If-None-Match") == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}
				w.Header().Set("Content-Type", "image/jpeg")
				http.ServeContent(w, r, "image.jpg", info.ModTime(), bytes.NewReader(data))
			}

			if key != "" {
				if cached := imgCache.Get(key); cached != nil {
					serveHEIF(cached)
					return
				}
			}

			jpegData, err := media.ConvertHEIFToJPEG(r.Context(), absPath)
			if err != nil {
				http.Error(w, "Failed to convert HEIF: "+err.Error(), http.StatusInternalServerError)
				return
			}

			if key != "" {
				imgCache.Set(key, jpegData)
				serveHEIF(jpegData)
				return
			}

			// stat failed — fall back to old uncacheable response
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeContent(w, r, "image.jpg", time.Time{}, bytes.NewReader(jpegData))
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
