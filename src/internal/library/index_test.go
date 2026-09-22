package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/media"
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

// A single-gallery channel holds many unrelated albums, and a photo can appear
// in more than one of them. indexSidecar used to collapse a channel's
// publications to the newest one, so re-indexing (or "Rebuild metadata") threw
// away every earlier album membership the sidecar still recorded.
func TestIndexSidecarKeepsEveryAlbumOfOneChannel(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "DSCF0001.jpg")
	if err := os.WriteFile(photo, []byte("not really a jpeg"), 0o644); err != nil {
		t.Fatalf("write photo: %v", err)
	}

	older := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	for _, pub := range []media.Publication{
		{Channel: "fotoshare", PostID: "aaa111", GalleryTitle: "Uli", PublishedAt: older},
		{Channel: "fotoshare", PostID: "bbb222", GalleryTitle: "Regenwanderung", PublishedAt: newer},
	} {
		if err := media.AppendPublication(photo, pub); err != nil {
			t.Fatalf("AppendPublication: %v", err)
		}
	}

	idx, store := newTestIndexer(t, dir)
	if err := store.UpsertPhoto("photo-1", photo, "DSCF0001.jpg", 0, time.Now(), "", "", "", "jpeg"); err != nil {
		t.Fatalf("UpsertPhoto: %v", err)
	}
	idx.indexSidecar(photo, "photo-1")

	entries, err := store.GetMeta("photo-1")
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.Key] = e.Value
	}

	for key, want := range map[string]string{
		"built:fotoshare:aaa111:title": "Uli",
		"built:fotoshare:bbb222:title": "Regenwanderung",
	} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q — album membership was lost on reindex", key, got[key], want)
		}
	}
	// The unqualified keys stay as the channel marker and still describe the
	// most recent album, which is what the channel filter matches on.
	if _, ok := got["built:fotoshare"]; !ok {
		t.Error("missing built:fotoshare channel marker")
	}
	if got["built:fotoshare:title"] != "Regenwanderung" {
		t.Errorf("built:fotoshare:title = %q, want the newest album", got["built:fotoshare:title"])
	}
}
