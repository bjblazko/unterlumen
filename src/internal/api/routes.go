package api

import (
	"io/fs"
	"net/http"
	"strings"

	"huepattl.de/unterlumen/internal/api/batchrename"
	"huepattl.de/unterlumen/internal/api/browse"
	apichannels "huepattl.de/unterlumen/internal/api/channels"
	apicrop "huepattl.de/unterlumen/internal/api/crop"
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
// homePath is the OS home directory expressed as a path relative to boundary (empty = boundary root).
// serverRole controls export behaviour: true = ZIP download only, false = local filesystem save + ZIP.
// libMgr is the library manager; may be nil if library support could not be initialised.
// chStore is the global channel store; may be nil if the lib dir is not configured.
// version is the build version string injected at link time (e.g. "v1.2.3" or "dev").
func NewRouter(boundary, startPath, homePath string, webFS fs.FS, serverRole bool, libMgr *library.Manager, chStore *channels.Store, version string) http.Handler {
	mux := http.NewServeMux()
	cache := media.NewScanCache()
	imageCache := media.NewImageCache(20)

	mux.HandleFunc("/api/config", handleConfig(boundary, startPath, homePath, serverRole, version))
	mux.HandleFunc("/api/tools/check", handleToolsCheck())
	mux.HandleFunc("/api/cache/info", handleCacheInfo())
	mux.HandleFunc("/api/cache/clear", handleCacheClear())
	mux.HandleFunc("/api/cache/evict", handleCacheEvict(boundary))

	browse.Handle(mux, boundary, cache, imageCache, libMgr)
	apiexport.Handle(mux, boundary, serverRole)
	apicrop.Handle(mux, boundary, cache)
	fileops.Handle(mux, boundary, cache, libMgr)
	location.Handle(mux, boundary, cache)
	batchrename.Handle(mux, boundary, cache)

	if chStore != nil {
		apichannels.Handle(mux, chStore)
	}
	if libMgr != nil {
		apilibrary.Handle(mux, libMgr, imageCache, boundary, serverRole, chStore)
	}

	mux.Handle("/", noCacheAssets(http.FileServer(http.FS(webFS))))
	return mux
}

// noCacheAssets wraps a handler and adds Cache-Control: no-cache for JS and CSS
// files so browsers always revalidate after a binary update rather than serving
// stale embed.FS content (embed files have a fixed zero modification time, which
// causes the same ETag across builds).
func noCacheAssets(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		h.ServeHTTP(w, r)
	})
}
