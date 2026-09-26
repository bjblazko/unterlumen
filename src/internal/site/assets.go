package site

import (
	"os"
)

// WriteAssets writes style.css, toggle.js, and lightbox.js into assetsDir,
// overwriting if present. toggle.js is fully static — it reads the default
// theme from data-default-theme on <html>. lightbox.js is also fully
// static — it reads its per-page photo list from the "ul-photos"
// application/json element on the page, rather than from a template
// variable, specifically so this file's content never needs to vary and it
// can be referenced (not inlined) from every album page. That split exists
// because a strict Content-Security-Policy (script-src 'self', no
// 'unsafe-inline') — as this project's own deployment docs recommend for a
// site hosting this generator's output — silently blocks any inline
// <script> from running at all, with no visible error to the visitor.
func WriteAssets(assetsDir string) error {
	if err := os.MkdirAll(assetsDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(assetsDir+"/style.css", []byte(siteCSS), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(assetsDir+"/toggle.js", []byte(siteToggleJS), 0o644); err != nil {
		return err
	}
	return os.WriteFile(assetsDir+"/lightbox.js", []byte(siteLightboxJS), 0o644)
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

// siteLightboxJS is a fully static lightbox/swipe/menu script shared by every
// album page. It reads its per-page photo list from a sibling
// application/json <script id="ul-photos"> element instead of a template
// variable — see WriteAssets for why this file has to be external and
// content-identical across pages.
const siteLightboxJS = `const photos = JSON.parse(document.getElementById('ul-photos').textContent);
let cur = 0;

function openLightbox(idx) {
  cur = idx;
  const lb = document.getElementById('lb');
  document.getElementById('lb-img').src = photos[idx].full;
  lb.classList.add('open');
  document.body.style.overflow = 'hidden';
  updateCounter();
}

function closeLightbox() {
  document.getElementById('lb').classList.remove('open');
  document.getElementById('lb-img').src = '';
  document.body.style.overflow = '';
}

function prev() { openLightbox((cur - 1 + photos.length) % photos.length); }
function next() { openLightbox((cur + 1) % photos.length); }

function updateCounter() {
  document.getElementById('lb-counter').textContent = (cur + 1) + ' / ' + photos.length;
}

document.getElementById('lb-close').addEventListener('click', closeLightbox);
document.getElementById('lb-prev').addEventListener('click', prev);
document.getElementById('lb-next').addEventListener('click', next);
document.getElementById('lb').addEventListener('click', e => { if (e.target === e.currentTarget) closeLightbox(); });

document.addEventListener('keydown', e => {
  if (!document.getElementById('lb').classList.contains('open')) return;
  if (e.key === 'ArrowLeft')  { e.preventDefault(); prev(); }
  if (e.key === 'ArrowRight') { e.preventDefault(); next(); }
  if (e.key === 'Escape')     { e.preventDefault(); closeLightbox(); }
});

document.querySelectorAll('#gallery figure').forEach(fig => {
  fig.addEventListener('click', () => openLightbox(parseInt(fig.dataset.index, 10)));
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
`

/* --- Site root index --- */
