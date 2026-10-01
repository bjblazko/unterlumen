package publish

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"huepattl.de/unterlumen/internal/api/sse"
	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/deploy"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
	"huepattl.de/unterlumen/internal/site"
)

// PublishedGallery is one published album/gallery merged across every
// channel, for the cross-channel "Published Galleries" overview.
type PublishedGallery struct {
	galleryListItem
	ChannelSlug string `json:"channelSlug"`
	ChannelName string `json:"channelName"`
	// ChannelHandler mirrors Channel.Handler ("rsync", or empty) — lets the
	// frontend offer a remote-delete option only where one is possible.
	ChannelHandler string `json:"channelHandler,omitempty"`
	// GalleryExport marks a single-gallery channel, where Unlisted is just a
	// meta tag and can still be toggled after publish (a site album's slug
	// encodes it, so there it is fixed).
	GalleryExport bool `json:"galleryExport,omitempty"`
	// URL is the resolved absolute public URL for this gallery, if computable.
	URL string `json:"url,omitempty"`
	// URLGuessed is true when URL was derived from the rsync handler's host
	// rather than the channel's explicitly configured SiteURL.
	URLGuessed bool `json:"urlGuessed,omitempty"`
	// Status summarizes this row's publish state for the frontend's Published
	// tab: "generated" (built, no pending draft), "draft" (collected photos
	// awaiting Generate, no built gallery yet), or "live-pending" (already
	// built/published, but a draft has more photos queued for it).
	Status string `json:"status"`
	// PendingCount is the number of photos queued in a not-yet-generated
	// draft for this row — the draft's own photo count for a draft-only row,
	// or the pending draft's photo count layered onto an already-generated
	// gallery. Zero when there is no pending draft.
	PendingCount int `json:"pendingCount"`
	// DraftID identifies the pending draft backing this row's status, if any
	// — lets the frontend open the Publish dialog against that exact draft.
	DraftID string `json:"draftID,omitempty"`
	// RowKey identifies this row uniquely across the whole table. Draft rows
	// have no PostID yet, so (channel, postID) collides for every draft of a
	// channel — a channel can hold several pending albums at once.
	RowKey string `json:"rowKey"`
}

// rowKeyFor builds a PublishedGallery.RowKey. Generated galleries are keyed by
// their album ID, pending drafts by their draft ID.
func rowKeyFor(channelSlug, postID, draftID string) string {
	if postID == "" {
		return channelSlug + "|draft:" + draftID
	}
	return channelSlug + "|" + postID
}

// resolveGalleryURL computes the public URL for a published gallery item, if
// one can be determined. This mirrors _deployBaseURL in channels.js — kept in
// sync deliberately rather than shared, since the reachability check (below)
// needs a URL server-side before the client has rendered anything.
func resolveGalleryURL(ch *channels.Channel, item galleryListItem) (url string, guessed bool) {
	base := ""
	isGuess := false
	if ch.SiteURL != "" {
		base = strings.TrimRight(ch.SiteURL, "/")
	} else if ch.Handler == "rsync" && ch.HandlerConfig["host"] != "" {
		base = "https://" + ch.HandlerConfig["host"]
		isGuess = true
	} else {
		return "", false
	}

	folder := item.FolderName
	if folder == "" {
		folder = item.PostID
	}
	if folder == "" {
		return base, isGuess
	}
	if ch.SiteExport {
		// A website keeps its album pages under albums/<slug>/.
		return base + "/albums/" + folder + "/", isGuess
	}
	return base + "/" + folder + "/", isGuess
}

