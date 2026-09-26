package site

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Subdir is the name of the subdirectory inside a channel's output
// directory where the generated, servable static site (index.html, albums/,
// robots.txt, assets/, site.json, etc.) is written. A channel's output
// directory may also hold unrelated non-site gallery folders (from
// single-gallery builds on the same channel), so anything that needs to
// deploy/serve exactly the site — and nothing else — must target this
// subdirectory rather than the channel's output directory itself.
const Subdir = "site"

// Dir returns the full path to a channel's site subdirectory, given the
// channel's output directory (channels.Store.OutputDir(slug)).
func Dir(channelDir string) string {
	return filepath.Join(channelDir, Subdir)
}

// Photo stores the filenames needed to regenerate an album page without re-exporting.
type Photo struct {
	PhotoID       string `json:"photoID,omitempty"`
	Filename      string `json:"filename"`
	ThumbFilename string `json:"thumbFilename"`
}

// Album records metadata for one published album in the site statefile.
type Album struct {
	PostID      string    `json:"postID"`
	Slug        string    `json:"slug,omitempty"` // human-readable folder name; falls back to PostID when empty
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"` // set on add-to-existing; zero for first publish
	PhotoCount  int       `json:"photoCount"`
	// GeneratedAt / DeployedAt: see GalleryState. Zero means "not recorded",
	// which is the case for every album published before ADR-0029.
	GeneratedAt time.Time `json:"generatedAt"`
	DeployedAt  time.Time `json:"deployedAt"`
	CoverFile   string    `json:"coverFile"` // relative to the album dir, e.g. "cover.jpg"
	HasZip      bool      `json:"hasZip"`
	Photos      []Photo   `json:"photos"`             // stored so album pages can be rebuilt without re-export
	Unlisted    bool      `json:"unlisted,omitempty"` // excluded from site index/sitemap; noindexed; set at creation and immutable thereafter
}

// AlbumFolderName returns the filesystem folder name for an album.
// New albums get a slug; albums without one fall back to PostID for backward compatibility.
func AlbumFolderName(album Album) string {
	if album.Slug != "" {
		return album.Slug
	}
	return album.PostID
}

// slugify converts a title into a URL-safe lowercase slug.
func slugify(title string) string {
	r := strings.NewReplacer(
		"ä", "ae", "Ä", "ae", "ö", "oe", "Ö", "oe", "ü", "ue", "Ü", "ue",
		"ß", "ss", "é", "e", "è", "e", "ê", "e", "à", "a", "â", "a",
		"ñ", "n", "ç", "c",
	)
	s := strings.ToLower(r.Replace(title))
	var b strings.Builder
	prev := '-'
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			prev = c
		} else if prev != '-' {
			b.WriteByte('-')
			prev = '-'
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "album"
	}
	return result
}

// randomSlugToken returns an 8-character hex token generated with crypto/rand,
// used to make unlisted-album slugs unguessable.
func randomSlugToken() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read on a supported platform practically never fails; if it
		// somehow does, fall back to a fixed-but-still-unique-enough marker rather
		// than panicking the build.
		return "00000000"
	}
	return hex.EncodeToString(b)
}

// ComputeSlug derives a unique slug for a new album.
// If the base slug collides with an existing one, it appends the publish month (and day if needed).
//
// When unlisted is true, the returned slug is always the human slug plus a random
// crypto/rand-generated 8-character token, regardless of collisions — this keeps the
// listed/public collision-fallback path (date suffixes) completely untouched.
func ComputeSlug(title string, publishedAt time.Time, existing []Album, unlisted bool) string {
	base := slugify(title)
	if unlisted {
		return base + "-" + randomSlugToken()
	}
	used := make(map[string]bool, len(existing))
	for _, a := range existing {
		used[AlbumFolderName(a)] = true
	}
	if !used[base] {
		return base
	}
	monthly := base + "-" + publishedAt.Format("2006-01")
	if !used[monthly] {
		return monthly
	}
	return base + "-" + publishedAt.Format("2006-01-02")
}

// DateRangeStr formats a publish date (and optional updated date) as a human-readable range.
// Same month/year → "January 2026". Different month, same year → "January – March 2026".
// Different year → "December 2025 – January 2026".
func DateRangeStr(published, updated time.Time) string {
	if updated.IsZero() || (updated.Year() == published.Year() && updated.Month() == published.Month()) {
		return published.Format("January 2006")
	}
	if updated.Year() == published.Year() {
		return published.Format("January") + " – " + updated.Format("January 2006")
	}
	return published.Format("January 2006") + " – " + updated.Format("January 2006")
}

func LoadState(statePath string) ([]Album, error) {
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var albums []Album
	return albums, json.Unmarshal(data, &albums)
}

func SaveState(statePath string, albums []Album) error {
	data, err := json.MarshalIndent(albums, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0o600)
}
