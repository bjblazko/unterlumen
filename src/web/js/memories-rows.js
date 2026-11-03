// memories-rows.js — all photos of a memory as justified rows: each photo at
// its own shape, a row's photos the same height, a chunk at a time.

const MEMORIES_GAP = SHEET_GAP;
const MEMORIES_CHUNK = 120;

// memoriesRow lays out photos in one row of the width: as tall as height,
// lower when they would not fit.
function memoriesRow(photos, width, height) {
    const sum = photos.reduce((n, p) => n + (p.ratio || 1.5), 0);
    const h = Math.max(1, Math.min(height, (width - MEMORIES_GAP * (photos.length - 1)) / sum));
    return photos.map(p => ({ photo: p, w: Math.floor((p.ratio || 1.5) * h), h: Math.floor(h) }));
}

// memoriesRows breaks photos into rows that fill the width at about height;
// the last row keeps height and does not stretch.
function memoriesRows(photos, width, height) {
    const rows = [];
    let row = [];
    let sum = 0;
    for (const p of photos) {
        row.push(p);
        sum += p.ratio || 1.5;
        if (sum * height + MEMORIES_GAP * (row.length - 1) >= width) {
            rows.push(memoriesRow(row, width, Infinity));
            row = [];
            sum = 0;
        }
    }
    if (row.length) rows.push(memoriesRow(row, width, height));
    return rows;
}

function memoriesTileHTML({ photo, w, h }, index) {
    return `<button type="button" class="memories-tile" data-index="${index}" style="width:${w}px;height:${h}px"
        aria-label="${escapeHtml(photo.name)}"><img src="${LibraryAPI.thumbURL(photo.lib, photo.id)}" alt="" loading="lazy" decoding="async"></button>`;
}

function memoriesRowHTML(cells, offset = 0) {
    return `<div class="memories-row">${cells.map((c, i) => memoriesTileHTML(c, offset + i)).join('')}</div>`;
}

// openMemoryPhotos opens the photos in the read-only viewer at index.
function openMemoryPhotos(photos, index) {
    openLibraryPhotos(photos.map(p => ({ lib: p.lib, id: p.id, name: p.name, taken: p.taken })), index);
}

// MemoriesGrid is all photos of a memory in rows, a chunk at a time as the
// page scrolls down; a click on one opens them all in the viewer.
class MemoriesGrid {
    constructor(photos, height) {
        this._photos = photos;
        this._height = height;
        this._shown = 0;
        this.el = document.createElement('div');
        this.el.className = 'memories-grid';
        this._rows = document.createElement('div');
        this._sentinel = document.createElement('div');
        this._sentinel.className = 'memories-sentinel';
        this.el.append(this._rows, this._sentinel);
        this.el.addEventListener('click', (e) => {
            const tile = e.target.closest('.memories-tile');
            if (tile) openMemoryPhotos(this._photos, Number(tile.dataset.index));
        });
        this._observer = new IntersectionObserver((entries) => {
            if (entries.some(e => e.isIntersecting)) this._more();
        }, { rootMargin: '600px' });
        this._observer.observe(this._sentinel);
        this._resize = new ResizeObserver(() => this._relayout());
        this._resize.observe(this.el);
    }

    // destroy lets go of the observers when the page is left.
    destroy() {
        this._observer.disconnect();
        this._resize.disconnect();
    }

    _more() {
        if (this._shown >= this._photos.length || !this.el.clientWidth) return;
        this._shown = Math.min(this._photos.length, this._shown + MEMORIES_CHUNK);
        this._relayout();
    }

    // Lays out every photo shown so far again, at the width there is now.
    _relayout() {
        const width = this.el.clientWidth;
        if (!width) return;
        if (!this._shown) { this._more(); return; }
        let at = 0;
        this._rows.innerHTML = memoriesRows(this._photos.slice(0, this._shown), width, this._height).map(cells => {
            const html = memoriesRowHTML(cells, at);
            at += cells.length;
            return html;
        }).join('');
    }
}
