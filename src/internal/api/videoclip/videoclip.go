// Package videoclip serves the short silent clip of a video that a tile plays
// under the pointer (ADR-0058), for Folders and libraries alike.
package videoclip

import (
	"net/http"

	"huepattl.de/unterlumen/internal/media"
)

// Serve makes the clip of the video at absPath when it is not in the cache
// yet, and serves it.
func Serve(w http.ResponseWriter, r *http.Request, absPath string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	file, err := media.VideoClipCached(r.Context(), absPath)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		http.Error(w, "The clip could not be made", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, file)
}
