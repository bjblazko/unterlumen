package apilibrary

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestRebuildAlbumZipSkipsAlbumWithoutZip(t *testing.T) {
	dir := t.TempDir()
	album := &SiteAlbum{Photos: []SitePhoto{{Filename: "a.jpg"}}}
	os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("a"), 0o644) //nolint:errcheck

	if name := rebuildAlbumZip(album, dir); name != "" {
		t.Errorf("zip name = %q, want none", name)
	}
	if _, err := os.Stat(filepath.Join(dir, "photos.zip")); !os.IsNotExist(err) {
		t.Errorf("photos.zip written for an album that never had one")
	}
}

func TestRebuildAlbumZipRewritesZipFoundOnDisk(t *testing.T) {
	dir := t.TempDir()
	album := &SiteAlbum{Photos: []SitePhoto{{Filename: "a.jpg"}, {Filename: "b.jpg"}}}
	for _, n := range []string{"a.jpg", "b.jpg", "photos.zip"} {
		os.WriteFile(filepath.Join(dir, n), []byte(n), 0o644) //nolint:errcheck
	}

	if name := rebuildAlbumZip(album, dir); name != "photos.zip" {
		t.Fatalf("zip name = %q, want photos.zip", name)
	}
	if !album.HasZip {
		t.Errorf("HasZip not set after the ZIP was rebuilt")
	}
	zr, err := zip.OpenReader(filepath.Join(dir, "photos.zip"))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 2 {
		t.Errorf("zip holds %d files, want 2", len(zr.File))
	}
}

func TestRebuildAlbumZipWritesZipTheRegisterRecords(t *testing.T) {
	dir := t.TempDir()
	album := &SiteAlbum{HasZip: true, Photos: []SitePhoto{{Filename: "a.jpg"}}}
	os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("a"), 0o644) //nolint:errcheck

	if name := rebuildAlbumZip(album, dir); name != "photos.zip" {
		t.Errorf("zip name = %q, want photos.zip", name)
	}
}
