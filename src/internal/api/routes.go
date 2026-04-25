package api

import (
	"io/fs"
	"net/http"

	"huepattl.de/unterlumen/internal/api/batchrename"
	"huepattl.de/unterlumen/internal/api/browse"
	apichannels "huepattl.de/unterlumen/internal/api/channels"
	apiexport "huepattl.de/unterlumen/internal/api/export"
	"huepattl.de/unterlumen/internal/api/fileops"
	apilibrary "huepattl.de/unterlumen/internal/api/library"
	"huepattl.de/unterlumen/internal/api/location"
	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// NewRouter sets up the HTTP routes for the application.
// boundary is the root directory that all file paths must remain within.
// startPath is the initial path (relative to boundary) the frontend should navigate to.
// serverRole controls export behaviour: true = ZIP download only, false = local filesystem save + ZIP.
// libMgr is the library manager; may be nil if library support could not be initialised.
// chStore is the global channel store; may be nil if the lib dir is not configured.
func NewRouter(boundary, startPath string, webFS fs.FS, serverRole bool, libMgr *library.Manager, chStore *channels.Store) http.Handler {
	mux := http.NewServeMux()
	cache := media.NewScanCache()

	mux.HandleFunc("/api/config", handleConfig(startPath, serverRole))
	mux.HandleFunc("/api/tools/check", handleToolsCheck())

	browse.Handle(mux, boundary, cache)
	apiexport.Handle(mux, boundary, serverRole)
	fileops.Handle(mux, boundary, cache)
	location.Handle(mux, boundary, cache)
	batchrename.Handle(mux, boundary, cache)

	if chStore != nil {
		apichannels.Handle(mux, chStore)
	}
	if libMgr != nil {
		apilibrary.Handle(mux, libMgr, boundary, chStore)
	}

	mux.Handle("/", http.FileServer(http.FS(webFS)))
	return mux
}
