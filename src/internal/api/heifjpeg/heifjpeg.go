// Package heifjpeg serves a HEIF photo as the JPEG a browser can show. Folders
// and the libraries both show HEIF photos in the viewer; this is the one place
// that converts, caches and answers for them.
package heifjpeg

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"huepattl.de/unterlumen/internal/media"
)

// Serve answers with the full-size JPEG of the HEIF at absPath: from memory,
// from the disk cache, or converted now. A prefetch (see IsPrefetch) gets it
// only when no conversion is needed, and otherwise an empty 204 at once.
func Serve(w http.ResponseWriter, r *http.Request, absPath string, imgCache *media.ImageCache) {
	info, statErr := os.Stat(absPath)
	if statErr != nil {
		http.Error(w, "photo not found on disk", http.StatusNotFound)
		return
	}
	key := absPath + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
	if cached := imgCache.Get(key); cached != nil {
		serve(w, r, absPath, info, cached)
		return
	}

	// Converting a photo that may never be looked at is what a prefetch must
	// not start: on the NAS one conversion took about 640 MB.
	if IsPrefetch(r) && !media.HEIFConverted(absPath) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	data, err := media.ConvertHEIFToJPEG(r.Context(), absPath)
	if err != nil {
		http.Error(w, "Failed to convert HEIF: "+err.Error(), http.StatusInternalServerError)
		return
	}
	imgCache.Set(key, data)
	serve(w, r, absPath, info, data)
}

// IsPrefetch reports whether a request only warms the browser's cache ahead
// of time. The viewer marks it with a header, not a query parameter, so the
// URL is the one it later shows and the cached answer is reused.
func IsPrefetch(r *http.Request) bool {
	return r.Header.Get("X-Prefetch") == "1"
}

func serve(w http.ResponseWriter, r *http.Request, absPath string, info os.FileInfo, data []byte) {
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
