// Info side panel — displays file metadata and EXIF data

const INFO_PANEL_PHONE = '(max-width: 700px)';
const INFO_PANEL_KEY = 'info-panel';

class InfoPanel {
    // Every panel built so far; those no longer on the page are dropped.
    static _live = new Set();

    constructor(container) {
        this.container = container;
        this.expanded = InfoPanel.startsOpen();
        InfoPanel._live.add(this);
        this.data = null;
        this.folderData = null;
        this.libraryStats = null;
        this.error = null;
        this.currentPath = null;
        this.loading = false;
        this.collapsedSections = new Set();
        // Remembered across photos and sessions: opening it every time would
        // bury the short list again on the next photo.
        this.metadataOpen = localStorage.getItem('info-all-metadata-open') === '1';
        this.onToggle = null;
        // toggleButton: the Info button in the place's head. With one, the
        // closed panel leaves no rail behind; the button opens it.
        this.toggleButton = null;
        this.onDirNavigate = null;
        // onMarkFace(ref, faces, names): set by the viewer, where a face that
        // was not found can be marked on the photo.
        this.onMarkFace = null;
        this._metaContext = null;
        this._folders = new FolderDashboard(this);
        this._people = new InfoPeople(this);
        this._things = new InfoThings(this);
        this._exif = new InfoExif(this);
        this._records = new InfoRecords(this);
        this.render();
    }

    // Open from the start on a desk, where there is room beside the photos,
    // until it is closed: then it stays closed, everywhere, for the session.
    // A phone opens it with its Info button.
    static startsOpen() {
        if (window.matchMedia(INFO_PANEL_PHONE).matches) return false;
        try { return sessionStorage.getItem(INFO_PANEL_KEY) !== 'closed'; } catch { return true; }
    }

    toggle() {
        this._setExpanded(!this.expanded);
        if (window.matchMedia(INFO_PANEL_PHONE).matches) return;
        try { sessionStorage.setItem(INFO_PANEL_KEY, this.expanded ? 'open' : 'closed'); } catch { /* this view only */ }
        // The other places' panels follow, so a panel closed over a photo is
        // closed in the library it came from too.
        for (const other of InfoPanel._live) {
            if (other === this) continue;
            if (!other.container?.isConnected) { InfoPanel._live.delete(other); continue; }
            if (other.expanded !== this.expanded) other._setExpanded(this.expanded);
        }
    }

    // attachToggle makes btn, in the place's head, the way to open the panel.
    // A panel built anew (the Timeline's) takes the button over.
    attachToggle(btn) {
        this.toggleButton = btn;
        btn.onclick = () => this.toggle();
        this.render();
    }

    _setExpanded(on) {
        this.expanded = on;
        this.render();
        if (this.onToggle) this.onToggle();
    }

    async loadInfo(path) {
        if (this._alreadyShowing(path, 'data')) return;
        this._startLoading(path);
        this._people.reset();
        this._things.reset();
        this.render();
        await this._settle(path, t('Failed to load info'), async () => {
            const result = await API.info(path);
            return () => { this.data = result; };
        });
    }

    async loadFromURL(url, key) {
        if (this._alreadyShowing(key, 'data')) return;
        this._startLoading(key);
        // A library photo's key is lib:<library>:<photo>; its faces come too.
        const [kind, lib, photo] = String(key).split(':');
        if (kind === 'lib' && lib && photo) {
            this._people.load(lib, photo);
            this._things.load(lib, photo);
        } else {
            this._people.reset();
            this._things.reset();
        }
        this.render();
        await this._settle(key, t('Failed to load info'), async () => {
            const r = await fetch(url, { signal: this._newRequest() });
            if (!r.ok) throw new Error(await r.text());
            const result = await r.json();
            return () => { this.data = result; };
        });
    }

    async loadFolderInfo(path, opts = {}) {
        if (this._alreadyShowing(path, 'folderData')) return;
        this._startLoading(path);
        this._metaContext = null;
        this._people.reset();
        this._things.reset();
        this.render();
        await this._settle(path, t('Failed to load folder info'), async () => {
            const [folderData, libStats] = await this._fetchFolder(path, opts);
            return () => { this.folderData = folderData; this.libraryStats = libStats || null; };
        });
    }

    // A folder's details, and for a library's folder its statistics too.
    _fetchFolder(path, opts) {
        const signal = this._newRequest();
        const statsURL = opts.libId
            ? `/api/library/${opts.libId}/folder-stats?path=${encodeURIComponent(path)}`
            : `/api/browse/folder-stats?path=${encodeURIComponent(path)}`;
        const fetches = [fetch(statsURL, { signal }).then(r => r.ok ? r.json() : r.text().then(text => { throw new Error(text); }))];
        if (opts.libId && opts.pathPrefix != null) {
            const statsQ = new URLSearchParams({ ids: opts.libId, pathPrefix: opts.pathPrefix });
            fetches.push(fetch(`/api/library/statistics?${statsQ}`, { signal }).then(r => r.ok ? r.json() : null).catch(() => null));
        }
        return Promise.all(fetches);
    }