// listAllGalleries returns every published gallery across every channel,
// merged into one flat, newest-first list, with pending drafts layered on:
// a draft targeting an already-generated gallery's PostID marks that row
// "live-pending" and carries the draft's photo count as PendingCount; a
// draft with no matching PostID (a gallery never generated) appears as its
// own synthetic "draft" row.
func listAllGalleries(chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		chs, err := chStore.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out := []PublishedGallery{}
		for _, ch := range chs {
			// Only gallery/site-export channels have generated statefiles to
			// read — but a plain-export channel (e.g. Instagram) can still
			// have a pending draft, and must not be skipped entirely or that
			// draft would never surface anywhere as a "draft" row below.
			var items []galleryListItem
			if ch.SiteExport || ch.GalleryExport {
				it, err := collectGalleryItems(ch, chStore)
				if err != nil {
					// One channel's broken/missing statefile shouldn't take
					// down the whole overview.
					continue
				}
				items = it
			}

			drafts := channelDrafts(draftStore, ch.Slug)
			byPostID := draftsByPostID(drafts)
			matched := map[string]bool{}

			for _, it := range items {
				url, guessed := resolveGalleryURL(ch, it)
				row := PublishedGallery{
					galleryListItem: it,
					ChannelSlug:     ch.Slug,
					ChannelName:     ch.Name,
					ChannelHandler:  ch.Handler,
					GalleryExport:   ch.GalleryExport,
					URL:             url,
					URLGuessed:      guessed,
					Status:          "generated",
					RowKey:          rowKeyFor(ch.Slug, it.PostID, ""),
				}
				if ds := byPostID[it.PostID]; len(ds) > 0 {
					row.Status = "live-pending"
					row.PendingCount = len(ds[0].Photos)
					row.DraftID = ds[0].ID
					matched[ds[0].ID] = true
				}
				out = append(out, row)
			}

			// Any draft not matched to a generated row above (either
			// targeting a brand-new gallery, or a PostID that doesn't
			// currently exist) becomes its own synthetic draft row.
			for _, d := range drafts {
				if matched[d.ID] {
					continue
				}
				out = append(out, draftOnlyRow(ch, d))
			}
		}

		sort.Slice(out, func(i, j int) bool {
			return out[i].PublishedAt.After(out[j].PublishedAt)
		})
		writeJSON(w, out)
	}
}

// channelDrafts returns the pending drafts for one channel, tolerating a nil
// draftStore (e.g. in callers/tests that don't wire one up) or a read error
// by treating either as "no drafts" — consistent with how collectGalleryItems
// errors are handled just above, one channel's problem shouldn't sink the
// whole overview.
func channelDrafts(draftStore *channels.DraftStore, slug string) []*channels.Draft {
	if draftStore == nil {
		return nil
	}
	drafts, err := draftStore.List(slug)
	if err != nil {
		return nil
	}
	return drafts
}

// draftsByPostID indexes drafts that target an existing gallery/album by
// PostID. Drafts targeting a brand-new gallery (Target.PostID empty) are
// omitted — those become synthetic draft-only rows instead. A PostID can hold
// several drafts; only the first layers onto the generated row, the rest get
// their own draft rows rather than disappearing.
func draftsByPostID(drafts []*channels.Draft) map[string][]*channels.Draft {
	out := map[string][]*channels.Draft{}
	for _, d := range drafts {
		if d.Target.PostID != "" {
			out[d.Target.PostID] = append(out[d.Target.PostID], d)
		}
	}
	return out
}

// draftOnlyRow builds a synthetic PublishedGallery row for a draft that has
// no corresponding generated gallery/album yet. Title falls back to the
// channel's own name when the draft has none — always the case for
// plain-export channels (e.g. Instagram-style), whose collect dialog never
// shows a title field.
func draftOnlyRow(ch *channels.Channel, d *channels.Draft) PublishedGallery {
	title := d.Target.Title
	if title == "" {
		title = ch.Name
	}
	// The album folder doesn't exist yet, so this resolves to the channel's
	// base address only — enough to show where the gallery is headed instead
	// of a bare "No URL configured".
	url, guessed := resolveGalleryURL(ch, galleryListItem{})
	return PublishedGallery{
		galleryListItem: galleryListItem{
			Title:      title,
			PhotoCount: len(d.Photos),
			Unlisted:   d.Target.Unlisted,
		},
		ChannelSlug:    ch.Slug,
		ChannelName:    ch.Name,
		ChannelHandler: ch.Handler,
		GalleryExport:  ch.GalleryExport,
		URL:            url,
		URLGuessed:     guessed,
		Status:         "draft",
		PendingCount:   len(d.Photos),
		DraftID:        d.ID,
		RowKey:         rowKeyFor(ch.Slug, "", d.ID),
	}
}

const (
	reachabilityMaxConcurrency = 8
	reachabilityTimeout        = 5 * time.Second
)

type reachabilityTarget struct {
	ChannelSlug string `json:"channelSlug"`
	PostID      string `json:"postID"`
	// RowKey is echoed back untouched so the client can route each result to
	// the exact table row it came from.
	RowKey string `json:"rowKey"`
	URL    string `json:"url"`
}

type reachabilityResult struct {
	ChannelSlug string `json:"channelSlug,omitempty"`
	PostID      string `json:"postID,omitempty"`
	RowKey      string `json:"rowKey,omitempty"`
	Reachable   bool   `json:"reachable,omitempty"`
	Error       string `json:"error,omitempty"`
	Complete    bool   `json:"complete,omitempty"`
}

