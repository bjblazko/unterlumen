package apilibrary

import (
	"fmt"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// galleryListItem is the JSON shape returned by GET /api/channels/{slug}/galleries.
type galleryListItem struct {
	PostID      string    `json:"postID"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt"` // zero-valued when never updated; omitempty does nothing on a time.Time
	PhotoCount  int       `json:"photoCount"`
	// GeneratedAt / DeployedAt let the frontend tell "built, not uploaded"
	// from "online" (ADR-0029). Zero means not recorded, not "never".
	GeneratedAt time.Time `json:"generatedAt"`
	DeployedAt  time.Time `json:"deployedAt"`
	// FolderName is the actual on-disk (and on-URL) folder name for this
	// album — the slugified title, or for unlisted albums the slug plus its
	// random token. Empty for GalleryExport (non-site) channels, which have
	// no per-album folder distinct from PostID.
	FolderName string `json:"folderName,omitempty"`
	// Unlisted mirrors SiteAlbum.Unlisted / GalleryState.Unlisted — true if
	// this album carries a noindex tag and is only reachable via direct link
	// (site channels additionally exclude it from their index and sitemap).
	Unlisted bool `json:"unlisted,omitempty"`
}

// collectGalleryItems loads the published-gallery list for a single channel
// from its statefile(s) — site.json for SiteExport channels, one gallery.json
// per subfolder for GalleryExport channels. Shared by listGalleries (scoped to
// one channel) and listAllGalleries (aggregated across every channel).
func collectGalleryItems(ch *channels.Channel, chStore *channels.Store) ([]galleryListItem, error) {
	var items []galleryListItem
	channelDir := chStore.OutputDir(ch.Slug)

	switch {
	case ch.SiteExport:
		albums, err := newSiteStore(chStore, ch.Slug).List()
		if err != nil {
			return nil, fmt.Errorf("read site state: %w", err)
		}
		for _, a := range albums {
			items = append(items, galleryListItem{
				PostID:      a.PostID,
				Title:       a.Title,
				PublishedAt: a.PublishedAt,
				UpdatedAt:   a.UpdatedAt,
				PhotoCount:  a.PhotoCount,
				GeneratedAt: a.GeneratedAt,
				DeployedAt:  a.DeployedAt,
				FolderName:  albumFolderName(a),
				Unlisted:    a.Unlisted,
			})
		}
		// Sort newest first (site index sorts the same way).
		sort.Slice(items, func(i, j int) bool {
			return items[i].PublishedAt.After(items[j].PublishedAt)
		})
	case ch.GalleryExport:
		entries, rdErr := os.ReadDir(channelDir)
		if rdErr != nil && !os.IsNotExist(rdErr) {
			return nil, fmt.Errorf("read channel dir: %w", rdErr)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			gs, gsErr := loadGalleryState(filepath.Join(channelDir, e.Name(), "gallery.json"))
			if gsErr != nil || gs == nil {
				continue
			}
			items = append(items, galleryListItem{
				PostID:      gs.PostID,
				Title:       gs.Title,
				PublishedAt: gs.PublishedAt,
				UpdatedAt:   gs.UpdatedAt,
				PhotoCount:  gs.PhotoCount,
				GeneratedAt: gs.GeneratedAt,
				DeployedAt:  gs.DeployedAt,
				FolderName:  e.Name(), // gallery-export mode: folder name == PostID, use the actual dir name on disk
				Unlisted:    gs.Unlisted,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].PublishedAt.After(items[j].PublishedAt)
		})
	}

	return items, nil
}

// listGalleries returns existing built galleries/albums for a channel.
func listGalleries(chStore *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusNotFound)
			return
		}
		items, err := collectGalleryItems(ch, chStore)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if items == nil {
			items = []galleryListItem{}
		}
		writeJSON(w, items)
	}
}

