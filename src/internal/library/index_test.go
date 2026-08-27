package library

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestIndexer(t *testing.T, sourcePath string) (*Indexer, *Store) {
	t.Helper()
	s := newTestStore(t)
	return NewIndexer(s, s.dir, sourcePath), s
}

// TestIndexFileDoesNotOrphanExistingDuplicate guards against a bug where
// scanning a byte-identical copy of an already-indexed photo at a second
// path silently repointed that photo's single content-addressed DB row at
// the new path — even though the original file was untouched and still on
// disk. The original would then vanish from the library (not deleted, just
// no longer referenced) until a full rescan happened to visit it again.
func TestIndexFileDoesNotOrphanExistingDuplicate(t *testing.T) {
	src := t.TempDir()
	original := filepath.Join(src, "original.jpg")
	duplicate := filepath.Join(src, "duplicate.jpg")

	content := []byte{0xFF, 0xD8, 0xFF, 0xD9, 1, 2, 3}
	if err := os.WriteFile(original, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(duplicate, content, 0644); err != nil {
		t.Fatal(err)
	}

	idx, store := newTestIndexer(t, src)
	if err := idx.IndexFile(original); err != nil {
		t.Fatalf("index original: %v", err)
	}
	if err := idx.IndexFile(duplicate); err != nil {
		t.Fatalf("index duplicate: %v", err)
	}

	refs, err := store.ListAllPhotoRefs()
	if err != nil {
		t.Fatalf("ListAllPhotoRefs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected exactly one photo row for byte-identical content, got %d", len(refs))
	}
	if refs[0].PathHint != original {
		t.Errorf("path_hint = %q, want the still-existing original %q — indexing the duplicate should not have repointed it", refs[0].PathHint, original)
	}
}

// TestIndexFileTreatsGoneOriginalAsRename verifies the fix doesn't break the
// legitimate rename case: if the previous path_hint no longer exists on
// disk, indexing content at a new path should update path_hint to it.
func TestIndexFileTreatsGoneOriginalAsRename(t *testing.T) {
	src := t.TempDir()
	oldPath := filepath.Join(src, "old-name.jpg")
	newPath := filepath.Join(src, "new-name.jpg")

	content := []byte{0xFF, 0xD8, 0xFF, 0xD9, 4, 5, 6}
	if err := os.WriteFile(oldPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	idx, store := newTestIndexer(t, src)
	if err := idx.IndexFile(oldPath); err != nil {
		t.Fatalf("index original: %v", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile(newPath); err != nil {
		t.Fatalf("index renamed file: %v", err)
	}

	refs, err := store.ListAllPhotoRefs()
	if err != nil {
		t.Fatalf("ListAllPhotoRefs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected exactly one photo row, got %d", len(refs))
	}
	if refs[0].PathHint != newPath {
		t.Errorf("path_hint = %q, want the renamed path %q (old path no longer exists)", refs[0].PathHint, newPath)
	}
}
