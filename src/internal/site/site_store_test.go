package site

import (
	"os"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
)

func newTestSiteStore(t *testing.T, shared string, browseRoot string) (*Store, string) {
	t.Helper()
	chStore := channels.NewStore(shared, t.TempDir()).WithBoundary(browseRoot)
	return NewStore(chStore, "website"), filepath.Join(chStore.OutputDir("website"), "site", "site.json")
}

// Two installations, each with its own output directory and stale cache: an
// album published on one is in the other's list, and the index a build writes
// from List() has both.
func TestSiteStoreListComesFromRegisterNotLocalCache(t *testing.T) {
	shared := t.TempDir()
	nas, _ := newTestSiteStore(t, shared, "/photos")
	mac, macCache := newTestSiteStore(t, shared, "/Volumes/nas/photos")

	if err := nas.Upsert(testAlbum("pA", "Alpha")); err != nil {
		t.Fatal(err)
	}
	if err := mac.Upsert(testAlbum("pB", "Beta")); err != nil {
		t.Fatal(err)
	}

	got, err := mac.List()
	if err != nil || len(got) != 2 {
		t.Fatalf("mac sees %d albums (%v), want 2", len(got), err)
	}
	cached, err := LoadState(macCache)
	if err != nil || len(cached) != 2 {
		t.Errorf("mac cache holds %d albums (%v), want 2", len(cached), err)
	}
}

func TestSiteStoreRemoveRefreshesCacheAndKeepsOthers(t *testing.T) {
	s, cache := newTestSiteStore(t, t.TempDir(), "/photos")
	_ = s.Upsert(testAlbum("pA", "Alpha"))
	_ = s.Upsert(testAlbum("pB", "Beta"))
	if err := s.Remove("pA"); err != nil {
		t.Fatal(err)
	}
	cached, _ := LoadState(cache)
	if len(cached) != 1 || cached[0].PostID != "pB" {
		t.Fatalf("cache = %+v, want only pB", cached)
	}
}

// Albums that only exist in a pre-register site.json are adopted once, when
// the register is still empty, so nothing published before this change vanishes.
func TestSiteStoreAdoptsLegacySiteJSONIntoEmptyRegister(t *testing.T) {
	s, cache := newTestSiteStore(t, t.TempDir(), "/photos")
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(cache, []Album{testAlbum("old1", "Old")}); err != nil {
		t.Fatal(err)
	}

	got, err := s.List()
	if err != nil || len(got) != 1 || got[0].PostID != "old1" {
		t.Fatalf("List = %+v, %v", got, err)
	}
	if regFiles := readRegisterFiles(t, s.reg.dir); len(regFiles) != 1 {
		t.Errorf("register holds %d files, want the adopted album", len(regFiles))
	}
}

// Once the register has albums, a stale local site.json must not add to it:
// it may contain an album that was deleted from the other installation.
func TestSiteStoreDoesNotAdoptWhenRegisterHasAlbums(t *testing.T) {
	shared := t.TempDir()
	first, _ := newTestSiteStore(t, shared, "/photos")
	_ = first.Upsert(testAlbum("pA", "Alpha"))

	second, cache := newTestSiteStore(t, shared, "/Volumes/nas/photos")
	_ = os.MkdirAll(filepath.Dir(cache), 0o755)
	_ = SaveState(cache, []Album{testAlbum("stale", "Stale")})

	got, _ := second.List()
	if len(got) != 1 || got[0].PostID != "pA" {
		t.Fatalf("List = %+v, want only pA", got)
	}
}

// "Empty" is not "never used": once the last album was removed, a stale
// site.json on another installation must not bring it back.
func TestSiteStoreDoesNotAdoptAfterTheLastAlbumWasRemoved(t *testing.T) {
	shared := t.TempDir()
	first, _ := newTestSiteStore(t, shared, "/photos")
	_ = first.Upsert(testAlbum("pA", "Alpha"))

	second, cache := newTestSiteStore(t, shared, "/Volumes/nas/photos")
	_ = os.MkdirAll(filepath.Dir(cache), 0o755)
	_ = SaveState(cache, []Album{testAlbum("pA", "Alpha")}) // its cache still has the album

	if err := first.Remove("pA"); err != nil {
		t.Fatal(err)
	}
	got, err := second.List()
	if err != nil || len(got) != 0 {
		t.Fatalf("List = %+v, %v; the removed album must stay removed", got, err)
	}
}
