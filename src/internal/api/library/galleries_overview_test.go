package apilibrary

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/site"
)

func TestListAllGalleriesGeneratedOnlyHasGeneratedStatus(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)

	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	outDir := filepath.Join(chStore.OutputDir("gal-ch"), "post1")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gs := &site.GalleryState{PostID: "post1", Title: "Generated Gallery", PublishedAt: time.Now(), PhotoCount: 4}
	if err := site.SaveGalleryState(filepath.Join(outDir, "gallery.json"), gs); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].Status != "generated" {
		t.Errorf("Status = %q, want %q", out[0].Status, "generated")
	}
	if out[0].PendingCount != 0 {
		t.Errorf("PendingCount = %d, want 0", out[0].PendingCount)
	}
}

func TestListAllGalleriesDraftOnlyAppearsAsSyntheticRow(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)

	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	draft, err := draftStore.Create("gal-ch", channels.DraftTarget{Title: "New Gallery"}, []channels.DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p1"},
		{LibraryID: "lib1", PhotoID: "p2"},
	})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	row := out[0]
	if row.Status != "draft" {
		t.Errorf("Status = %q, want %q", row.Status, "draft")
	}
	if row.PostID != "" {
		t.Errorf("PostID = %q, want empty for a draft-only row", row.PostID)
	}
	if row.PhotoCount != 2 {
		t.Errorf("PhotoCount = %d, want 2", row.PhotoCount)
	}
	if row.PendingCount != 2 {
		t.Errorf("PendingCount = %d, want 2", row.PendingCount)
	}
	if row.DraftID != draft.ID {
		t.Errorf("DraftID = %q, want %q", row.DraftID, draft.ID)
	}
	if row.ChannelSlug != "gal-ch" || row.ChannelName != "Gallery Channel" {
		t.Errorf("channel tagging wrong: %+v", row)
	}
	if row.Title != "New Gallery" {
		t.Errorf("Title = %q, want %q", row.Title, "New Gallery")
	}
}

func TestListAllGalleriesDraftOnlyFallsBackToChannelNameWhenTitleBlank(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)

	// Plain-export channel (no GalleryExport/SiteExport at all — e.g. the
	// real Instagram builtin): the collect dialog never shows a title field,
	// so Target.Title is always empty for these drafts. A plain channel must
	// still surface its draft as a row — it must not be skipped just because
	// it has no gallery/site export flag set (see
	// TestListAllGalleriesPlainExportChannelDraftAppearsAsRow for the
	// regression this guards against).
	ch := &channels.Channel{Slug: "insta-ch", Name: "Instagram Channel"}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := draftStore.Create("insta-ch", channels.DraftTarget{}, []channels.DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p1"},
	}); err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].Title != "Instagram Channel" {
		t.Errorf("Title = %q, want fallback to channel name %q", out[0].Title, "Instagram Channel")
	}
}

// Regression test: a channel with neither GalleryExport nor SiteExport set
// (a plain-export channel, e.g. the real Instagram builtin) was previously
// skipped entirely by an early "continue" in listAllGalleries, so a photo
// collected into it via "Add to channel..." could never be found or
// published — it never appeared in the Published tab at all, with no way to
// reach the Publish dialog for it.
func TestListAllGalleriesPlainExportChannelDraftAppearsAsRow(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)

	ch := &channels.Channel{Slug: "instagram", Name: "Instagram"}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	draft, err := draftStore.Create("instagram", channels.DraftTarget{}, []channels.DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p1"},
	})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1 — a plain-export channel's draft must not be skipped", len(out))
	}
	row := out[0]
	if row.Status != "draft" {
		t.Errorf("Status = %q, want %q", row.Status, "draft")
	}
	if row.ChannelSlug != "instagram" {
		t.Errorf("ChannelSlug = %q, want %q", row.ChannelSlug, "instagram")
	}
	if row.DraftID != draft.ID {
		t.Errorf("DraftID = %q, want %q", row.DraftID, draft.ID)
	}
	if row.PendingCount != 1 {
		t.Errorf("PendingCount = %d, want 1", row.PendingCount)
	}
}

