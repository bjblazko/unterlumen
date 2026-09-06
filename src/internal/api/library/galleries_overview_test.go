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
	gs := &GalleryState{PostID: "post1", Title: "Generated Gallery", PublishedAt: time.Now(), PhotoCount: 4}
	if err := saveGalleryState(filepath.Join(outDir, "gallery.json"), gs); err != nil {
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

	// Plain-export channel: the collect dialog never shows a title field, so
	// Target.Title is always empty for these drafts.
	ch := &channels.Channel{Slug: "insta-ch", Name: "Instagram Channel"}
	ch.GalleryExport = true // still needs an export flag to be listed at all
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
	gs := &GalleryState{PostID: "post1", Title: "Existing Gallery", PublishedAt: time.Now(), PhotoCount: 4}
	if err := saveGalleryState(filepath.Join(outDir, "gallery.json"), gs); err != nil {
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
	albums := []SiteAlbum{
		{PostID: "aaa", Slug: "album-one", Title: "Album One", PublishedAt: time.Now().Add(-time.Hour), PhotoCount: 3},
		{PostID: "bbb", Slug: "album-two", Title: "Album Two", PublishedAt: time.Now(), PhotoCount: 5, Unlisted: true},
	}
	if err := saveSiteState(filepath.Join(siteDir, "site.json"), albums); err != nil {
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
	gs := &GalleryState{PostID: "ccc", Title: "Solo Gallery", PublishedAt: time.Now().Add(-2 * time.Hour), PhotoCount: 1}
	if err := saveGalleryState(filepath.Join(galAlbumDir, "gallery.json"), gs); err != nil {
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
	if one.URL != "https://example.com/album-one/" {
		t.Errorf("album-one URL = %q, want https://example.com/album-one/", one.URL)
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
	albums := []SiteAlbum{{PostID: "ok1", Slug: "ok-album", Title: "OK Album", PublishedAt: time.Now(), PhotoCount: 1}}
	if err := saveSiteState(filepath.Join(goodSiteDir, "site.json"), albums); err != nil {
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
	gs := &GalleryState{PostID: "post123", Title: "Old Title", PublishedAt: time.Now(), PhotoCount: 0}
	if err := saveGalleryState(filepath.Join(outDir, "gallery.json"), gs); err != nil {
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
	got, err := loadGalleryState(filepath.Join(outDir, "gallery.json"))
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
	albums := []SiteAlbum{{
		PostID: "abc", Slug: "old-slug", Title: "Old Album Title", PublishedAt: time.Now(), PhotoCount: 1,
		Photos: []SitePhoto{{Filename: "p1.jpg", ThumbFilename: "p1.jpg"}},
	}}
	if err := saveSiteState(filepath.Join(siteDir, "site.json"), albums); err != nil {
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
	got, err := loadSiteState(filepath.Join(siteDir, "site.json"))
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
	if err := saveGalleryState(filepath.Join(outDir, "gallery.json"), &GalleryState{PostID: "post123", Title: "Keep Me"}); err != nil {
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
	got, _ := loadGalleryState(filepath.Join(outDir, "gallery.json"))
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
	if err := saveGalleryState(filepath.Join(outDir, "gallery.json"), &GalleryState{PostID: "post123", Title: "Bye"}); err != nil {
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
	albums := []SiteAlbum{
		{PostID: "keep", Slug: "keep-me", Title: "Keep Album", PublishedAt: time.Now(), PhotoCount: 1,
			Photos: []SitePhoto{{Filename: "p1.jpg", ThumbFilename: "p1.jpg"}}},
		{PostID: "gone", Slug: "delete-me", Title: "Delete Album", PublishedAt: time.Now(), PhotoCount: 1,
			Photos: []SitePhoto{{Filename: "p1.jpg", ThumbFilename: "p1.jpg"}}},
	}
	if err := saveSiteState(filepath.Join(siteDir, "site.json"), albums); err != nil {
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
	remaining, err := loadSiteState(filepath.Join(siteDir, "site.json"))
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
	if err := saveGalleryState(filepath.Join(outDir, "gallery.json"), &GalleryState{PostID: "post123", Title: "Bye"}); err != nil {
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
