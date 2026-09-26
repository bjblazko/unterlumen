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

		// For pending:{slug} keys, the meta row is a derived signal that this
		// photo sits in an unfinished draft for that channel — removing it must
		// also remove the photo from the draft itself (drafts.json), not just
		// delete the meta row and leave the draft still holding a reference the
		// UI no longer shows.
		const pendingPrefix = "pending:"
		if strings.HasPrefix(key, pendingPrefix) && draftStore != nil {
			// Two key shapes: "pending:<slug>" names the draft in its value,
			// "pending:<slug>:<draftID>" names it in the key itself (its value
			// is the album title).
			slug, draftID, qualified := strings.Cut(strings.TrimPrefix(key, pendingPrefix), ":")
			if !qualified {
				if entries, metaErr := store.GetMeta(photoID); metaErr == nil {
					for _, e := range entries {
						if e.Key == key {
							draftID = e.Value
							break
						}
					}
				}
			}
			if draftID != "" {
				draftStore.RemovePhoto(slug, draftID, id, photoID) //nolint:errcheck
			}
			store.DeleteMeta(photoID, key) //nolint:errcheck
			if qualified {
				clearPendingMarkers(store, photoID, slug, draftID)
			} else {
				store.DeleteMeta(photoID, pendingPrefix+slug+":"+draftID) //nolint:errcheck
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// For built:{slug} keys on site-export channels, remove the photo from
		// the site (site.json + physical files) and delete all related keys. The
		// frontend only ever sends the normalized built: prefix (see getMeta), but a
		// photo may still carry legacy published:{slug} entries from before this
		// channel-membership key was renamed, so both prefixes are cleaned up here.
		if strings.HasPrefix(key, buildPrefix) && chStore != nil {
			rest := strings.TrimPrefix(key, buildPrefix)
			slug, albumPostID, _ := strings.Cut(rest, ":")
			if ch, chErr := chStore.Get(slug); chErr == nil {
				switch {
				// Removing a publication from a site-export photo means taking
				// it off the site entirely — there is no per-album removal —
				// so both the channel marker and one album's key land here.
				case ch.SiteExport && !reservedMetaSuffix(albumPostID):
					if rmErr := removePhotoFromSite(store, ch, chStore, photoID, slug); rmErr != nil {
						http.Error(w, "remove from site: "+rmErr.Error(), http.StatusInternalServerError)
						return
					}
					deleteChannelPublicationKeys(store, photoID, slug)
					w.WriteHeader(http.StatusNoContent)
					return
				// A gallery channel's albums are independent: drop just this
				// album, and the channel marker only if it was the last one.
				case albumPostID != "" && !reservedMetaSuffix(albumPostID):
					deleteAlbumKeys(store, photoID, slug, albumPostID)
					if entries, metaErr := store.GetMeta(photoID); metaErr == nil && len(albumPostIDsForChannel(entries, slug)) == 0 {
						deleteChannelPublicationKeys(store, photoID, slug)
					}
					w.WriteHeader(http.StatusNoContent)
					return
				case albumPostID == "":
					deleteChannelPublicationKeys(store, photoID, slug)
					w.WriteHeader(http.StatusNoContent)
					return
				}
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

// removePhotoFromSite removes a photo from every album in a site-export channel:
// updates site.json, deletes the exported file and thumbnail, regenerates album HTML
// and the site index. Meta key deletion is handled by the caller.
func removePhotoFromSite(store *lib.Store, ch *channels.Channel, chStore *channels.Store, photoID, slug string) error {
	channelDir := chStore.OutputDir(slug)
	siteDir := filepath.Join(channelDir, "site")
	sites := newSiteStore(chStore, slug)

	albums, err := sites.List()
	if err != nil {
		return fmt.Errorf("load site state: %w", err)
	}

	// Reconstruct the expected filename prefix as a fallback for pre-photoID entries.
	pathHint, _ := store.GetPhotoPathHint(photoID)
	var legacyPrefix string
	if pathHint != "" {
		if metaEntries, metaErr := store.GetMeta(photoID); metaErr == nil {
			buildKey := "built:" + slug
			legacyKey := "published:" + slug
			for _, e := range metaEntries {
				if e.Key == buildKey || e.Key == legacyKey {
					if t, tErr := time.Parse(time.RFC3339, e.Value); tErr == nil {
						ts := t.UTC().Format("20060102T150405Z")
						base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
						legacyPrefix = slug + "_" + ts + "_" + base
					}
					break
				}
			}
		}
	}

	modified := false
	touched := map[string]bool{} // postIDs of albums that lost photos
	for i := range albums {
		var kept []SitePhoto
		albumDir := filepath.Join(siteDir, "albums", albumFolderName(albums[i]))
		for _, sp := range albums[i].Photos {
			match := (sp.PhotoID != "" && sp.PhotoID == photoID) ||
				(legacyPrefix != "" && strings.HasPrefix(sp.Filename, legacyPrefix))
			if match {
				// Delete exported file and thumbnail, and take the album
				// out of the photo's own sidecar so a rebuild cannot find it.
				if pathHint != "" {
					media.RemovePublication(pathHint, slug, albums[i].PostID) //nolint:errcheck
				}
				os.Remove(filepath.Join(albumDir, sp.Filename)) //nolint:errcheck
				if sp.ThumbFilename != "" {
					os.Remove(filepath.Join(albumDir, sp.ThumbFilename)) //nolint:errcheck
				}
				modified = true
			} else {
				kept = append(kept, sp)
			}
		}
		if len(kept) != len(albums[i].Photos) {
			albums[i].Photos = kept
			albums[i].PhotoCount = len(kept)
			touched[albums[i].PostID] = true
		}
	}

	if !modified {
		return nil
	}

	// Remove albums that are now empty and regenerate HTML for those that remain.
	rootNav := buildSiteNavContext(ch, siteDir, true)
	albumNav := buildSiteNavContext(ch, siteDir, false)
	var remaining []SiteAlbum
	for _, album := range albums {
		if album.PhotoCount == 0 {
			os.RemoveAll(filepath.Join(siteDir, "albums", albumFolderName(album))) //nolint:errcheck
			if err := sites.Delete(album.PostID); err != nil {                     // its last photo was taken off the site on purpose
				return fmt.Errorf("save site state: %w", err)
			}
			continue
		}
		remaining = append(remaining, album)
		if touched[album.PostID] {
			if err := sites.Upsert(album); err != nil {
				return fmt.Errorf("save site state: %w", err)
			}
		}
		albumDir := filepath.Join(siteDir, "albums", albumFolderName(album))
		items := buildGalleryItems(album.Photos)
		zipName := ""
		zipPath := filepath.Join(albumDir, "photos.zip")
		if album.HasZip || func() bool { _, e := os.Stat(zipPath); return e == nil }() {
			// Rebuild ZIP without the removed photo.
			zipResults := make([]buildResult, len(album.Photos))
			for i, sp := range album.Photos {
				zipResults[i] = buildResult{Filename: sp.Filename}
			}
			if zipErr := createGalleryZip(zipResults, albumDir, "photos.zip"); zipErr == nil {
				zipName = "photos.zip"
			}
		}
		dateStr := dateRangeStr(album.PublishedAt, album.UpdatedAt)
		albumHTML := GenerateSiteGallery(album.Title, ch.SiteTheme, items, GalleryOptions{
			ZipFilename: zipName,
			SiteTitle:   ch.SiteTitle,
			DateStr:     dateStr,
			SiteURL:     ch.SiteURL,
			AlbumSlug:   albumFolderName(album),
			PublishedAt: album.PublishedAt,
			Unlisted:    album.Unlisted,
			Nav:         albumNav,
		})
		os.WriteFile(filepath.Join(albumDir, "index.html"), albumHTML, 0o644) //nolint:errcheck
	}

	siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, remaining, rootNav)
	os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644) //nolint:errcheck
	generateAboutPage(siteDir, ch, avatarExistsAt(siteDir), rootNav)    //nolint:errcheck
	generateImprintPage(siteDir, ch, rootNav)                           //nolint:errcheck
	generateRobotsTxt(siteDir, ch.SiteURL)                              //nolint:errcheck
	if ch.SiteURL != "" {
		generateSitemap(siteDir, remaining, ch.SiteURL) //nolint:errcheck
	}
	return nil
}
