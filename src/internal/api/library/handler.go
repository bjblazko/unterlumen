// Package apilibrary provides HTTP handlers for the DAM library feature.
package apilibrary

import (
	"encoding/json"
	_ "image/jpeg"
	_ "image/png"
	"net/http"

	_ "golang.org/x/image/webp"
	"huepattl.de/unterlumen/internal/api/publish"
	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// Handle registers all library API routes on mux.
// root is the browse boundary directory; serverRole is true when running in server/container mode.
func Handle(mux *http.ServeMux, mgr *lib.Manager, imgCache *media.ImageCache, root string, serverRole bool, chStore *channels.Store, draftStore *channels.DraftStore) {
	mux.HandleFunc("GET /api/library/", listLibraries(mgr, root))
	mux.HandleFunc("POST /api/library/", createLibrary(mgr, root))
	mux.HandleFunc("PUT /api/library-order", setLibraryOrder(mgr))
	mux.HandleFunc("GET /api/settings", getSettings(mgr))
	mux.HandleFunc("PATCH /api/settings", patchSettings(mgr))
	mux.HandleFunc("GET /api/library/detect", detectLibrary(mgr, root))
	mux.HandleFunc("GET /api/library/search", searchLibraries(mgr))
	mux.HandleFunc("GET /api/library/exif-ranges", globalExifRanges(mgr))
	mux.HandleFunc("GET /api/library/exif-values", globalExifValues(mgr))
	mux.HandleFunc("GET /api/library/meta-keys", globalMetaKeys(mgr))
	mux.HandleFunc("GET /api/library/meta-values", globalMetaValues(mgr))
	mux.HandleFunc("GET /api/library/album-titles", globalAlbumTitles(mgr))
	mux.HandleFunc("GET /api/library/exif-fields", globalExifFields(mgr))
	mux.HandleFunc("GET /api/library/statistics", libraryStatistics(mgr))
	mux.HandleFunc("GET /api/library/timeline", libraryTimeline(mgr))
	mux.HandleFunc("GET /api/library/geo", libraryGeo(mgr))
	mux.HandleFunc("GET /api/library/{id}", getLibrary(mgr, root))
	mux.HandleFunc("PATCH /api/library/{id}", updateLibrary(mgr, root))
	mux.HandleFunc("DELETE /api/library/{id}", deleteLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/reindex", reindexLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/scan-new", scanNewLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/cleanup", cleanupLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/regen-previews-missing", regenMissingPreviewsLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/regen-previews-all", rebuildAllPreviewsLibrary(mgr))
	mux.HandleFunc("GET /api/library/{id}/browse", browseFolder(mgr, root))
	mux.HandleFunc("GET /api/library/{id}/browse-recursive", browseFolderRecursive(mgr))
	mux.HandleFunc("GET /api/library/{id}/folder-stats", libraryFolderStats(mgr))
	mux.HandleFunc("GET /api/library/{id}/photos", listPhotos(mgr))
	mux.HandleFunc("GET /api/library/{id}/exif-ranges", exifRanges(mgr))
	mux.HandleFunc("GET /api/library/{id}/folder-previews", folderPreviews(mgr))
	mux.HandleFunc("GET /api/library/{id}/thumb/{photoID}", serveThumb(mgr))
	mux.HandleFunc("GET /api/library/{id}/thumb-by-path", thumbByPath(mgr, root))
	mux.HandleFunc("GET /api/library/{id}/photo-id-by-path", photoIDByPath(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}", servePhoto(mgr, imgCache))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}/info", photoInfo(mgr))
	mux.HandleFunc("DELETE /api/library/{id}/photo/{photoID}", deleteLibraryPhoto(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}/meta", getMeta(mgr))
	mux.HandleFunc("PUT /api/library/{id}/photo/{photoID}/meta", upsertMeta(mgr))
	mux.HandleFunc("DELETE /api/library/{id}/photo/{photoID}/meta", deleteMeta(mgr, chStore, draftStore))
	publish.Handle(mux, mgr, chStore, draftStore)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
