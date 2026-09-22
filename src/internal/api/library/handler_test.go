package apilibrary

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
)

func newTestManager(t *testing.T) *lib.Manager {
	t.Helper()
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

// TestDeleteLibraryPhotoStalePathHint verifies that deleting a photo whose
// path_hint no longer points at a real file fails loudly instead of quietly
// dropping the library record while the actual photo sits untouched (and
// untracked) on disk elsewhere.
func TestDeleteLibraryPhotoStalePathHint(t *testing.T) {
	mgr := newTestManager(t)
	source := t.TempDir()

	l, err := mgr.CreateLibrary("Test", "", source)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	store, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}

	// The real file lives under source/actual.jpg, but the DB thinks it's at
	// source/stale.jpg (simulating a desynced path_hint).
	realPath := filepath.Join(source, "actual.jpg")
	if err := os.WriteFile(realPath, []byte("jpeg"), 0o644); err != nil {
		t.Fatalf("write real file: %v", err)
	}
	stalePath := filepath.Join(source, "stale.jpg")
	if err := store.UpsertPhoto("photo1", stalePath, "stale.jpg", 4, time.Now(), "{}", "", "", "jpeg"); err != nil {
		t.Fatalf("UpsertPhoto: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/library/"+l.ID+"/photo/photo1", nil)
	req.SetPathValue("id", l.ID)
	req.SetPathValue("photoID", "photo1")
	rec := httptest.NewRecorder()

	deleteLibraryPhoto(mgr)(rec, req)

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if success, _ := resp["success"].(bool); success {
		t.Error("delete reported success despite the file at path_hint never existing")
	}

	// The real file must survive untouched.
	if _, err := os.Stat(realPath); err != nil {
		t.Errorf("real file was removed even though path_hint pointed elsewhere: %v", err)
	}

	// The DB record must survive so a reindex can still find/relink the photo,
	// instead of the library silently losing track of it.
	if hint, err := store.GetPhotoPathHint("photo1"); err != nil || hint == "" {
		t.Errorf("photo record was deleted from the DB despite the file op failing (hint=%q, err=%v)", hint, err)
	}
}

// TestDeleteLibraryPhotoSuccess verifies the ordinary case still works:
// when path_hint is accurate, both the file and the DB record are removed.
func TestDeleteLibraryPhotoSuccess(t *testing.T) {
	mgr := newTestManager(t)
	source := t.TempDir()

	l, err := mgr.CreateLibrary("Test", "", source)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	store, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}

	realPath := filepath.Join(source, "actual.jpg")
	if err := os.WriteFile(realPath, []byte("jpeg"), 0o644); err != nil {
		t.Fatalf("write real file: %v", err)
	}
	if err := store.UpsertPhoto("photo1", realPath, "actual.jpg", 4, time.Now(), "{}", "", "", "jpeg"); err != nil {
		t.Fatalf("UpsertPhoto: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/library/"+l.ID+"/photo/photo1", nil)
	req.SetPathValue("id", l.ID)
	req.SetPathValue("photoID", "photo1")
	rec := httptest.NewRecorder()

	deleteLibraryPhoto(mgr)(rec, req)

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if success, _ := resp["success"].(bool); !success {
		t.Errorf("delete reported failure for the ordinary case: %+v", resp)
	}
	if _, err := os.Stat(realPath); !os.IsNotExist(err) {
		t.Errorf("real file was not removed: err=%v", err)
	}
	if hint, err := store.GetPhotoPathHint("photo1"); err != nil || hint != "" {
		t.Errorf("photo record still present after successful delete (hint=%q, err=%v)", hint, err)
	}
}

// writeTestJPEG writes a minimal valid JPEG of the given dimensions to path,
// so readImageDimensions has something real to decode.
func writeTestJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write test jpeg: %v", err)
	}
}