func TestListAllGalleriesGeneratedPlusDraftOnSamePostIDMergesIntoOneRow(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)

	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	outDir := filepath.Join(chStore.OutputDir("gal-ch"), "post1")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gs := &site.GalleryState{PostID: "post1", Title: "Existing Gallery", PublishedAt: time.Now(), PhotoCount: 4}
	if err := site.SaveGalleryState(filepath.Join(outDir, "gallery.json"), gs); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}
	draft, err := draftStore.Create("gal-ch", channels.DraftTarget{PostID: "post1"}, []channels.DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p5"},
		{LibraryID: "lib1", PhotoID: "p6"},
		{LibraryID: "lib1", PhotoID: "p7"},
	})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1 merged row (not two)", len(out))
	}
	row := out[0]
	if row.PostID != "post1" {
		t.Errorf("PostID = %q, want %q", row.PostID, "post1")
	}
	if row.Status != "live-pending" {
		t.Errorf("Status = %q, want %q", row.Status, "live-pending")
	}
	if row.PendingCount != 3 {
		t.Errorf("PendingCount = %d, want 3", row.PendingCount)
	}
	if row.DraftID != draft.ID {
		t.Errorf("DraftID = %q, want %q", row.DraftID, draft.ID)
	}
	if row.PhotoCount != 4 {
		t.Errorf("PhotoCount = %d, want 4 (generated count unchanged)", row.PhotoCount)
	}
}

func TestListAllGalleriesMergesSiteAndGalleryChannels(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)

	siteCh := &channels.Channel{Slug: "site-ch", Name: "Site Channel", SiteExport: true, SiteURL: "https://example.com"}
	if err := chStore.Save(siteCh); err != nil {
		t.Fatalf("Save site channel: %v", err)
	}
	siteDir := filepath.Join(chStore.OutputDir("site-ch"), "site")
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		t.Fatalf("mkdir site dir: %v", err)
	}
	albums := []site.Album{
		{PostID: "aaa", Slug: "album-one", Title: "Album One", PublishedAt: time.Now().Add(-time.Hour), PhotoCount: 3},
		{PostID: "bbb", Slug: "album-two", Title: "Album Two", PublishedAt: time.Now(), PhotoCount: 5, Unlisted: true},
	}
	if err := site.SaveState(filepath.Join(siteDir, "site.json"), albums); err != nil {
		t.Fatalf("saveSiteState: %v", err)
	}

	galCh := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true, Handler: "rsync",
		HandlerConfig: map[string]string{"host": "gal.example.com"}}
	if err := chStore.Save(galCh); err != nil {
		t.Fatalf("Save gallery channel: %v", err)
	}
	galAlbumDir := filepath.Join(chStore.OutputDir("gal-ch"), "ccc")
	if err := os.MkdirAll(galAlbumDir, 0o755); err != nil {
		t.Fatalf("mkdir gallery album dir: %v", err)
	}
	gs := &site.GalleryState{PostID: "ccc", Title: "Solo Gallery", PublishedAt: time.Now().Add(-2 * time.Hour), PhotoCount: 1}
	if err := site.SaveGalleryState(filepath.Join(galAlbumDir, "gallery.json"), gs); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	// Newest first: album-two (now), gallery ccc (-2h) comes after album-one (-1h)... verify strict ordering.
	for i := 1; i < len(out); i++ {
		if out[i-1].PublishedAt.Before(out[i].PublishedAt) {
			t.Errorf("result not sorted newest-first at index %d", i)
		}
	}

	byPostID := map[string]PublishedGallery{}
	for _, g := range out {
		byPostID[g.PostID] = g
	}

	one := byPostID["aaa"]
	if one.ChannelSlug != "site-ch" || one.ChannelName != "Site Channel" {
		t.Errorf("album-one channel tagging wrong: %+v", one)
	}
	if one.URL != "https://example.com/albums/album-one/" {
		t.Errorf("album-one URL = %q, want https://example.com/albums/album-one/", one.URL)
	}
	if one.URLGuessed {
		t.Error("album-one URL should not be marked as guessed (SiteURL is set)")
	}

	solo := byPostID["ccc"]
	if solo.ChannelSlug != "gal-ch" {
		t.Errorf("solo gallery channel tagging wrong: %+v", solo)
	}
	if solo.URL != "https://gal.example.com/ccc/" {
		t.Errorf("solo gallery URL = %q, want guessed rsync host URL", solo.URL)
	}
	if !solo.URLGuessed {
		t.Error("solo gallery URL should be marked as guessed (no SiteURL, rsync host only)")
	}
}