    // Asked for what is shown already, or on its way: nothing to load.
    _alreadyShowing(key, field) {
        if (key !== this.currentPath || !(this[field] || this.loading)) return false;
        if (!this.loading) this.render();
        return true;
    }

    // What was shown goes; the panel reads key.
    _startLoading(key) {
        this.currentPath = key;
        this.loading = true;
        this.error = null;
        this.data = null;
        this.folderData = null;
        this.libraryStats = null;
    }

    // load fetches and returns how to show the answer; an answer for what is
    // no longer asked about is dropped.
    async _settle(key, failText, load) {
        try {
            const show = await load();
            if (this.currentPath !== key) return;
            show();
        } catch (err) {
            if (this.currentPath !== key) return;
            this.data = null;
            this.error = err.message || failText;
        }
        this.loading = false;
        this.render();
    }

    // _newRequest ends the request for what was shown before, and returns the
    // signal of the next one. A folder's details walk all of it; a request
    // left running for every folder the selection passed took the browser's
    // few connections from everything else, the search among them.
    _newRequest() {
        this._request?.abort();
        this._request = new AbortController();
        return this._request.signal;
    }

    clear() {
        this._request?.abort();
        this.data = null;
        this.folderData = null;
        this.libraryStats = null;
        this.error = null;
        this.currentPath = null;
        this.loading = false;
        this._metaContext = null;
        this._people.reset();
        this._things.reset();
        this.destroyMap();
        this.render();
    }

    setMetaContext(ctx) {
        this._metaContext = ctx;
        if (this.expanded && this.data) this.render();
        if (ctx?.entries?.some(e => e.key.startsWith('built:'))) this._loadFilesDestinations(ctx);
    }

    // Only a files destination's gallery can be left from here: it is nothing
    // but the photo's records and one file in a folder. A gallery or a site
    // would keep showing the photo on its pages.
    async _loadFilesDestinations(ctx) {
        try {
            const channels = await ChannelAPI.list();
            ctx.filesDestinations = new Set(channels.filter(c => !c.galleryExport && !c.siteExport).map(c => c.slug));
        } catch {
            return; // without the list, no card offers to leave
        }
        if (this._metaContext === ctx && this.expanded && this.data) this.render();
    }

    destroyMap() {
        if (this.map) {
            this.map.remove();
            this.map = null;
        }
    }

    render() {
        this.destroyMap();
        setPanelToggle(this.toggleButton, this.expanded);
        if (!this.expanded) {
            this._renderClosed();
            return;
        }
        this.container.innerHTML =
            '<div class="info-panel expanded">' +
                '<div class="info-panel-header">' +
                    '<span class="info-panel-title">' + t('Info') + '</span>' +
                    panelHideButtonHTML('info-collapse-btn', t('Hide info'), 'I') +
                '</div>' +
                '<div class="info-panel-body">' + this._bodyHTML() + '</div>' +
            '</div>';
        this._wireRendered();
        this.initMap();
    }

    // Closed with a button in the place's head, nothing is left; without one
    // a narrow rail opens it again.
    _renderClosed() {
        if (this.toggleButton) {
            this.container.innerHTML = '';
            return;
        }
        this.container.innerHTML =
            '<div class="info-panel collapsed">' +
                '<button class="info-toggle-btn" title="' + t('Show info (I)') + '" aria-label="' + t('Show info') + '">' + panelIconSVG(false) + '</button>' +
            '</div>';
        this.container.querySelector('.info-toggle-btn').addEventListener('click', () => this.toggle());
    }

    _bodyHTML() {
        if (this.loading) return '<div class="info-loading"></div>';
        if (this.error) return '<div class="info-empty">' + escapeHtml(t('Error: {error}', { error: this.error })) + '</div>';
        if (this.folderData) return this._folders.render(this.folderData);
        if (!this.data) return '<div class="info-empty">' + t('Select an image to view info') + '</div>';
        return this.renderData(this.data);
    }

    _wireRendered() {
        this.container.querySelector('.info-collapse-btn').addEventListener('click', () => this.toggle());
        const loadingEl = this.container.querySelector('.info-loading');
        if (loadingEl) Activity.in(loadingEl, t('Reading the details…'));
        const allMeta = this.container.querySelector('.info-all-meta');
        allMeta?.addEventListener('toggle', () => {
            this.metadataOpen = allMeta.open;
            localStorage.setItem('info-all-metadata-open', allMeta.open ? '1' : '0');
        });
        this.container.querySelectorAll('.info-section-title').forEach(el => {
            el.addEventListener('click', () => {
                const name = el.dataset.section;
                if (!this.collapsedSections.delete(name)) this.collapsedSections.add(name);
                this.render();
            });
        });
        if (this._metaContext) this._records.attachEvents(this.container, this._metaContext);
        if (this.folderData) this._folders.attachEvents();
        if (this.data) {
            this._people.attachEvents(this.container);
            this._things.attachEvents(this.container);
        }
    }

