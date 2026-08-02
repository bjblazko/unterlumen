package apilibrary

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestComputeSlugListedNoCollision(t *testing.T) {
	got := computeSlug("Summer 2026", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), nil, false)
	if got != "summer-2026" {
		t.Errorf("got %q, want %q", got, "summer-2026")
	}
}

func TestComputeSlugListedCollisionFallsBackToMonth(t *testing.T) {
	existing := []SiteAlbum{{Slug: "summer-2026"}}
	published := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	got := computeSlug("Summer 2026", published, existing, false)
	want := "summer-2026-2026-07"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComputeSlugListedMonthCollisionFallsBackToDay(t *testing.T) {
	existing := []SiteAlbum{{Slug: "summer-2026"}, {Slug: "summer-2026-2026-07"}}
	published := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	got := computeSlug("Summer 2026", published, existing, false)
	want := "summer-2026-2026-07-15"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestComputeSlugUnlistedAlwaysTokenSuffixed verifies unlisted albums always get a
// random token suffix, even when there is no title collision at all.
func TestComputeSlugUnlistedAlwaysTokenSuffixed(t *testing.T) {
	published := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	got := computeSlug("Family Reunion", published, nil, true)

	re := regexp.MustCompile(`^family-reunion-[0-9a-f]{8}$`)
	if !re.MatchString(got) {
		t.Errorf("unlisted slug %q does not match expected shape (human-slug-<8 hex chars>)", got)
	}

	// Two calls must not collide (crypto/rand-backed).
	got2 := computeSlug("Family Reunion", published, nil, true)
	if got == got2 {
		t.Errorf("expected two unlisted slug computations to differ, both were %q", got)
	}
}

func TestComputeSlugUnlistedIgnoresCollisionLogic(t *testing.T) {
	// Even with existing albums that would normally force a date suffix, an
	// unlisted album must still get a token suffix, not a date suffix.
	existing := []SiteAlbum{{Slug: "family-reunion"}}
	published := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	got := computeSlug("Family Reunion", published, existing, true)
	if strings.Contains(got, "2026-07") {
		t.Errorf("unlisted slug %q should not use the date-collision fallback", got)
	}
	re := regexp.MustCompile(`^family-reunion-[0-9a-f]{8}$`)
	if !re.MatchString(got) {
		t.Errorf("unlisted slug %q does not match expected shape", got)
	}
}

func testAlbums() []SiteAlbum {
	return []SiteAlbum{
		{
			PostID:      "p1",
			Slug:        "listed-album",
			Title:       "Listed Album",
			PublishedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			PhotoCount:  3,
			CoverFile:   "cover.jpg",
		},
		{
			PostID:      "p2",
			Slug:        "secret-album-deadbeef",
			Title:       "Secret Album",
			PublishedAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			PhotoCount:  2,
			CoverFile:   "cover.jpg",
			Unlisted:    true,
		},
	}
}

func TestGenerateSiteIndexExcludesUnlisted(t *testing.T) {
	html := string(GenerateSiteIndex("My Site", "light", "", testAlbums(), SiteNavContext{}))

	if !strings.Contains(html, "Listed Album") {
		t.Error("listed album title missing from generated site index")
	}
	if !strings.Contains(html, "listed-album") {
		t.Error("listed album folder name missing from generated site index")
	}
	if strings.Contains(html, "Secret Album") {
		t.Error("unlisted album title leaked into generated site index")
	}
	if strings.Contains(html, "secret-album-deadbeef") {
		t.Error("unlisted album folder name leaked into generated site index")
	}
}

func TestGenerateSitemapExcludesUnlisted(t *testing.T) {
	dir := t.TempDir()
	if err := generateSitemap(dir, testAlbums(), "https://example.com"); err != nil {
		t.Fatalf("generateSitemap: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "sitemap.xml"))
	if err != nil {
		t.Fatalf("read sitemap.xml: %v", err)
	}
	sitemap := string(data)

	if !strings.Contains(sitemap, "/albums/listed-album/") {
		t.Error("listed album URL missing from sitemap.xml")
	}
	if strings.Contains(sitemap, "secret-album-deadbeef") {
		t.Error("unlisted album URL leaked into sitemap.xml")
	}
}

func TestGenerateSiteGalleryNoindexForUnlisted(t *testing.T) {
	items := []GalleryItem{
		{Filename: "photo1.jpg", ThumbFilename: "thumbs/photo1.jpg"},
	}
	unlistedHTML := string(GenerateSiteGallery("Secret Album", "light", items, GalleryOptions{Unlisted: true}))
	if !strings.Contains(unlistedHTML, `<meta name="robots" content="noindex, nofollow">`) {
		t.Error("unlisted album page missing noindex robots meta tag")
	}

	listedHTML := string(GenerateSiteGallery("Listed Album", "light", items, GalleryOptions{Unlisted: false}))
	if strings.Contains(listedHTML, `noindex`) {
		t.Error("listed album page unexpectedly contains a noindex meta tag")
	}
}