// TestRebuildGalleriesRegeneratesFromStateWithoutDuplicating verifies that
// rebuildGalleries regenerates index.html for an existing single-gallery
// build from its stored gallery.json + the already-exported photo files,
// without touching (or duplicating) any photo files.
func TestRebuildGalleriesRegeneratesFromStateWithoutDuplicating(t *testing.T) {
	chDir := t.TempDir()
	chStore := channels.NewStore(t.TempDir(), chDir)
	ch := &channels.Channel{Slug: "test-gallery", Name: "Test Gallery", Format: "jpeg", GalleryExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save channel: %v", err)
	}

	outDir := filepath.Join(chStore.OutputDir("test-gallery"), "post123")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir outDir: %v", err)
	}
	writeTestJPEG(t, filepath.Join(outDir, "photo1.jpg"), 120, 80)

	gs := &GalleryState{
		PostID:      "post123",
		Title:       "Old Title Before Rebuild",
		PublishedAt: time.Now(),
		PhotoCount:  1,
		Photos:      []SitePhoto{{Filename: "photo1.jpg", ThumbFilename: "photo1.jpg"}},
	}
	if err := saveGalleryState(filepath.Join(outDir, "gallery.json"), gs); err != nil {
		t.Fatalf("saveGalleryState: %v", err)
	}
	// A stale index.html from an old template version, to confirm it gets overwritten.
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte("<html>stale</html>"), 0o644); err != nil {
		t.Fatalf("write stale index.html: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/test-gallery/rebuild-galleries", nil)
	req.SetPathValue("slug", "test-gallery")
	rec := httptest.NewRecorder()

	rebuildGalleries(chStore)(rec, req)

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if n, _ := resp["rebuilt"].(float64); n != 1 {
		t.Errorf("rebuilt = %v, want 1 (response: %+v)", resp["rebuilt"], resp)
	}

	html, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("read regenerated index.html: %v", err)
	}
	if bytes.Contains(html, []byte("stale")) {
		t.Error("index.html was not regenerated — still contains the stale placeholder")
	}
	if !bytes.Contains(html, []byte(gs.Title)) {
		t.Errorf("regenerated index.html missing the gallery's title %q", gs.Title)
	}
	// Width/height recovered by decoding the actual photo file on disk.
	if !bytes.Contains(html, []byte(`width="120"`)) || !bytes.Contains(html, []byte(`height="80"`)) {
		t.Error("regenerated index.html missing width/height recovered from the on-disk photo")
	}
	// The lightbox/theme script must be referenced externally, not inlined —
	// a strict CSP (script-src 'self', no 'unsafe-inline') silently blocks
	// inline <script> content, which is exactly what broke the deployed site.
	if bytes.Contains(html, []byte("function openLightbox")) {
		t.Error("index.html still inlines lightbox JS instead of referencing gallery.js")
	}
	if !bytes.Contains(html, []byte(`<script src="gallery.js"></script>`)) {
		t.Error("index.html missing external gallery.js script reference")
	}
	if !bytes.Contains(html, []byte(`<script src="theme-init.js"></script>`)) {
		t.Error("index.html missing external theme-init.js script reference")
	}
	for _, asset := range []string{"theme-init.js", "gallery.js"} {
		if _, err := os.Stat(filepath.Join(outDir, asset)); err != nil {
			t.Errorf("rebuild did not write %s: %v", asset, err)
		}
	}

	// The one photo file must be untouched — no duplication, no re-export.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read outDir: %v", err)
	}
	var jpgCount int
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".jpg" {
			jpgCount++
		}
	}
	if jpgCount != 1 {
		t.Errorf("photo count in outDir = %d, want 1 (rebuild must not duplicate or re-export photos)", jpgCount)
	}
}