func TestListAllGalleriesSkipsNonPublishingChannels(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)
	ch := &channels.Channel{Slug: "plain", Name: "Plain Channel"}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("len(out) = %d, want 0 for a channel with neither export flag set", len(out))
	}
}

func TestListAllGalleriesToleratesOneBrokenChannel(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)

	broken := &channels.Channel{Slug: "broken", Name: "Broken", SiteExport: true}
	if err := chStore.Save(broken); err != nil {
		t.Fatalf("Save broken: %v", err)
	}
	brokenSiteDir := filepath.Join(chStore.OutputDir("broken"), "site")
	if err := os.MkdirAll(brokenSiteDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenSiteDir, "site.json"), []byte("not json"), 0o644); err != nil {
		t.Fatalf("write broken site.json: %v", err)
	}

	good := &channels.Channel{Slug: "good", Name: "Good", SiteExport: true}
	if err := chStore.Save(good); err != nil {
		t.Fatalf("Save good: %v", err)
	}
	goodSiteDir := filepath.Join(chStore.OutputDir("good"), "site")
	if err := os.MkdirAll(goodSiteDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	albums := []site.Album{{PostID: "ok1", Slug: "ok-album", Title: "OK Album", PublishedAt: time.Now(), PhotoCount: 1}}
	if err := site.SaveState(filepath.Join(goodSiteDir, "site.json"), albums); err != nil {
		t.Fatalf("saveSiteState: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200 even with one broken channel", rec.Code)
	}
	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 || out[0].PostID != "ok1" {
		t.Errorf("out = %+v, want exactly the good channel's one album", out)
	}
}

func TestResolveGalleryURLPrefersSiteURL(t *testing.T) {
	ch := &channels.Channel{SiteURL: "https://example.com/", Handler: "rsync", HandlerConfig: map[string]string{"host": "ignored.example.com"}}
	url, guessed := resolveGalleryURL(ch, galleryListItem{PostID: "p1", FolderName: "my-album"})
	if url != "https://example.com/my-album/" || guessed {
		t.Errorf("url=%q guessed=%v, want https://example.com/my-album/ guessed=false", url, guessed)
	}
}

// A website keeps its album pages under albums/<slug>/ (the site index links to
// albums/<slug>/index.html), unlike a share-links gallery, whose folder sits at
// the root. Asking for the root path always answered 404.
func TestResolveGalleryURLPutsSiteAlbumsUnderAlbums(t *testing.T) {
	ch := &channels.Channel{SiteExport: true, SiteURL: "https://example.com"}
	url, _ := resolveGalleryURL(ch, galleryListItem{PostID: "p1", FolderName: "photos-2019"})
	if url != "https://example.com/albums/photos-2019/" {
		t.Errorf("url = %q, want https://example.com/albums/photos-2019/", url)
	}
	if url, _ := resolveGalleryURL(ch, galleryListItem{PostID: "p1"}); url != "https://example.com/albums/p1/" {
		t.Errorf("an album without a slug is in the folder of its post ID: %q", url)
	}
}

