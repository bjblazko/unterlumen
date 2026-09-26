package apilibrary

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// --- Build ---

type buildResult struct {
	PhotoID       string `json:"photoID"`
	OutputPath    string `json:"outputPath,omitempty"`
	Filename      string `json:"filename,omitempty"`
	ThumbFilename string `json:"thumbFilename,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	Error         string `json:"error,omitempty"`
}

func generateDraft(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil || draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		draftID := r.PathValue("draftID")

		var body struct {
			PublishedAt string `json:"publishedAt,omitempty"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck // empty body is valid; PublishedAt defaults below

		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}

		// draftID == "-" is the sentinel for "regenerate this gallery/album with
		// no newly collected photos" — used by the Publish dialog for a Live
		// gallery that has no pending draft at all. It subsumes the old
		// Rebuild/Rebuild-site actions: same code path below, just with an
		// empty Photos list, driven by a synthetic in-memory draft instead of
		// one loaded from drafts.json.
		var draft *channels.Draft
		if draftID == "-" {
			postID := r.URL.Query().Get("postID")
			if postID == "" {
				http.Error(w, "postID query parameter required when regenerating without a draft", http.StatusBadRequest)
				return
			}
			draft = &channels.Draft{Target: channels.DraftTarget{PostID: postID}}
		} else {
			draft, err = draftStore.Get(slug, draftID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if draft.Target.Account != "" && ch.AccountByID(draft.Target.Account) == nil {
			http.Error(w, "account not found: "+draft.Target.Account, http.StatusBadRequest)
			return
		}

		publishedAt := time.Now().UTC()
		if body.PublishedAt != "" {
			if t, parseErr := time.Parse(time.RFC3339, body.PublishedAt); parseErr == nil {
				publishedAt = t
			}
		}

		// Open one *lib.Store per distinct library referenced by the draft —
		// a draft's photos can span multiple libraries, collected across
		// separate sessions (ADR-0016: channel output is library-independent).
		stores := map[string]*lib.Store{}
		defer func() {
			for _, s := range stores {
				s.Close()
			}
		}()
		for _, dp := range draft.Photos {
			if _, ok := stores[dp.LibraryID]; ok {
				continue
			}
			s, openErr := mgr.OpenStore(dp.LibraryID)
			if openErr != nil {
				http.Error(w, "library not found: "+dp.LibraryID, http.StatusNotFound)
				return
			}
			stores[dp.LibraryID] = s
		}

		ts := publishedAt.UTC().Format("20060102T150405Z")

		addToExisting := draft.Target.PostID != ""
		galleryMode := ch.GalleryExport && (draft.Target.Title != "" || addToExisting)
		siteMode := ch.SiteExport && (draft.Target.Title != "" || addToExisting)
		channelDir := chStore.OutputDir(slug)

		target, status, targetErr := resolveAlbumTarget(draft, channelDir, newSiteStore(chStore, slug), publishedAt, galleryMode, siteMode)
		if targetErr != nil {
			http.Error(w, targetErr.Error(), status)
			return
		}
		albumPostID := target.postID
		albumSlug := target.slug
		outDir := target.outDir
		existingPhotos := target.existingPhotos
		existingTitle := target.existingTitle
		existingPublishedAt := target.existingPublishedAt
		albumUnlisted := target.unlisted

		// PostID must name the album the photos actually land in. On
		// add-to-existing that is the target album — minting a fresh ID here
		// would write XMP sidecars and built: meta pointing at a gallery that
		// is never created.
		pub := media.Publication{
			Channel: slug, Account: draft.Target.Account, PostID: albumPostID,
			GalleryTitle: draft.Target.Title, PublishedAt: publishedAt,
		}
		if siteMode {
			// The slug is the album's URL and cannot be derived again, so
			// each photo's sidecar carries it (with the flag it encodes).
			pub.Slug, pub.Unlisted = albumSlug, albumUnlisted
		}

		if err := os.MkdirAll(outDir, 0o700); err != nil {
			http.Error(w, "create output dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// When adding to an existing album, ensure the album title is recorded in meta
		// for each newly built photo so they appear in album-based library searches.
		if existingTitle != "" && pub.GalleryTitle == "" {
			pub.GalleryTitle = existingTitle
		}

		// clearSucceededPhotos removes only the draft photos that were actually
		// exported without error, leaving failed ones (missing source file,
		// unreadable image, disk full, ...) pending in the draft so the user can
		// see and retry them — buildOne reports failure per-photo via res.Error
		// rather than aborting the whole batch, so a draft can partially succeed.
		// clearSucceededPhotos pairs results with draft.Photos by index: both the
		// synchronous and streaming build loops append exactly one buildResult per
		// draft.Photos entry, in the same order, with no filtering in between — so
		// results[i] always corresponds to draft.Photos[i]. This index pairing (rather
		// than a map keyed by the bare content-hash PhotoID) is required because the
		// same PhotoID can legitimately appear under two different LibraryIDs in one
		// draft (duplicate-content imports across libraries): keying by PhotoID alone
		// would let one library's success mark the other library's failed entry as
		// succeeded too, silently clearing its pending: meta.
		// rememberAlbum pins the draft to the album its photos just landed in.
		// Photos that failed to export stay in the draft; without this, the
		// retry would mint a fresh postID and build a second album with the
		// same title instead of completing the first one.
		rememberAlbum := func() {
			if addToExisting || draftID == "-" {
				return
			}
			draftStore.SetTargetPostID(slug, draftID, albumPostID) //nolint:errcheck
		}

		clearSucceededPhotos := func(results []buildResult) {
			if draftID == "-" {
				return // synthetic draft — nothing was ever persisted
			}
			for i, dp := range draft.Photos {
				if i >= len(results) || results[i].Error != "" {
					continue // failed export: keep in draft and keep pending: meta
				}
				draftStore.RemovePhoto(slug, draftID, dp.LibraryID, dp.PhotoID) //nolint:errcheck
				if s, ok := stores[dp.LibraryID]; ok {
					clearPendingMarkers(s, dp.PhotoID, slug, draftID)
				}
			}
		}

		if !galleryMode && !siteMode {
			// Fast synchronous path for regular (non-gallery) builds.
			var results []buildResult
			for _, dp := range draft.Photos {
				res := buildOne(stores[dp.LibraryID], ch, pub, ts, outDir, "", dp.PhotoID, true)
				results = append(results, res)
			}
			clearSucceededPhotos(results)
			writeJSON(w, map[string]any{"postID": albumPostID, "results": results})
			return
		}

		// Gallery path: stream SSE progress, generate thumbnails, ZIP, and HTML.
		thumbDir := filepath.Join(outDir, "thumbs")
		if err := os.MkdirAll(thumbDir, 0o700); err != nil {
			http.Error(w, "create thumbs dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		publishTitle := draft.Target.Title
		if existingTitle != "" {
			publishTitle = existingTitle
		}
		reporter := &publishReporter{job: mgr.Jobs().Start("publish", fmt.Sprintf("Publishing %q", publishTitle), "galleries")}
		// A run that returns without its last word was cut short.
		defer reporter.job.Finish(errors.New("it stopped before it finished"))
		emit := func(v map[string]any) {
			data, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			reporter.report(v)
		}

		total := len(draft.Photos)
		var results []buildResult
		for i, dp := range draft.Photos {
			res := buildOne(stores[dp.LibraryID], ch, pub, ts, outDir, thumbDir, dp.PhotoID, true)
			results = append(results, res)
			emit(map[string]any{"step": "photo", "done": i + 1, "total": total, "file": res.Filename})
		}

		// Build merged items list: existing photos first, then newly exported,
		// each file once.
		items := mergePhotoItems(existingPhotos, results)

		// The ZIP holds the same files as the page, including the previous
		// photos when adding to an existing album.
		zipResults := make([]buildResult, len(items))
		for i, it := range items {
			zipResults[i] = buildResult{Filename: it.Filename}
		}

		// ZIP of full-res photos.
		emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "Creating ZIP…"})
		zipName := "photos.zip"
		if zipErr := createGalleryZip(zipResults, outDir, zipName); zipErr != nil {
			emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "ZIP failed: " + zipErr.Error()})
			zipName = ""
		} else {
			emit(map[string]any{"step": "zip", "done": 1, "total": 1, "file": "ZIP ready"})
		}

		// Gallery title: use existing title when adding to an existing gallery.
		galleryTitle := draft.Target.Title
		if existingTitle != "" {
			galleryTitle = existingTitle
		}

		// Compute the date range string for SEO metadata.
		albumPublishedAt := publishedAt
		var albumUpdatedAt time.Time
		if !existingPublishedAt.IsZero() {
			albumPublishedAt = existingPublishedAt
			albumUpdatedAt = publishedAt
		}
		dateStr := dateRangeStr(albumPublishedAt, albumUpdatedAt)

		// Generate HTML gallery.
		emit(map[string]any{"step": "html", "done": 0, "total": 1, "file": "Generating gallery…"})
		var html []byte
		if siteMode {
			html = GenerateSiteGallery(galleryTitle, ch.SiteTheme, items, GalleryOptions{
				ZipFilename: zipName,
				SiteTitle:   ch.SiteTitle,
				DateStr:     dateStr,
				SiteURL:     ch.SiteURL,
				AlbumSlug:   albumSlug,
				PublishedAt: albumPublishedAt,
				Unlisted:    albumUnlisted,
				Nav:         buildSiteNavContext(ch, filepath.Join(channelDir, "site"), false),
			})
		} else {
			html = GenerateGallery(galleryTitle, items, GalleryOptions{ZipFilename: zipName, DateStr: dateStr, Unlisted: albumUnlisted})
		}
		indexPath := filepath.Join(outDir, "index.html")
		if err := os.WriteFile(indexPath, html, 0o644); err != nil {
			emit(map[string]any{"error": "write gallery: " + err.Error()})
			return
		}
		if galleryMode {
			if err := writeGalleryAssets(outDir); err != nil {
				emit(map[string]any{"error": "write gallery assets: " + err.Error()})
				return
			}
		}

		// Write gallery.json statefile for single-gallery mode.
		if galleryMode && !siteMode {
			gsPublishedAt := publishedAt
			var gsUpdatedAt time.Time
			if !existingPublishedAt.IsZero() {
				gsPublishedAt = existingPublishedAt
				gsUpdatedAt = publishedAt
			}
			sitePhotos := make([]SitePhoto, len(items))
			for i, item := range items {
				sitePhotos[i] = SitePhoto{PhotoID: item.PhotoID, Filename: item.Filename, ThumbFilename: item.ThumbFilename}
			}
			// The build time is now; the deploy time belongs to the last
			// upload and is carried over, so a rebuild without an upload
			// leaves the gallery visibly "built, not uploaded" (ADR-0029).
			var gsDeployedAt time.Time
			if existing, _ := loadGalleryState(filepath.Join(outDir, "gallery.json")); existing != nil {
				gsDeployedAt = existing.DeployedAt
			}
			gs := &GalleryState{
				PostID:      albumPostID,
				Title:       galleryTitle,
				PublishedAt: gsPublishedAt,
				UpdatedAt:   gsUpdatedAt,
				PhotoCount:  len(items),
				GeneratedAt: time.Now().UTC(),
				DeployedAt:  gsDeployedAt,
				HasZip:      zipName != "",
				Unlisted:    albumUnlisted,
				Photos:      sitePhotos,
			}
			saveGalleryState(filepath.Join(outDir, "gallery.json"), gs) //nolint:errcheck
		}

		if siteMode && len(items) > 0 {
			emit(map[string]any{"step": "site", "done": 0, "total": 1, "file": "Updating site index…"})
			siteDir := filepath.Join(channelDir, "site")
			// Only update cover on initial build (new album).
			if !addToExisting {
				if cover, rdErr := os.ReadFile(filepath.Join(outDir, items[0].ThumbFilename)); rdErr == nil {
					os.WriteFile(filepath.Join(outDir, "cover.jpg"), cover, 0o644) //nolint:errcheck
				}
			}
			if assetsErr := writeSiteAssets(filepath.Join(siteDir, "assets")); assetsErr != nil {
				emit(map[string]any{"error": "write site assets: " + assetsErr.Error()})
				return
			}
			sites := newSiteStore(chStore, slug)
			siteAlbums, listErr := sites.List()
			if listErr != nil {
				emit(map[string]any{"error": "read site state: " + listErr.Error()})
				return
			}
			sitePhotos := make([]SitePhoto, len(items))
			for i, item := range items {
				sitePhotos[i] = SitePhoto{PhotoID: item.PhotoID, Filename: item.Filename, ThumbFilename: item.ThumbFilename}
			}
			// Only the album this build touched is written; the others belong
			// to whichever installation published them.
			var touched SiteAlbum
			if addToExisting {
				// Update existing album entry; preserve PublishedAt for sort
				// order and DeployedAt, which describes the last upload.
				if idx := indexOfAlbum(siteAlbums, albumPostID); idx >= 0 {
					touched = siteAlbums[idx]
					touched.Photos = sitePhotos
					touched.PhotoCount = len(items)
					touched.HasZip = zipName != ""
					touched.UpdatedAt = publishedAt
					touched.GeneratedAt = time.Now().UTC()
				}
			} else {
				touched = SiteAlbum{
					PostID:      albumPostID,
					Slug:        albumSlug,
					Title:       galleryTitle,
					PublishedAt: publishedAt,
					PhotoCount:  len(items),
					GeneratedAt: time.Now().UTC(),
					CoverFile:   "cover.jpg",
					HasZip:      zipName != "",
					Photos:      sitePhotos,
					Unlisted:    albumUnlisted,
				}
			}
			if touched.PostID != "" {
				if saveErr := sites.Upsert(touched); saveErr != nil {
					emit(map[string]any{"error": "save site state: " + saveErr.Error()})
					return
				}
			}
			// The index is derived from the whole register, not from the
			// albums this machine happens to know.
			siteAlbums, listErr = sites.List()
			if listErr != nil {
				emit(map[string]any{"error": "read site state: " + listErr.Error()})
				return
			}
			rootNav := buildSiteNavContext(ch, siteDir, true)
			siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, siteAlbums, rootNav)
			if writeErr := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); writeErr != nil {
				emit(map[string]any{"error": "write site index: " + writeErr.Error()})
				return
			}
			generateAboutPage(siteDir, ch, avatarExistsAt(siteDir), rootNav) //nolint:errcheck
			generateImprintPage(siteDir, ch, rootNav)                        //nolint:errcheck
			generateRobotsTxt(siteDir, ch.SiteURL)                           //nolint:errcheck
			if ch.SiteURL != "" {
				generateSitemap(siteDir, siteAlbums, ch.SiteURL) //nolint:errcheck
			}
			emit(map[string]any{"step": "site", "done": 1, "total": 1, "file": "Site index updated"})
			rememberAlbum()
			clearSucceededPhotos(results)
			emit(map[string]any{"complete": true, "postID": albumPostID, "galleryPath": outDir, "sitePath": siteDir, "results": results})
		} else {
			rememberAlbum()
			clearSucceededPhotos(results)
			emit(map[string]any{"complete": true, "postID": albumPostID, "galleryPath": outDir, "results": results})
		}
	}
}