// TestListGalleriesPerChannelBehaviorPreserved is a regression guard for the
// collectGalleryItems extraction: the per-channel GET .../galleries endpoint
// must return exactly what it did before the shared helper was factored out.
func TestListGalleriesPerChannelBehaviorPreserved(t *testing.T) {
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	ch := &channels.Channel{Slug: "site-ch", Name: "Site Channel", SiteExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	siteDir := filepath.Join(chStore.OutputDir("site-ch"), "site")
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		t.Fatalf("mkdir site dir: %v", err)
	}
	albums := []SiteAlbum{
		{PostID: "aaa", Slug: "album-one", Title: "Album One", PublishedAt: time.Now(), PhotoCount: 2, Unlisted: true},
	}
	if err := saveSiteState(filepath.Join(siteDir, "site.json"), albums); err != nil {
		t.Fatalf("saveSiteState: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/channels/site-ch/galleries", nil)
	req.SetPathValue("slug", "site-ch")
	rec := httptest.NewRecorder()
	listGalleries(chStore)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var items []galleryListItem
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.PostID != "aaa" || item.Title != "Album One" || item.PhotoCount != 2 || !item.Unlisted || item.FolderName != "album-one" {
		t.Errorf("item = %+v, want the album's fields preserved through collectGalleryItems", item)
	}
}

// TestRebuildGalleriesRejectsNonGalleryChannel verifies rebuildGalleries
// refuses to run against a channel that isn't configured for single-gallery
// export, rather than silently doing nothing or misinterpreting its output dir.
func TestRebuildGalleriesRejectsNonGalleryChannel(t *testing.T) {
	chDir := t.TempDir()
	chStore := channels.NewStore(t.TempDir(), chDir)
	ch := &channels.Channel{Slug: "not-gallery", Name: "Not Gallery", Format: "jpeg"}
	if err := chStore.Save(ch); err != nil {
		t.Fatalf("Save channel: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/not-gallery/rebuild-galleries", nil)
	req.SetPathValue("slug", "not-gallery")
	rec := httptest.NewRecorder()

	rebuildGalleries(chStore)(rec, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 for a non-gallery-export channel", rec.Code)
	}
}

// --- deleteMeta ---

// TestDeleteMeta_PendingKey_RemovesFromDraft verifies that deleting a
// pending:<slug> meta key doesn't just remove the meta row — it also removes
// the photo from the underlying draft (drafts.json), since pending: is a
// derived signal for "this photo is in an unfinished draft for this
// channel", not an independent fact.
func TestDeleteMeta_PendingKey_RemovesFromDraft(t *testing.T) {
	mgr := newTestManager(t)
	dir := t.TempDir()
	chStore := channels.NewStore(dir, dir)
	draftStore := channels.NewDraftStore(chStore)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/library/{id}/photo/{photoID}/meta", deleteMeta(mgr, chStore, draftStore))

	libID := seedLibraryPhoto(t, mgr, "photo1")

	draft, err := draftStore.Create("website", channels.DraftTarget{Title: "Pending Gallery"}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "photo1"}})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.UpsertMeta("photo1", "pending:website", draft.ID); err != nil {
		t.Fatalf("UpsertMeta pending: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/library/"+libID+"/photo/photo1/meta?key=pending:website", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if _, err := draftStore.Get("website", draft.ID); err == nil {
		t.Error("expected draft to be deleted since it had only one photo")
	}

	entries, err := store.GetMeta("photo1")
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	for _, e := range entries {
		if e.Key == "pending:website" {
			t.Error("expected pending:website meta to be removed")
		}
	}
}

// --- generateDraft ---

// setupGenerateTestMux mirrors setupDraftTestMux (drafts_test.go) but wires
// only the generate route, which is all these tests exercise.
func setupGenerateTestMux(t *testing.T) (*http.ServeMux, *lib.Manager, *channels.Store, *channels.DraftStore) {
	t.Helper()
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	dir := t.TempDir()
	chStore := channels.NewStore(dir, dir)
	draftStore := channels.NewDraftStore(chStore)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/channels/{slug}/drafts/{draftID}/generate", generateDraft(mgr, chStore, draftStore))
	return mux, mgr, chStore, draftStore
}

// seedLibraryPhoto creates a library with one real on-disk JPEG registered as
// a photo, so buildOne's store.GetPhotoPathHint/media.ExportImage have a real
// file to work with (mirrors writeTestJPEG + UpsertPhoto usage elsewhere in
// this file — this codebase has no photo fixtures for Go tests).
func seedLibraryPhoto(t *testing.T, mgr *lib.Manager, photoID string) (libID string) {
	t.Helper()
	source := t.TempDir()
	l, err := mgr.CreateLibrary("Test-"+photoID, "", source)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	store, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	realPath := filepath.Join(source, photoID+".jpg")
	writeTestJPEG(t, realPath, 40, 30)
	if err := store.UpsertPhoto(photoID, realPath, photoID+".jpg", 4, time.Now(), "{}", "", "", "jpeg"); err != nil {
		t.Fatalf("UpsertPhoto: %v", err)
	}
	return l.ID
}

func TestGenerateDraft_PlainChannel_ExportsAndClearsDraft(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "instagram", Name: "Instagram", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	libID := seedLibraryPhoto(t, mgr, "photo1")

	draft, err := draftStore.Create("instagram", channels.DraftTarget{}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "photo1"}})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	// Seed the pending:instagram meta key generateDraft is expected to clear —
	// draftStore.Create only writes drafts.json, it never touches library meta,
	// so without this the "cleared" assertion below would pass vacuously.
	if err := store.UpsertMeta("photo1", "pending:instagram", draft.ID); err != nil {
		t.Fatalf("UpsertMeta pending: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/instagram/drafts/"+draft.ID+"/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if _, err := draftStore.Get("instagram", draft.ID); err == nil {
		t.Fatal("expected draft to be deleted after generate")
	}

	entries, err := store.GetMeta("photo1")
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	var hasPending, hasBuilt bool
	for _, e := range entries {
		if e.Key == "pending:instagram" {
			hasPending = true
		}
		if e.Key == "built:instagram" {
			hasBuilt = true
		}
	}
	if hasPending {
		t.Error("expected pending:instagram meta to be cleared")
	}
	if !hasBuilt {
		t.Error("expected built:instagram meta to be written")
	}
}

// TestGenerateDraft_PartialFailure_KeepsFailedPhotoPending covers the bug where
// clearDraft used to run unconditionally after the export loop: buildOne reports
// per-photo failure via res.Error rather than aborting, so a draft with a mix of
// succeeded and failed photos must keep the failed one (and its pending: meta)
// so the user can see and retry it, while the succeeded photo is cleared normally.
func TestGenerateDraft_PartialFailure_KeepsFailedPhotoPending(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "instagram", Name: "Instagram", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	goodLibID := seedLibraryPhoto(t, mgr, "good-photo")

	// A library whose "missing-photo" row points at a path that doesn't exist on
	// disk — buildOne's media.ExportImage fails to open it, simulating a missing
	// source file / unreadable image without needing a real filesystem fault.
	badSource := t.TempDir()
	badLib, err := mgr.CreateLibrary("Bad-Lib", "", badSource)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	badStore, err := mgr.OpenStore(badLib.ID)
	if err != nil {
		t.Fatalf("OpenStore (bad): %v", err)
	}
	defer badStore.Close()
	missingPath := filepath.Join(badSource, "missing-photo.jpg")
	if err := badStore.UpsertPhoto("missing-photo", missingPath, "missing-photo.jpg", 0, time.Now(), "{}", "", "", "jpeg"); err != nil {
		t.Fatalf("UpsertPhoto (bad): %v", err)
	}
	goodStore, err := mgr.OpenStore(goodLibID)
	if err != nil {
		t.Fatalf("OpenStore (good): %v", err)
	}
	defer goodStore.Close()

	if err := goodStore.UpsertMeta("good-photo", "pending:instagram", "x"); err != nil {
		t.Fatalf("UpsertMeta pending (good): %v", err)
	}
	if err := badStore.UpsertMeta("missing-photo", "pending:instagram", "x"); err != nil {
		t.Fatalf("UpsertMeta pending (bad): %v", err)
	}

	draft, err := draftStore.Create("instagram", channels.DraftTarget{}, []channels.DraftPhoto{
		{LibraryID: goodLibID, PhotoID: "good-photo"},
		{LibraryID: badLib.ID, PhotoID: "missing-photo"},
	})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/instagram/drafts/"+draft.ID+"/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	remaining, err := draftStore.Get("instagram", draft.ID)
	if err != nil {
		t.Fatalf("expected draft to survive partial failure, got: %v", err)
	}
	if len(remaining.Photos) != 1 || remaining.Photos[0].PhotoID != "missing-photo" {
		t.Fatalf("expected only missing-photo to remain in draft, got %+v", remaining.Photos)
	}

	goodEntries, err := goodStore.GetMeta("good-photo")
	if err != nil {
		t.Fatalf("GetMeta (good): %v", err)
	}
	for _, e := range goodEntries {
		if e.Key == "pending:instagram" {
			t.Error("expected pending:instagram to be cleared for the succeeded photo")
		}
	}

	badEntries, err := badStore.GetMeta("missing-photo")
	if err != nil {
		t.Fatalf("GetMeta (bad): %v", err)
	}
	var stillPending bool
	for _, e := range badEntries {
		if e.Key == "pending:instagram" {
			stillPending = true
		}
	}
	if !stillPending {
		t.Error("expected pending:instagram to be kept for the failed photo")
	}
}

// TestGenerateDraft_PartialFailure_CrossLibraryPhotoIDCollision_KeepsFailedPending
// covers a narrower version of the bug above: clearSucceededPhotos used to key its
// "did this succeed" map by the bare content-hash PhotoID, ignoring LibraryID. The
// draft/meta model explicitly allows the SAME PhotoID (content hash) to appear under
// two different LibraryIDs in one draft — e.g. duplicate-content imports across
// libraries. If one library's copy succeeds and the other's fails, keying by PhotoID
// alone let the successful library's entry mark the failed library's entry as
// succeeded too, silently clearing its pending: meta and removing it from the draft
// even though nothing was exported for it.
func TestGenerateDraft_PartialFailure_CrossLibraryPhotoIDCollision_KeepsFailedPending(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "instagram", Name: "Instagram", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}

	const sharedPhotoID = "shared-photo"

	goodLibID := seedLibraryPhoto(t, mgr, sharedPhotoID)
	goodStore, err := mgr.OpenStore(goodLibID)
	if err != nil {
		t.Fatalf("OpenStore (good): %v", err)
	}
	defer goodStore.Close()

	// A second library whose row uses the SAME PhotoID but points at a path that
	// doesn't exist on disk — buildOne's media.ExportImage fails to open it.
	badSource := t.TempDir()
	badLib, err := mgr.CreateLibrary("Bad-Lib-Collision", "", badSource)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	badStore, err := mgr.OpenStore(badLib.ID)
	if err != nil {
		t.Fatalf("OpenStore (bad): %v", err)
	}
	defer badStore.Close()
	missingPath := filepath.Join(badSource, sharedPhotoID+".jpg")
	if err := badStore.UpsertPhoto(sharedPhotoID, missingPath, sharedPhotoID+".jpg", 0, time.Now(), "{}", "", "", "jpeg"); err != nil {
		t.Fatalf("UpsertPhoto (bad): %v", err)
	}

	if err := goodStore.UpsertMeta(sharedPhotoID, "pending:instagram", "x"); err != nil {
		t.Fatalf("UpsertMeta pending (good): %v", err)
	}
	if err := badStore.UpsertMeta(sharedPhotoID, "pending:instagram", "x"); err != nil {
		t.Fatalf("UpsertMeta pending (bad): %v", err)
	}

	draft, err := draftStore.Create("instagram", channels.DraftTarget{}, []channels.DraftPhoto{
		{LibraryID: goodLibID, PhotoID: sharedPhotoID},
		{LibraryID: badLib.ID, PhotoID: sharedPhotoID},
	})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/instagram/drafts/"+draft.ID+"/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	remaining, err := draftStore.Get("instagram", draft.ID)
	if err != nil {
		t.Fatalf("expected draft to survive partial failure, got: %v", err)
	}
	if len(remaining.Photos) != 1 || remaining.Photos[0].LibraryID != badLib.ID || remaining.Photos[0].PhotoID != sharedPhotoID {
		t.Fatalf("expected only the bad library's entry to remain in draft, got %+v", remaining.Photos)
	}

	goodEntries, err := goodStore.GetMeta(sharedPhotoID)
	if err != nil {
		t.Fatalf("GetMeta (good): %v", err)
	}
	for _, e := range goodEntries {
		if e.Key == "pending:instagram" {
			t.Error("expected pending:instagram to be cleared for the succeeded library's photo")
		}
	}

	badEntries, err := badStore.GetMeta(sharedPhotoID)
	if err != nil {
		t.Fatalf("GetMeta (bad): %v", err)
	}
	var stillPending bool
	for _, e := range badEntries {
		if e.Key == "pending:instagram" {
			stillPending = true
		}
	}
	if !stillPending {
		t.Error("expected pending:instagram to be kept for the failed library's photo despite the PhotoID collision")
	}
}

