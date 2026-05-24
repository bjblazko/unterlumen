package browse

import (
	"net/http"
	"os"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

func handleFolderStats(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relPath := r.URL.Query().Get("path")
		absPath, ok := pathguard.SafePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		fi, err := os.Stat(absPath)
		if err != nil {
			http.Error(w, "Path not found", http.StatusNotFound)
			return
		}
		if !fi.IsDir() {
			http.Error(w, "Not a directory", http.StatusBadRequest)
			return
		}

		stats, err := media.WalkFolderStats(absPath, relPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, stats)
	}
}
