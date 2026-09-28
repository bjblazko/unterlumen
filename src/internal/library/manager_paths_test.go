package library

import (
	"path/filepath"
	"testing"
)

// Two libraries may cover the same folder — one of a parent, one of the folder
// itself. Folders names both, so the lookup must not stop at the first.
func TestLibrariesForPathFindsEveryLibraryCoveringIt(t *testing.T) {
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	root := t.TempDir()
	child := filepath.Join(root, "2024")
	outer, err := mgr.CreateLibrary("All", "", root)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	inner, err := mgr.CreateLibrary("2024", "", child)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if _, err := mgr.CreateLibrary("Elsewhere", "", t.TempDir()); err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	got := ids(mgr.LibrariesForPath(filepath.Join(child, "trip")))
	if len(got) != 2 || !got[outer.ID] || !got[inner.ID] {
		t.Errorf("a folder inside both: got %v, want %s and %s", got, outer.ID, inner.ID)
	}
	if got := ids(mgr.LibrariesForPath(root)); len(got) != 1 || !got[outer.ID] {
		t.Errorf("the outer folder itself: got %v, want only %s", got, outer.ID)
	}
	if got := mgr.LibrariesForPath(root + "-sibling"); len(got) != 0 {
		t.Errorf("a sibling sharing a prefix: got %d libraries, want none", len(got))
	}
}

func ids(libs []*Library) map[string]bool {
	m := map[string]bool{}
	for _, l := range libs {
		m[l.ID] = true
	}
	return m
}