    initMap() {
        const mapEl = this.container.querySelector('#info-map');
        if (!mapEl) return;
        // Say so rather than leaving an empty box: a blank rectangle where a
        // map should be looks like the photo has no location.
        if (typeof maplibregl === 'undefined') {
            mapEl.classList.add('info-map-unavailable');
            mapEl.textContent = t('The map library could not be loaded. The coordinates below are unaffected.');
            return;
        }
        const lat = parseFloat(mapEl.dataset.lat);
        const lon = parseFloat(mapEl.dataset.lon);
        if (isNaN(lat) || isNaN(lon)) return;

        this.map = photoLocationMap(mapEl, lon, lat);
        const controls = this.container.querySelector('.info-map-controls');
        if (!controls) return;
        const styleWrap = controls.querySelector('.info-map-style-wrap');
        if (styleWrap) {
            Toggle.create(styleWrap, { initial: false, labelOn: '3D', labelOff: '2D', onChange: (on) => this._tiltMap(on) });
        }
        controls.querySelector('[data-action="open"]')?.addEventListener('click', () => {
            window.open('https://www.openstreetmap.org/#map=16/' + lat + '/' + lon, '_blank');
        });
    }

    // 3D tilts the map close enough that the buildings stand; 2D looks
    // straight down again, north up.
    _tiltMap(on) {
        if (on) {
            this.map.easeTo({ pitch: 60, zoom: Math.max(this.map.getZoom(), 16), duration: 240 });
        } else {
            this.map.setPitch(0);
            this.map.setBearing(0);
        }
    }

    // The panel reads top to bottom in the order a photo is looked at: the few
    // facts that matter while culling, then where it was taken, then — folded
    // away — everything the file knows (ADR-0029's info panel rules).
    renderData(d) {
        const ctx = this._metaContext;
        const tags = d.exif?.tags || {};
        const used = new Set();
        return [
            ctx ? this._records.titleField(ctx) : '',
            this._exif.shortList(d, tags, used),
            this._people.render(),
            this._things.render(),
            this._exif.location(d, used),
            d.exif ? this._exif.allMetadata(d, tags, used, this.metadataOpen) : '',
            ctx ? this._records.galleriesSection(ctx) : '',
            ctx ? this._records.fieldsSection(ctx) : '',
        ].join('');
    }

    section(title, rows) {
        const isCollapsed = this.collapsedSections.has(title);
        const cls = 'info-section' + (isCollapsed ? ' collapsed' : '');
        const chevron = isCollapsed ? '\u25b8' : '\u25be';
        return '<div class="' + cls + '">' +
            '<div class="info-section-title" data-section="' + escapeHtml(title) + '">' +
                '<span>' + t(title) + '</span>' +
                '<span class="info-chevron">' + chevron + '</span>' +
            '</div>' +
            '<div class="info-section-body">' + rows.join('') + '</div>' +
            '</div>';
    }

    // Labels and values are text: file names, paths and EXIF tags and values
    // come from the file, so none of them may be read as markup.
    row(label, value) {
        return '<div class="info-row">' +
            '<span class="info-label">' + escapeHtml(t(label)) + '</span>' +
            '<span class="info-value">' + (value ? escapeHtml(value) : '\u2014') + '</span>' +
            '</div>';
    }

    // The same neutral chip the thumbnails use: format and film simulation are
    // data, not status, so they get the data voice rather than a colour each
    // (ADR-0030).
    badgeRow(label, value) {
        if (!value) return this.row(label, value);
        return '<div class="info-row">' +
            '<span class="info-label">' + escapeHtml(t(label)) + '</span>' +
            '<span class="info-value"><span class="info-chip">' + escapeHtml(value) + '</span></span>' +
            '</div>';
    }

    formatDate(iso) {
        return iso ? formatDateTime(iso) : '\u2014';
    }
}

// photoLocationMap is the small map of where one photo was taken: colour
// tiles, a marker, the zoom buttons, and the attribution folded until asked.
function photoLocationMap(mapEl, lon, lat) {
    const map = new maplibregl.Map({
        container: mapEl,
        style: 'https://tiles.openfreemap.org/styles/liberty',
        center: [lon, lat],
        locale: mapControlWords(),
        zoom: 14,
        scrollZoom: false,
        attributionControl: false,
    });
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');
    map.addControl(new maplibregl.AttributionControl({ compact: true }));
    map.on('load', () => {
        const attr = mapEl.querySelector('.maplibregl-ctrl-attrib');
        attr?.classList.remove('maplibregl-compact-show');
        attr?.removeAttribute('open');
    });
    new maplibregl.Marker().setLngLat([lon, lat]).addTo(map);
    return map;
}