func TestGenerateDraft_GalleryChannel_CreatesGalleryAndClearsDraft(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	libID := seedLibraryPhoto(t, mgr, "photo1")

	draft, err := draftStore.Create("website", channels.DraftTarget{Title: "My Gallery"}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "photo1"}})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/website/drafts/"+draft.ID+"/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var complete map[string]any
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatalf("decode SSE event: %v (line=%q)", err, line)
		}
		if done, _ := ev["complete"].(bool); done {
			complete = ev
		}
	}
	if complete == nil {
		t.Fatalf("no complete:true event in SSE stream, body=%s", rec.Body.String())
	}
	galleryPath, _ := complete["galleryPath"].(string)
	if galleryPath == "" {
		t.Fatal("expected non-empty galleryPath in complete event")
	}

	if _, err := draftStore.Get("website", draft.ID); err == nil {
		t.Fatal("expected draft to be deleted after generate")
	}
	if _, err := os.Stat(filepath.Join(galleryPath, "gallery.json")); err != nil {
		t.Errorf("expected gallery.json at %s: %v", galleryPath, err)
	}
}

func TestGenerateDraft_MultiLibraryDraft_MergesAllPhotos(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	libID1 := seedLibraryPhoto(t, mgr, "photoA")
	libID2 := seedLibraryPhoto(t, mgr, "photoB")

	draft, err := draftStore.Create("website", channels.DraftTarget{Title: "Multi-Library"}, []channels.DraftPhoto{
		{LibraryID: libID1, PhotoID: "photoA"},
	})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}
	if _, err := draftStore.AppendPhotos("website", draft.ID, []channels.DraftPhoto{{LibraryID: libID2, PhotoID: "photoB"}}); err != nil {
		t.Fatalf("AppendPhotos: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/website/drafts/"+draft.ID+"/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var galleryPath string
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev) //nolint:errcheck
		if done, _ := ev["complete"].(bool); done {
			galleryPath, _ = ev["galleryPath"].(string)
		}
	}
	if galleryPath == "" {
		t.Fatalf("no complete event with galleryPath, body=%s", rec.Body.String())
	}

	gs, err := loadGalleryState(filepath.Join(galleryPath, "gallery.json"))
	if err != nil || gs == nil {
		t.Fatalf("loadGalleryState: %v", err)
	}
	if gs.PhotoCount != 2 {
		t.Errorf("PhotoCount = %d, want 2 (photos from both libraries)", gs.PhotoCount)
	}
}

