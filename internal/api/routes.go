package api

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// NewRouter sets up the HTTP routes for the application.
// boundary is the root directory that all file paths must remain within.
// startPath is the initial path (relative to boundary) the frontend should navigate to.
func NewRouter(boundary, startPath string, webFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	cache := media.NewScanCache()

	mux.HandleFunc("/api/config", handleConfig(startPath))
	mux.HandleFunc("/api/browse", handleBrowse(boundary, cache))
	mux.HandleFunc("/api/browse/dates", handleBrowseDates(boundary, cache))
	mux.HandleFunc("/api/thumbnail", handleThumbnail(boundary))
	mux.HandleFunc("/api/image", handleImage(boundary))
	mux.HandleFunc("/api/copy", handleCopy(boundary, cache))
	mux.HandleFunc("/api/move", handleMove(boundary, cache))
	mux.HandleFunc("/api/delete", handleDelete(boundary, cache))
	mux.HandleFunc("/api/info", handleInfo(boundary))

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
