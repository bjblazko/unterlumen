package media

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeSized(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkFolderStats(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shoot")
	writeSized(t, filepath.Join(root, "a.jpg"), 3)
	writeSized(t, filepath.Join(root, "noext"), 2)
	writeSized(t, filepath.Join(root, ".hidden"), 100)
	writeSized(t, filepath.Join(root, ".cache", "x.jpg"), 100)
	writeSized(t, filepath.Join(root, "sub1", "b.JPG"), 5)
	writeSized(t, filepath.Join(root, "sub1", "deep", "c.raf"), 7)
	for _, d := range []string{filepath.Join(root, "sub1", "deep", "deeper"), filepath.Join(root, "sub2")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	st, err := WalkFolderStats(root, "photos/shoot")
	if err != nil {
		t.Fatal(err)
	}
	want := &FolderStats{
		Name:      "shoot",
		Path:      "photos/shoot",
		Modified:  st.Modified,
		TotalSize: 17,
		FileCount: 4,
		DirCount:  4,
		MaxDepth:  3,
		Subfolders: []SubfolderStats{
			{Name: "sub1", Size: 12, FileCount: 2, DirCount: 2, MaxDepth: 2},
			{Name: "sub2"},
		},
		FileTypes: map[string]int{"jpg": 2, "raf": 1},
	}
	if !reflect.DeepEqual(st, want) {
		t.Errorf("WalkFolderStats =\n%+v\nwant\n%+v", st, want)
	}
}

func TestWalkFolderStatsStopsTenLevelsDown(t *testing.T) {
	root := t.TempDir()
	parts := []string{root}
	for i := 1; i <= 11; i++ {
		parts = append(parts, "d")
	}
	writeSized(t, filepath.Join(append(parts, "too-deep.jpg")...), 1)

	st, err := WalkFolderStats(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if st.DirCount != 10 || st.MaxDepth != 10 || st.FileCount != 0 {
		t.Errorf("dirs %d, depth %d, files %d; want 10, 10, 0", st.DirCount, st.MaxDepth, st.FileCount)
	}
	if len(st.Subfolders) != 1 || st.Subfolders[0].DirCount != 9 || st.Subfolders[0].MaxDepth != 9 {
		t.Errorf("subfolders = %+v, want d with 9 dirs, depth 9", st.Subfolders)
	}
}

func TestWalkFolderStatsOfAMissingFolderFails(t *testing.T) {
	if _, err := WalkFolderStats(filepath.Join(t.TempDir(), "gone"), ""); err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("err = %v, want one naming the folder", err)
	}
}
