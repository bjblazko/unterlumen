package publish

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"huepattl.de/unterlumen/internal/api/sse"
	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/site"
)

// generateDraft publishes a draft's photos into its destination: plain files
// synchronously, or — for gallery and site destinations — a gallery page, ZIP
// and site index, streaming progress as SSE.
func generateDraft(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil || draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		run, status, err := newPublishRun(r, mgr, chStore, draftStore)
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}
		defer run.closeStores()

		if err := os.MkdirAll(run.target.outDir, 0o700); err != nil {
			http.Error(w, "create output dir: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if !run.galleryMode && !run.siteMode {
			run.publishFiles(w)
			return
		}
		run.publishGallery(w, mgr)
	}
}

// publishRun is one publish of one draft into one album.
type publishRun struct {
	chStore    *channels.Store
	draftStore *channels.DraftStore
	ch         *channels.Channel
	slug       string
	draftID    string
	draft      *channels.Draft
	// One store per distinct library referenced by the draft — a draft's
	// photos can span multiple libraries, collected across separate sessions
	// (ADR-0016: channel output is library-independent).
	stores        map[string]*lib.Store
	publishedAt   time.Time
	ts            string
	addToExisting bool
	galleryMode   bool
	siteMode      bool
	channelDir    string
	target        albumTarget
	pub           media.Publication
}

// newPublishRun resolves the channel, draft, libraries and target album of a
// publish request. A non-nil error carries the HTTP status to fail with.
func newPublishRun(r *http.Request, mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) (*publishRun, int, error) {
	slug := r.PathValue("slug")
	draftID := r.PathValue("draftID")
	publishedAt := requestPublishedAt(r)

	ch, err := chStore.Get(slug)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("channel not found: " + err.Error())
	}
	draft, err := loadPublishDraft(r, draftStore, slug, draftID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	if draft.Target.Account != "" && ch.AccountByID(draft.Target.Account) == nil {
		return nil, http.StatusBadRequest, errors.New("account not found: " + draft.Target.Account)
	}
	stores, err := openDraftStores(mgr, draft.Photos)
	if err != nil {
		return nil, http.StatusNotFound, err
	}

	run := &publishRun{
		chStore: chStore, draftStore: draftStore, ch: ch,
		slug: slug, draftID: draftID, draft: draft, stores: stores,
		publishedAt:   publishedAt,
		ts:            publishedAt.UTC().Format("20060102T150405Z"),
		addToExisting: draft.Target.PostID != "",
		channelDir:    chStore.OutputDir(slug),
	}
	namesAlbum := draft.Target.Title != "" || run.addToExisting
	run.galleryMode = ch.GalleryExport && namesAlbum
	run.siteMode = ch.SiteExport && namesAlbum

	target, status, err := resolveAlbumTarget(draft, run.channelDir, site.NewStore(chStore, slug), publishedAt, run.galleryMode, run.siteMode)
	if err != nil {
		run.closeStores()
		return nil, status, err
	}
	if run.addToExisting && !run.galleryMode && !run.siteMode {
		// A plain export keeps no list of its posts; the libraries know the
		// post's title and first date from its photos' records.
		if post, ok := mgr.PublishedPost(slug, target.postID); ok {
			target.existingTitle, target.existingPublishedAt = post.Title, post.PublishedAt
		}
	}
	run.target = target
	run.pub = run.publication()
	return run, 0, nil
}

