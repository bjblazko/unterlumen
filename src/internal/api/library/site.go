package apilibrary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"huepattl.de/unterlumen/internal/channels"
)

// SitePhoto stores the filenames needed to regenerate an album page without re-exporting.
type SitePhoto struct {
	PhotoID       string `json:"photoID,omitempty"`
	Filename      string `json:"filename"`
	ThumbFilename string `json:"thumbFilename"`
}

// SiteAlbum records metadata for one published album in the site statefile.
type SiteAlbum struct {
	PostID      string      `json:"postID"`
	Slug        string      `json:"slug,omitempty"` // human-readable folder name; falls back to PostID when empty
	Title       string      `json:"title"`
	PublishedAt time.Time   `json:"publishedAt"`
	UpdatedAt   time.Time   `json:"updatedAt,omitempty"` // set on add-to-existing; zero for first publish
	PhotoCount  int         `json:"photoCount"`
	CoverFile   string      `json:"coverFile"` // relative to the album dir, e.g. "cover.jpg"
	HasZip      bool        `json:"hasZip"`
	Photos      []SitePhoto `json:"photos"` // stored so album pages can be rebuilt without re-export
}

// albumFolderName returns the filesystem folder name for an album.
// New albums get a slug; albums without one fall back to PostID for backward compatibility.
func albumFolderName(album SiteAlbum) string {
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

// computeSlug derives a unique slug for a new album.
// If the base slug collides with an existing one, it appends the publish month (and day if needed).
func computeSlug(title string, publishedAt time.Time, existing []SiteAlbum) string {
	used := make(map[string]bool, len(existing))
	for _, a := range existing {
		used[albumFolderName(a)] = true
	}
	base := slugify(title)
	if !used[base] {
		return base
	}
	monthly := base + "-" + publishedAt.Format("2006-01")
	if !used[monthly] {
		return monthly
	}
	return base + "-" + publishedAt.Format("2006-01-02")
}

// dateRangeStr formats a publish date (and optional updated date) as a human-readable range.
// Same month/year → "January 2026". Different month, same year → "January – March 2026".
// Different year → "December 2025 – January 2026".
func dateRangeStr(published, updated time.Time) string {
	if updated.IsZero() || (updated.Year() == published.Year() && updated.Month() == published.Month()) {
		return published.Format("January 2006")
	}
	if updated.Year() == published.Year() {
		return published.Format("January") + " – " + updated.Format("January 2006")
	}
	return published.Format("January 2006") + " – " + updated.Format("January 2006")
}

func loadSiteState(statePath string) ([]SiteAlbum, error) {
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var albums []SiteAlbum
	return albums, json.Unmarshal(data, &albums)
}

func saveSiteState(statePath string, albums []SiteAlbum) error {
	data, err := json.MarshalIndent(albums, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0o600)
}

// SiteNavContext carries optional page-link and contact data passed to all site template generators.
type SiteNavContext struct {
	HasAbout     bool
	HasImprint   bool
	ContactEmail string
	ContactURL   string
	LogoExists   bool
	LogoPath     string // "assets/logo.jpg" for root-level pages; "../../assets/logo.jpg" for album pages
	SiteName     string
}

// markdownToHTML converts markdown text to safe HTML using goldmark.
// HTML passthrough is enabled so advanced users can embed raw HTML.
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func markdownToHTML(src string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(src))
	}
	return template.HTML(buf.String())
}

// avatarExistsAt reports whether site/assets/avatar.jpg exists in siteDir.
func avatarExistsAt(siteDir string) bool {
	_, err := os.Stat(filepath.Join(siteDir, "assets", "avatar.jpg"))
	return err == nil
}

// logoExistsAt reports whether site/assets/logo.jpg exists in siteDir.
func logoExistsAt(siteDir string) bool {
	_, err := os.Stat(filepath.Join(siteDir, "assets", "logo.jpg"))
	return err == nil
}

// buildSiteNavContext constructs a SiteNavContext from channel config and current site state.
// rootLevel=true uses "assets/logo.jpg" (root index, about, legal); false uses "../../assets/logo.jpg" (album pages).
func buildSiteNavContext(ch *channels.Channel, siteDir string, rootLevel bool) SiteNavContext {
	logoPath := "assets/logo.jpg"
	if !rootLevel {
		logoPath = "../../assets/logo.jpg"
	}
	return SiteNavContext{
		HasAbout:     ch.SiteAbout != "",
		HasImprint:   ch.SiteImprint != "",
		ContactEmail: ch.SiteContactEmail,
		ContactURL:   ch.SiteContactURL,
		LogoExists:   logoExistsAt(siteDir),
		LogoPath:     logoPath,
		SiteName:     ch.SiteTitle,
	}
}

