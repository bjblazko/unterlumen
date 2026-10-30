package browse

import (
	"net/http"

	"huepattl.de/unterlumen/internal/api/videoclip"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

// handleClip serves a video's short silent clip, the one a tile plays under
// the pointer.
func handleClip(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		absPath, ok := pathguard.SafePath(root, r.URL.Query().Get("path"))
		if !ok || !media.IsVideo(absPath) {
			http.Error(w, "Not a video", http.StatusBadRequest)
			return
		}
		videoclip.Serve(w, r, absPath)
	}
}
