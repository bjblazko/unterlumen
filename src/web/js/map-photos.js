// MapPhotos — the column beside the Map: the photos in the part of the map
// on screen, newest first, as square tiles (ADR-0039). Closed, the map has
// the whole width; the place decides when it opens.

const MAP_PHOTOS_CHUNK = 120;

class MapPhotos {
    // onOpen(photos, index): open these photos in the viewer at index.
    // onClose(): Done was pressed.
    constructor({ onOpen, onClose }) {
        this._onOpen = onOpen;
        this._photos = [];
        this._rendered = 0;

        this.el = document.createElement('aside');
        this.el.className = 'map-photos';
        this.el.id = 'map-photos';
        this.el.setAttribute('aria-label', 'Photos in this part of the map');
        this.el.innerHTML = `
            <div class="map-photos-head">
                <span class="map-photos-title"></span>
                <button class="btn btn-sm map-photos-close">Done</button>
            </div>
            <div class="map-photos-body">
                <div class="map-photos-grid"></div>
                <p class="map-photos-empty" hidden>No photos in this part of the map. Move the map or zoom out.</p>
                <div class="map-photos-sentinel"></div>
            </div>`;
        this._title = this.el.querySelector('.map-photos-title');
        this._grid = this.el.querySelector('.map-photos-grid');
        this._empty = this.el.querySelector('.map-photos-empty');
        this._body = this.el.querySelector('.map-photos-body');
        this.el.querySelector('.map-photos-close').addEventListener('click', onClose);
        this._grid.addEventListener('click', (e) => {
            const tile = e.target.closest('.map-photo');
            if (tile) this._onOpen(this._photos, Number(tile.dataset.index));
        });
        // Tiles come in chunks as the column scrolls, so ten thousand photos
        // in view do not become ten thousand images at once.
        new IntersectionObserver((entries) => {
            if (entries.some(e => e.isIntersecting)) this._renderChunk();
        }, { root: this._body, rootMargin: '400px' }).observe(this.el.querySelector('.map-photos-sentinel'));
    }

    // photos: newest first.
    show(photos) {
        this._photos = photos;
        this._rendered = 0;
        this._grid.innerHTML = '';
        this._body.scrollTop = 0;
        this._title.textContent = `${formatCount(photos.length)} ${photos.length === 1 ? 'photo' : 'photos'}`;
        this._empty.hidden = photos.length > 0;
        this._renderChunk();
    }

    _renderChunk() {
        const end = Math.min(this._rendered + MAP_PHOTOS_CHUNK, this._photos.length);
        let html = '';
        for (let i = this._rendered; i < end; i++) {
            const p = this._photos[i];
            html += `<button type="button" class="map-photo" data-index="${i}" aria-label="${escapeHtml(markerLabel(p, 1))}">` +
                `<img src="${LibraryAPI.thumbURL(p.lib, p.id)}" alt="" loading="lazy" decoding="async"></button>`;
        }
        this._grid.insertAdjacentHTML('beforeend', html);
        this._rendered = end;
    }
}