func TestResolveGalleryURLFallsBackToRsyncHostGuess(t *testing.T) {
	ch := &channels.Channel{Handler: "rsync", HandlerConfig: map[string]string{"host": "photos.example.com"}}
	url, guessed := resolveGalleryURL(ch, galleryListItem{PostID: "p1"})
	if url != "https://photos.example.com/p1/" || !guessed {
		t.Errorf("url=%q guessed=%v, want https://photos.example.com/p1/ guessed=true", url, guessed)
	}
}

func TestResolveGalleryURLEmptyWhenNeitherConfigured(t *testing.T) {
	ch := &channels.Channel{}
	url, guessed := resolveGalleryURL(ch, galleryListItem{PostID: "p1"})
	if url != "" || guessed {
		t.Errorf("url=%q guessed=%v, want empty/false when no SiteURL or rsync host", url, guessed)
	}
}

func TestCheckGalleryReachabilityStreamsPerTargetResults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/missing/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"targets":[
		{"channelSlug":"c1","postID":"p1","url":"` + srv.URL + `/ok/"},
		{"channelSlug":"c2","postID":"p2","url":"` + srv.URL + `/missing/"}
	]}`
	req := httptest.NewRequest("POST", "/api/channels/galleries/reachability", strings.NewReader(body))
	rec := httptest.NewRecorder()
	checkGalleryReachability()(rec, req)

	events := parseSSEEvents(t, rec.Body.String())
	if len(events) != 3 {
		t.Fatalf("got %d SSE events, want 3 (2 results + complete)", len(events))
	}
	byPostID := map[string]reachabilityResult{}
	var sawComplete bool
	for _, e := range events {
		if e.Complete {
			sawComplete = true
			continue
		}
		byPostID[e.PostID] = e
	}
	if !sawComplete {
		t.Error("missing final complete:true sentinel event")
	}
	if r := byPostID["p1"]; !r.Reachable {
		t.Errorf("p1 (200 response) reported unreachable: %+v", r)
	}
	if r := byPostID["p2"]; r.Reachable {
		t.Errorf("p2 (404 response) reported reachable: %+v", r)
	}
}

func TestCheckGalleryReachabilityRespectsConcurrencyLimit(t *testing.T) {
	var mu sync.Mutex
	inFlight, peak := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	var sb strings.Builder
	sb.WriteString(`{"targets":[`)
	for i := 0; i < 32; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"channelSlug":"c","postID":"p` + strconv.Itoa(i) + `","url":"` + srv.URL + `/"}`)
	}
	sb.WriteString(`]}`)

	req := httptest.NewRequest("POST", "/api/channels/galleries/reachability", strings.NewReader(sb.String()))
	rec := httptest.NewRecorder()
	checkGalleryReachability()(rec, req)

	if peak > reachabilityMaxConcurrency {
		t.Errorf("peak concurrency = %d, want <= %d", peak, reachabilityMaxConcurrency)
	}
}

func TestRenameGalleryGalleryExport(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	outDir := filepath.Join(chStore.OutputDir("gal-ch"), "post123")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gs := &site.GalleryState{PostID: "post123", Title: "Old Title", PublishedAt: time.Now(), PhotoCount: 0}
	if err := site.SaveGalleryState(filepath.Join(outDir, "gallery.json"), gs); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/api/channels/gal-ch/galleries/post123", strings.NewReader(`{"title":"New Title"}`))
	req.SetPathValue("slug", "gal-ch")
	req.SetPathValue("postID", "post123")
	rec := httptest.NewRecorder()
	renameGallery(chStore, nil)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, err := site.LoadGalleryState(filepath.Join(outDir, "gallery.json"))
	if err != nil || got == nil {
		t.Fatalf("loadGalleryState: %v", err)
	}
	if got.Title != "New Title" {
		t.Errorf("Title = %q, want %q", got.Title, "New Title")
	}
	html, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil || !bytes.Contains(html, []byte("New Title")) {
		t.Errorf("regenerated index.html missing new title (err=%v)", err)
	}
}

