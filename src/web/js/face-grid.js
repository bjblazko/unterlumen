// face-grid.js — pictures of faces, each one a switch: in or out of what
// the next action does. People's group and person pages are built on it.

class FaceGrid {
    // faces: [{ id, photoId, x, y, w, h }], none of them switched on at the
    // start. onChange is called with the selection after every switch;
    // onOpen with the index of a face that is double-clicked. cover is the
    // face that shows the person, marked as such.
    constructor({ lib, faces, cover = 0, onChange = () => {}, onOpen = () => {} }) {
        this.selected = new Set();
        this.el = document.createElement('div');
        this.el.className = 'face-grid';
        this.el.innerHTML = faces.map((f, i) => `
            <button type="button" class="face-tile" data-face="${f.id}" data-index="${i}"
                aria-pressed="false" aria-label="Face ${i + 1}">
                <img src="${PeopleAPI.faceURL(lib, f.id)}" alt="" loading="lazy" width="96" height="96">
                ${f.id === cover ? '<span class="face-tile-cover">Cover</span>' : ''}
                <svg class="face-tile-mark" width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
                    <rect width="16" height="16" rx="2"/><path d="M4 8.5l2.5 2.5L12 5.5" fill="none" stroke-width="1.8"/>
                </svg>
            </button>`).join('');
        this.el.addEventListener('click', (e) => {
            const tile = e.target.closest('.face-tile');
            if (!tile) return;
            const id = Number(tile.dataset.face);
            const on = !this.selected.has(id);
            if (on) this.selected.add(id); else this.selected.delete(id);
            tile.setAttribute('aria-pressed', String(on));
            onChange(this.selected);
        });
        this.el.addEventListener('dblclick', (e) => {
            const tile = e.target.closest('.face-tile');
            if (tile) onOpen(Number(tile.dataset.index));
        });
        this._peek = new FacePeek(lib, faces, this.el);
    }

    // clear switches every face off, without telling onChange.
    clear() {
        this.selected.clear();
        for (const tile of this.el.querySelectorAll('.face-tile[aria-pressed="true"]')) tile.setAttribute('aria-pressed', 'false');
    }
}

// FacePeek shows a face in its whole photo, the face framed and the rest
// dimmed, while the pointer rests on its tile or the tile has keyboard
// focus: a small face is often recognised only by where it is, and in a
// group photo the frame says which of the people is meant.
class FacePeek {
    // Long enough that a pointer passing on its way to a face shows nothing.
    static DELAY = 450;

    static pct(v) { return `${(v * 100).toFixed(2)}%`; }

    // grid holds the faces' elements, found by selector, each with the index
    // of its face in faces as data-index.
    constructor(lib, faces, grid, selector = '.face-tile') {
        this._lib = lib;
        this._faces = faces;
        this._sel = selector;
        this._timer = 0;
        this._el = null;
        this._tile = null;
        // A page drawn anew (a cover chosen, a pass finished) takes the
        // tile away without a pointerout or a focusout: the peek checks on
        // every frame while it shows, and goes with its tile.
        this._watch = () => {
            if (!this._tile?.isConnected) { this.hide(); return; }
            this._frame = requestAnimationFrame(this._watch);
        };
        grid.addEventListener('pointerover', (e) => {
            const tile = e.target.closest(this._sel);
            if (!tile || e.pointerType === 'touch') return;
            clearTimeout(this._timer);
            this._timer = setTimeout(() => this.show(tile), FacePeek.DELAY);
        });
        grid.addEventListener('pointerout', (e) => {
            const tile = e.target.closest(this._sel);
            if (tile && !tile.contains(e.relatedTarget)) this.hide();
        });
        grid.addEventListener('focusin', (e) => {
            const tile = e.target.closest(this._sel);
            if (tile && tile.matches(':focus-visible')) this.show(tile);
        });
        grid.addEventListener('focusout', () => this.hide());
        grid.addEventListener('keydown', (e) => { if (e.key === 'Escape') this.hide(); });
        grid.addEventListener('dblclick', () => this.hide());
    }

    show(tile) {
        const f = this._faces[Number(tile.dataset.index)];
        if (!f || !tile.isConnected) return;
        this.hide();
        const el = document.createElement('div');
        el.className = 'face-peek';
        el.setAttribute('role', 'tooltip');
        el.innerHTML = `
            <div class="face-peek-photo">
                <img src="${LibraryAPI.thumbURL(this._lib, f.photoId)}" alt="">
                <span class="face-peek-frame" style="left:${FacePeek.pct(f.x)};top:${FacePeek.pct(f.y)};width:${FacePeek.pct(f.w)};height:${FacePeek.pct(f.h)}"></span>
            </div>`;
        document.body.appendChild(el);
        this._el = el;
        this._tile = tile;
        this._frame = requestAnimationFrame(this._watch);
        tile.setAttribute('aria-describedby', 'face-peek');
        el.id = 'face-peek';
        const img = el.querySelector('img');
        const place = () => this._place(el, tile);
        if (img.complete) place(); else img.addEventListener('load', place, { once: true });
        place();
    }

    // Beside the tile, on the side with more room, kept inside the window
    // and clear of a bar at the bottom of the page, where the work is.
    _place(el, tile) {
        if (!tile.isConnected) { this.hide(); return; }
        const gap = 8, margin = 16;
        const t = tile.getBoundingClientRect();
        const r = el.getBoundingClientRect();
        const bar = [...document.querySelectorAll('.people-bar')].find(b => !b.hidden && b.offsetParent);
        const bottom = Math.min(window.innerHeight, bar ? bar.getBoundingClientRect().top : Infinity) - margin;
        const right = window.innerWidth - t.right;
        let left = right >= r.width + gap + margin || right >= t.left ? t.right + gap : t.left - gap - r.width;
        left = Math.max(margin, Math.min(left, window.innerWidth - r.width - margin));
        const top = Math.max(margin, Math.min(t.top + t.height / 2 - r.height / 2, bottom - r.height));
        el.style.left = `${left}px`;
        el.style.top = `${top}px`;
    }

    hide() {
        clearTimeout(this._timer);
        cancelAnimationFrame(this._frame);
        this._tile = null;
        if (!this._el) return;
        document.querySelector('[aria-describedby="face-peek"]')?.removeAttribute('aria-describedby');
        this._el.remove();
        this._el = null;
    }
}
