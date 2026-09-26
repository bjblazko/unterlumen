package apilibrary

import (
	"net/http"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
)

// registerPublishRoutes registers publishing: drafts, generating galleries
// and sites, rebuilding them and managing published galleries.
func registerPublishRoutes(mux *http.ServeMux, mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) {
	mux.HandleFunc("POST /api/channels/{slug}/drafts/{draftID}/generate", generateDraft(mgr, chStore, draftStore))
	mux.HandleFunc("POST /api/library/{id}/build-download", buildDownload(mgr, chStore))
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-site", trackDestination(mgr, chStore, "Rebuilding the site of", rebuildSite(chStore, mgr)))
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-album-list", trackDestination(mgr, chStore, "Rebuilding the album list of", rebuildAlbumList(chStore, mgr)))
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-galleries", trackDestination(mgr, chStore, "Rebuilding the galleries of", rebuildGalleries(chStore)))
	mux.HandleFunc("GET /api/channels/{slug}/galleries", listGalleries(chStore))
	mux.HandleFunc("PATCH /api/channels/{slug}/galleries/{postID}", trackDestination(mgr, chStore, "Updating a gallery of", renameGallery(chStore, mgr)))
	mux.HandleFunc("DELETE /api/channels/{slug}/galleries/{postID}", trackDestination(mgr, chStore, "Unpublishing a gallery of", deleteGallery(chStore, mgr)))
	mux.HandleFunc("GET /api/channels/galleries", listAllGalleries(chStore, draftStore))
	mux.HandleFunc("POST /api/channels/galleries/reachability", checkGalleryReachability())
	registerDraftRoutes(mux, mgr, chStore, draftStore)
}
