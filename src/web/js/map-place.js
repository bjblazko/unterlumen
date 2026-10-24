// map-place.js — the Map place: every located photo of every library on one
// map, grouped by place, the newest photo of each group on top (ADR-0039).

// Grey tiles by default, light or dark with the theme: the map is ground
// for the photos, not a picture of its own. The dark one is fiord, a
// blue-grey: OpenFreeMap's "dark" was too dark to read. Colour is the info
// panel's style; OpenFreeMap has no dark version of it.
const MAP_STYLE_URLS = {
    light: 'https://tiles.openfreemap.org/styles/positron',
    dark: 'https://tiles.openfreemap.org/styles/fiord',
    colour: 'https://tiles.openfreemap.org/styles/liberty',
};

// The world as a globe unless this browser chose otherwise: a globe shows
// the sizes of countries as they are, and close up it turns into Mercator by
// itself. Mercator is the flat map; 3D the flat map tilted.
function readProjection() {
    let view = null;
    try { view = localStorage.getItem('map-projection'); } catch { /* per browser only */ }
    return ['mercator', '3d'].includes(view) ? view : 'globe';
}

// The grey styles draw buildings flat; tilted, they stand in the grey of
// the style's own buildings. The colour style has standing buildings of its own.
function standBuildings(map) {
    const style = map.getStyle();
    if (style.layers.some(l => l.type === 'fill-extrusion') || !style.sources.openmaptiles) return;
    const flat = style.layers.find(l => l.id === 'building');
    map.addLayer({
        id: 'unterlumen-buildings-3d',
        type: 'fill-extrusion',
        source: 'openmaptiles',
        'source-layer': 'building',
        minzoom: 14,
        paint: {
            'fill-extrusion-color': flat?.paint?.['fill-color'] ?? '#d9d9d9',
            'fill-extrusion-height': ['coalesce', ['get', 'render_height'], 0],
            'fill-extrusion-base': ['coalesce', ['get', 'render_min_height'], 0],
            'fill-extrusion-opacity': 0.8,
        },
    }, flat ? style.layers[style.layers.indexOf(flat) + 1]?.id : undefined);
}