func TestGenerateDraft_UnknownDraft_Returns400(t *testing.T) {
	mux, _, chStore, _ := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "instagram", Name: "Instagram", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/instagram/drafts/does-not-exist/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestGenerateDraft_SentinelRegeneratesWithoutDraft exercises the draftID=="-"
// path used by the Publish dialog to regenerate an already-Live gallery that
// has no pending draft: it must succeed using the postID query param alone.
func TestGenerateDraft_SentinelRegeneratesWithoutDraft(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	libID := seedLibraryPhoto(t, mgr, "photo1")

	// First, generate a real draft to create the gallery.
	draft, err := draftStore.Create("website", channels.DraftTarget{Title: "Existing Gallery"}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "photo1"}})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/channels/website/drafts/"+draft.ID+"/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("initial generate status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var postID string
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev) //nolint:errcheck
		if done, _ := ev["complete"].(bool); done {
			postID, _ = ev["postID"].(string)
		}
	}
	if postID == "" {
		t.Fatalf("no postID from initial generate, body=%s", rec.Body.String())
	}

	// Now regenerate with no draft at all, via the sentinel.
	req = httptest.NewRequest("POST", "/api/channels/website/drafts/-/generate?postID="+postID, strings.NewReader("{}"))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sentinel regenerate status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var complete map[string]any
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev) //nolint:errcheck
		if done, _ := ev["complete"].(bool); done {
			complete = ev
		}
	}
	if complete == nil {
		t.Fatalf("no complete event for sentinel regenerate, body=%s", rec.Body.String())
	}
	// Note: the "postID" field in the complete event is always a freshly
	// generated build ID (see media.Publication.PostID), not the album's
	// identity — that's unchanged from the pre-refactor buildPhotos behavior.
	// What must be reused is the *album*, keyed by gallery.json's own PostID.
	galleryPath, _ := complete["galleryPath"].(string)
	gs, err := loadGalleryState(filepath.Join(galleryPath, "gallery.json"))
	if err != nil || gs == nil {
		t.Fatalf("loadGalleryState after sentinel regenerate: %v", err)
	}
	if gs.PostID != postID {
		t.Errorf("gallery.json PostID = %q, want %q (should reuse existing album, not create a new one)", gs.PostID, postID)
	}
	if gs.PhotoCount != 1 {
		t.Errorf("PhotoCount = %d, want 1 (regenerate added no new photos)", gs.PhotoCount)
	}
}

