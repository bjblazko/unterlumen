package apilibrary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/site"
)

// --- Metadata ---

func getMeta(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		entries, err := store.GetMeta(photoID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, normalizeBuiltMetaKeys(entries))
	}
}

// normalizeBuiltMetaKeys rewrites legacy "published:"-prefixed meta keys to the
// current "built:" prefix for API responses, so the frontend only ever has to deal
// with the current name. If a photo already has a "built:" entry with the same
// suffix (e.g. after a rebuild), the legacy entry is dropped instead of renamed, to
// avoid presenting duplicate/conflicting entries for the same channel.
func normalizeBuiltMetaKeys(entries []lib.MetaEntry) []lib.MetaEntry {
	existing := make(map[string]bool, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "built:") {
			existing[e.Key] = true
		}
	}
	out := make([]lib.MetaEntry, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "published:") {
			builtKey := "built:" + strings.TrimPrefix(e.Key, "published:")
			if existing[builtKey] {
				continue // superseded by a current built: entry
			}
			e.Key = builtKey
		}
		out = append(out, e)
	}
	return out
}

func upsertMeta(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		var body struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
			http.Error(w, "key and value required", http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		if err := store.UpsertMeta(photoID, body.Key, body.Value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if body.Key == "title" {
			if pathHint, phErr := store.GetPhotoPathHint(photoID); phErr == nil && pathHint != "" {
				media.WriteTitle(pathHint, body.Value) //nolint:errcheck
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteMeta(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "key query param required", http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		if strings.HasPrefix(key, pendingPrefix) && draftStore != nil {
			deletePendingMeta(store, draftStore, id, photoID, key)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(key, buildPrefix) && chStore != nil {
			handled, rmErr := deleteBuiltMeta(store, chStore, photoID, key)
			if rmErr != nil {
				http.Error(w, "remove from site: "+rmErr.Error(), http.StatusInternalServerError)
				return
			}
			if handled {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		if key == "title" {
			if pathHint, phErr := store.GetPhotoPathHint(photoID); phErr == nil && pathHint != "" {
				media.WriteTitle(pathHint, "") //nolint:errcheck
			}
		}
		if err := store.DeleteMeta(photoID, key); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

const pendingPrefix = "pending:"

// deletePendingMeta removes a pending:{slug} key. The meta row is a derived
// signal that this photo sits in an unfinished draft for that channel —
// removing it must also remove the photo from the draft itself (drafts.json),
// not just delete the meta row and leave the draft still holding a reference
// the UI no longer shows.
func deletePendingMeta(store *lib.Store, draftStore *channels.DraftStore, libraryID, photoID, key string) {
	// Two key shapes: "pending:<slug>" names the draft in its value,
	// "pending:<slug>:<draftID>" names it in the key itself (its value
	// is the album title).
	slug, draftID, qualified := strings.Cut(strings.TrimPrefix(key, pendingPrefix), ":")
	if !qualified {
		draftID = metaValue(store, photoID, key)
	}
	if draftID != "" {
		draftStore.RemovePhoto(slug, draftID, libraryID, photoID) //nolint:errcheck
	}
	store.DeleteMeta(photoID, key) //nolint:errcheck
	if qualified {
		clearPendingMarkers(store, photoID, slug, draftID)
	} else {
		store.DeleteMeta(photoID, pendingPrefix+slug+":"+draftID) //nolint:errcheck
	}
}

// metaValue returns the value of a photo's meta key, or "" when it has none.
func metaValue(store *lib.Store, photoID, key string) string {
	entries, err := store.GetMeta(photoID)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.Key == key {
			return e.Value
		}
	}
	return ""
}

// deleteBuiltMeta handles built:{slug} keys; handled is false when the key
// is a plain meta row the caller deletes itself. On site-export channels it
// removes the photo from the site (site.json + physical files) and deletes all
// related keys. The frontend only ever sends the normalized built: prefix (see
// getMeta), but a photo may still carry legacy published:{slug} entries from
// before this channel-membership key was renamed, so both prefixes are cleaned
// up here.
func deleteBuiltMeta(store *lib.Store, chStore *channels.Store, photoID, key string) (handled bool, err error) {
	slug, albumPostID, _ := strings.Cut(strings.TrimPrefix(key, buildPrefix), ":")
	ch, chErr := chStore.Get(slug)
	if chErr != nil {
		return false, nil
	}
	switch {
	// Removing a publication from a site-export photo means taking
	// it off the site entirely — there is no per-album removal —
	// so both the channel marker and one album's key land here.
	case ch.SiteExport && !reservedMetaSuffix(albumPostID):
		if err := removePhotoFromSite(store, ch, chStore, photoID, slug); err != nil {
			return false, err
		}
		deleteChannelPublicationKeys(store, photoID, slug)
		return true, nil
	// A gallery channel's albums are independent: drop just this
	// album, and the channel marker only if it was the last one.
	case albumPostID != "" && !reservedMetaSuffix(albumPostID):
		deleteAlbumKeys(store, photoID, slug, albumPostID)
		if entries, metaErr := store.GetMeta(photoID); metaErr == nil && len(albumPostIDsForChannel(entries, slug)) == 0 {
			deleteChannelPublicationKeys(store, photoID, slug)
		}
		return true, nil
	case albumPostID == "":
		deleteChannelPublicationKeys(store, photoID, slug)
		return true, nil
	}
	return false, nil
}

// removePhotoFromSite removes a photo from every album in a site-export channel:
// updates site.json, deletes the exported file and thumbnail, regenerates album HTML
// and the site index. Meta key deletion is handled by the caller.
func removePhotoFromSite(store *lib.Store, ch *channels.Channel, chStore *channels.Store, photoID, slug string) error {
	siteDir := filepath.Join(chStore.OutputDir(slug), "site")
	sites := site.NewSiteStore(chStore, slug)
	albums, err := sites.List()
	if err != nil {
		return fmt.Errorf("load site state: %w", err)
	}

	pathHint, _ := store.GetPhotoPathHint(photoID)
	legacyPrefix := legacySiteFilePrefix(store, photoID, slug, pathHint)
	touched := map[string]bool{} // postIDs of albums that lost photos
	for i := range albums {
		kept := removeFromSiteAlbum(albums[i], siteDir, slug, photoID, legacyPrefix, pathHint)
		if len(kept) != len(albums[i].Photos) {
			albums[i].Photos = kept
			albums[i].PhotoCount = len(kept)
			touched[albums[i].PostID] = true
		}
	}
	if len(touched) == 0 {
		return nil
	}

	remaining, err := saveSiteRemoval(albums, touched, sites, siteDir, ch)
	if err != nil {
		return err
	}
	return writeSitePages(ch, siteDir, remaining)
}

// legacySiteFilePrefix reconstructs the expected filename prefix as a
// fallback for pre-photoID entries, or "" when it cannot.
func legacySiteFilePrefix(store *lib.Store, photoID, slug, pathHint string) string {
	if pathHint == "" {
		return ""
	}
	metaEntries, err := store.GetMeta(photoID)
	if err != nil {
		return ""
	}
	buildKey := "built:" + slug
	legacyKey := "published:" + slug
	for _, e := range metaEntries {
		if e.Key != buildKey && e.Key != legacyKey {
			continue
		}
		t, tErr := time.Parse(time.RFC3339, e.Value)
		if tErr != nil {
			return ""
		}
		base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
		return slug + "_" + t.UTC().Format("20060102T150405Z") + "_" + base
	}
	return ""
}

// removeFromSiteAlbum deletes the photo's exported file and thumbnail from one
// album, and takes the album out of the photo's own sidecar so a rebuild
// cannot find it. It returns the album's other photos.
func removeFromSiteAlbum(album site.SiteAlbum, siteDir, slug, photoID, legacyPrefix, pathHint string) []site.SitePhoto {
	albumDir := filepath.Join(siteDir, "albums", site.AlbumFolderName(album))
	var kept []site.SitePhoto
	for _, sp := range album.Photos {
		match := (sp.PhotoID != "" && sp.PhotoID == photoID) ||
			(legacyPrefix != "" && strings.HasPrefix(sp.Filename, legacyPrefix))
		if !match {
			kept = append(kept, sp)
			continue
		}
		if pathHint != "" {
			media.RemovePublication(pathHint, slug, album.PostID) //nolint:errcheck
		}
		os.Remove(filepath.Join(albumDir, sp.Filename)) //nolint:errcheck
		if sp.ThumbFilename != "" {
			os.Remove(filepath.Join(albumDir, sp.ThumbFilename)) //nolint:errcheck
		}
	}
	return kept
}

// saveSiteRemoval removes albums that are now empty, saves the ones that lost
// a photo and regenerates the HTML of every remaining album. It returns the
// albums that remain.
func saveSiteRemoval(albums []site.SiteAlbum, touched map[string]bool, sites *site.SiteStore, siteDir string, ch *channels.Channel) ([]site.SiteAlbum, error) {
	albumNav := site.BuildSiteNavContext(ch, siteDir, false)
	var remaining []site.SiteAlbum
	for _, album := range albums {
		albumDir := filepath.Join(siteDir, "albums", site.AlbumFolderName(album))
		if album.PhotoCount == 0 {
			os.RemoveAll(albumDir)                             //nolint:errcheck
			if err := sites.Delete(album.PostID); err != nil { // its last photo was taken off the site on purpose
				return nil, fmt.Errorf("save site state: %w", err)
			}
			continue
		}
		remaining = append(remaining, album)
		if touched[album.PostID] {
			if err := sites.Upsert(album); err != nil {
				return nil, fmt.Errorf("save site state: %w", err)
			}
		}
		writeSiteAlbumPage(&album, albumDir, site.BuildGalleryItems(album.Photos), ch, albumNav)
	}
	return remaining, nil
}
