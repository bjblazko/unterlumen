package apilibrary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
)

func newTestSiteStore(t *testing.T, shared string, browseRoot string) (*siteStore, string) {
	t.Helper()
	chStore := channels.NewStore(shared, t.TempDir()).WithBoundary(browseRoot)
	return newSiteStore(chStore, "website"), filepath.Join(chStore.OutputDir("website"), "site", "site.json")
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
	cached, err := loadSiteState(macCache)
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
	cached, _ := loadSiteState(cache)
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
	if err := saveSiteState(cache, []SiteAlbum{testAlbum("old1", "Old")}); err != nil {
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
	_ = saveSiteState(cache, []SiteAlbum{testAlbum("stale", "Stale")})

	got, _ := second.List()
	if len(got) != 1 || got[0].PostID != "pA" {
		t.Fatalf("List = %+v, want only pA", got)
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

	if err := newSiteStore(nas, "website").Upsert(testAlbum("pA", "Alpha")); err != nil {
		t.Fatal(err)
	}
	if err := newSiteStore(mac, "website").Upsert(testAlbum("pB", "Beta")); err != nil {
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