// checkGalleryReachability streams a live HEAD-request reachability check
// over Server-Sent Events for a client-supplied list of gallery URLs, bounded
// to reachabilityMaxConcurrency in-flight requests at a time so a large
// published-gallery collection doesn't open unbounded outbound connections.
func checkGalleryReachability() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Targets []reachabilityTarget `json:"targets"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		flusher, ok := sse.Start(w)
		if !ok {
			return
		}

		results := make(chan reachabilityResult)
		sem := make(chan struct{}, reachabilityMaxConcurrency)
		var wg sync.WaitGroup
		client := &http.Client{Timeout: reachabilityTimeout}

		// Dispatch runs in its own goroutine so it can't deadlock against the
		// receive loop below: with more targets than reachabilityMaxConcurrency,
		// filling sem here would otherwise block before anyone starts draining
		// results, and the in-flight workers would in turn block sending to the
		// unbuffered results channel.
		go func() {
			for _, t := range body.Targets {
				if t.URL == "" {
					continue
				}
				wg.Add(1)
				sem <- struct{}{}
				go func(t reachabilityTarget) {
					defer wg.Done()
					defer func() { <-sem }()
					results <- probeReachability(r.Context(), client, t)
				}(t)
			}
			wg.Wait()
			close(results)
		}()

		for res := range results {
			sse.Send(w, res) //nolint:errcheck
			flusher.Flush()
		}
		sse.Send(w, reachabilityResult{Complete: true}) //nolint:errcheck
		flusher.Flush()
	}
}

func probeReachability(parent context.Context, client *http.Client, t reachabilityTarget) reachabilityResult {
	ctx, cancel := context.WithTimeout(parent, reachabilityTimeout)
	defer cancel()

	res := reachabilityResult{ChannelSlug: t.ChannelSlug, PostID: t.PostID, RowKey: t.RowKey}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, t.URL, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	resp, err := client.Do(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	res.Reachable = resp.StatusCode < 400
	if !res.Reachable {
		res.Error = resp.Status
	}
	return res
}

// renameGallery changes the title of one already-published album/gallery and
// regenerates its HTML (and, for site-export, the site index/sitemap, since
// those also list every album's title). Only the title can be changed —
// Unlisted is immutable after creation by design (see site.Album.Unlisted):
// changing it would change the slug/URL and break any link already shared.
func renameGallery(chStore *channels.Store, mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		postID := r.PathValue("postID")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusNotFound)
			return
		}
		title, unlisted, status, err := readRenameBody(r, ch)
		if err == nil {
			switch {
			case ch.SiteExport:
				status, err = renameSiteAlbum(chStore, mgr, ch, postID, title)
			case ch.GalleryExport:
				status, err = renameGalleryFolder(chStore, slug, postID, title, unlisted)
			default:
				status, err = http.StatusBadRequest, errors.New("channel is not configured for gallery or site export")
			}
		}
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, map[string]any{"success": true, "title": title})
	}
}

// readRenameBody reads the new title and, optionally, the new listedness.
func readRenameBody(r *http.Request, ch *channels.Channel) (title string, unlisted *bool, status int, err error) {
	var body struct {
		Title    string `json:"title"`
		Unlisted *bool  `json:"unlisted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", nil, http.StatusBadRequest, errors.New("invalid JSON")
	}
	title = strings.TrimSpace(body.Title)
	if title == "" {
		return "", nil, http.StatusBadRequest, errors.New("title must not be empty")
	}
	// A site album's listedness is baked into its slug, so flipping it
	// would change the URL of a link that may already be shared. A
	// gallery-export album's folder is the random PostID either way, so
	// there the flag is just a meta tag and safe to change.
	if body.Unlisted != nil && !ch.GalleryExport {
		return "", nil, http.StatusBadRequest, errors.New("unlisted cannot be changed after publish for site albums")
	}
	return title, body.Unlisted, 0, nil
}

// renameSiteAlbum retitles a site album in the register and regenerates the site.
func renameSiteAlbum(chStore *channels.Store, mgr *lib.Manager, ch *channels.Channel, postID, title string) (int, error) {
	sites := site.NewStore(chStore, ch.Slug)
	albums, err := sites.List()
	if err != nil {
		return http.StatusInternalServerError, errors.New("read site state: " + err.Error())
	}
	idx := indexOfAlbum(albums, postID)
	if idx == -1 {
		return http.StatusNotFound, errors.New("gallery not found")
	}
	renamed := albums[idx]
	renamed.Title = title
	if err := sites.Upsert(renamed); err != nil {
		return http.StatusInternalServerError, errors.New("save site state: " + err.Error())
	}
	if _, _, err := rebuildSiteChannel(chStore, mgr, ch); err != nil {
		return http.StatusInternalServerError, errors.New("regenerate site: " + err.Error())
	}
	return 0, nil
}

