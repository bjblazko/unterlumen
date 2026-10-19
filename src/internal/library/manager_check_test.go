package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A shared library follows its folder without a click: what the other
// installation added or deleted shows after the next check. A library that
// is not shared is left to its owner's scans.
func TestTheCheckFollowsSharedLibrariesOnly(t *testing.T) {
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sharedDir, ownDir := t.TempDir(), t.TempDir()
	shared, _ := mgr.CreateLibrary("Shared", "", sharedDir)
	own, _ := mgr.CreateLibrary("Own", "", ownDir)
	if _, err := mgr.ShareLibrary(shared.ID); err != nil {
		t.Fatal(err)
	}
	count := func(id string) int {
		s, _ := mgr.OpenStore(id)
		n, _ := s.CountPhotos()
		return n
	}

	writePhoto(t, filepath.Join(sharedDir, "from-the-nas.jpg"), "a", day)
	writePhoto(t, filepath.Join(sharedDir, "stays.jpg"), "c", day)
	writePhoto(t, filepath.Join(ownDir, "not-scanned-yet.jpg"), "b", day)
	mgr.checkSharedLibraries(context.Background())
	if count(shared.ID) != 2 || count(own.ID) != 0 {
		t.Fatalf("after the check: shared %d, own %d photos; want 2 and 0", count(shared.ID), count(own.ID))
	}

	os.Remove(filepath.Join(sharedDir, "from-the-nas.jpg")) //nolint:errcheck
	mgr.checkSharedLibraries(context.Background())
	if n := count(shared.ID); n != 1 {
		t.Errorf("a photo deleted on the other installation is still listed (%d)", n)
	}
}