// rebuildSite regenerates site assets and HTML from site.json, cross-checking each
// photo against: (1) file existence on disk, (2) build meta in the library DB
// (when a photoID is stored) or a base-name reverse lookup (for pre-photoID entries).
// Photos that fail either check are pruned from site.json before regenerating.
func rebuildSite(chStore *channels.Store, mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		channelSlug := r.PathValue("slug")

		ch, err := chStore.Get(channelSlug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !ch.SiteExport {
			http.Error(w, "channel is not configured for site export", http.StatusBadRequest)
			return
		}

		siteDir, albumCount, err := rebuildSiteChannel(chStore, mgr, ch)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"sitePath": siteDir, "albumCount": albumCount})
	}
}

// rebuildSiteChannel regenerates site assets and HTML from site.json for a
// site-export channel, cross-checking each photo against: (1) file existence
// on disk, (2) build meta in the library DB (when a photoID is stored) or a
// base-name reverse lookup (for pre-photoID entries). Photos that fail either
// check are pruned from site.json before regenerating. Shared by the
// rebuild-site HTTP handler and renameGallery/deleteGallery, which need the
// same full regeneration after editing site.json directly.
func rebuildSiteChannel(chStore *channels.Store, mgr *lib.Manager, ch *channels.Channel) (siteDir string, albumCount int, err error) {
	siteDir = filepath.Join(chStore.OutputDir(ch.Slug), "site")
	sites := newSiteStore(chStore, ch.Slug)
	albums, err := sites.List()
	if err != nil {
		return "", 0, fmt.Errorf("read site state: %w", err)
	}
	if err := writeSiteAssets(filepath.Join(siteDir, "assets")); err != nil {
		return "", 0, fmt.Errorf("write site assets: %w", err)
	}

	assignMissingSlugs(albums)
	modified := dedupeSitePhotos(albums)
	pruneSitePhotos(albums, modified, siteDir, ch.Slug, mgr)
	remaining := saveSitePruning(albums, modified, sites, siteDir)

	albumNav := buildSiteNavContext(ch, siteDir, false)
	for i := range remaining {
		regenerateSiteAlbum(&remaining[i], siteDir, ch, mgr, albumNav)
	}
	if err := writeSitePages(ch, siteDir, remaining); err != nil {
		return "", 0, err
	}
	return siteDir, len(remaining), nil
}

// assignMissingSlugs gives albums without a slug one (migration for pre-slug
// albums).
func assignMissingSlugs(albums []SiteAlbum) {
	for i := range albums {
		if albums[i].Slug == "" {
			others := make([]SiteAlbum, 0, len(albums)-1)
			others = append(others, albums[:i]...)
			others = append(others, albums[i+1:]...)
			albums[i].Slug = computeSlug(albums[i].Title, albums[i].PublishedAt, others, albums[i].Unlisted)
		}
	}
}

// dedupeSitePhotos repairs repeats left by older runs (a photo added to an
// album it was already in was listed twice). It returns the indexes of the
// albums it changed.
func dedupeSitePhotos(albums []SiteAlbum) map[int]bool {
	modified := make(map[int]bool)
	for i := range albums {
		if unique := dedupePhotos(albums[i].Photos); len(unique) != len(albums[i].Photos) {
			albums[i].Photos, albums[i].PhotoCount = unique, len(unique)
			modified[i] = true
		}
	}
	return modified
}

// pruneSitePhotos removes photos whose build metadata was removed from the
// library, marking the albums it changes in modified.
// Photos with a stored PhotoID can be re-exported from library source files if
// their exported file is missing, so we only remove them on a metadata failure.
// Legacy entries without a PhotoID cannot be re-exported; they are also pruned
// when their exported file is absent from disk.
func pruneSitePhotos(albums []SiteAlbum, modified map[int]bool, siteDir, channelSlug string, mgr *lib.Manager) {
	// A reverse index (base name → photoID) from all library stores, built
	// once, so we avoid scanning every library for every photo in the album.
	baseToPhotoID := buildBasePhotoIndex(mgr)
	for i := range albums {
		kept, pruned := keptSitePhotos(albums[i], siteDir, channelSlug, mgr, baseToPhotoID)
		if pruned {
			modified[i] = true
		}
		if modified[i] {
			albums[i].Photos = kept
			albums[i].PhotoCount = len(kept)
		}
	}
}

