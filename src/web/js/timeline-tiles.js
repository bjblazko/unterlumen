// TimelineTiles — the photo tiles of a timeline lane, kept only while near
// the screen (ADR-0040). show() adds the tiles that are missing and removes
// the rest; a removed tile's image stops loading and is released, so the
// number of images in the page stays bounded however long the stream is.

class TimelineTiles {
    constructor(layer, stream) {
        this.layer = layer;
        this.stream = stream;
        this.tiles = new Map();
        this.selected = -1;
    }

    get size() { return this.tiles.size; }

    show(items) {
        const keep = new Set();
        for (const it of items) {
            keep.add(it.i);
            let el = this.tiles.get(it.i);
            if (!el) {
                el = this._create(it.i);
                this.tiles.set(it.i, el);
                this.layer.appendChild(el);
            }
            const s = el.style;
            s.left = it.x.toFixed(1) + 'px';
            s.top = it.y.toFixed(1) + 'px';
            s.width = it.w.toFixed(1) + 'px';
            s.height = it.h.toFixed(1) + 'px';
        }
        for (const [i, el] of this.tiles) {
            if (!keep.has(i)) { this._release(el); this.tiles.delete(i); }
        }
    }

    // fill gives the tiles whose details have arrived their image.
    fill() {
        for (const [i, el] of this.tiles) if (!el.firstChild) this._image(el, i);
    }

    select(i) {
        this.tiles.get(this.selected)?.classList.remove('is-selected');
        this.selected = i;
        this.tiles.get(i)?.classList.add('is-selected');
    }

    clear() {
        for (const el of this.tiles.values()) this._release(el);
        this.tiles.clear();
    }

    _create(i) {
        const el = document.createElement('div');
        el.className = 'timeline-tile' + (i === this.selected ? ' is-selected' : '');
        el.dataset.i = i;
        this._image(el, i);
        return el;
    }

    _image(el, i) {
        const p = this.stream.detail(i);
        if (!p) return;
        const img = document.createElement('img');
        img.alt = p.name;
        img.decoding = 'async';
        img.draggable = false;
        img.src = LibraryAPI.thumbURL(p.lib, p.id);
        el.appendChild(img);
    }

    _release(el) {
        el.firstChild?.removeAttribute('src');
        el.remove();
    }
}