func TestRenameGallerySiteExport(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "site-ch", Name: "Site Channel", SiteExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	siteDir := filepath.Join(chStore.OutputDir("site-ch"), "site")
	albumDir := filepath.Join(siteDir, "albums", "old-slug")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeTestJPEG(t, filepath.Join(albumDir, "p1.jpg"), 10, 10)
	albums := []site.Album{{
		PostID: "abc", Slug: "old-slug", Title: "Old Album Title", PublishedAt: time.Now(), PhotoCount: 1,
		Photos: []site.Photo{{Filename: "p1.jpg", ThumbFilename: "p1.jpg"}},
	}}
	if err := site.SaveState(filepath.Join(siteDir, "site.json"), albums); err != nil {
		t.Fatalf("saveSiteState: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/api/channels/site-ch/galleries/abc", strings.NewReader(`{"title":"New Album Title"}`))
	req.SetPathValue("slug", "site-ch")
	req.SetPathValue("postID", "abc")
	rec := httptest.NewRecorder()
	renameGallery(chStore, nil)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, err := site.LoadState(filepath.Join(siteDir, "site.json"))
	if err != nil || len(got) != 1 || got[0].Title != "New Album Title" {
		t.Fatalf("site state not updated: %+v, err=%v", got, err)
	}
	// Slug/folder must not change — renaming must never move the album or break its URL.
	if got[0].Slug != "old-slug" {
		t.Errorf("Slug changed to %q, want unchanged old-slug", got[0].Slug)
	}
	albumHTML, err := os.ReadFile(filepath.Join(albumDir, "index.html"))
	if err != nil || !bytes.Contains(albumHTML, []byte("New Album Title")) {
		t.Errorf("regenerated album index.html missing new title (err=%v)", err)
	}
	siteHTML, err := os.ReadFile(filepath.Join(siteDir, "index.html"))
	if err != nil || !bytes.Contains(siteHTML, []byte("New Album Title")) {
		t.Errorf("regenerated site index.html missing new title (err=%v)", err)
	}
}

func TestRenameGalleryNotFound(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Channel output dir exists (e.g. other galleries published) but not this postID.
	if err := os.MkdirAll(chStore.OutputDir("gal-ch"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/api/channels/gal-ch/galleries/nope", strings.NewReader(`{"title":"X"}`))
	req.SetPathValue("slug", "gal-ch")
	req.SetPathValue("postID", "nope")
	rec := httptest.NewRecorder()
	renameGallery(chStore, nil)(rec, req)

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestRenameGalleryRejectsEmptyTitle(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	outDir := filepath.Join(chStore.OutputDir("gal-ch"), "post123")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := site.SaveGalleryState(filepath.Join(outDir, "gallery.json"), &site.GalleryState{PostID: "post123", Title: "Keep Me"}); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/api/channels/gal-ch/galleries/post123", strings.NewReader(`{"title":"   "}`))
	req.SetPathValue("slug", "gal-ch")
	req.SetPathValue("postID", "post123")
	rec := httptest.NewRecorder()
	renameGallery(chStore, nil)(rec, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 for blank title", rec.Code)
	}
	got, _ := site.LoadGalleryState(filepath.Join(outDir, "gallery.json"))
	if got == nil || got.Title != "Keep Me" {
		t.Errorf("title was changed despite rejected request: %+v", got)
	}
}

func TestDeleteGalleryGalleryExportRemovesLocalFolder(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	outDir := filepath.Join(chStore.OutputDir("gal-ch"), "post123")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := site.SaveGalleryState(filepath.Join(outDir, "gallery.json"), &site.GalleryState{PostID: "post123", Title: "Bye"}); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/channels/gal-ch/galleries/post123", nil)
	req.SetPathValue("slug", "gal-ch")
	req.SetPathValue("postID", "post123")
	rec := httptest.NewRecorder()
	deleteGallery(chStore, nil)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Errorf("local gallery folder still exists after delete: err=%v", err)
	}
}

func TestDeleteGallerySiteExportRemovesAlbumAndRegeneratesIndex(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "site-ch", Name: "Site Channel", SiteExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	siteDir := filepath.Join(chStore.OutputDir("site-ch"), "site")
	keepDir := filepath.Join(siteDir, "albums", "keep-me")
	goneDir := filepath.Join(siteDir, "albums", "delete-me")
	for _, d := range []string{keepDir, goneDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
		writeTestJPEG(t, filepath.Join(d, "p1.jpg"), 10, 10)
	}
	albums := []site.Album{
		{PostID: "keep", Slug: "keep-me", Title: "Keep Album", PublishedAt: time.Now(), PhotoCount: 1,
			Photos: []site.Photo{{Filename: "p1.jpg", ThumbFilename: "p1.jpg"}}},
		{PostID: "gone", Slug: "delete-me", Title: "Delete Album", PublishedAt: time.Now(), PhotoCount: 1,
			Photos: []site.Photo{{Filename: "p1.jpg", ThumbFilename: "p1.jpg"}}},
	}
	if err := site.SaveState(filepath.Join(siteDir, "site.json"), albums); err != nil {
		t.Fatalf("saveSiteState: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/channels/site-ch/galleries/gone", nil)
	req.SetPathValue("slug", "site-ch")
	req.SetPathValue("postID", "gone")
	rec := httptest.NewRecorder()
	deleteGallery(chStore, nil)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(goneDir); !os.IsNotExist(err) {
		t.Errorf("deleted album's local folder still exists: err=%v", err)
	}
	if _, err := os.Stat(keepDir); err != nil {
		t.Errorf("unrelated album's folder was removed: err=%v", err)
	}
	remaining, err := site.LoadState(filepath.Join(siteDir, "site.json"))
	if err != nil || len(remaining) != 1 || remaining[0].PostID != "keep" {
		t.Fatalf("site state after delete = %+v, err=%v", remaining, err)
	}
	siteHTML, err := os.ReadFile(filepath.Join(siteDir, "index.html"))
	if err != nil {
		t.Fatalf("read site index: %v", err)
	}
	if bytes.Contains(siteHTML, []byte("Delete Album")) {
		t.Error("regenerated site index still references the deleted album")
	}
	if !bytes.Contains(siteHTML, []byte("Keep Album")) {
		t.Error("regenerated site index lost the surviving album")
	}
}

func TestDeleteGalleryNotFound(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.MkdirAll(chStore.OutputDir("gal-ch"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/channels/gal-ch/galleries/nope", nil)
	req.SetPathValue("slug", "gal-ch")
	req.SetPathValue("postID", "nope")
	rec := httptest.NewRecorder()
	deleteGallery(chStore, nil)(rec, req)

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteGalleryRemoteOptInOnNonRsyncChannelReportsError(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "gal-ch", Name: "Gallery Channel", GalleryExport: true} // no Handler set
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	outDir := filepath.Join(chStore.OutputDir("gal-ch"), "post123")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := site.SaveGalleryState(filepath.Join(outDir, "gallery.json"), &site.GalleryState{PostID: "post123", Title: "Bye"}); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/channels/gal-ch/galleries/post123", strings.NewReader(`{"deleteRemote":true}`))
	req.SetPathValue("slug", "gal-ch")
	req.SetPathValue("postID", "post123")
	rec := httptest.NewRecorder()
	deleteGallery(chStore, nil)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["remoteDeleteError"]; !ok {
		t.Errorf("expected remoteDeleteError for a non-rsync channel, got %+v", resp)
	}
	// Local delete must still have happened even though remote delete was declined.
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Errorf("local folder should still be removed regardless of remote-delete outcome")
	}
}

func parseSSEEvents(t *testing.T, raw string) []reachabilityResult {
	t.Helper()
	var out []reachabilityResult
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var res reachabilityResult
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &res); err != nil {
			t.Fatalf("parse SSE line %q: %v", line, err)
		}
		out = append(out, res)
	}
	return out
}

// A single-gallery channel is one host holding many unrelated albums, so two
// albums can be pending at once. Both rows must be distinguishable: they share
// an empty PostID, so a (channel, postID) key collides and the frontend would
// route every status update to whichever row happened to render first.
func TestListAllGalleriesTwoPendingDraftsGetDistinctRowKeys(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	draftStore := channels.NewDraftStore(chStore)

	if err := chStore.Save(&channels.Channel{Slug: "fotoshare", Name: "Fotoshare", GalleryExport: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	a, err := draftStore.Create("fotoshare", channels.DraftTarget{Title: "Uli"}, []channels.DraftPhoto{{LibraryID: "l", PhotoID: "p1"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	b, err := draftStore.Create("fotoshare", channels.DraftTarget{Title: "Regenwanderung"}, []channels.DraftPhoto{{LibraryID: "l", PhotoID: "p2"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2 — both pending albums must get their own row", len(out))
	}
	if out[0].RowKey == out[1].RowKey {
		t.Fatalf("both rows share RowKey %q", out[0].RowKey)
	}
	keys := map[string]bool{out[0].RowKey: true, out[1].RowKey: true}
	for _, d := range []string{a.ID, b.ID} {
		if !keys["fotoshare|draft:"+d] {
			t.Errorf("no row keyed to draft %s; got %v", d, keys)
		}
	}
}

// A second draft queued against an already-published album must not vanish:
// only the first layers onto the generated row as "live-pending".
func TestListAllGalleriesSecondDraftOnSamePostIDGetsItsOwnRow(t *testing.T) {
	base := t.TempDir()
	chStore := channels.NewStore(base, base)
	draftStore := channels.NewDraftStore(chStore)

	if err := chStore.Save(&channels.Channel{Slug: "fotoshare", Name: "Fotoshare", GalleryExport: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	outDir := filepath.Join(chStore.OutputDir("fotoshare"), "abc123")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	gs := &site.GalleryState{PostID: "abc123", Title: "Uli", PublishedAt: time.Now().UTC(), PhotoCount: 1}
	if err := site.SaveGalleryState(filepath.Join(outDir, "gallery.json"), gs); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := draftStore.Create("fotoshare", channels.DraftTarget{PostID: "abc123"}, []channels.DraftPhoto{{LibraryID: "l", PhotoID: "p" + strconv.Itoa(i)}}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/api/channels/galleries", nil)
	rec := httptest.NewRecorder()
	listAllGalleries(chStore, draftStore)(rec, req)

	var out []PublishedGallery
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2 (the live album plus the leftover draft)", len(out))
	}
	var live, draft int
	for _, r := range out {
		switch r.Status {
		case "live-pending":
			live++
		case "draft":
			draft++
		}
	}
	if live != 1 || draft != 1 {
		t.Fatalf("statuses = %d live-pending / %d draft, want 1 and 1", live, draft)
	}
}

// Toggling Unlisted is safe for a single-gallery album (its folder is the
// random PostID either way) but must stay refused for a site album, whose
// slug encodes it — changing that would break links already shared.
func TestRenameGalleryTogglesUnlistedForGalleryExportOnly(t *testing.T) {
	base := t.TempDir()
	chStore := channels.NewStore(base, base)
	if err := chStore.Save(&channels.Channel{Slug: "fotoshare", Name: "Fotoshare", GalleryExport: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	outDir := filepath.Join(chStore.OutputDir("fotoshare"), "abc123")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	statePath := filepath.Join(outDir, "gallery.json")
	if err := site.SaveGalleryState(statePath, &site.GalleryState{PostID: "abc123", Title: "Uli"}); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/api/channels/fotoshare/galleries/abc123", bytes.NewBufferString(`{"title":"Uli","unlisted":true}`))
	req.SetPathValue("slug", "fotoshare")
	req.SetPathValue("postID", "abc123")
	rec := httptest.NewRecorder()
	renameGallery(chStore, nil)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	gs, err := site.LoadGalleryState(statePath)
	if err != nil || gs == nil || !gs.Unlisted {
		t.Fatalf("Unlisted was not persisted: %+v (err %v)", gs, err)
	}
	html, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("read regenerated page: %v", err)
	}
	if !strings.Contains(string(html), `name="robots"`) {
		t.Error("regenerated page is missing the noindex tag")
	}

	if err := chStore.Save(&channels.Channel{Slug: "site-ch", Name: "Site", SiteExport: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	req = httptest.NewRequest("PATCH", "/api/channels/site-ch/galleries/abc123", bytes.NewBufferString(`{"title":"Uli","unlisted":true}`))
	req.SetPathValue("slug", "site-ch")
	req.SetPathValue("postID", "abc123")
	rec = httptest.NewRecorder()
	renameGallery(chStore, nil)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("site album: status = %d, want 400 (slug encodes listedness)", rec.Code)
	}
}

// generatedAt/deployedAt are what tell "built, not uploaded" from "online"
// (ADR-0029). A deploy stamps every gallery of the channel, because rsync
// pushes the whole output directory; a later build moves generatedAt past it
// again.
func TestMarkDeployed_StampsEveryGalleryOfTheChannel(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	channelDir := chStore.OutputDir("gal")
	for _, id := range []string{"aaa111", "bbb222"} {
		dir := filepath.Join(channelDir, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		gs := &site.GalleryState{PostID: id, Title: id, GeneratedAt: time.Now().Add(-time.Hour).UTC()}
		if err := site.SaveGalleryState(filepath.Join(dir, "gallery.json"), gs); err != nil {
			t.Fatalf("saveGalleryState: %v", err)
		}
	}

	at := time.Now().UTC()
	MarkDeployed(chStore, "gal", false, at)

	for _, id := range []string{"aaa111", "bbb222"} {
		gs, err := site.LoadGalleryState(filepath.Join(channelDir, id, "gallery.json"))
		if err != nil || gs == nil {
			t.Fatalf("loadGalleryState(%s): %v", id, err)
		}
		if gs.DeployedAt.IsZero() {
			t.Errorf("gallery %s was not stamped as deployed", id)
		}
		if gs.DeployedAt.Before(gs.GeneratedAt) {
			t.Errorf("gallery %s: deployedAt %v is before generatedAt %v — it would still read as 'built, not uploaded'",
				id, gs.DeployedAt, gs.GeneratedAt)
		}
	}
}

// A site channel deploys only its site/ subdirectory, so single-gallery
// folders sharing the same output directory must not be marked as uploaded.
func TestMarkDeployed_SiteChannelLeavesGalleryFoldersAlone(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	channelDir := chStore.OutputDir("web")
	siteDir := site.Dir(channelDir)
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := site.SaveState(filepath.Join(siteDir, "site.json"), []site.Album{{PostID: "album1", Title: "Album"}}); err != nil {
		t.Fatalf("saveSiteState: %v", err)
	}
	galDir := filepath.Join(channelDir, "ccc333")
	if err := os.MkdirAll(galDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := site.SaveGalleryState(filepath.Join(galDir, "gallery.json"), &site.GalleryState{PostID: "ccc333"}); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}

	MarkDeployed(chStore, "web", true, time.Now().UTC())

	albums, err := site.NewStore(chStore, "web").List()
	if err != nil || len(albums) != 1 {
		t.Fatalf("List: %v (%d albums)", err, len(albums))
	}
	if albums[0].DeployedAt.IsZero() {
		t.Error("the site album was not stamped as deployed")
	}
	gs, err := site.LoadGalleryState(filepath.Join(galDir, "gallery.json"))
	if err != nil || gs == nil {
		t.Fatalf("loadGalleryState: %v", err)
	}
	if !gs.DeployedAt.IsZero() {
		t.Error("a single-gallery folder was stamped although only site/ was uploaded")
	}
}