// TestGenerateDraft_SentinelWithoutPostID_Returns400 verifies the sentinel
// path requires the postID query param — it has no draft to fall back on.
func TestGenerateDraft_SentinelWithoutPostID_Returns400(t *testing.T) {
	mux, _, chStore, _ := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", GalleryExport: true}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/channels/website/drafts/-/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// generateSSEComplete runs a generate request and returns its complete: event.
func generateSSEComplete(t *testing.T, mux http.Handler, slug, draftID string) map[string]any {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/channels/"+slug+"/drafts/"+draftID+"/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var complete map[string]any
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatalf("decode SSE event: %v (line=%q)", err, line)
		}
		if done, _ := ev["complete"].(bool); done {
			complete = ev
		}
	}
	if complete == nil {
		t.Fatalf("no complete:true event, body=%s", rec.Body.String())
	}
	return complete
}

func photoMeta(t *testing.T, mgr *lib.Manager, libID, photoID string) map[string]string {
	t.Helper()
	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	entries, err := store.GetMeta(photoID)
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		out[e.Key] = e.Value
	}
	return out
}

// The point of a single-gallery channel: one host, many unrelated albums. Each
// album must keep its own membership keys — the unqualified built:<slug>:title
// is overwritten by every publish, so it can only ever name the newest one.
func TestGenerateDraft_TwoAlbumsInOneChannel_KeepSeparateMeta(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "fotoshare", Name: "Fotoshare", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	libID := seedLibraryPhoto(t, mgr, "photo1")
	libID2 := seedLibraryPhoto(t, mgr, "photo2")

	first, err := draftStore.Create("fotoshare", channels.DraftTarget{Title: "Uli"}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "photo1"}})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}
	firstDone := generateSSEComplete(t, mux, "fotoshare", first.ID)
	firstPostID, _ := firstDone["postID"].(string)

	second, err := draftStore.Create("fotoshare", channels.DraftTarget{Title: "Regenwanderung"}, []channels.DraftPhoto{{LibraryID: libID2, PhotoID: "photo2"}})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}
	secondDone := generateSSEComplete(t, mux, "fotoshare", second.ID)
	secondPostID, _ := secondDone["postID"].(string)

	if firstPostID == "" || secondPostID == "" || firstPostID == secondPostID {
		t.Fatalf("expected two distinct album IDs, got %q and %q", firstPostID, secondPostID)
	}

	m1 := photoMeta(t, mgr, libID, "photo1")
	if got := m1["built:fotoshare:"+firstPostID+":title"]; got != "Uli" {
		t.Errorf("photo1 album title = %q, want %q", got, "Uli")
	}
	m2 := photoMeta(t, mgr, libID2, "photo2")
	if got := m2["built:fotoshare:"+secondPostID+":title"]; got != "Regenwanderung" {
		t.Errorf("photo2 album title = %q, want %q", got, "Regenwanderung")
	}
	// Publishing the second album must not have touched the first photo's
	// record of which album it belongs to.
	if got := m1["built:fotoshare:"+secondPostID+":title"]; got != "" {
		t.Errorf("photo1 gained the second album's key: %q", got)
	}
}

