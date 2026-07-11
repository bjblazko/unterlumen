package apilibrary

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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
