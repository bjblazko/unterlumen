package apilibrary

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSidecarSharedWithARawBeside(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a [1].jpg", "a [1].raf", "a [1].xmp", "b.jpg", "b.xmp"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !sidecarShared(filepath.Join(dir, "a [1].jpg")) {
		t.Error("a [1].jpg shares its sidecar with a [1].raf")
	}
	if sidecarShared(filepath.Join(dir, "b.jpg")) {
		t.Error("b.jpg has its sidecar to itself")
	}
}

// deleteFileRequest asks to delete path from library id.
func deleteFileRequest(id, path string) *http.Request {
	req := httptest.NewRequest("DELETE", "/api/library/"+id+"/file?path="+url.QueryEscape(path), nil)
	req.SetPathValue("id", id)
	return req
}

func TestDeleteLibraryFileOnlyAFileTheLibraryHolds(t *testing.T) {
	mgr := newTestManager(t)
	source := t.TempDir()
	l, err := mgr.CreateLibrary("Test", "", source)
	if err != nil {
		t.Fatal(err)
	}
	store, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	held := filepath.Join(source, "held.jpg")
	other := filepath.Join(t.TempDir(), "other.jpg")
	for _, p := range []string{held, other} {
		if err := os.WriteFile(p, []byte("jpeg"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertPhoto("held", held, "held.jpg", 4, time.Now(), "{}", "", "", "jpg"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPathCache(held, "held", 1, 4); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	deleteLibraryFile(mgr)(rec, deleteFileRequest(l.ID, other))
	if rec.Code != http.StatusNotFound {
		t.Errorf("a file outside the library: status %d, want 404", rec.Code)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("a file outside the library was deleted: %v", err)
	}

	rec = httptest.NewRecorder()
	deleteLibraryFile(mgr)(rec, deleteFileRequest(l.ID, held))
	if rec.Code != http.StatusOK {
		t.Errorf("the library's own file: status %d, want 200", rec.Code)
	}
	if _, err := os.Stat(held); !os.IsNotExist(err) {
		t.Errorf("the library's own file is still there: %v", err)
	}
}
