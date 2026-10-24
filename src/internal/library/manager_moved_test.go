package library

import (
	"os"
	"path/filepath"
	"testing"
)

// A rename or move keeps a file's content, so the library keeps the photo it
// knew and only its path changes — without reading the file again, which on
// a network share took seconds per photo. The renamed file is made
// unreadable to show it is not read.
func TestAMovedFileKeepsItsPhotoWithoutBeingRead(t *testing.T) {
	mgr := newTestManager(t)
	root := t.TempDir()
	lib, err := mgr.CreateLibrary("Fotos", "", root)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(root, "a", "old.jpg")
	writePhoto(t, oldPath, "the same content", day)
	if !mgr.IndexFilesSync(lib.ID, []string{oldPath}) {
		t.Fatal("first index reported nothing")
	}
	store, _ := mgr.OpenStore(lib.ID)
	before := onlyID(t, store)

	newPath := filepath.Join(root, "b", "new.jpg")
	os.MkdirAll(filepath.Dir(newPath), 0o755) //nolint:errcheck
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	os.Chmod(newPath, 0)                           //nolint:errcheck
	t.Cleanup(func() { os.Chmod(newPath, 0o644) }) //nolint:errcheck

	if !mgr.IndexMovedFiles(lib.ID, [][2]string{{oldPath, newPath}}) {
		t.Fatal("IndexMovedFiles reported nothing")
	}
	ids := photoIDs(t, store)
	if len(ids) != 1 || ids[before] != "new.jpg" {
		t.Fatalf("want photo %s at new.jpg, have %v", before, ids)
	}
	if id, _, _, found, _ := store.GetPathCache(newPath); !found || id != before {
		t.Errorf("path cache at the new path: %q, %v", id, found)
	}
	if _, _, _, found, _ := store.GetPathCache(oldPath); found {
		t.Error("the old path is still known")
	}
}
