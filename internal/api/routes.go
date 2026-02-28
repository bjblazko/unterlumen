package api

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// NewRouter sets up the HTTP routes for the application.
func NewRouter(root string, webFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	cache := media.NewScanCache()

	mux.HandleFunc("/api/browse", handleBrowse(root, cache))
	mux.HandleFunc("/api/browse/dates", handleBrowseDates(root, cache))
	mux.HandleFunc("/api/thumbnail", handleThumbnail(root))
	mux.HandleFunc("/api/image", handleImage(root))
	mux.HandleFunc("/api/copy", handleCopy(root, cache))
	mux.HandleFunc("/api/move", handleMove(root, cache))
	mux.HandleFunc("/api/delete", handleDelete(root, cache))
	mux.HandleFunc("/api/info", handleInfo(root))

	// Serve static files from embedded web/ filesystem
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	return mux
}

// safePath resolves a relative path within the root and ensures it doesn't escape.
// Returns the absolute path or an error.
func safePath(root, relative string) (string, bool) {
	if relative == "" {
		return root, true
	}

	// Clean the path to remove .., . etc.
	cleaned := filepath.Clean(relative)

	// Reject absolute paths
	if filepath.IsAbs(cleaned) {
		return "", false
	}

	// Join with root
	full := filepath.Join(root, cleaned)

	// Resolve symlinks and verify it's still under root
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		// File might not exist yet (for destinations), try parent
		parent := filepath.Dir(full)
		resolvedParent, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return "", false
		}
		if !strings.HasPrefix(resolvedParent, root) {
			return "", false
		}
		return filepath.Join(resolvedParent, filepath.Base(full)), true
	}

	if !strings.HasPrefix(resolved, root) {
		return "", false
	}

	return resolved, true
}
