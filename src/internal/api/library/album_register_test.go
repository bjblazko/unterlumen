package apilibrary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/channels"
)

// installation stands for one machine: its own browse root and its own
// -lib-dir, sharing only the channel directory with the other machine.
func installation(t *testing.T, sharedDir, browseRoot string) *channels.Store {
	t.Helper()
	libDir := t.TempDir()
	return channels.NewStore(sharedDir, libDir).WithBoundary(browseRoot)
}

func testAlbum(postID, title string) SiteAlbum {
	return SiteAlbum{
		PostID:      postID,
		Slug:        slugify(title),
		Title:       title,
		PublishedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		PhotoCount:  1,
		CoverFile:   "cover.jpg",
		Photos:      []SitePhoto{{PhotoID: "abc", Filename: "a.jpg", ThumbFilename: "thumbs/a.jpg"}},
	}
}

func readRegisterFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = b
	}
	return out
}

// The same album registered from two installations that disagree about the
// browse root must produce byte-identical register files: nothing in the
// register may encode a machine-local path.
func TestAlbumRegisterIsIdenticalFromTwoBrowseRoots(t *testing.T) {
	shared1, shared2 := t.TempDir(), t.TempDir()
	nas := installation(t, shared1, "/photos")
	mac := installation(t, shared2, "/Volumes/nas/photos")

	if err := newAlbumRegister(nas.AlbumRegisterDir("website")).Upsert(testAlbum("p1", "Iceland")); err != nil {
		t.Fatal(err)
	}
	if err := newAlbumRegister(mac.AlbumRegisterDir("website")).Upsert(testAlbum("p1", "Iceland")); err != nil {
		t.Fatal(err)
	}

	a := readRegisterFiles(t, nas.AlbumRegisterDir("website"))
	b := readRegisterFiles(t, mac.AlbumRegisterDir("website"))
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("want one file per album, got %d and %d", len(a), len(b))
	}
	for name, content := range a {
		if string(content) != string(b[name]) {
			t.Errorf("%s differs between roots:\n%s\n---\n%s", name, content, b[name])
		}
		for _, root := range []string{"/photos", "/Volumes"} {
			if strings.Contains(string(content), root) {
				t.Errorf("%s contains machine-local path %q", name, root)
			}
		}
	}
}

// Album A published from one installation and album B from the other: each
// sees both, because the register is shared and neither writes a list.
func TestAlbumRegisterKeepsAlbumsOfBothInstallations(t *testing.T) {
	shared := t.TempDir()
	nas := installation(t, shared, "/photos")
	mac := installation(t, shared, "/Volumes/nas/photos")

	if err := newAlbumRegister(nas.AlbumRegisterDir("website")).Upsert(testAlbum("pA", "Alpha")); err != nil {
		t.Fatal(err)
	}
	if err := newAlbumRegister(mac.AlbumRegisterDir("website")).Upsert(testAlbum("pB", "Beta")); err != nil {
		t.Fatal(err)
	}

	for name, s := range map[string]*channels.Store{"nas": nas, "mac": mac} {
		got, err := newAlbumRegister(s.AlbumRegisterDir("website")).List()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("%s sees %d albums, want 2", name, len(got))
		}
	}
}

func TestAlbumRegisterRemoveDeletesOnlyThatAlbum(t *testing.T) {
	r := newAlbumRegister(t.TempDir())
	_ = r.Upsert(testAlbum("pA", "Alpha"))
	_ = r.Upsert(testAlbum("pB", "Beta"))
	if err := r.Remove("pA"); err != nil {
		t.Fatal(err)
	}
	got, _ := r.List()
	if len(got) != 1 || got[0].PostID != "pB" {
		t.Fatalf("got %+v, want only pB", got)
	}
	if err := r.Remove("pA"); err != nil {
		t.Errorf("removing an absent album should not fail: %v", err)
	}
}

func TestAlbumRegisterListOnMissingDirIsEmpty(t *testing.T) {
	got, err := newAlbumRegister(filepath.Join(t.TempDir(), "nope")).List()
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

// A postID comes from a request body in places; it must not escape the register.
func TestAlbumRegisterRejectsPostIDThatIsAPath(t *testing.T) {
	r := newAlbumRegister(t.TempDir())
	if err := r.Upsert(testAlbum("../evil", "X")); err == nil {
		t.Error("want error for postID containing a path separator")
	}
	if err := r.Remove("../evil"); err == nil {
		t.Error("want error for postID containing a path separator")
	}
}

// A deleted album leaves a tombstone in the shared register, so neither
// installation brings it back — not by rebuilding, not by a stale write.
func TestAlbumRegisterDeleteLeavesATombstone(t *testing.T) {
	dir := t.TempDir()
	r := newAlbumRegister(dir)
	_ = r.Upsert(testAlbum("pA", "Alpha"))
	_ = r.Upsert(testAlbum("pB", "Beta"))

	if err := r.Delete("pA"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.List(); len(got) != 1 || got[0].PostID != "pB" {
		t.Fatalf("List = %+v, want only pB", got)
	}
	// Another installation reads the same directory.
	other := newAlbumRegister(dir)
	if !other.IsDeleted("pA") || other.IsDeleted("pB") {
		t.Error("the tombstone must be visible to every installation, and only for pA")
	}
	if err := other.Upsert(testAlbum("pA", "Alpha")); err == nil {
		t.Error("writing a deleted album must be refused")
	}
}

// Remove without a tombstone stays available for albums that only lost their
// list entry (an emptied album); such an album may be restored from sidecars.
func TestAlbumRegisterRemoveLeavesNoTombstone(t *testing.T) {
	r := newAlbumRegister(t.TempDir())
	_ = r.Upsert(testAlbum("pA", "Alpha"))
	_ = r.Remove("pA")
	if r.IsDeleted("pA") {
		t.Error("Remove must not tombstone")
	}
	if err := r.Upsert(testAlbum("pA", "Alpha")); err != nil {
		t.Errorf("an album that was only removed can be written again: %v", err)
	}
}

func TestSiteStoreDeleteTombstonesAndRefreshesCache(t *testing.T) {
	s, cache := newTestSiteStore(t, t.TempDir(), "/photos")
	_ = s.Upsert(testAlbum("pA", "Alpha"))
	_ = s.Upsert(testAlbum("pB", "Beta"))
	if err := s.Delete("pA"); err != nil {
		t.Fatal(err)
	}
	if !s.IsDeleted("pA") {
		t.Error("pA must be tombstoned")
	}
	if cached, _ := LoadSiteState(cache); len(cached) != 1 || cached[0].PostID != "pB" {
		t.Errorf("cache = %+v", cached)
	}
}
