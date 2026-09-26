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
