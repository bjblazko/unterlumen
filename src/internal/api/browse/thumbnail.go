package browse

import (
	"net/http"
	"strconv"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

const thumbnailMaxDim = 300

const (
	thumbnailQualityStandard = "standard"
	thumbnailQualityHigh     = "high"
)

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
		quality := parseThumbnailQuality(r.URL.Query().Get("quality"))
		ctx := r.Context()

		if media.IsHEIF(absPath) {
			serveHEIFThumbnail(w, r, absPath, size, quality)
			return
		}

		// Standard quality: try the embedded EXIF thumbnail first (single NAS read,
		// disk-cached, concurrency-limited). Fall through only when unavailable.
		if quality == thumbnailQualityStandard && size <= thumbnailMaxDim {
			thumb, ct, err := media.ExtractThumbnailCached(ctx, absPath)
			if err == nil {
				serveThumbnail(w, thumb, ct)
				return
			}
			if ctx.Err() != nil {
				return
			}
		}

		// Full decode fallback (always used for high quality).
		orientation := media.ExtractOrientation(absPath)
		thumb, ct, err := media.GenerateThumbnailCached(ctx, absPath, size, orientation)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
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

func parseThumbnailQuality(s string) string {
	if s == thumbnailQualityHigh {
		return thumbnailQualityHigh
	}
	return thumbnailQualityStandard
}

func serveHEIFThumbnail(w http.ResponseWriter, r *http.Request, absPath string, size int, quality string) {
	ctx := r.Context()
	var (
		thumb []byte
		err   error
	)

	if quality == thumbnailQualityHigh {
		thumb, err = media.GenerateHEIFThumbnail(ctx, absPath, size)
	} else {
		thumb, err = media.ExtractHEIFPreviewThumbnail(ctx, absPath, size)
	}
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		http.Error(w, "Failed to generate HEIF thumbnail", http.StatusInternalServerError)
		return
	}
	serveThumbnail(w, thumb, "image/jpeg")
}

func serveThumbnail(w http.ResponseWriter, data []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}
