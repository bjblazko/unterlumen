package apilibrary

import (
	"bytes"
	"encoding/json"
	"html/template"
	"os"
	"sort"
	"time"
)

// SitePhoto stores the filenames needed to regenerate an album page without re-exporting.
type SitePhoto struct {
	Filename      string `json:"filename"`
	ThumbFilename string `json:"thumbFilename"`
}

// SiteAlbum records metadata for one published album in the site statefile.
type SiteAlbum struct {
	PostID      string      `json:"postID"`
	Title       string      `json:"title"`
	PublishedAt time.Time   `json:"publishedAt"`
	PhotoCount  int         `json:"photoCount"`
	CoverFile   string      `json:"coverFile"` // relative to the album dir, e.g. "cover.jpg"
	HasZip      bool        `json:"hasZip"`
	Photos      []SitePhoto `json:"photos"` // stored so album pages can be rebuilt without re-export
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
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
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
}

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
  color: var(--text-dim);
  text-decoration: none;
  font-weight: 400;
  font-size: 1.1rem;
  margin-right: 0.5rem;
}
.site-back:hover { color: var(--accent); }

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
  display: flex;
  gap: 0.75rem;
  align-items: center;
  flex-shrink: 0;
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
  background: rgba(0,0,0,0.93);
  z-index: 9999;
  align-items: center;
  justify-content: center;
}
#lb.open { display: flex; }
#lb-img {
  max-width: 92vw; max-height: 92vh;
  object-fit: contain;
  border-radius: 2px;
  box-shadow: 0 8px 40px rgba(0,0,0,0.6);
  user-select: none;
}
#lb-close {
  position: fixed; top: 1.2rem; right: 1.5rem;
  background: none; border: none;
  color: #fff; font-size: 2rem; line-height: 1;
  cursor: pointer; opacity: 0.7; padding: 0.3rem 0.5rem;
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
	PostID     string
	Title      string
	DateStr    string
	PhotoCount int
	CoverFile  string
}

var siteTmpl = template.Must(template.New("site").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<link rel="stylesheet" href="assets/style.css">
<script src="assets/toggle.js"></script>
</head>
<body>
<header>
  <h1>{{.Title}}</h1>
  <div class="header-actions">
    <button id="theme-toggle" class="theme-btn">Dark</button>
  </div>
</header>

<main class="albums">
{{range .Albums}}  <a class="album-card" href="albums/{{.PostID}}/index.html">
    <img class="album-cover" src="albums/{{.PostID}}/{{.CoverFile}}" alt="{{.Title}}" loading="lazy">
    <div class="album-title">{{.Title}}</div>
    <div class="album-meta">{{.DateStr}} &middot; {{.PhotoCount}} photo{{if ne .PhotoCount 1}}s{{end}}</div>
  </a>
{{end}}</main>

<footer>Built with Unterlumen</footer>
</body>
</html>
`))

// GenerateSiteIndex produces a static root index.html referencing shared assets.
// Albums are ordered newest first by PublishedAt.
func GenerateSiteIndex(siteTitle, defaultTheme string, albums []SiteAlbum) []byte {
	if siteTitle == "" {
		siteTitle = "Photo Albums"
	}
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	// Sort newest first by publish date.
	sorted := make([]SiteAlbum, len(albums))
	copy(sorted, albums)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
	})
	items := make([]siteAlbumData, len(sorted))
	for i, a := range sorted {
		items[i] = siteAlbumData{
			PostID:     a.PostID,
			Title:      a.Title,
			DateStr:    a.PublishedAt.Format("January 2006"),
			PhotoCount: a.PhotoCount,
			CoverFile:  a.CoverFile,
		}
	}
	var buf bytes.Buffer
	siteTmpl.Execute(&buf, struct { //nolint:errcheck
		Title        string
		DefaultTheme string
		Albums       []siteAlbumData
	}{Title: siteTitle, DefaultTheme: defaultTheme, Albums: items})
	return buf.Bytes()
}

/* --- Site album gallery --- */

var siteGalleryTmpl = template.Must(template.New("sitegallery").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<link rel="stylesheet" href="../../assets/style.css">
<script src="../../assets/toggle.js"></script>
</head>
<body>
<header>
  <h1><a class="site-back" href="../../index.html" title="Back to albums">&#8592;</a>{{.Title}}</h1>
  <div class="header-actions">
    {{if .ZipFilename}}<a class="dl-btn" href="{{.ZipFilename}}" download>&#8595; Download all photos</a>{{end}}
    <button id="theme-toggle" class="theme-btn">Dark</button>
  </div>
</header>

<main class="gallery" id="gallery"></main>

<div id="lb">
  <button id="lb-close" title="Close (Esc)">&times;</button>
  <button class="lb-nav" id="lb-prev" title="Previous (←)">&#8249;</button>
  <img id="lb-img" src="" alt="">
  <button class="lb-nav" id="lb-next" title="Next (→)">&#8250;</button>
  <div id="lb-counter"></div>
</div>

<script>
const photos = {{.PhotosJSON}};
let cur = 0;

function buildGallery() {
  const g = document.getElementById('gallery');
  photos.forEach((p, i) => {
    const fig = document.createElement('figure');
    const img = document.createElement('img');
    img.src = p.thumb; img.loading = 'lazy'; img.alt = p.full;
    fig.appendChild(img);
    fig.addEventListener('click', () => open(i));
    g.appendChild(fig);
  });
}

function open(idx) {
  cur = idx;
  const lb = document.getElementById('lb');
  const img = document.getElementById('lb-img');
  img.src = photos[idx].full; img.alt = photos[idx].full;
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

buildGallery();
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
	photos := make([]siteGalleryPhoto, 0, len(items))
	for _, item := range items {
		photos = append(photos, siteGalleryPhoto{Full: item.Filename, Thumb: item.ThumbFilename})
	}
	photosJSON, _ := json.Marshal(photos)
	var buf bytes.Buffer
	siteGalleryTmpl.Execute(&buf, struct { //nolint:errcheck
		Title        string
		DefaultTheme string
		PhotosJSON   template.JS
		ZipFilename  string
	}{
		Title:        title,
		DefaultTheme: defaultTheme,
		PhotosJSON:   template.JS(photosJSON),
		ZipFilename:  opts.ZipFilename,
	})
	return buf.Bytes()
}