// requestPublishedAt reads the optional publishedAt from the request body; an
// empty or unparsable body means now.
func requestPublishedAt(r *http.Request) time.Time {
	var body struct {
		PublishedAt string `json:"publishedAt,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck // empty body is valid; PublishedAt defaults below
	if body.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, body.PublishedAt); err == nil {
			return t
		}
	}
	return time.Now().UTC()
}

// loadPublishDraft returns the draft to publish. draftID == "-" is the
// sentinel for "regenerate this gallery/album with no newly collected photos"
// — used by the Publish dialog for a Live gallery that has no pending draft at
// all. It subsumes the old Rebuild/Rebuild-site actions: same code path, just
// with an empty Photos list, driven by a synthetic in-memory draft instead of
// one loaded from drafts.json.
func loadPublishDraft(r *http.Request, draftStore *channels.DraftStore, slug, draftID string) (*channels.Draft, error) {
	if draftID != "-" {
		return draftStore.Get(slug, draftID)
	}
	postID := r.URL.Query().Get("postID")
	if postID == "" {
		return nil, errors.New("postID query parameter required when regenerating without a draft")
	}
	return &channels.Draft{Target: channels.DraftTarget{PostID: postID}}, nil
}

// openDraftStores opens one store per distinct library of the draft's photos.
func openDraftStores(mgr *lib.Manager, photos []channels.DraftPhoto) (map[string]*lib.Store, error) {
	stores := map[string]*lib.Store{}
	for _, dp := range photos {
		if _, ok := stores[dp.LibraryID]; ok {
			continue
		}
		s, err := mgr.OpenStore(dp.LibraryID)
		if err != nil {
			closeStores(stores)
			return nil, errors.New("library not found: " + dp.LibraryID)
		}
		stores[dp.LibraryID] = s
	}
	return stores, nil
}

func closeStores(stores map[string]*lib.Store) {
	for _, s := range stores {
		s.Close()
	}
}

func (p *publishRun) closeStores() { closeStores(p.stores) }

// publication is what each photo's sidecar records. PostID must name the album
// the photos actually land in. On add-to-existing that is the target album —
// minting a fresh ID here would write XMP sidecars and built: meta pointing at
// a gallery that is never created.
func (p *publishRun) publication() media.Publication {
	pub := media.Publication{
		Channel: p.slug, Account: p.draft.Target.Account, PostID: p.target.postID,
		GalleryTitle: p.draft.Target.Title, PublishedAt: p.publishedAt,
	}
	if p.siteMode {
		// The slug is the album's URL and cannot be derived again, so
		// each photo's sidecar carries it (with the flag it encodes).
		pub.Slug, pub.Unlisted = p.target.slug, p.target.unlisted
	}
	// When adding to an existing album, ensure the album title is recorded in meta
	// for each newly built photo so they appear in album-based library searches.
	if p.target.existingTitle != "" && pub.GalleryTitle == "" {
		pub.GalleryTitle = p.target.existingTitle
	}
	return pub
}

// albumTitle is the existing album's title when adding to one, otherwise the
// draft's.
func (p *publishRun) albumTitle() string {
	if p.target.existingTitle != "" {
		return p.target.existingTitle
	}
	return p.draft.Target.Title
}

// albumDates keeps an existing album's first publish date and records this run
// as its update; a new album is published now and never updated.
func (p *publishRun) albumDates() (publishedAt, updatedAt time.Time) {
	if p.target.existingPublishedAt.IsZero() {
		return p.publishedAt, time.Time{}
	}
	return p.target.existingPublishedAt, p.publishedAt
}

// buildPhotos exports every draft photo, one buildResult per draft.Photos
// entry in the same order, calling each after every photo.
func (p *publishRun) buildPhotos(thumbDir string, each func(i int, res buildResult)) []buildResult {
	var results []buildResult
	for i, dp := range p.draft.Photos {
		res := buildOne(p.stores[dp.LibraryID], p.ch, p.pub, p.ts, p.target.outDir, thumbDir, dp.PhotoID, true)
		results = append(results, res)
		each(i, res)
	}
	return results
}

// rememberAlbum pins the draft to the album its photos just landed in.
// Photos that failed to export stay in the draft; without this, the
// retry would mint a fresh postID and build a second album with the
// same title instead of completing the first one.
func (p *publishRun) rememberAlbum() {
	if p.addToExisting || p.draftID == "-" {
		return
	}
	p.draftStore.SetTargetPostID(p.slug, p.draftID, p.target.postID) //nolint:errcheck
}

// clearSucceededPhotos removes only the draft photos that were actually
// exported without error, leaving failed ones (missing source file,
// unreadable image, disk full, ...) pending in the draft so the user can
// see and retry them — buildOne reports failure per-photo via res.Error
// rather than aborting the whole batch, so a draft can partially succeed.
// It pairs results with draft.Photos by index: buildPhotos appends exactly
// one buildResult per draft.Photos entry, in the same order, with no
// filtering in between — so results[i] always corresponds to draft.Photos[i].
// This index pairing (rather than a map keyed by the bare content-hash
// PhotoID) is required because the same PhotoID can legitimately appear under
// two different LibraryIDs in one draft (duplicate-content imports across
// libraries): keying by PhotoID alone would let one library's success mark the
// other library's failed entry as succeeded too, silently clearing its
// pending: meta.
func (p *publishRun) clearSucceededPhotos(results []buildResult) {
	if p.draftID == "-" {
		return // synthetic draft — nothing was ever persisted
	}
	for i, dp := range p.draft.Photos {
		if i >= len(results) || results[i].Error != "" {
			continue // failed export: keep in draft and keep pending: meta
		}
		p.draftStore.RemovePhoto(p.slug, p.draftID, dp.LibraryID, dp.PhotoID) //nolint:errcheck
		if s, ok := p.stores[dp.LibraryID]; ok {
			clearPendingMarkers(s, dp.PhotoID, p.slug, p.draftID)
		}
	}
}

// publishFiles is the fast synchronous path for regular (non-gallery) builds.
func (p *publishRun) publishFiles(w http.ResponseWriter) {
	results := p.buildPhotos("", func(int, buildResult) {})
	p.clearSucceededPhotos(results)
	writeJSON(w, map[string]any{"postID": p.target.postID, "results": results})
}

// publishGallery streams SSE progress while it exports photos with thumbnails,
// then writes the ZIP, the gallery page and — for a site — the site index.
func (p *publishRun) publishGallery(w http.ResponseWriter, mgr *lib.Manager) {
	thumbDir := filepath.Join(p.target.outDir, "thumbs")
	if err := os.MkdirAll(thumbDir, 0o700); err != nil {
		http.Error(w, "create thumbs dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	flusher, ok := sse.Start(w)
	if !ok {
		return
	}

	reporter := &publishReporter{job: mgr.Jobs().Start("publish", fmt.Sprintf("Publishing %q", p.albumTitle()), "galleries")}
	// A run that returns without its last word was cut short.
	defer reporter.job.Finish(errors.New("it stopped before it finished"))
	emit := func(v map[string]any) {
		sse.Send(w, v) //nolint:errcheck
		flusher.Flush()
		reporter.report(v)
	}

	total := len(p.draft.Photos)
	results := p.buildPhotos(thumbDir, func(i int, res buildResult) {
		emit(map[string]any{"step": "photo", "done": i + 1, "total": total, "file": res.Filename})
	})
	// Existing photos first, then newly exported, each file once.
	items := mergePhotoItems(p.target.existingPhotos, results)
	zipName := p.writeZip(items, emit)
	if err := p.writeGalleryPage(items, zipName, emit); err != nil {
		emit(map[string]any{"error": err.Error()})
		return
	}
	if p.galleryMode && !p.siteMode {
		p.saveGalleryState(items, zipName)
	}

	complete := map[string]any{"complete": true, "postID": p.target.postID, "galleryPath": p.target.outDir, "results": results}
	if p.siteMode && len(items) > 0 {
		siteDir, err := p.updateSite(items, zipName, emit)
		if err != nil {
			emit(map[string]any{"error": err.Error()})
			return
		}
		emit(map[string]any{"step": "site", "done": 1, "total": 1, "file": "Site index updated"})
		complete["sitePath"] = siteDir
	}
	p.rememberAlbum()
	p.clearSucceededPhotos(results)
	emit(complete)
}

// writeZip packs the same files as the page, including the previous photos
// when adding to an existing album. It returns the ZIP's name, or "" when it
// could not be written.
func (p *publishRun) writeZip(items []site.GalleryItem, emit func(map[string]any)) string {
	zipResults := make([]buildResult, len(items))
	for i, it := range items {
		zipResults[i] = buildResult{Filename: it.Filename}
	}
	emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "Creating ZIP…"})
	zipName := "photos.zip"
	if err := createGalleryZip(zipResults, p.target.outDir, zipName); err != nil {
		emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "ZIP failed: " + err.Error()})
		return ""
	}
	emit(map[string]any{"step": "zip", "done": 1, "total": 1, "file": "ZIP ready"})
	return zipName
}

// writeGalleryPage writes the album's index.html and, for a gallery
// destination, its assets.
func (p *publishRun) writeGalleryPage(items []site.GalleryItem, zipName string, emit func(map[string]any)) error {
	emit(map[string]any{"step": "html", "done": 0, "total": 1, "file": "Generating gallery…"})
	publishedAt, updatedAt := p.albumDates()
	dateStr := site.DateRangeStr(publishedAt, updatedAt)
	var html []byte
	if p.siteMode {
		html = site.GenerateAlbum(p.albumTitle(), p.ch.SiteTheme, items, site.GalleryOptions{
			ZipFilename: zipName,
			SiteTitle:   p.ch.SiteTitle,
			DateStr:     dateStr,
			SiteURL:     p.ch.SiteURL,
			AlbumSlug:   p.target.slug,
			PublishedAt: publishedAt,
			Unlisted:    p.target.unlisted,
			Nav:         site.BuildNavContext(p.ch, filepath.Join(p.channelDir, "site"), false),
		})
	} else {
		html = site.GenerateGallery(p.albumTitle(), items, site.GalleryOptions{ZipFilename: zipName, DateStr: dateStr, Unlisted: p.target.unlisted})
	}
	if err := os.WriteFile(filepath.Join(p.target.outDir, "index.html"), html, 0o644); err != nil {
		return errors.New("write gallery: " + err.Error())
	}
	if p.galleryMode {
		if err := site.WriteGalleryAssets(p.target.outDir); err != nil {
			return errors.New("write gallery assets: " + err.Error())
		}
	}
	return nil
}

// saveGalleryState writes gallery.json for single-gallery mode.
func (p *publishRun) saveGalleryState(items []site.GalleryItem, zipName string) {
	statePath := filepath.Join(p.target.outDir, "gallery.json")
	publishedAt, updatedAt := p.albumDates()
	// The build time is now; the deploy time belongs to the last
	// upload and is carried over, so a rebuild without an upload
	// leaves the gallery visibly "built, not uploaded" (ADR-0029).
	var deployedAt time.Time
	if existing, _ := site.LoadGalleryState(statePath); existing != nil {
		deployedAt = existing.DeployedAt
	}
	site.SaveGalleryState(statePath, &site.GalleryState{ //nolint:errcheck
		PostID:      p.target.postID,
		Title:       p.albumTitle(),
		PublishedAt: publishedAt,
		UpdatedAt:   updatedAt,
		PhotoCount:  len(items),
		GeneratedAt: time.Now().UTC(),
		DeployedAt:  deployedAt,
		HasZip:      zipName != "",
		Unlisted:    p.target.unlisted,
		Photos:      sitePhotosOf(items),
	})
}

func sitePhotosOf(items []site.GalleryItem) []site.Photo {
	photos := make([]site.Photo, len(items))
	for i, item := range items {
		photos[i] = site.Photo{PhotoID: item.PhotoID, Filename: item.Filename, ThumbFilename: item.ThumbFilename}
	}
	return photos
}

// updateSite records the album in the site register and regenerates the site
// index and its fixed pages. It returns the site folder.
func (p *publishRun) updateSite(items []site.GalleryItem, zipName string, emit func(map[string]any)) (string, error) {
	emit(map[string]any{"step": "site", "done": 0, "total": 1, "file": "Updating site index…"})
	siteDir := filepath.Join(p.channelDir, "site")
	// Only update cover on initial build (new album).
	if !p.addToExisting {
		if cover, err := os.ReadFile(filepath.Join(p.target.outDir, items[0].ThumbFilename)); err == nil {
			os.WriteFile(filepath.Join(p.target.outDir, "cover.jpg"), cover, 0o644) //nolint:errcheck
		}
	}
	if err := site.WriteAssets(filepath.Join(siteDir, "assets")); err != nil {
		return "", errors.New("write site assets: " + err.Error())
	}
	sites := site.NewStore(p.chStore, p.slug)
	if err := p.upsertSiteAlbum(sites, items, zipName); err != nil {
		return "", err
	}
	// The index is derived from the whole register, not from the
	// albums this machine happens to know.
	siteAlbums, err := sites.List()
	if err != nil {
		return "", errors.New("read site state: " + err.Error())
	}
	if err := writeSitePages(p.ch, siteDir, siteAlbums); err != nil {
		return "", err
	}
	return siteDir, nil
}

// upsertSiteAlbum writes only the album this build touched; the others belong
// to whichever installation published them.
func (p *publishRun) upsertSiteAlbum(sites *site.Store, items []site.GalleryItem, zipName string) error {
	siteAlbums, err := sites.List()
	if err != nil {
		return errors.New("read site state: " + err.Error())
	}
	touched := p.touchedSiteAlbum(siteAlbums, items, zipName)
	if touched.PostID == "" {
		return nil
	}
	if err := sites.Upsert(touched); err != nil {
		return errors.New("save site state: " + err.Error())
	}
	return nil
}

// touchedSiteAlbum is the register entry for this build's album. An existing
// entry keeps PublishedAt for sort order and DeployedAt, which describes the
// last upload; an existing album missing from the register yields a zero
// entry, which is not written.
func (p *publishRun) touchedSiteAlbum(siteAlbums []site.Album, items []site.GalleryItem, zipName string) site.Album {
	if p.addToExisting {
		idx := indexOfAlbum(siteAlbums, p.target.postID)
		if idx < 0 {
			return site.Album{}
		}
		touched := siteAlbums[idx]
		touched.Photos = sitePhotosOf(items)
		touched.PhotoCount = len(items)
		touched.HasZip = zipName != ""
		touched.UpdatedAt = p.publishedAt
		touched.GeneratedAt = time.Now().UTC()
		return touched
	}
	return site.Album{
		PostID:      p.target.postID,
		Slug:        p.target.slug,
		Title:       p.albumTitle(),
		PublishedAt: p.publishedAt,
		PhotoCount:  len(items),
		GeneratedAt: time.Now().UTC(),
		CoverFile:   "cover.jpg",
		HasZip:      zipName != "",
		Photos:      sitePhotosOf(items),
		Unlisted:    p.target.unlisted,
	}
}

// mergePhotoItems is what an album shows after a publish run: the photos it
// already had, then the ones just exported, each file once. A photo that is
// added again keeps its place; one that failed to export is left out.
func mergePhotoItems(existing []site.Photo, results []buildResult) []site.GalleryItem {
	items := site.BuildGalleryItems(existing)
	have := make(map[string]bool, len(items))
	for _, it := range items {
		have[it.Filename] = true
	}
	for _, res := range results {
		if res.Error != "" || res.Filename == "" || have[res.Filename] {
			continue
		}
		have[res.Filename] = true
		items = append(items, site.GalleryItem{
			PhotoID:       res.PhotoID,
			Filename:      res.Filename,
			ThumbFilename: res.ThumbFilename,
			Width:         res.Width,
			Height:        res.Height,
		})
	}
	return items
}
