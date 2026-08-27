package fileops

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// writeTestJPEG writes minimal-but-valid, content-distinct JPEG bytes to path
// so each fixture hashes to a different photo ID.
func writeTestJPEG(t *testing.T, path string, salt byte) {
	t.Helper()
	data := []byte{0xFF, 0xD8, 0xFF, 0xD9, salt, salt, salt}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// resolvedTempDir returns a fresh temp dir with symlinks resolved (e.g. macOS's
// /tmp -> /private/tmp), matching what pathguard.SafePath resolves internally —
// otherwise FindLibraryForPath's string-prefix match against an unresolved
// library.SourcePath silently fails in this test environment.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return resolved
}

func newTestLibrary(t *testing.T, root string) (*library.Manager, *library.Library) {
	t.Helper()
	libRoot := t.TempDir()
	mgr, err := library.NewManager(libRoot)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	lib, err := mgr.CreateLibrary("test", "", root)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	return mgr, lib
}

func waitForScanIdle(t *testing.T, mgr *library.Manager, id string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if !mgr.IsScanning(id) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan did not finish in time")
}

func doPost(t *testing.T, h http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func libPhotoStatuses(t *testing.T, mgr *library.Manager, id string) map[string]string {
	t.Helper()
	store, err := mgr.OpenStore(id)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	refs, err := store.ListAllPhotoRefs()
	if err != nil {
		t.Fatalf("ListAllPhotoRefs: %v", err)
	}
	out := make(map[string]string, len(refs))
	for _, r := range refs {
		out[r.PathHint] = "ok"
	}
	return out
}

// TestHandleDelete_SyncsLibraryIndex reproduces the bug where deleting a
// library-tracked file through the generic /api/delete endpoint left a
// stale, still-"ok" row in the library DB pointing at a file that no longer
// exists on disk (broken thumbnail, dead link) until a manual rescan.
func TestHandleDelete_SyncsLibraryIndex(t *testing.T) {
	root := resolvedTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, "folder"), 0755); err != nil {
		t.Fatal(err)
	}
	writeTestJPEG(t, filepath.Join(root, "folder", "a.jpg"), 1)

	mgr, lib := newTestLibrary(t, root)
	if _, err := mgr.OpenStore(lib.ID); err != nil {
		t.Fatal(err)
	}
	if ok := mgr.IndexFilesSync(lib.ID, []string{filepath.Join(root, "folder", "a.jpg")}); !ok {
		t.Fatal("expected initial index to report new photos")
	}

	cache := media.NewScanCache()
	rec := doPost(t, handleDelete(root, cache, mgr), fileOpRequest{Files: []string{"folder/a.jpg"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: HTTP %d: %s", rec.Code, rec.Body.String())
	}

	waitForScanIdle(t, mgr, lib.ID)

	statuses := libPhotoStatuses(t, mgr, lib.ID)
	if _, stillTracked := statuses[filepath.Join(root, "folder", "a.jpg")]; stillTracked {
		t.Error("deleted file's photo row is still present as 'ok' in the library DB — desync not fixed")
	}
}

// TestHandleRename_SyncsLibraryIndex reproduces the bug where renaming a
// library-tracked file through /api/rename left the library DB pointing at
// the old (now-nonexistent) filename.
func TestHandleRename_SyncsLibraryIndex(t *testing.T) {
	root := resolvedTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, "folder"), 0755); err != nil {
		t.Fatal(err)
	}
	writeTestJPEG(t, filepath.Join(root, "folder", "old.jpg"), 2)

	mgr, lib := newTestLibrary(t, root)
	if _, err := mgr.OpenStore(lib.ID); err != nil {
		t.Fatal(err)
	}
	if ok := mgr.IndexFilesSync(lib.ID, []string{filepath.Join(root, "folder", "old.jpg")}); !ok {
		t.Fatal("expected initial index to report new photos")
	}

	cache := media.NewScanCache()
	rec := doPost(t, handleRename(root, cache, mgr), struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}{Path: "folder/old.jpg", Name: "new.jpg"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename: HTTP %d: %s", rec.Code, rec.Body.String())
	}

	waitForScanIdle(t, mgr, lib.ID)

	statuses := libPhotoStatuses(t, mgr, lib.ID)
	if _, stale := statuses[filepath.Join(root, "folder", "old.jpg")]; stale {
		t.Error("library DB still points at the pre-rename filename — desync not fixed")
	}
	if _, fresh := statuses[filepath.Join(root, "folder", "new.jpg")]; !fresh {
		t.Error("library DB was not updated to the post-rename filename")
	}
}