function mapStyleURL(colour) {
    if (colour) return MAP_STYLE_URLS.colour;
    return MAP_STYLE_URLS[document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'];
}

class MapPane {
    constructor(container) {
        this.container = container;
        this._map = null;
        this._markers = null;
        this._points = [];
        this._shown = [];
        this._photosOpen = photoColumnStartsOpen('map-photos-open');
        this._colour = readFlag('map-colour');
        this._projection = readProjection();
    }

    // Reads the locations again on every visit — a scan may have added some —
    // but keeps the map where it was left.
    async render() {
        if (!this.container.firstChild) this._buildLayout();
        else this._filter.sync();
        const gen = this._gen = (this._gen || 0) + 1;
        let geo, libraries;
        try {
            [geo, libraries] = await this._whileLoading(() => Promise.all([LibraryAPI.geo(ScopeState.query()), LibraryAPI.list()]));
        } catch (err) {
            this._note(t('The locations could not be read: {err}. Reload the page to try again.', { err: escapeHtml(err.message) }), true);
            return;
        }
        if (gen !== this._gen) return;
        this._points = mapPoints(geo);
        if (!this._explainNothing(libraries)) this._showPoints();
    }

    _buildLayout() {
        this.container.innerHTML = `
            <div class="map-pane">
                <div class="map-body">
                    <div class="place-main">
                ${placeHeaderHTML({
                    title: t('Map'),
                    meta: '<span class="map-count"></span>',
                    actions: `<span class="map-filter-slot"></span><span class="map-more"></span>`,
                    lede: placeLede(t('Every photo with a location, from all {libraries}. A folder no library catalogs does not appear.', { libraries: placeLink('library', 'libraries', t('libraries')) })),
                })}
                    <div class="map-view">
                        <div class="map-canvas"></div>
                        <div class="map-note" hidden></div>
                    </div>
                    </div>
                </div>
            </div>`;
        this._head = this.container.querySelector('.place-header');
        // The shared filter (ADR-0050): the server sends only the photos
        // within it. Its column and the photos' share the right edge.
        this._filter = new ScopeFilter({
            onOpen: () => { if (this._photosOpen) this._setPhotosOpen(false); this._map?.resize(); },
            onClose: () => this._map?.resize(),
        });
        this._head.querySelector('.map-filter-slot').replaceWith(this._filter.el);
        ScopeState.onChange(() => {
            if (App.mode !== 'map') return;
            clearTimeout(this._filterTimer);
            this._filterTimer = setTimeout(() => this.render(), 250);
        });
        this._photosBtn = panelToggleButton('Photos', 'map-photos-toggle');
        this._photosBtn.setAttribute('aria-controls', 'map-photos');
        this._photosBtn.hidden = true;
        this._photosBtn.addEventListener('click', () => this._setPhotosOpen(!this._photosOpen));
        this._head.querySelector('.map-more').replaceWith(...this._headSettings());
        this._photos = new PhotoColumn({
            onOpen: (photos, index) => openLibraryPhotos(photos, index),
            onClose: () => this._setPhotosOpen(false),
            label: t('Photos in this part of the map'),
            emptyText: t('No photos in this part of the map. Move the map or zoom out.'),
        });
        this._photos.el.id = 'map-photos';
        const body = this.container.querySelector('.map-body');
        body.append(this._filter.panel, this._photos.el, toolStrip(this._filter.toggle, this._photosBtn));
        this._countEl = this.container.querySelector('.map-count');
        this._canvas = this.container.querySelector('.map-canvas');
        this._noteEl = this.container.querySelector('.map-note');
        // A hash link alone does not fire popstate; go there as the sidebar does.
        this._noteEl.addEventListener('click', (e) => {
            const link = e.target.closest('a[data-mode]');
            if (!link || e.metaKey || e.ctrlKey || e.shiftKey) return;
            e.preventDefault();
            App.setMode(link.dataset.mode);
        });
    }

    // Words only after 400 ms (Activity), and only while nothing is on the
    // map yet; a revisit refreshes quietly behind what is already there.
    async _whileLoading(read) {
        if (this._map) return read();
        this._noteEl.hidden = false;
        Activity.in(this._noteEl, t('Reading the locations…'), { area: true });
        const result = await read();
        this._noteEl.hidden = true;
        this._noteEl.innerHTML = ''; // ends the activity line and its clock
        return result;
    }

    // Says why there is nothing to show, and returns whether it did.
    _explainNothing(libraries) {
        if (libraries.length === 0) {
            this._note('No libraries yet. Photos appear here once they are in a library — <a href="#libraries" data-mode="library">add one under Libraries</a>.');
        } else if (this._points.length === 0 && ScopeState.narrowed()) {
            this._note(t('No photo with a location matches the filter. Widen it, or reset it.'));
        } else if (this._points.length === 0) {
            const total = libraries.reduce((n, l) => n + (l.photoCount || 0), 0);
            this._note(t('None of the {total} photos in your libraries has a location.', { total: formatCount(total) }));
        } else {
            return false;
        }
        this._canvas.hidden = true;
        this._countEl.textContent = '';
        this._photosBtn.hidden = true;
        this._photos.el.hidden = true;
        return true;
    }

    _note(html, isError = false) {
        this._noteEl.hidden = false;
        this._noteEl.classList.toggle('map-note-error', isError);
        this._noteEl.innerHTML = `<p>${html}</p>`;
    }

    _showPoints() {
        this._canvas.hidden = false;
        this._noteEl.hidden = true;
        this._photosBtn.hidden = false;
        this._drawPhotosOpen();
        if (!this._map) {
            this._buildMap();
            return;
        }
        this._map.resize();
        this._applyRange();
    }

    // The Map's settings stand in its head, views you switch while looking:
    // a ⋯ would hide them.
    _headSettings() {
        return [
            segmentField({
                label: t('View'), options: [['globe', t('Globe')], ['mercator', t('Mercator')], ['3d', t('3D')]], value: this._projection,
                onChange: (v) => this._setProjection(v),
            }),
            segmentField({
                label: t('Map'), options: [['grey', t('Grey')], ['colour', t('Colour')]], value: this._colour ? 'colour' : 'grey',
                onChange: (v) => this._setColour(v === 'colour'),
            }),
        ];
    }

    _buildMap() {
        if (typeof maplibregl === 'undefined') {
            this._note(t('The map library could not be loaded. Reload the page to try again.'), true);
            return;
        }
        this._map = new maplibregl.Map({
            container: this._canvas,
            locale: mapControlWords(),
            style: mapStyleURL(this._colour),
            bounds: pointBounds(this._points),
            fitBoundsOptions: { padding: 72, maxZoom: 12 },
            attributionControl: false,
        });
        this._map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');
        this._map.addControl(new maplibregl.AttributionControl({ compact: true }));
        this._markers = new MapMarkers(this._map, {
            photoAt: (rank) => this._points[rank],
            onOpen: (photos) => openLibraryPhotos(photos),
        });
        // Fires for the first style and again after every theme switch.
        this._map.on('style.load', () => {
            this._styleLoaded = true;
            // A projection belongs to the style, so every new style needs it again.
            this._applyProjection();
            this._applyRange();
        });
        this._map.on('moveend', () => this._updateInView());
        new MutationObserver(() => this._map.setStyle(mapStyleURL(this._colour)))
            .observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
        // Before the style has loaded, an error means there is no map at
        // all; after it, a missing tile only leaves a blank patch.
        this._map.on('error', (e) => {
            if (this._styleLoaded) return;
            this._note(t('The map could not be loaded from OpenFreeMap: {error}. It needs a connection to the internet.', { error: escapeHtml(e.error?.message || t('no answer')) }), true);
        });
    }

    // The server has filtered the points already (ADR-0050).
    _applyRange() {
        this._shown = this._points;
        const n = formatCount(this._points.length);
        const noun = this._points.length === 1 ? 'photo' : 'photos';
        this._countEl.innerHTML = countHTML(this._points.length, 'photo');
        if (this._styleLoaded) this._markers.show(this._points);
        this._noteEl.hidden = true;
        this._updateInView();
    }

    _setProjection(view) {
        this._projection = view;
        try { localStorage.setItem('map-projection', view); } catch { /* per browser only */ }
        if (this._styleLoaded) this._applyProjection();
    }

    // 3D is the flat map tilted, its buildings standing; Globe and Mercator
    // look straight down.
    _applyProjection() {
        const tilted = this._projection === '3d';
        this._map.setProjection({ type: this._projection === 'globe' ? 'globe' : 'mercator' });
        if (tilted) standBuildings(this._map);
        // At once: a move begun before an animated tilt ended would leave
        // the globe tilted.
        this._map.jumpTo({ pitch: tilted ? 60 : 0, bearing: tilted ? this._map.getBearing() : 0 });
    }

    _setColour(on) {
        this._colour = on;
        writeFlag('map-colour', on);
        this._map?.setStyle(mapStyleURL(on));
    }

    /* --- The photos column --- */

    // Escape closes the column; returns whether there was one to close.
    closePhotos() {
        if (!this._photosOpen || this._photosBtn.hidden) return false;
        this._setPhotosOpen(false);
        return true;
    }

    _setPhotosOpen(open) {
        this._photosOpen = open;
        if (open) this._filter.close();
        keepPhotoColumnOpen('map-photos-open', open);
        this._drawPhotosOpen();
        this._updateInView();
    }

    _drawPhotosOpen() {
        const open = this._photosOpen;
        this._photos.el.hidden = !open;
        setPanelToggle(this._photosBtn, open);
        this._map?.resize();
    }

    // The photos in the part of the map on screen, newest first: their number
    // on the button, and the photos themselves while the column is open.
    _updateInView() {
        if (!this._map || !this._styleLoaded) return;
        const bounds = this._map.getBounds();
        const inView = [];
        for (let i = this._shown.length - 1; i >= 0; i--) {
            const p = this._shown[i];
            if (bounds.contains([p.lon, p.lat])) inView.push(p);
        }
        this._photosBtn.querySelector('.lib-filter-btn-count').textContent = formatCount(inView.length);
        if (this._photosOpen) this._photos.show(inView);
    }
}

// The column and the colours are remembered per browser; without storage
// they start closed and grey.
function readFlag(key) {
    try { return localStorage.getItem(key) === '1'; } catch { return false; }
}

function writeFlag(key, on) {
    try { localStorage.setItem(key, on ? '1' : '0'); } catch { /* per browser only */ }
}

// mapPoints flattens the server's answer into one list, oldest first, and
// numbers it: points[rank] is the photo, and a higher rank is newer.
// Undated photos count as the oldest.
function mapPoints(geo) {
    const points = [];
    // A photo in several libraries (one ID, its content hash) is one point,
    // as on the Timeline: two points on one spot are a group that never
    // splits and opens the same photo twice.
    const seen = new Set();
    for (const lib of geo.libraries || []) {
        for (const [id, lat, lon, taken, name] of lib.points) {
            if (seen.has(id)) continue;
            seen.add(id);
            points.push({ lib: lib.id, id, lat, lon, taken, name, month: monthOf(taken) });
        }
    }
    points.sort((a, b) => (a.taken < b.taken ? -1 : a.taken > b.taken ? 1 : 0));
    points.forEach((p, i) => { p.rank = i; });
    return points;
}

function pointBounds(points) {
    let w = 180, s = 90, e = -180, n = -90;
    for (const p of points) {
        w = Math.min(w, p.lon); e = Math.max(e, p.lon);
        s = Math.min(s, p.lat); n = Math.max(n, p.lat);
    }
    return [[w, s], [e, n]];
}