// buildDownload exports selected library photos with channel settings and delivers
// the result as a ZIP download. By default it does not update XMP sidecars or the
// library database; pass RecordXMP=true to opt in.
func buildDownload(mgr *lib.Manager, chStore *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		id := r.PathValue("id")

		var body struct {
			PhotoIDs  []string `json:"photoIDs"`
			Channel   string   `json:"channel"`
			RecordXMP *bool    `json:"recordXMP,omitempty"` // nil → false (default for download)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if len(body.PhotoIDs) == 0 || body.Channel == "" {
			http.Error(w, "photoIDs and channel required", http.StatusBadRequest)
			return
		}

		ch, err := chStore.Get(body.Channel)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		tmpFile, err := os.CreateTemp("", "unterlumen-channel-zip-*.zip")
		if err != nil {
			http.Error(w, "create temp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		recordXMP := body.RecordXMP != nil && *body.RecordXMP
		publishedAt := time.Now().UTC()
		ts := publishedAt.Format("20060102T150405Z")
		opts := ch.ExportOptions()
		ext := "." + ch.Format
		if ch.Format == "jpeg" {
			ext = ".jpg"
		}

		var pub media.Publication
		if recordXMP {
			pub = media.Publication{
				Channel:     body.Channel,
				PostID:      newPostID(),
				PublishedAt: publishedAt,
			}
		}

		zw := zip.NewWriter(tmpFile)
		for _, photoID := range body.PhotoIDs {
			pathHint, pathErr := store.GetPhotoPathHint(photoID)
			if pathErr != nil || pathHint == "" {
				continue
			}
			if recordXMP {
				media.AppendPublication(pathHint, pub) //nolint:errcheck
				chKey := "built:" + pub.Channel
				qualKey := chKey + ":" + pub.PostID
				tsVal := publishedAt.Format(time.RFC3339)
				store.UpsertMeta(photoID, chKey, tsVal)   //nolint:errcheck
				store.UpsertMeta(photoID, qualKey, tsVal) //nolint:errcheck
				if pub.PostID != "" {
					store.UpsertMeta(photoID, chKey+":postid", pub.PostID) //nolint:errcheck
				}
				if pub.GalleryTitle != "" {
					store.UpsertMeta(photoID, chKey+":title", pub.GalleryTitle)   //nolint:errcheck
					store.UpsertMeta(photoID, qualKey+":title", pub.GalleryTitle) //nolint:errcheck
				}
			}
			data, expErr := media.ExportImage(pathHint, opts)
			if expErr != nil {
				continue
			}
			base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
			outName := ch.Slug + "_" + ts + "_" + base + ext
			if fw, fwErr := zw.Create(outName); fwErr == nil {
				fw.Write(data) //nolint:errcheck
			}
		}
		zw.Close()
		tmpFile.Close()

		f, err := os.Open(tmpPath)
		if err != nil {
			http.Error(w, "open temp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if info, statErr := os.Stat(tmpPath); statErr == nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		}
		fname := ch.Slug + "-export.zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fname))
		io.Copy(w, f) //nolint:errcheck
	}
}

func createGalleryZip(results []buildResult, outDir, zipName string) error {
	f, err := os.Create(filepath.Join(outDir, zipName))
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for _, res := range results {
		if res.Error != "" || res.Filename == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(outDir, res.Filename))
		if err != nil {
			continue
		}
		w, err := zw.Create(res.Filename)
		if err != nil {
			continue
		}
		w.Write(data) //nolint:errcheck
	}
	return nil
}

var galleryThumbOpts = media.ExportOptions{
	Format:   "jpeg",
	Quality:  78,
	ExifMode: "strip",
	Scale: media.ScaleOptions{
		Mode:         media.ScaleModeMaxDim,
		MaxDimension: "width",
		MaxValue:     700,
	},
}

func buildOne(store *lib.Store, ch *channels.Channel, pub media.Publication, ts, outDir, thumbDir, photoID string, recordXMP bool) buildResult {
	pathHint, err := store.GetPhotoPathHint(photoID)
	if err != nil || pathHint == "" {
		return buildResult{PhotoID: photoID, Error: "photo not found"}
	}

	if recordXMP {
		if err := media.AppendPublication(pathHint, pub); err != nil {
			return buildResult{PhotoID: photoID, Error: "xmp: " + err.Error()}
		}
		metaVal := pub.PublishedAt.UTC().Format(time.RFC3339)
		chKey := "built:" + pub.Channel
		qualKey := chKey + ":" + pub.PostID
		store.UpsertMeta(photoID, chKey, metaVal)   //nolint:errcheck
		store.UpsertMeta(photoID, qualKey, metaVal) //nolint:errcheck
		if pub.Account != "" {
			store.UpsertMeta(photoID, chKey+":account", pub.Account)   //nolint:errcheck
			store.UpsertMeta(photoID, qualKey+":account", pub.Account) //nolint:errcheck
		}
		if pub.PostID != "" {
			store.UpsertMeta(photoID, chKey+":postid", pub.PostID) //nolint:errcheck
		}
		if pub.GalleryTitle != "" {
			store.UpsertMeta(photoID, chKey+":title", pub.GalleryTitle)   //nolint:errcheck
			store.UpsertMeta(photoID, qualKey+":title", pub.GalleryTitle) //nolint:errcheck
		}
	}

	exported, err := media.ExportImage(pathHint, ch.ExportOptions())
	if err != nil {
		return buildResult{PhotoID: photoID, Error: "export: " + err.Error()}
	}

	ext := "." + ch.Format
	if ch.Format == "jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
	outName := ch.Slug + "_" + ts + "_" + base + ext
	outPath := filepath.Join(outDir, outName)

	if err := os.WriteFile(outPath, exported, 0o644); err != nil {
		return buildResult{PhotoID: photoID, Error: "write export: " + err.Error()}
	}

	res := buildResult{PhotoID: photoID, OutputPath: outPath, Filename: outName}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(exported)); err == nil {
		res.Width = cfg.Width
		res.Height = cfg.Height
	}

	if thumbDir != "" {
		if thumb, err := media.ExportImage(pathHint, galleryThumbOpts); err == nil {
			thumbName := "thumbs/" + outName
			if err := os.WriteFile(filepath.Join(thumbDir, outName), thumb, 0o644); err == nil {
				res.ThumbFilename = thumbName
			}
		}
	}

	return res
}

// scanAlbumPhotos reconstructs a GalleryItem list from the files on disk.
// Used when rebuilding albums that were built before photo metadata was stored in site.json.
func scanAlbumPhotos(albumDir string) []GalleryItem {
	entries, err := os.ReadDir(albumDir)
	if err != nil {
		return nil
	}
	skip := map[string]bool{"index.html": true, "photos.zip": true, "cover.jpg": true}
	var items []GalleryItem
	for _, e := range entries {
		if e.IsDir() || skip[e.Name()] {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}
		thumbName := "thumbs/" + e.Name()
		if _, statErr := os.Stat(filepath.Join(albumDir, thumbName)); statErr != nil {
			thumbName = e.Name() // no thumb — fall back to full-res
		}
		items = append(items, GalleryItem{Filename: e.Name(), ThumbFilename: thumbName})
	}
	return items
}

// albumTarget is where a draft's photos are written: the album's stable ID, its
// output folder, and — when appending to an album that already exists — that
// album's current state.
type albumTarget struct {
	postID              string
	slug                string // human-readable folder name; site mode only
	outDir              string
	existingPhotos      []SitePhoto
	existingTitle       string
	existingPublishedAt time.Time
	unlisted            bool
}

// resolveAlbumTarget decides whether a draft appends to an existing album or
// starts a new one, loading the existing album's state in the former case. A
// non-nil error carries the HTTP status the request should fail with.
//
// Unlisted is fixed at album creation: on add-to-existing it comes from the
// stored album, never from the draft, so appending photos can't silently
// un-hide an album whose link has already been shared.
func resolveAlbumTarget(draft *channels.Draft, channelDir string, sites *siteStore, publishedAt time.Time, galleryMode, siteMode bool) (albumTarget, int, error) {
	t := albumTarget{outDir: channelDir}

	if draft.Target.PostID == "" {
		t.postID = newPostID()
		t.unlisted = draft.Target.Unlisted
		switch {
		case galleryMode:
			t.outDir = filepath.Join(channelDir, t.postID)
		case siteMode:
			existingAlbums, _ := sites.List()
			t.slug = computeSlug(draft.Target.Title, publishedAt, existingAlbums, draft.Target.Unlisted)
			t.outDir = filepath.Join(channelDir, "site", "albums", t.slug)
		}
		return t, 0, nil
	}

	t.postID = draft.Target.PostID
	switch {
	case galleryMode:
		t.outDir = filepath.Join(channelDir, t.postID)
		gs, err := loadGalleryState(filepath.Join(t.outDir, "gallery.json"))
		if err != nil || gs == nil {
			return t, http.StatusBadRequest, fmt.Errorf("gallery not found: %s", t.postID)
		}
		t.existingPhotos, t.existingTitle, t.existingPublishedAt = gs.Photos, gs.Title, gs.PublishedAt
		t.unlisted = gs.Unlisted
	case siteMode:
		siteAlbums, err := sites.List()
		if err != nil {
			return t, http.StatusInternalServerError, fmt.Errorf("read site state: %w", err)
		}
		for i := range siteAlbums {
			if siteAlbums[i].PostID != t.postID {
				continue
			}
			t.existingPhotos, t.existingTitle = siteAlbums[i].Photos, siteAlbums[i].Title
			t.existingPublishedAt, t.unlisted = siteAlbums[i].PublishedAt, siteAlbums[i].Unlisted
			t.slug = albumFolderName(siteAlbums[i])
			break
		}
		if t.existingTitle == "" {
			return t, http.StatusBadRequest, fmt.Errorf("album not found: %s", t.postID)
		}
		t.outDir = filepath.Join(channelDir, "site", "albums", t.slug)
	}

	if _, err := os.Stat(t.outDir); os.IsNotExist(err) {
		return t, http.StatusBadRequest, fmt.Errorf("gallery folder not found: %s", t.postID)
	}
	return t, 0, nil
}