// On add-to-existing the publication must name the album the photos actually
// land in. It used to record a freshly minted ID, so the XMP sidecar and meta
// pointed at a gallery folder that was never created.
func TestGenerateDraft_AddToExisting_RecordsRealAlbumID(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "fotoshare", Name: "Fotoshare", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	libID := seedLibraryPhoto(t, mgr, "photo1")
	libID2 := seedLibraryPhoto(t, mgr, "photo2")

	first, _ := draftStore.Create("fotoshare", channels.DraftTarget{Title: "Uli"}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "photo1"}})
	albumID, _ := generateSSEComplete(t, mux, "fotoshare", first.ID)["postID"].(string)

	second, _ := draftStore.Create("fotoshare", channels.DraftTarget{PostID: albumID}, []channels.DraftPhoto{{LibraryID: libID2, PhotoID: "photo2"}})
	done := generateSSEComplete(t, mux, "fotoshare", second.ID)

	if got, _ := done["postID"].(string); got != albumID {
		t.Errorf("complete event postID = %q, want the target album %q", got, albumID)
	}
	m := photoMeta(t, mgr, libID2, "photo2")
	if _, ok := m["built:fotoshare:"+albumID]; !ok {
		t.Errorf("photo2 has no membership key for the album it was added to; meta = %v", m)
	}
	if got := m["built:fotoshare:"+albumID+":title"]; got != "Uli" {
		t.Errorf("album title = %q, want the existing album's title %q", got, "Uli")
	}
	// Exactly one album folder must exist — not a second one for the retry.
	entries, err := os.ReadDir(chStore.OutputDir("fotoshare"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	dirs := 0
	for _, e := range entries {
		if e.IsDir() {
			dirs++
		}
	}
	if dirs != 1 {
		t.Errorf("album folders = %d, want 1", dirs)
	}
}
