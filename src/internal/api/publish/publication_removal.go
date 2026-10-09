package publish

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/site"
)

const PendingPrefix = "pending:"

// DeletePendingMeta removes a pending:{slug} key. The meta row is a derived
// signal that this photo sits in an unfinished draft for that channel —
// removing it must also remove the photo from the draft itself (drafts.json),
// not just delete the meta row and leave the draft still holding a reference
// the UI no longer shows.
func DeletePendingMeta(store *lib.Store, draftStore *channels.DraftStore, libraryID, photoID, key string) {
	// Two key shapes: "pending:<slug>" names the draft in its value,
	// "pending:<slug>:<draftID>" names it in the key itself (its value
	// is the album title).
	slug, draftID, qualified := strings.Cut(strings.TrimPrefix(key, PendingPrefix), ":")
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
		store.DeleteMeta(photoID, PendingPrefix+slug+":"+draftID) //nolint:errcheck
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

// DeleteBuiltMeta handles built:{slug} keys; handled is false when the key
// is a plain meta row the caller deletes itself. On site-export channels it
// removes the photo from the site (site.json + physical files) and deletes all
// related keys. The frontend only ever sends the normalized built: prefix (see
// getMeta), but a photo may still carry legacy published:{slug} entries from
// before this channel-membership key was renamed, so both prefixes are cleaned
// up here.
func DeleteBuiltMeta(store *lib.Store, chStore *channels.Store, photoID, key string) (handled bool, err error) {
	slug, albumPostID, _ := strings.Cut(strings.TrimPrefix(key, BuildPrefix), ":")
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
	// A files destination builds no pages, so its album is only the
	// photo's records and one file in the folder: all of them go, or
	// the next scan reads the album back from the sidecar.
	case isFilesDestination(ch) && !reservedMetaSuffix(albumPostID):
		if err := removeFromFilesDestination(store, chStore, ch, photoID, albumPostID); err != nil {
			return false, err
		}
		forgetAlbumKeys(store, photoID, slug, albumPostID)
		return true, nil
	// A gallery channel's albums are independent: drop just this
	// album, and the channel marker only if it was the last one.
	case !reservedMetaSuffix(albumPostID):
		forgetAlbumKeys(store, photoID, slug, albumPostID)
		return true, nil
	}
	return false, nil
}

func isFilesDestination(ch *channels.Channel) bool { return !ch.GalleryExport && !ch.SiteExport }

// forgetAlbumKeys drops one album's keys, and the channel marker once the
// photo is in none of the channel's albums. An empty postID names the
// channel marker itself.
func forgetAlbumKeys(store *lib.Store, photoID, slug, albumPostID string) {
	if albumPostID == "" {
		deleteChannelPublicationKeys(store, photoID, slug)
		return
	}
	deleteAlbumKeys(store, photoID, slug, albumPostID)
	if entries, err := store.GetMeta(photoID); err == nil && len(albumPostIDsForChannel(entries, slug)) == 0 {
		deleteChannelPublicationKeys(store, photoID, slug)
	}
}

// removeFromFilesDestination takes the album out of the photo's sidecar and
// deletes the photo's exported file from the destination's folder.
func removeFromFilesDestination(store *lib.Store, chStore *channels.Store, ch *channels.Channel, photoID, postID string) error {
	pathHint, _ := store.GetPhotoPathHint(photoID)
	if pathHint == "" {
		return nil
	}
	if err := media.RemovePublication(pathHint, ch.Slug, postID); err != nil {
		return fmt.Errorf("sidecar: %w", err)
	}
	dir := chStore.OutputDir(ch.Slug)
	if name := exportedFileName(dir, ch, pathHint); name != "" {
		os.Remove(filepath.Join(dir, name)) //nolint:errcheck // a file already gone is the goal
	}
	return nil
}

// exportedFileName finds the photo's file among "{slug}_{stamp}_{base}{ext}"
// in dir. The stamp is not recorded, so a base name that matches more than one
// file (the same photo in two albums, or two photos of one name) is left
// alone rather than guessed: it returns "" then.
func exportedFileName(dir string, ch *channels.Channel, pathHint string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	const stampLen = len("20060102T150405Z")
	prefix := ch.Slug + "_"
	suffix := "_" + strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint)) + exportExt(ch)
	var found []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && len(name) == len(prefix)+stampLen+len(suffix) && strings.HasSuffix(name, suffix) {
			found = append(found, name)
		}
	}
	if len(found) != 1 {
		return ""
	}
	return found[0]
}

// removePhotoFromSite removes a photo from every album in a site-export channel:
// updates site.json, deletes the exported file and thumbnail, regenerates album HTML
// and the site index. Meta key deletion is handled by the caller.
func removePhotoFromSite(store *lib.Store, ch *channels.Channel, chStore *channels.Store, photoID, slug string) error {
	siteDir := filepath.Join(chStore.OutputDir(slug), "site")
	sites := site.NewStore(chStore, slug)
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
func removeFromSiteAlbum(album site.Album, siteDir, slug, photoID, legacyPrefix, pathHint string) []site.Photo {
	albumDir := filepath.Join(siteDir, "albums", site.AlbumFolderName(album))
	var kept []site.Photo
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
func saveSiteRemoval(albums []site.Album, touched map[string]bool, sites *site.Store, siteDir string, ch *channels.Channel) ([]site.Album, error) {
	albumNav := site.BuildNavContext(ch, siteDir, false)
	var remaining []site.Album
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
