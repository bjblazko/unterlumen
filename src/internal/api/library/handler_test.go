package apilibrary

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"os"
	"path/filepath"
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