// keptSitePhotos returns the album's photos that survive pruning and whether
// any did not.
func keptSitePhotos(album SiteAlbum, siteDir, channelSlug string, mgr *lib.Manager, baseToPhotoID map[string]string) (kept []SitePhoto, pruned bool) {
	albumDir := filepath.Join(siteDir, "albums", albumFolderName(album))
	_, albumDirErr := os.Stat(albumDir)
	albumDirPresent := albumDirErr == nil
	for _, sp := range album.Photos {
		pid := sp.PhotoID
		if pid == "" {
			pid = baseToPhotoID[sitePhotoBase(sp.Filename, channelSlug)]
		}
		// Remove photos that lost their build metadata in the library.
		if pid != "" && mgr != nil && !sitePhotoHasMeta(mgr, pid, channelSlug, album.PostID) {
			pruned = true
			continue
		}
		// Legacy entries (no stored PhotoID) are also pruned when their exported
		// file is absent — they cannot be re-exported from the library.
		if sp.PhotoID == "" && albumDirPresent {
			if _, statErr := os.Stat(filepath.Join(albumDir, sp.Filename)); os.IsNotExist(statErr) {
				pruned = true
				continue
			}
		}
		kept = append(kept, sp)
	}
	return kept, pruned
}

// saveSitePruning writes only what pruning changed: albums that lost photos
// are upserted, albums that became empty are removed. Untouched albums stay as
// they are in the shared register. It returns the albums that remain.
func saveSitePruning(albums []SiteAlbum, modified map[int]bool, sites *siteStore, siteDir string) []SiteAlbum {
	var remaining []SiteAlbum
	for i, album := range albums {
		switch {
		case !modified[i]:
			remaining = append(remaining, album)
		case album.PhotoCount == 0:
			os.RemoveAll(filepath.Join(siteDir, "albums", albumFolderName(album))) //nolint:errcheck
			sites.Remove(album.PostID)                                             //nolint:errcheck
		default:
			remaining = append(remaining, album)
			sites.Upsert(album) //nolint:errcheck
		}
	}
	return remaining
}

// regenerateSiteAlbum regenerates one album page and rebuilds its ZIP if one
// exists.
func regenerateSiteAlbum(album *SiteAlbum, siteDir string, ch *channels.Channel, mgr *lib.Manager, albumNav SiteNavContext) {
	albumDir := filepath.Join(siteDir, "albums", albumFolderName(*album))
	os.MkdirAll(filepath.Join(albumDir, "thumbs"), 0o700) //nolint:errcheck
	if mgr != nil {
		restoreMissingExports(album.Photos, albumDir, ch, mgr)
	}

	items := buildGalleryItems(album.Photos)
	if len(items) == 0 {
		items = scanAlbumPhotos(albumDir)
	}
	if len(items) == 0 {
		return
	}
	zipName := rebuildAlbumZip(album, albumDir)
	albumHTML := GenerateSiteGallery(album.Title, ch.SiteTheme, items, GalleryOptions{
		ZipFilename: zipName,
		SiteTitle:   ch.SiteTitle,
		DateStr:     dateRangeStr(album.PublishedAt, album.UpdatedAt),
		SiteURL:     ch.SiteURL,
		AlbumSlug:   albumFolderName(*album),
		PublishedAt: album.PublishedAt,
		Unlisted:    album.Unlisted,
		Nav:         albumNav,
	})
	os.WriteFile(filepath.Join(albumDir, "index.html"), albumHTML, 0o644) //nolint:errcheck
}

// restoreMissingExports re-exports any photos whose exported file is missing
// from disk. This restores the full album after the output folder has been
// wiped, using the original source files in the library.
func restoreMissingExports(photos []SitePhoto, albumDir string, ch *channels.Channel, mgr *lib.Manager) {
	for _, sp := range photos {
		if sp.PhotoID == "" {
			continue // legacy entry — no source link, cannot re-export
		}
		photoPath := filepath.Join(albumDir, sp.Filename)
		if _, statErr := os.Stat(photoPath); statErr == nil {
			continue // file already on disk
		}
		srcPath, srcErr := findPhotoSourcePath(mgr, sp.PhotoID)
		if srcErr != nil || srcPath == "" {
			continue
		}
		if exported, exportErr := media.ExportImage(srcPath, ch.ExportOptions()); exportErr == nil {
			os.WriteFile(photoPath, exported, 0o644) //nolint:errcheck
		}
		if sp.ThumbFilename != "" {
			thumbPath := filepath.Join(albumDir, sp.ThumbFilename)
			if _, statErr := os.Stat(thumbPath); os.IsNotExist(statErr) {
				if thumb, thumbErr := media.ExportImage(srcPath, galleryThumbOpts); thumbErr == nil {
					os.WriteFile(thumbPath, thumb, 0o644) //nolint:errcheck
				}
			}
		}
	}
}