// writeSiteAssets writes style.css and toggle.js into assetsDir, overwriting if present.
// toggle.js is fully static — it reads the default theme from data-default-theme on <html>.
func writeSiteAssets(assetsDir string) error {
	if err := os.MkdirAll(assetsDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(assetsDir+"/style.css", []byte(siteCSS), 0o644); err != nil {
		return err
	}
	return os.WriteFile(assetsDir+"/toggle.js", []byte(siteToggleJS), 0o644)
}

// generateRobotsTxt writes a robots.txt to the site root.
// If siteURL is non-empty, a Sitemap line is included.
func generateRobotsTxt(siteDir, siteURL string) error {
	var b strings.Builder
	b.WriteString("User-agent: *\nAllow: /\n")
	if siteURL != "" {
		fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", strings.TrimRight(siteURL, "/"))
	}
	return os.WriteFile(filepath.Join(siteDir, "robots.txt"), []byte(b.String()), 0o644)
}

// generateSitemap writes a sitemap.xml to the site root.
// Only called when siteURL is non-empty; sitemap requires absolute URLs.
func generateSitemap(siteDir string, albums []SiteAlbum, siteURL string) error {
	base := strings.TrimRight(siteURL, "/")
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	fmt.Fprintf(&b, "  <url><loc>%s/</loc></url>\n", base)
	for _, a := range albums {
		fmt.Fprintf(&b, "  <url><loc>%s/albums/%s/</loc></url>\n", base, albumFolderName(a))
	}
	b.WriteString("</urlset>\n")
	return os.WriteFile(filepath.Join(siteDir, "sitemap.xml"), []byte(b.String()), 0o644)
}

// siteCSS is the shared stylesheet for all site pages (root index + album pages).
// Uses CSS custom properties so both themes are defined in one file.
const siteCSS = `/* --- Theme variables --- */
:root {
  --bg:         #f5f2ed;
  --text:       #2a2520;
  --text-dim:   #756d64;
  --text-muted: #a09890;
  --border:     #ccc8c2;
  --card-bg:    #e0dbd4;
  --accent:     #d35400;
  --heading:    #1a1714;
}
html.theme-dark {
  --bg:         #111;
  --text:       #ddd;
  --text-dim:   #999;
  --text-muted: rgba(255,255,255,0.45);
  --border:     #333;
  --card-bg:    #222;
  --accent:     #d35400;
  --heading:    #fff;
}

/* --- Base --- */
html { overflow-x: hidden; }
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: Helvetica, "Helvetica Neue", Arial, sans-serif;
  background: var(--bg);
  color: var(--text);
  padding: 2.5rem 1.5rem 5rem;
  max-width: 1100px;
  margin: 0 auto;
  transition: background 0.2s, color 0.2s;
}
header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 2.5rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid var(--border);
}
h1 {
  font-size: 1.6rem;
  font-weight: 500;
  letter-spacing: -0.02em;
  color: var(--heading);
}
footer {
  margin-top: 4rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border);
  font-size: 0.75rem;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}
footer a { color: var(--text-muted); text-decoration: none; }
footer a:hover { color: var(--text-dim); }
.footer-contact { display: flex; gap: 0.75rem; margin-left: auto; flex-wrap: wrap; justify-content: flex-end; min-width: 0; max-width: 100%; }
.footer-contact a { color: var(--text-muted); text-decoration: none; font-size: 0.75rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 100%; }
.footer-contact a:hover { color: var(--text-dim); }

/* --- Site brand (persistent header identity) --- */
.site-masthead {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}
.site-brand {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
}
.site-logo {
  height: 28px;
  width: auto;
  max-width: 100px;
  object-fit: contain;
  flex-shrink: 0;
}
.site-name {
  font-size: 1.6rem;
  font-weight: 500;
  color: var(--heading);
  text-decoration: none;
  letter-spacing: -0.02em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.site-name:hover { color: var(--accent); }
.page-title {
  width: 100%;
  font-size: 0.9rem;
  font-weight: 400;
  color: var(--text-dim);
  display: flex;
  align-items: center;
}

/* --- Site navigation (header page links) --- */
.site-nav { display: flex; gap: 1rem; font-size: 0.82rem; align-items: baseline; }
.site-nav a { color: var(--text-dim); text-decoration: none; }
.site-nav a:hover { color: var(--accent); }

/* --- Theme toggle button --- */
.theme-btn {
  background: none;
  border: 1px solid var(--border);
  border-radius: 2px;
  color: var(--text-dim);
  font-family: inherit;
  font-size: 0.78rem;
  padding: 0.3rem 0.65rem;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
}
.theme-btn:hover { color: var(--text); border-color: var(--text-dim); }

/* --- Back link (album pages) --- */
.site-back {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  padding: 0 0.75rem;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 14px;
  color: var(--text-dim);
  text-decoration: none;
  font-size: 0.85rem;
  gap: 0.2rem;
  flex-shrink: 0;
  margin-right: 0.75rem;
}
.site-back:hover { color: var(--accent); border-color: var(--accent); }

/* --- Download button (album pages) --- */
.dl-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.9rem;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 3px;
  color: var(--text-dim);
  text-decoration: none;
  font-size: 0.8rem;
  transition: background 0.15s, border-color 0.15s;
}
.dl-btn:hover { color: var(--text); }

/* --- Header actions group --- */
.header-actions {
  position: relative;
  display: flex;
  align-items: center;
  flex-shrink: 0;
}
.header-actions-inner {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}
.menu-btn { display: none; }

@media (max-width: 600px) {
  .menu-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: 1px solid var(--border);
    border-radius: 2px;
    color: var(--text-dim);
    font-size: 1.3rem;
    line-height: 1;
    padding: 0.25rem 0.6rem;
    cursor: pointer;
    min-height: 36px;
    font-family: inherit;
  }
  .menu-btn:hover { color: var(--text); border-color: var(--text-dim); }
  .header-actions-inner {
    display: none;
    position: absolute;
    right: 0;
    top: calc(100% + 0.5rem);
    flex-direction: column;
    align-items: stretch;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0.4rem;
    gap: 0.4rem;
    z-index: 200;
    min-width: 180px;
    box-shadow: 0 4px 16px rgba(0,0,0,0.12);
  }
  .header-actions-inner.open { display: flex; }
  .header-actions-inner .dl-btn,
  .header-actions-inner .theme-btn { width: 100%; justify-content: flex-start; }
}

/* --- Album grid (root index) --- */
.albums {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 2rem;
}
@media (max-width: 720px) { .albums { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 440px) { .albums { grid-template-columns: 1fr; } }

.album-card { text-decoration: none; color: inherit; display: block; }
.album-cover {
  width: 100%;
  aspect-ratio: 4 / 3;
  object-fit: cover;
  display: block;
  border-radius: 2px;
  background: var(--card-bg);
  margin-bottom: 0.75rem;
  transition: opacity 0.15s;
}
.album-card:hover .album-cover { opacity: 0.88; }
.album-title {
  font-size: 0.95rem;
  font-weight: 500;
  color: var(--heading);
  margin-bottom: 0.2rem;
}
.album-meta { font-size: 0.78rem; color: var(--text-dim); }
.album-card:hover .album-title { color: var(--accent); }

/* --- Prose content (about, imprint pages) --- */
.prose { max-width: 680px; line-height: 1.7; }
.prose h2 { font-size: 1.1rem; font-weight: 500; margin: 2rem 0 0.5rem; color: var(--heading); }
.prose h3 { font-size: 0.95rem; font-weight: 500; margin: 1.5rem 0 0.4rem; color: var(--heading); }
.prose p  { margin-bottom: 1rem; }
.prose a  { color: var(--accent); }
.prose ul, .prose ol { padding-left: 1.5rem; margin-bottom: 1rem; }
.prose li { margin-bottom: 0.25rem; }
.prose strong { font-weight: 500; }
.prose hr { border: none; border-top: 1px solid var(--border); margin: 2rem 0; }
.about-layout { display: flex; gap: 2.5rem; align-items: flex-start; flex-wrap: wrap; }
.about-photo { width: 160px; height: 160px; object-fit: cover; border-radius: 50%; flex-shrink: 0; background: var(--card-bg); }

/* --- Masonry gallery (album pages) --- */
.gallery { column-count: 2; column-gap: 12px; }
@media (max-width: 540px) { .gallery { column-count: 1; } }
.gallery figure {
  break-inside: avoid;
  margin: 0 0 12px;
  cursor: pointer;
  overflow: hidden;
  border-radius: 2px;
  background: var(--card-bg);
}
.gallery figure:hover img { opacity: 0.88; }
.gallery img { display: block; width: 100%; height: auto; transition: opacity 0.15s; }

/* --- Lightbox --- */
#lb {
  display: none;
  position: fixed; inset: 0;
  background: #000;
  z-index: 9999;
  align-items: center;
  justify-content: center;
}
#lb.open { display: flex; }
#lb-img {
  width: 100%; height: 100%;
  object-fit: contain;
  user-select: none;
}
#lb-close {
  position: fixed; top: 1.2rem; right: 1.5rem;
  background: none; border: none;
  color: #fff; font-size: 2rem; line-height: 1;
  cursor: pointer; opacity: 0.7; padding: 0.6rem 0.8rem;
}
#lb-close:hover { opacity: 1; }
.lb-nav {
  position: fixed; top: 50%; transform: translateY(-50%);
  background: rgba(255,255,255,0.08); border: none;
  color: #fff; font-size: 1.8rem; line-height: 1;
  cursor: pointer; padding: 1rem 0.9rem; border-radius: 3px;
  opacity: 0.6; transition: opacity 0.15s, background 0.15s;
  user-select: none;
}
.lb-nav:hover { opacity: 1; background: rgba(255,255,255,0.15); }
#lb-prev { left: 1rem; }
#lb-next { right: 1rem; }
#lb-counter {
  position: fixed; bottom: 1.2rem; left: 50%; transform: translateX(-50%);
  font-size: 0.8rem; color: rgba(255,255,255,0.45); letter-spacing: 0.06em;
  pointer-events: none;
}
@media (pointer: coarse) {
  .lb-nav { display: none; }
}
`

// siteToggleJS is a fully static theme-toggle script.
// The default theme is read from data-default-theme on <html> so that this file
// never needs to change content — caching it indefinitely is safe.
const siteToggleJS = `(function () {
  var KEY = 'ul-theme';
  var root = document.documentElement;
  var def = root.dataset.defaultTheme || 'light';
  function apply(t) { root.classList.toggle('theme-dark', t === 'dark'); }
  apply(localStorage.getItem(KEY) || def);
  // Re-apply when browser restores page from back/forward cache.
  window.addEventListener('pageshow', function (e) {
    if (e.persisted) apply(localStorage.getItem(KEY) || def);
  });
  document.addEventListener('DOMContentLoaded', function () {
    var btn = document.getElementById('theme-toggle');
    if (!btn) return;
    function sync() { btn.textContent = root.classList.contains('theme-dark') ? 'Light' : 'Dark'; }
    btn.addEventListener('click', function () {
      var next = root.classList.contains('theme-dark') ? 'light' : 'dark';
      apply(next);
      localStorage.setItem(KEY, next);
      sync();
    });
    sync();
  });
})();
`

/* --- Site root index --- */

type siteAlbumData struct {
	FolderName string // slug if set, else postID — used in URLs
	Title      string
	DateStr    string
	PhotoCount int
	CoverFile  string
	Loading    string // "eager" for above-the-fold covers, "lazy" for the rest
}

var siteTmpl = template.Must(template.New("site").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
{{- if .SiteURL}}
<link rel="canonical" href="{{.SiteURL}}/">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:type" content="website">
{{- end}}
<script type="application/ld+json">{{.LDJSON}}</script>
<link rel="stylesheet" href="assets/style.css">
<script src="assets/toggle.js"></script>
</head>
<body>
<header>
  <div class="site-brand">
    {{- if .Nav.LogoExists}}
    <img class="site-logo" src="{{.Nav.LogoPath}}" alt="" loading="eager">
    {{- end}}
    <h1>{{.Title}}</h1>
  </div>
  <div class="header-actions">
    {{- if or .Nav.HasAbout .Nav.HasImprint}}
    <nav class="site-nav">
      {{- if .Nav.HasAbout}}<a href="about.html">About</a>{{end}}
      {{- if .Nav.HasImprint}}<a href="legal.html">Legal</a>{{end}}
    </nav>
    {{- end}}
    <button id="theme-toggle" class="theme-btn">Dark</button>
  </div>
</header>

<main class="albums">
{{range .Albums}}  <a class="album-card" href="albums/{{.FolderName}}/index.html">
    <img class="album-cover" src="albums/{{.FolderName}}/{{.CoverFile}}" alt="{{.Title}}" loading="{{.Loading}}">
    <div class="album-title">{{.Title}}</div>
    <div class="album-meta">{{.DateStr}} &middot; {{.PhotoCount}} photo{{if ne .PhotoCount 1}}s{{end}}</div>
  </a>
{{end}}</main>

<footer>
  <span>Built with <svg width="13" height="13" viewBox="0 0 24 24" fill="var(--accent)" aria-hidden="true" style="vertical-align:-1px"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg> <a href="https://huepattl.de/products/unterlumen.html" target="_blank" rel="noopener">Unterlumen</a></span>
  <a href="https://github.com/bjblazko/unterlumen" target="_blank" rel="noopener" title="View on GitHub"><svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg></a>
  {{- if or .Nav.ContactEmail .Nav.ContactURL}}
  <div class="footer-contact">
    {{- if .Nav.ContactEmail}}<a href="mailto:{{.Nav.ContactEmail}}">{{.Nav.ContactEmail}}</a>{{end}}
    {{- if .Nav.ContactURL}}<a href="{{.Nav.ContactURL}}" target="_blank" rel="noopener">{{.Nav.ContactURL}}</a>{{end}}
  </div>
  {{- end}}
</footer>
</body>
</html>
`))

// GenerateSiteIndex produces a static root index.html referencing shared assets.
// Albums are ordered newest first by PublishedAt.
func GenerateSiteIndex(siteTitle, defaultTheme, siteURL string, albums []SiteAlbum, nav SiteNavContext) []byte {
	if siteTitle == "" {
		siteTitle = "Photo Albums"
	}
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	sorted := make([]SiteAlbum, len(albums))
	copy(sorted, albums)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
	})
	items := make([]siteAlbumData, len(sorted))
	for i, a := range sorted {
		loading := "lazy"
		if i < 2 {
			loading = "eager"
		}
		items[i] = siteAlbumData{
			FolderName: albumFolderName(a),
			Title:      a.Title,
			DateStr:    dateRangeStr(a.PublishedAt, a.UpdatedAt),
			PhotoCount: a.PhotoCount,
			CoverFile:  a.CoverFile,
			Loading:    loading,
		}
	}

	description := fmt.Sprintf("Photography collection — %d album", len(albums))
	if len(albums) != 1 {
		description += "s"
	}
	description += "."

	ldMap := map[string]any{
		"@context": "https://schema.org",
		"@type":    "CollectionPage",
		"name":     siteTitle,
		"description": description,
	}
	if siteURL != "" {
		ldMap["url"] = strings.TrimRight(siteURL, "/") + "/"
	}
	ldJSON, _ := json.Marshal(ldMap)

	cleanSiteURL := ""
	if siteURL != "" {
		cleanSiteURL = strings.TrimRight(siteURL, "/")
	}

	var buf bytes.Buffer
	siteTmpl.Execute(&buf, struct { //nolint:errcheck
		Title        string
		DefaultTheme string
		Description  string
		SiteURL      string
		LDJSON       template.JS
		Albums       []siteAlbumData
		Nav          SiteNavContext
	}{
		Title:        siteTitle,
		DefaultTheme: defaultTheme,
		Description:  description,
		SiteURL:      cleanSiteURL,
		LDJSON:       template.JS(ldJSON),
		Albums:       items,
		Nav:          nav,
	})
	return buf.Bytes()
}

/* --- Site album gallery --- */

var siteGalleryTmpl = template.Must(template.New("sitegallery").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
<title>{{.PageTitle}}</title>
<meta name="description" content="{{.Description}}">
{{- if .SiteURL}}
<link rel="canonical" href="{{.AlbumURL}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
{{- if .CoverURL}}
<meta property="og:image" content="{{.CoverURL}}">{{end}}
<meta property="og:type" content="website">
{{- end}}
<script type="application/ld+json">{{.LDJSON}}</script>
<link rel="stylesheet" href="../../assets/style.css">
<script src="../../assets/toggle.js"></script>
</head>
<body>
<header>
  <div class="site-masthead">
    <div class="site-brand">
      {{- if .Nav.LogoExists}}
      <img class="site-logo" src="{{.Nav.LogoPath}}" alt="" loading="eager">
      {{- end}}
      <a class="site-name" href="../../index.html">{{.Nav.SiteName}}</a>
    </div>
    <div class="header-actions">
      <button class="menu-btn" id="menu-btn" aria-label="Menu" aria-expanded="false">&#x22EF;</button>
      <div class="header-actions-inner">
        {{if .ZipFilename}}<a class="dl-btn" href="{{.ZipFilename}}" download><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> Download all photos</a>{{end}}
        <button id="theme-toggle" class="theme-btn">Dark</button>
      </div>
    </div>
  </div>
  <div class="page-title">
    <a class="site-back" href="../../index.html" title="Back to albums"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="vertical-align:-4px"><polyline points="15 18 9 12 15 6"/></svg></a>{{.Title}}
  </div>
</header>

<main class="gallery" id="gallery">
{{range .Figures}}<figure data-index="{{.Index}}">
  <img src="{{.Thumb}}" loading="{{.Loading}}" alt="{{.Alt}}">
</figure>
{{end}}</main>

<div id="lb">
  <button id="lb-close" title="Close (Esc)">&times;</button>
  <button class="lb-nav" id="lb-prev" title="Previous (←)"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="15 18 9 12 15 6"/></svg></button>
  <img id="lb-img" src="" alt="">
  <button class="lb-nav" id="lb-next" title="Next (→)"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg></button>
  <div id="lb-counter"></div>
</div>

<footer>
  <span>Built with <svg width="13" height="13" viewBox="0 0 24 24" fill="var(--accent)" aria-hidden="true" style="vertical-align:-1px"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg> <a href="https://huepattl.de/products/unterlumen.html" target="_blank" rel="noopener">Unterlumen</a></span>
  <a href="https://github.com/bjblazko/unterlumen" target="_blank" rel="noopener" title="View on GitHub"><svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg></a>
  {{- if or .Nav.HasAbout .Nav.HasImprint .Nav.ContactEmail .Nav.ContactURL}}
  <div class="footer-contact">
    {{- if .Nav.HasAbout}}<a href="../../about.html">About</a>{{end}}
    {{- if .Nav.HasImprint}}<a href="../../legal.html">Legal</a>{{end}}
    {{- if .Nav.ContactEmail}}<a href="mailto:{{.Nav.ContactEmail}}">{{.Nav.ContactEmail}}</a>{{end}}
    {{- if .Nav.ContactURL}}<a href="{{.Nav.ContactURL}}" target="_blank" rel="noopener">{{.Nav.ContactURL}}</a>{{end}}
  </div>
  {{- end}}
</footer>

<script>
const photos = {{.PhotosJSON}};
let cur = 0;

function open(idx) {
  cur = idx;
  const lb = document.getElementById('lb');
  document.getElementById('lb-img').src = photos[idx].full;
  lb.classList.add('open');
  document.body.style.overflow = 'hidden';
  updateCounter();
}

function close() {
  document.getElementById('lb').classList.remove('open');
  document.getElementById('lb-img').src = '';
  document.body.style.overflow = '';
}

function prev() { open((cur - 1 + photos.length) % photos.length); }
function next() { open((cur + 1) % photos.length); }

function updateCounter() {
  document.getElementById('lb-counter').textContent = (cur + 1) + ' / ' + photos.length;
}

document.getElementById('lb-close').addEventListener('click', close);
document.getElementById('lb-prev').addEventListener('click', prev);
document.getElementById('lb-next').addEventListener('click', next);
document.getElementById('lb').addEventListener('click', e => { if (e.target === e.currentTarget) close(); });

document.addEventListener('keydown', e => {
  if (!document.getElementById('lb').classList.contains('open')) return;
  if (e.key === 'ArrowLeft')  { e.preventDefault(); prev(); }
  if (e.key === 'ArrowRight') { e.preventDefault(); next(); }
  if (e.key === 'Escape')     { e.preventDefault(); close(); }
});

document.querySelectorAll('#gallery figure').forEach(fig => {
  fig.addEventListener('click', () => open(parseInt(fig.dataset.index, 10)));
});

let swipeStartX = 0, swipeStartY = 0;
document.getElementById('lb').addEventListener('touchstart', e => {
  swipeStartX = e.changedTouches[0].clientX;
  swipeStartY = e.changedTouches[0].clientY;
}, { passive: true });
document.getElementById('lb').addEventListener('touchend', e => {
  const dx = swipeStartX - e.changedTouches[0].clientX;
  const dy = swipeStartY - e.changedTouches[0].clientY;
  if (Math.abs(dx) > Math.abs(dy) && Math.abs(dx) > 40) {
    if (dx > 0) next(); else prev();
  }
}, { passive: true });

const menuBtn = document.getElementById('menu-btn');
if (menuBtn) {
  menuBtn.addEventListener('click', function(e) {
    e.stopPropagation();
    const inner = menuBtn.nextElementSibling;
    const isOpen = inner.classList.toggle('open');
    menuBtn.setAttribute('aria-expanded', isOpen);
  });
  document.addEventListener('click', function() {
    const inner = menuBtn.nextElementSibling;
    if (inner) {
      inner.classList.remove('open');
      menuBtn.setAttribute('aria-expanded', 'false');
    }
  });
}
</script>
</body>
</html>
`))