// renameGalleryFolder retitles a single gallery, sets its listedness when
// given, and regenerates its page.
func renameGalleryFolder(chStore *channels.Store, slug, postID, title string, unlisted *bool) (int, error) {
	outDir, ok := pathguard.SafePath(chStore.OutputDir(slug), postID)
	if !ok {
		return http.StatusBadRequest, errors.New("invalid gallery id")
	}
	statePath := filepath.Join(outDir, "gallery.json")
	gs, err := site.LoadGalleryState(statePath)
	if err != nil || gs == nil {
		return http.StatusNotFound, errors.New("gallery not found")
	}
	gs.Title = title
	if unlisted != nil {
		gs.Unlisted = *unlisted
	}
	if err := site.SaveGalleryState(statePath, gs); err != nil {
		return http.StatusInternalServerError, errors.New("save gallery state: " + err.Error())
	}
	if err := regenerateGalleryFolder(outDir, gs); err != nil {
		return http.StatusInternalServerError, errors.New("regenerate gallery: " + err.Error())
	}
	return 0, nil
}

func indexOfAlbum(albums []site.Album, postID string) int {
	for i := range albums {
		if albums[i].PostID == postID {
			return i
		}
	}
	return -1
}

// deleteGallery removes one published album/gallery: its statefile entry and
// local output folder always; its remote copy too, best-effort, only when
// the caller opts in via {"deleteRemote": true} and the channel uses the
// rsync handler. Remote deletion runs over SSH and is entirely separate from
// deploy (which never uses rsync --delete — see rsyncArgs — so a gallery
// removed locally has never been automatically removed from the remote).
func deleteGallery(chStore *channels.Store, mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		postID := r.PathValue("postID")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusNotFound)
			return
		}

		var body struct {
			DeleteRemote bool `json:"deleteRemote"`
		}
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck // absent/empty body just means deleteRemote=false
		}

		var removed galleryRemoval
		var status int
		switch {
		case ch.SiteExport:
			removed, status, err = removeSiteAlbum(chStore, mgr, ch, postID)
		case ch.GalleryExport:
			removed, status, err = removeGalleryFolder(chStore, mgr, slug, postID)
		default:
			status, err = http.StatusBadRequest, errors.New("channel is not configured for gallery or site export")
		}
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}

		result := map[string]any{"success": true}
		if removed.regenErr != nil {
			result["regenerateError"] = removed.regenErr.Error()
		}
		if removed.sidecarsNotCleared > 0 {
			result["sidecarsNotCleared"] = removed.sidecarsNotCleared
		}
		if body.DeleteRemote {
			deleteRemoteCopy(ch, removed.remoteSubpath, result)
		}
		writeJSON(w, result)
	}
}

// galleryRemoval is what removing one album or gallery left to report.
type galleryRemoval struct {
	remoteSubpath      string // its folder below the destination's remote root
	regenErr           error  // the site could not be regenerated afterwards
	sidecarsNotCleared int    // photos this installation could not reach
}

// removeSiteAlbum takes an album off a site: out of its photos, into the
// register as deleted, its folder gone, the site regenerated.
func removeSiteAlbum(chStore *channels.Store, mgr *lib.Manager, ch *channels.Channel, postID string) (galleryRemoval, int, error) {
	siteDir := filepath.Join(chStore.OutputDir(ch.Slug), "site")
	sites := site.NewStore(chStore, ch.Slug)
	albums, err := sites.List()
	if err != nil {
		return galleryRemoval{}, http.StatusInternalServerError, errors.New("read site state: " + err.Error())
	}
	idx := indexOfAlbum(albums, postID)
	if idx == -1 {
		return galleryRemoval{}, http.StatusNotFound, errors.New("gallery not found")
	}
	folder := site.AlbumFolderName(albums[idx])
	localDir, ok := pathguard.SafePath(siteDir, filepath.Join("albums", folder))
	if !ok {
		return galleryRemoval{}, http.StatusBadRequest, errors.New("invalid gallery path")
	}
	removed := galleryRemoval{remoteSubpath: "albums/" + folder}

	// Take the album out of its photos first, then leave the tombstone
	// for the photos this installation cannot reach.
	removed.sidecarsNotCleared = forgetAlbumInPhotos(mgr, ch.Slug, postID, albums[idx].Photos)
	if err := sites.Delete(postID); err != nil {
		return galleryRemoval{}, http.StatusInternalServerError, errors.New("save site state: " + err.Error())
	}
	os.RemoveAll(localDir) //nolint:errcheck
	if _, _, err := rebuildSiteChannel(chStore, mgr, ch); err != nil {
		removed.regenErr = err
	}
	return removed, 0, nil
}