// rebuildAlbumZip rewrites an album's ZIP when it has or had one, returning
// its name, or "" when there is none.
func rebuildAlbumZip(album *SiteAlbum, albumDir string) string {
	_, statErr := os.Stat(filepath.Join(albumDir, "photos.zip"))
	if !album.HasZip && statErr != nil {
		return ""
	}
	zipResults := make([]buildResult, len(album.Photos))
	for j, sp := range album.Photos {
		zipResults[j] = buildResult{Filename: sp.Filename}
	}
	if err := createGalleryZip(zipResults, albumDir, "photos.zip"); err != nil {
		return ""
	}
	album.HasZip = true
	return "photos.zip"
}

// writeSitePages regenerates the site index, about, imprint, robots.txt and,
// with a site URL, the sitemap from the whole album register.
func writeSitePages(ch *channels.Channel, siteDir string, albums []SiteAlbum) error {
	rootNav := buildSiteNavContext(ch, siteDir, true)
	siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, albums, rootNav)
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); err != nil {
		return fmt.Errorf("write site index: %w", err)
	}
	generateAboutPage(siteDir, ch, avatarExistsAt(siteDir), rootNav) //nolint:errcheck
	generateImprintPage(siteDir, ch, rootNav)                        //nolint:errcheck
	generateRobotsTxt(siteDir, ch.SiteURL)                           //nolint:errcheck
	if ch.SiteURL != "" {
		generateSitemap(siteDir, albums, ch.SiteURL) //nolint:errcheck
	}
	return nil
}

// rebuildGalleries regenerates index.html for every existing single-gallery
// build on a GalleryExport channel, from each build's gallery.json state and
// its already-exported photo files on disk. It re-runs the current
// GenerateGallery template (picking up template fixes/changes) without
// re-exporting or duplicating any photos — the counterpart to rebuildSite
// for channels that use single-gallery export instead of a multi-album site.
func rebuildGalleries(chStore *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !ch.GalleryExport {
			http.Error(w, "channel is not configured for single-gallery export", http.StatusBadRequest)
			return
		}

		channelDir := chStore.OutputDir(slug)
		entries, err := os.ReadDir(channelDir)
		if err != nil {
			if os.IsNotExist(err) {
				writeJSON(w, map[string]any{"rebuilt": 0})
				return
			}
			http.Error(w, "read channel dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var rebuilt int
		var errs []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			outDir := filepath.Join(channelDir, e.Name())
			gs, gsErr := loadGalleryState(filepath.Join(outDir, "gallery.json"))
			if gsErr != nil || gs == nil {
				continue // not a gallery folder (or unreadable state) — skip, not an error
			}
			if regenErr := regenerateGalleryFolder(outDir, gs); regenErr != nil {
				errs = append(errs, e.Name()+": "+regenErr.Error())
				continue
			}
			rebuilt++
		}

		if len(errs) > 0 {
			writeJSON(w, map[string]any{"rebuilt": rebuilt, "errors": errs})
			return
		}
		writeJSON(w, map[string]any{"rebuilt": rebuilt})
	}
}