type siteGalleryPhoto struct {
	Full  string `json:"full"`
	Thumb string `json:"thumb"`
}

// GenerateSiteGallery produces an album index.html for site mode.
// It references shared assets via ../../assets/ instead of embedding CSS.
func GenerateSiteGallery(title, defaultTheme string, items []GalleryItem, opts GalleryOptions) []byte {
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	total := len(items)
	photos := make([]siteGalleryPhoto, 0, total)
	figures := make([]galleryFigureData, 0, total)
	for i, item := range items {
		photos = append(photos, siteGalleryPhoto{Full: item.Filename, Thumb: item.ThumbFilename})
		loading := "lazy"
		if i < 2 {
			loading = "eager"
		}
		alt := fmt.Sprintf("%s – Photo %d of %d", title, i+1, total)
		figures = append(figures, galleryFigureData{
			Index:   i,
			Thumb:   item.ThumbFilename,
			Full:    item.Filename,
			Loading: loading,
			Alt:     alt,
		})
	}
	photosJSON, _ := json.Marshal(photos)

	dateStr := opts.DateStr
	description := fmt.Sprintf("A collection of %d photo", total)
	if total != 1 {
		description += "s"
	}
	if dateStr != "" {
		description += ", " + dateStr
	}
	description += "."

	pageTitle := title
	if opts.SiteTitle != "" {
		pageTitle = title + " | " + opts.SiteTitle
	}

	ldMap := map[string]any{
		"@context":      "https://schema.org",
		"@type":         "ImageGallery",
		"name":          title,
		"description":   description,
		"numberOfItems": total,
	}
	if !opts.PublishedAt.IsZero() {
		ldMap["datePublished"] = opts.PublishedAt.UTC().Format("2006-01-02")
	}
	albumURL := ""
	coverURL := ""
	if opts.SiteURL != "" && opts.AlbumSlug != "" {
		base := strings.TrimRight(opts.SiteURL, "/")
		albumURL = base + "/albums/" + opts.AlbumSlug + "/"
		coverURL = albumURL + "cover.jpg"
		ldMap["url"] = albumURL
		ldMap["image"] = coverURL
	}
	ldJSON, _ := json.Marshal(ldMap)

	var buf bytes.Buffer
	siteGalleryTmpl.Execute(&buf, struct { //nolint:errcheck
		Title        string
		PageTitle    string
		DefaultTheme string
		Description  string
		PhotosJSON   template.JS
		LDJSON       template.JS
		ZipFilename  string
		Figures      []galleryFigureData
		SiteURL      string
		AlbumURL     string
		CoverURL     string
		Nav          SiteNavContext
	}{
		Title:        title,
		PageTitle:    pageTitle,
		DefaultTheme: defaultTheme,
		Description:  description,
		PhotosJSON:   template.JS(photosJSON),
		LDJSON:       template.JS(ldJSON),
		ZipFilename:  opts.ZipFilename,
		Figures:      figures,
		SiteURL:      opts.SiteURL,
		AlbumURL:     albumURL,
		CoverURL:     coverURL,
		Nav:          opts.Nav,
	})
	return buf.Bytes()
}