// removeGalleryFolder takes a single gallery out of its photos and removes
// its folder.
func removeGalleryFolder(chStore *channels.Store, mgr *lib.Manager, slug, postID string) (galleryRemoval, int, error) {
	localDir, ok := pathguard.SafePath(chStore.OutputDir(slug), postID)
	if !ok {
		return galleryRemoval{}, http.StatusBadRequest, errors.New("invalid gallery id")
	}
	if _, err := os.Stat(filepath.Join(localDir, "gallery.json")); err != nil {
		return galleryRemoval{}, http.StatusNotFound, errors.New("gallery not found")
	}
	removed := galleryRemoval{remoteSubpath: postID}
	// Take the gallery out of its photos before its state goes.
	if gs, _ := site.LoadGalleryState(filepath.Join(localDir, "gallery.json")); gs != nil {
		removed.sidecarsNotCleared = forgetAlbumInPhotos(mgr, slug, postID, gs.Photos)
	}
	os.RemoveAll(localDir) //nolint:errcheck
	return removed, 0, nil
}

// deleteRemoteCopy removes the gallery from an rsync destination's remote,
// best-effort, and records the outcome in result.
func deleteRemoteCopy(ch *channels.Channel, remoteSubpath string, result map[string]any) {
	if ch.Handler != "rsync" {
		result["remoteDeleteError"] = "channel does not use the rsync handler"
		return
	}
	target, err := deploy.TargetFromConfig(ch.HandlerConfig)
	if err != nil {
		result["remoteDeleteError"] = err.Error()
		return
	}
	if out, err := deploy.DeleteRemote(target, remoteSubpath); err != nil {
		result["remoteDeleteError"] = err.Error() + "\n" + out
		return
	}
	result["remoteDeleted"] = true
}

// forgetAlbumInPhotos takes one album out of its photos: its record leaves the
// photo's sidecar and its keys leave the library, so nothing on the photo
// claims it is published there any more (and a rebuild cannot find it). The
// channel marker goes with the photo's last album of that destination. A
// photo has the same id in every library that holds it, and each library has
// its own keys, so every one of them is cleared; the sidecar is one file and
// is cleared once. It returns how many photos it could not reach from this
// installation — in no library here, not mounted, or an entry without a
// photo ID.
func forgetAlbumInPhotos(mgr *lib.Manager, channelSlug, postID string, photos []site.Photo) (notCleared int) {
	f := albumForgetting{channel: channelSlug, postID: postID, reached: map[string]bool{}}
	var ids []string
	for _, sp := range photos {
		if sp.PhotoID == "" {
			notCleared++
		} else {
			ids = append(ids, sp.PhotoID)
		}
	}
	if mgr != nil {
		libs, _ := mgr.ListLibraries()
		for _, l := range libs {
			if store, err := mgr.OpenStore(l.ID); err == nil {
				f.inLibrary(store, ids)
				store.Close()
			}
		}
	}
	for _, id := range ids {
		if !f.reached[id] {
			notCleared++
		}
	}
	return notCleared + f.sidecarsFailed
}

// albumForgetting is one album being taken out of its photos, library by library.
type albumForgetting struct {
	channel, postID string
	reached         map[string]bool // photos found in a library and on disk; their sidecar is done
	sidecarsFailed  int
}

func (f *albumForgetting) inLibrary(store *lib.Store, ids []string) {
	for _, id := range ids {
		hint, err := store.GetPhotoPathHint(id)
		if err != nil || hint == "" {
			continue
		}
		if _, err := os.Stat(hint); err != nil {
			continue // in this library, but not reachable right now
		}
		if !f.reached[id] {
			f.reached[id] = true
			if media.RemovePublication(hint, f.channel, f.postID) != nil {
				f.sidecarsFailed++
			}
		}
		deleteAlbumKeys(store, id, f.channel, f.postID)
		if entries, err := store.GetMeta(id); err == nil && len(albumPostIDsForChannel(entries, f.channel)) == 0 {
			deleteChannelPublicationKeys(store, id, f.channel)
		}
	}
}
