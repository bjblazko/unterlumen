package apilibrary

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/site"
)

func TestRebuildAlbumZipSkipsAlbumWithoutZip(t *testing.T) {
	dir := t.TempDir()
	album := &site.Album{Photos: []site.Photo{{Filename: "a.jpg"}}}
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
	album := &site.Album{Photos: []site.Photo{{Filename: "a.jpg"}, {Filename: "b.jpg"}}}
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
	album := &site.Album{HasZip: true, Photos: []site.Photo{{Filename: "a.jpg"}}}
	os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("a"), 0o644) //nolint:errcheck

	if name := rebuildAlbumZip(album, dir); name != "photos.zip" {
		t.Errorf("zip name = %q, want photos.zip", name)
	}
}

// The reported problem: album A published from one installation and B from the
// other. Whichever machine builds last must write an index and a sitemap that
// list both.
func TestRebuildSiteListsAlbumsOfBothInstallations(t *testing.T) {
	shared := t.TempDir()
	newInstall := func(root string) *channels.Store {
		st := channels.NewStore(shared, t.TempDir()).WithBoundary(root)
		if err := st.Save(&channels.Channel{Slug: "website", Name: "Website", SiteExport: true, SiteTitle: "Site", SiteURL: "https://example.org"}); err != nil {
			t.Fatal(err)
		}
		return st
	}
	nas := newInstall("/photos")
	mac := newInstall("/Volumes/nas/photos")

	if err := site.NewStore(nas, "website").Upsert(testAlbum("pA", "Alpha")); err != nil {
		t.Fatal(err)
	}
	if err := site.NewStore(mac, "website").Upsert(testAlbum("pB", "Beta")); err != nil {
		t.Fatal(err)
	}

	ch, err := mac.Get("website")
	if err != nil {
		t.Fatal(err)
	}
	siteDir, count, err := rebuildSiteChannel(mac, nil, ch)
	if err != nil || count != 2 {
		t.Fatalf("rebuild: count=%d err=%v", count, err)
	}
	index, _ := os.ReadFile(filepath.Join(siteDir, "index.html"))
	sitemap, _ := os.ReadFile(filepath.Join(siteDir, "sitemap.xml"))
	for _, want := range []string{"Alpha", "Beta"} {
		if !strings.Contains(string(index), want) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	for _, want := range []string{"/alpha", "/beta"} {
		if !strings.Contains(string(sitemap), want) {
			t.Errorf("sitemap.xml lacks %q", want)
		}
	}
}