/* --- About page --- */

var siteAboutTmpl = template.Must(template.New("siteabout").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>About | {{.SiteTitle}}</title>
<meta name="description" content="About {{.SiteTitle}}">
<link rel="stylesheet" href="assets/style.css">
<script src="assets/toggle.js"></script>
</head>
<body>
<header>
  <div class="site-masthead">
    <div class="site-brand">
      {{- if .Nav.LogoExists}}
      <img class="site-logo" src="{{.Nav.LogoPath}}" alt="" loading="eager">
      {{- end}}
      <a class="site-name" href="index.html">{{.Nav.SiteName}}</a>
    </div>
    <div class="header-actions">
      <button id="theme-toggle" class="theme-btn">Dark</button>
    </div>
  </div>
  <div class="page-title">
    <a class="site-back" href="index.html" title="Back to albums"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="vertical-align:-4px"><polyline points="15 18 9 12 15 6"/></svg></a>About
  </div>
</header>

<main>
  <div class="about-layout">
    {{- if .AvatarExists}}
    <img class="about-photo" src="assets/avatar.jpg" alt="Portrait">
    {{- end}}
    <article class="prose">{{.Content}}</article>
  </div>
</main>

<footer>
  <span>Built with <svg width="13" height="13" viewBox="0 0 24 24" fill="var(--accent)" aria-hidden="true" style="vertical-align:-1px"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg> <a href="https://huepattl.de/products/unterlumen.html" target="_blank" rel="noopener">Unterlumen</a></span>
  {{- if .Nav.HasImprint}}<a href="legal.html" style="color:var(--text-muted);text-decoration:none;font-size:.75rem">Legal</a>{{end}}
  {{- if or .Nav.ContactEmail .Nav.ContactURL}}
  <div class="footer-contact">
    {{- if .Nav.ContactEmail}}<a href="mailto:{{.Nav.ContactEmail}}">{{.Nav.ContactEmail}}</a>{{end}}
    {{- if .Nav.ContactURL}}<a href="{{.Nav.ContactURL}}" target="_blank" rel="noopener">{{.Nav.ContactURL}}</a>{{end}}
  </div>
  {{- end}}
</footer>
</body>
</html>
`))

// generateAboutPage produces about.html at the site root.
// Does nothing if SiteAbout is empty.
func generateAboutPage(siteDir string, ch *channels.Channel, avatarExists bool, nav SiteNavContext) error {
	if ch.SiteAbout == "" {
		return nil
	}
	defaultTheme := ch.SiteTheme
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	var buf bytes.Buffer
	if err := siteAboutTmpl.Execute(&buf, struct {
		SiteTitle    string
		DefaultTheme string
		Content      template.HTML
		AvatarExists bool
		Nav          SiteNavContext
	}{
		SiteTitle:    ch.SiteTitle,
		DefaultTheme: defaultTheme,
		Content:      markdownToHTML(ch.SiteAbout),
		AvatarExists: avatarExists,
		Nav:          nav,
	}); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(siteDir, "about.html"), buf.Bytes(), 0o644)
}

/* --- Imprint / legal page --- */

var siteImprintTmpl = template.Must(template.New("siteimprint").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Legal | {{.SiteTitle}}</title>
<meta name="description" content="Legal notice for {{.SiteTitle}}">
<link rel="stylesheet" href="assets/style.css">
<script src="assets/toggle.js"></script>
</head>
<body>
<header>
  <div class="site-masthead">
    <div class="site-brand">
      {{- if .Nav.LogoExists}}
      <img class="site-logo" src="{{.Nav.LogoPath}}" alt="" loading="eager">
      {{- end}}
      <a class="site-name" href="index.html">{{.Nav.SiteName}}</a>
    </div>
    <div class="header-actions">
      <button id="theme-toggle" class="theme-btn">Dark</button>
    </div>
  </div>
  <div class="page-title">
    <a class="site-back" href="index.html" title="Back to albums"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="vertical-align:-4px"><polyline points="15 18 9 12 15 6"/></svg></a>Legal Notice
  </div>
</header>

<main>
  <article class="prose">{{.Content}}</article>
</main>

<footer>
  <span>Built with <svg width="13" height="13" viewBox="0 0 24 24" fill="var(--accent)" aria-hidden="true" style="vertical-align:-1px"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg> <a href="https://huepattl.de/products/unterlumen.html" target="_blank" rel="noopener">Unterlumen</a></span>
  {{- if .Nav.HasAbout}}<a href="about.html" style="color:var(--text-muted);text-decoration:none;font-size:.75rem">About</a>{{end}}
  {{- if or .Nav.ContactEmail .Nav.ContactURL}}
  <div class="footer-contact">
    {{- if .Nav.ContactEmail}}<a href="mailto:{{.Nav.ContactEmail}}">{{.Nav.ContactEmail}}</a>{{end}}
    {{- if .Nav.ContactURL}}<a href="{{.Nav.ContactURL}}" target="_blank" rel="noopener">{{.Nav.ContactURL}}</a>{{end}}
  </div>
  {{- end}}
</footer>
</body>
</html>
`))

// generateImprintPage produces legal.html at the site root.
// Does nothing if SiteImprint is empty.
func generateImprintPage(siteDir string, ch *channels.Channel, nav SiteNavContext) error {
	if ch.SiteImprint == "" {
		return nil
	}
	defaultTheme := ch.SiteTheme
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	var buf bytes.Buffer
	if err := siteImprintTmpl.Execute(&buf, struct {
		SiteTitle    string
		DefaultTheme string
		Content      template.HTML
		Nav          SiteNavContext
	}{
		SiteTitle:    ch.SiteTitle,
		DefaultTheme: defaultTheme,
		Content:      markdownToHTML(ch.SiteImprint),
		Nav:          nav,
	}); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(siteDir, "legal.html"), buf.Bytes(), 0o644)
}
