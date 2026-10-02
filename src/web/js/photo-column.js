// PhotoColumn — a column of photos beside a place, newest first, as square
// tiles: the Map's photos in view (ADR-0039) and the photos of a value picked
// in Statistics (ADR-0043). Closed, the place has the whole width; the place
// decides when it opens, by default with photoColumnStartsOpen.

const PHOTO_COLUMN_CHUNK = 120;

// Whether a place's column starts open. On the desk it opens by itself: there
// is room beside the map or the charts. On a phone it would take half the
// screen, so it waits for its button. Once opened or closed, the choice is
// kept in this browser under key.
function photoColumnStartsOpen(key) {
    try {
        const kept = localStorage.getItem(key);
        if (kept !== null) return kept === '1';
    } catch { /* storage blocked: fall back to the layout */ }
    return !matchMedia('(max-width: 700px)').matches;
}

function keepPhotoColumnOpen(key, open) {
    try { localStorage.setItem(key, open ? '1' : '0'); } catch { /* per browser only */ }
}

class PhotoColumn {
    // onOpen(photos, index): open these photos in the viewer at index.
    // onClose(): Done was pressed.
    // label: what the column holds, for assistive technology.
    // emptyText: the sentence shown when there is nothing to show.
    constructor({ onOpen, onClose, label, emptyText }) {
        this._onOpen = onOpen;
        this._photos = [];
        this._rendered = 0;
        this._total = 0;
        this._more = null;
        this._loading = false;

        this.el = document.createElement('aside');
        this.el.className = 'photo-column';
        this.el.setAttribute('aria-label', label);
        this.el.innerHTML = `
            <div class="photo-column-head">
                <div class="photo-column-heading">
                    <span class="photo-column-subject" hidden></span>
                    <span class="photo-column-title"></span>
                </div>
                <button class="btn btn-sm photo-column-close">Done</button>
            </div>
            <div class="photo-column-body">
                <div class="photo-column-grid"></div>
                <p class="photo-column-empty" hidden>${escapeHtml(emptyText)}</p>
                <div class="photo-column-sentinel"></div>
            </div>`;
        this._subject = this.el.querySelector('.photo-column-subject');
        this._title = this.el.querySelector('.photo-column-title');
        this._grid = this.el.querySelector('.photo-column-grid');
        this._empty = this.el.querySelector('.photo-column-empty');
        this._body = this.el.querySelector('.photo-column-body');
        this.el.querySelector('.photo-column-close').addEventListener('click', onClose);
        this._grid.addEventListener('click', (e) => {
            const tile = e.target.closest('.photo-column-tile');
            if (tile) this._onOpen(this._photos, Number(tile.dataset.index));
        });
        // Tiles come in chunks as the column scrolls, so ten thousand photos
        // in view do not become ten thousand images at once.
        new IntersectionObserver((entries) => {
            if (entries.some(e => e.isIntersecting)) this._renderChunk();
        }, { root: this._body, rootMargin: '400px' }).observe(this.el.querySelector('.photo-column-sentinel'));
    }

    // photos: newest first, each { lib, id, name, taken }.
    // subject: what they have in common, shown above their number.
    // total and more(): when there are more photos than given, their number
    // and a function that reads the next ones.
    show(photos, { subject = '', total = photos.length, more = null } = {}) {
        this._photos = photos;
        this._total = total;
        this._more = more;
        this._rendered = 0;
        this._grid.innerHTML = '';
        this._body.scrollTop = 0;
        this._subject.hidden = !subject;
        this._subject.textContent = subject;
        this._title.textContent = `${formatCount(total)} ${total === 1 ? 'photo' : 'photos'}`;
        this._empty.hidden = total > 0;
        this._renderChunk();
    }

    async _renderChunk() {
        if (this._rendered >= this._photos.length) await this._readMore();
        const end = Math.min(this._rendered + PHOTO_COLUMN_CHUNK, this._photos.length);
        let html = '';
        for (let i = this._rendered; i < end; i++) {
            const p = this._photos[i];
            html += `<button type="button" class="photo-column-tile" data-index="${i}" aria-label="${escapeHtml(photoLabel(p))}">` +
                `<img src="${LibraryAPI.thumbURL(p.lib, p.id)}" alt="" loading="lazy" decoding="async"></button>`;
        }
        this._grid.insertAdjacentHTML('beforeend', html);
        this._rendered = end;
    }

    // One read at a time; a column shown anew meanwhile drops what arrives.
    async _readMore() {
        if (!this._more || this._loading || this._photos.length >= this._total) return;
        this._loading = true;
        const photos = this._photos;
        try {
            const next = await this._more(photos.length);
            if (photos === this._photos) photos.push(...next);
        } finally {
            this._loading = false;
        }
    }
}

// A photo's name and the day it was taken, for a tile's accessible name.
function photoLabel(p) {
    const date = p.taken ? formatTakenDate(p.taken) : '';
    return date ? `${p.name}, ${date}` : p.name;
}

function formatTakenDate(iso) {
    const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
    if (!m) return '';
    return new Date(Date.UTC(+m[1], +m[2] - 1, +m[3]))
        .toLocaleDateString('en-GB', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' });
}

// Photos seen outside a library are looked at, not culled: the viewer opens
// read-only (no crop, no marking for deletion).
function openLibraryPhotos(photos, index = 0) {
    const byKey = new Map(photos.map(p => [`${p.lib}/${p.id}/${p.name}`, p]));
    const keys = [...byKey.keys()];
    App.showViewer(keys[index], keys, {
        readOnly: true,
        libraryRef: (k) => ({ lib: byKey.get(k).lib, id: byKey.get(k).id }),
        imageURLFn: (k) => LibraryAPI.photoURL(byKey.get(k).lib, byKey.get(k).id),
        thumbURLFn: (k) => LibraryAPI.thumbURL(byKey.get(k).lib, byKey.get(k).id),
        previewURLFn: (k) => LibraryAPI.thumbURL(byKey.get(k).lib, byKey.get(k).id),
        infoLoadFn: (k, panel) => {
            const p = byKey.get(k);
            panel.loadFromURL(`/api/library/${p.lib}/photo/${p.id}/info`, `lib:${p.lib}:${p.id}`);
        },
    });
}
