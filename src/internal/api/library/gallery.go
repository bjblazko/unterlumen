package apilibrary

import (
	"bytes"
	"encoding/json"
	"html/template"
)

// GalleryItem describes one photo in the exported HTML gallery.
type GalleryItem struct {
	Filename      string // full-res filename (relative to index.html)
	ThumbFilename string // thumbnail filename (relative to index.html)
	Width, Height int    // full-res dimensions
}

type galleryPhoto struct {
	Full  string `json:"full"`
	Thumb string `json:"thumb"`
}

// GalleryOptions carries optional extras for the generated gallery page.
type GalleryOptions struct {
	ZipFilename string // if non-empty, a download link for the ZIP is shown
}

var galleryTmpl = template.Must(template.New("gallery").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="dark">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<script>(function(){var t=localStorage.getItem('ul-theme')||'dark';if(t==='light')document.documentElement.classList.add('theme-light');}());</script>
<style>
/* --- Theme variables (dark default) --- */
:root {
  --bg:       #111;
  --text:     #ddd;
  --text-dim: #aaa;
  --heading:  #fff;
  --card-bg:  #222;
  --border:   #333;
  --accent:   #d35400;
}
html.theme-light {
  --bg:       #f5f2ed;
  --text:     #2a2520;
  --text-dim: #756d64;
  --heading:  #1a1714;
  --card-bg:  #e0dbd4;
  --border:   #ccc8c2;
}

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
  margin-bottom: 2rem;
}

h1 {
  font-size: 1.6rem;
  font-weight: 500;
  letter-spacing: -0.02em;
  color: var(--heading);
}

.header-actions {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  flex-shrink: 0;
}

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
}
.theme-btn:hover { color: var(--text); border-color: var(--text-dim); }

/* --- Masonry grid --- */
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
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.93);
  z-index: 9999;
  align-items: center;
  justify-content: center;
}
#lb.open { display: flex; }

#lb-img {
  max-width: 92vw;
  max-height: 92vh;
  object-fit: contain;
  border-radius: 2px;
  box-shadow: 0 8px 40px rgba(0,0,0,0.6);
  user-select: none;
}

#lb-close {
  position: fixed;
  top: 1.2rem; right: 1.5rem;
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
</style>
</head>
<body>
<header>
  <h1>{{.Title}}</h1>
  <div class="header-actions">
    {{if .ZipFilename}}<a class="dl-btn" href="{{.ZipFilename}}" download>&#8595; Download all photos</a>{{end}}
    <button id="theme-toggle" class="theme-btn">Light</button>
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

(function () {
  var root = document.documentElement;
  function applyTheme(t) { root.classList.toggle('theme-light', t === 'light'); }
  function syncBtn() {
    var btn = document.getElementById('theme-toggle');
    if (btn) btn.textContent = root.classList.contains('theme-light') ? 'Dark' : 'Light';
  }
  document.getElementById('theme-toggle').addEventListener('click', function () {
    var next = root.classList.contains('theme-light') ? 'dark' : 'light';
    applyTheme(next);
    localStorage.setItem('ul-theme', next);
    syncBtn();
  });
  syncBtn();
}());

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

// GenerateGallery returns a self-contained index.html for the given title, photos, and options.
func GenerateGallery(title string, items []GalleryItem, opts GalleryOptions) []byte {
	photos := make([]galleryPhoto, 0, len(items))
	for _, item := range items {
		photos = append(photos, galleryPhoto{Full: item.Filename, Thumb: item.ThumbFilename})
	}
	photosJSON, _ := json.Marshal(photos)

	var buf bytes.Buffer
	galleryTmpl.Execute(&buf, struct { //nolint:errcheck
		Title       string
		PhotosJSON  template.JS
		ZipFilename string
	}{
		Title:       title,
		PhotosJSON:  template.JS(photosJSON),
		ZipFilename: opts.ZipFilename,
	})
	return buf.Bytes()
}
