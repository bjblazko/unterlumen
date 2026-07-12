package batchrename

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// TestBatchRenameSyncsLibrary verifies that renaming an already-indexed
// library photo updates its database record (path_hint) in place instead of
// leaving the library pointing at the file's old, now-nonexistent name until
// a manual reindex — the file was moving correctly on disk, but the library
// had no idea a rename happened at all.
func TestBatchRenameSyncsLibrary(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks root: %v", err)
	}
	libSource := filepath.Join(root, "lib")
	if err := os.MkdirAll(libSource, 0o755); err != nil {
		t.Fatalf("mkdir libSource: %v", err)
	}

	origAbs := filepath.Join(libSource, "IMG_0001.jpg")
	if err := os.WriteFile(origAbs, []byte("not a real jpeg, just needs to hash"), 0o644); err != nil {
		t.Fatalf("write original file: %v", err)
	}

	mgr, err := library.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	lib, err := mgr.CreateLibrary("Test", "", libSource)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if !mgr.IndexFilesSync(lib.ID, []string{origAbs}) {
		t.Fatal("initial IndexFilesSync reported no update")
	}
	store, err := mgr.OpenStore(lib.ID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	origID, err := store.GetPhotoIDByPathHint(origAbs)
	if err != nil || origID == "" {
		t.Fatalf("photo not indexed at original path (id=%q, err=%v)", origID, err)
	}

	cache := media.NewScanCache()
	body, _ := json.Marshal(map[string]any{
		"files":   []string{"lib/IMG_0001.jpg"},
		"pattern": "renamed-{original}",
	})
	req := httptest.NewRequest("POST", "/api/batch-rename/execute", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleBatchRenameExecute(root, cache, mgr)(rec, req)

	var resp batchRenameExecuteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 1 || !resp.Results[0].Success {
		t.Fatalf("rename did not succeed: %+v", resp.Results)
	}
	if !resp.LibraryUpdated {
		t.Error("response did not report libraryUpdated despite renaming an already-indexed photo")
	}

	newAbs := filepath.Join(libSource, "renamed-IMG_0001.jpg")
	if _, err := os.Stat(origAbs); !os.IsNotExist(err) {
		t.Errorf("original file still exists on disk: err=%v", err)
	}
	if _, err := os.Stat(newAbs); err != nil {
		t.Errorf("renamed file missing on disk: %v", err)
	}

	if staleID, err := store.GetPhotoIDByPathHint(origAbs); err != nil || staleID != "" {
		t.Errorf("library still has a record at the old path_hint after rename (id=%q, err=%v)", staleID, err)
	}
	newID, err := store.GetPhotoIDByPathHint(newAbs)
	if err != nil || newID == "" {
		t.Fatalf("library has no record at the new path after rename (id=%q, err=%v)", newID, err)
	}
	if newID != origID {
		t.Errorf("rename created a new photo record (%q) instead of updating the existing one (%q)", newID, origID)
	}
}