// regenerateGalleryFolder regenerates index.html (and gallery assets) for one
// already-exported single-gallery build, from its gallery.json state and the
// photo files already on disk in outDir. Shared by rebuildGalleries (which
// loops every folder in a channel) and renameGallery/deleteGallery (which
// only need to touch the one affected folder after editing gallery.json).
func regenerateGalleryFolder(outDir string, gs *GalleryState) error {
	items := make([]GalleryItem, 0, len(gs.Photos))
	for _, sp := range gs.Photos {
		w, h := readImageDimensions(filepath.Join(outDir, sp.Filename))
		items = append(items, GalleryItem{
			PhotoID:       sp.PhotoID,
			Filename:      sp.Filename,
			ThumbFilename: sp.ThumbFilename,
			Width:         w,
			Height:        h,
		})
	}

	zipName := ""
	if gs.HasZip {
		zipName = "photos.zip"
	}
	html := GenerateGallery(gs.Title, items, GalleryOptions{
		ZipFilename: zipName,
		DateStr:     dateRangeStr(gs.PublishedAt, gs.UpdatedAt),
		Unlisted:    gs.Unlisted,
	})
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), html, 0o644); err != nil {
		return err
	}
	return writeGalleryAssets(outDir)
}

// readImageDimensions decodes just enough of the image at path to report its
// pixel dimensions, without loading the full image into memory. Used by
// rebuildGalleries to recover width/height for GalleryItem, since
// GalleryState/SitePhoto don't persist them (they're only known at export
// time) — returns 0, 0 (which GalleryItem/thumbDimensions treats as "omit
// width/height attributes") if the file is missing or undecodable.
func readImageDimensions(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// sitePhotoBase extracts the original source-file base name from a SitePhoto filename.
// SitePhoto filenames follow the pattern "{slug}_{ts}_{base}{ext}".
func sitePhotoBase(filename, slug string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	prefix := slug + "_"
	if !strings.HasPrefix(name, prefix) {
		return ""
	}
	rest := name[len(prefix):]
	idx := strings.IndexByte(rest, '_')
	if idx < 0 {
		return ""
	}
	return strings.ToLower(rest[idx+1:])
}

// buildBasePhotoIndex scans all library stores and returns a map from
// lower-cased path_hint base name → photo ID. Used to resolve legacy SitePhoto
// entries that don't have a stored photoID.
func buildBasePhotoIndex(mgr *lib.Manager) map[string]string {
	m := make(map[string]string)
	if mgr == nil {
		return m
	}
	libs, err := mgr.ListLibraries()
	if err != nil {
		return m
	}
	for _, l := range libs {
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		refs, err := store.ListAllPhotoRefs()
		store.Close()
		if err != nil {
			continue
		}
		for _, ref := range refs {
			base := strings.ToLower(strings.TrimSuffix(filepath.Base(ref.PathHint), filepath.Ext(ref.PathHint)))
			if base != "" {
				m[base] = ref.ID
			}
		}
	}
	return m
}

// findPhotoSourcePath searches all library stores for photoID and returns its disk path.
func findPhotoSourcePath(mgr *lib.Manager, photoID string) (string, error) {
	libs, err := mgr.ListLibraries()
	if err != nil {
		return "", err
	}
	for _, l := range libs {
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		path, storeErr := store.GetPhotoPathHint(photoID)
		store.Close()
		if storeErr == nil && path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("photo %s not found in any library", photoID)
}

// sitePhotoHasMeta reports whether any library store confirms this photo is still
// built to the given channel+album. It checks the qualified key
// built:{channelSlug}:{albumPostID} first (written by current builds), then falls
// back to the unqualified built:{channelSlug} for legacy entries. It also checks the
// pre-rename published:{channelSlug}[:{albumPostID}] keys, for photos that haven't
// been rebuilt since the built: key prefix replaced published:.
func sitePhotoHasMeta(mgr *lib.Manager, photoID, channelSlug, albumPostID string) bool {
	libs, err := mgr.ListLibraries()
	if err != nil {
		return true // keep on error
	}
	qualKey := "built:" + channelSlug + ":" + albumPostID
	legacyKey := "built:" + channelSlug
	legacyQualKey := "published:" + channelSlug + ":" + albumPostID
	legacyUnqualKey := "published:" + channelSlug
	for _, l := range libs {
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		entries, err := store.GetMeta(photoID)
		store.Close()
		if err != nil {
			continue
		}
		for _, e := range entries {
			if albumPostID != "" && (e.Key == qualKey || e.Key == legacyQualKey) {
				return true
			}
			if e.Key == legacyKey || e.Key == legacyUnqualKey {
				return true
			}
		}
	}
	return false
}
