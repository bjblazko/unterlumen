// Info side panel — displays file metadata and EXIF data

class InfoPanel {
    constructor(container) {
        this.container = container;
        this.expanded = false;
        this.data = null;
        this.folderData = null;
        this.libraryStats = null;
        this.error = null;
        this.currentPath = null;
        this.loading = false;
        this.collapsedSections = new Set();
        this.onToggle = null;
        this.onDirNavigate = null;
        this._metaContext = null;
        this.render();
    }

    toggle() {
        this.expanded = !this.expanded;
        this.render();
        if (this.onToggle) this.onToggle();
    }

    async loadInfo(path) {
        if (path === this.currentPath && (this.data || this.loading)) {
            if (!this.loading) this.render();
            return;
        }
        this.currentPath = path;
        this.loading = true;
        this.error = null;
        this.data = null;
        this.render();
        const requestPath = path;
        try {
            const result = await API.info(path);
            if (this.currentPath !== requestPath) return;
            this.data = result;
        } catch (err) {
            if (this.currentPath !== requestPath) return;
            this.data = null;
            this.error = err.message || 'Failed to load info';
        }
        this.loading = false;
        this.render();
    }

    async loadFromURL(url, key) {
        if (key === this.currentPath && (this.data || this.loading)) {
            if (!this.loading) this.render();
            return;
        }
        this.currentPath = key;
        this.loading = true;
        this.error = null;
        this.data = null;
        this.folderData = null;
        this.libraryStats = null;
        this.render();
        const requestKey = key;
        try {
            const r = await fetch(url);
            if (!r.ok) throw new Error(await r.text());
            const result = await r.json();
            if (this.currentPath !== requestKey) return;
            this.data = result;
        } catch (err) {
            if (this.currentPath !== requestKey) return;
            this.data = null;
            this.error = err.message || 'Failed to load info';
        }
        this.loading = false;
        this.render();
    }

    async loadFolderInfo(path, opts = {}) {
        if (path === this.currentPath && (this.folderData || this.loading)) {
            if (!this.loading) this.render();
            return;
        }
        this.currentPath = path;
        this.data = null;
        this.folderData = null;
        this.libraryStats = null;
        this.loading = true;
        this.error = null;
        this._metaContext = null;
        this.render();
        const requestPath = path;

        try {
            const statsURL = opts.libId
                ? `/api/library/${opts.libId}/folder-stats?path=${encodeURIComponent(path)}`
                : `/api/browse/folder-stats?path=${encodeURIComponent(path)}`;

            const fetches = [fetch(statsURL).then(r => r.ok ? r.json() : r.text().then(t => { throw new Error(t); }))];

            if (opts.libId && opts.pathPrefix != null) {
                const statsQ = new URLSearchParams({ ids: opts.libId, pathPrefix: opts.pathPrefix });
                fetches.push(fetch(`/api/library/statistics?${statsQ}`).then(r => r.ok ? r.json() : null).catch(() => null));
            }

            const [folderData, libStats] = await Promise.all(fetches);
            if (this.currentPath !== requestPath) return;
            this.folderData = folderData;
            this.libraryStats = libStats || null;
        } catch (err) {
            if (this.currentPath !== requestPath) return;
            this.error = err.message || 'Failed to load folder info';
        }
        this.loading = false;
        this.render();
    }

    clear() {
        this.data = null;
        this.folderData = null;
        this.libraryStats = null;
        this.error = null;
        this.currentPath = null;
        this.loading = false;
        this._metaContext = null;
        this.destroyMap();
        this.render();
    }

    setMetaContext(ctx) {
        this._metaContext = ctx;
        if (this.expanded && this.data) this.render();
    }

    destroyMap() {
        if (this.map) {
            this.map.remove();
            this.map = null;
        }
    }

    render() {
        this.destroyMap();

        if (!this.expanded) {
            this.container.innerHTML =
                '<div class="info-panel collapsed">' +
                    '<button class="info-toggle-btn" title="Show info (I)" aria-label="Show info">' +
                    '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true">' +
                        '<circle cx="8" cy="8" r="6.5"/>' +
                        '<line x1="8" y1="7.5" x2="8" y2="11"/>' +
                        '<circle cx="8" cy="5" r="0.6" fill="currentColor" stroke="none"/>' +
                    '</svg>' +
                '</button>' +
                '</div>';
            this.container.querySelector('.info-toggle-btn')
                .addEventListener('click', () => this.toggle());
            return;
        }

        let body = '';
        if (this.loading) {
            body = '<div class="info-empty">Loading\u2026</div>';
        } else if (this.error) {
            body = '<div class="info-empty">Error: ' + this.error + '</div>';
        } else if (this.folderData) {
            body = this.renderFolderData(this.folderData);
        } else if (!this.data) {
            body = '<div class="info-empty">Select an image to view info</div>';
        } else {
            body = this.renderData(this.data);
        }

        this.container.innerHTML =
            '<div class="info-panel expanded">' +
                '<div class="info-panel-header">' +
                    '<span class="info-panel-title">Info</span>' +
                    '<button class="info-collapse-btn" title="Hide info (I)">\u2715</button>' +
                '</div>' +
                '<div class="info-panel-body">' + body + '</div>' +
            '</div>';

        this.container.querySelector('.info-collapse-btn')
            .addEventListener('click', () => this.toggle());

        this.container.querySelectorAll('.info-section-title').forEach(el => {
            el.addEventListener('click', () => {
                const name = el.dataset.section;
                if (this.collapsedSections.has(name)) {
                    this.collapsedSections.delete(name);
                } else {
                    this.collapsedSections.add(name);
                }
                this.render();
            });
        });

        if (this._metaContext) this._attachMetaEvents();
        if (this.folderData) this._attachFolderEvents();

        this.initMap();
    }

    initMap() {
        const mapEl = this.container.querySelector('#info-map');
        if (!mapEl || typeof maplibregl === 'undefined') return;

        const lat = parseFloat(mapEl.dataset.lat);
        const lon = parseFloat(mapEl.dataset.lon);
        if (isNaN(lat) || isNaN(lon)) return;

        this.map = new maplibregl.Map({
            container: mapEl,
            style: 'https://tiles.openfreemap.org/styles/liberty',
            center: [lon, lat],
            zoom: 14,
            scrollZoom: false,
            attributionControl: false
        });
        this.map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');
        this.map.addControl(new maplibregl.AttributionControl({ compact: true }));
        this.map.on('load', () => {
            const attr = mapEl.querySelector('.maplibregl-ctrl-attrib');
            if (attr) {
                attr.classList.remove('maplibregl-compact-show');
                attr.removeAttribute('open');
            }
        });

        new maplibregl.Marker().setLngLat([lon, lat]).addTo(this.map);

        this.mapStyle = '2d';
        const controls = this.container.querySelector('.info-map-controls');
        if (!controls) return;

        const styleWrap = controls.querySelector('.info-map-style-wrap');
        if (styleWrap) {
            Toggle.create(styleWrap, {
                initial: false,
                labelOn: '3D',
                labelOff: '2D',
                onChange: (on) => {
                    this.mapStyle = on ? '3d' : '2d';
                    if (on) {
                        this.map.setPitch(60);
                    } else {
                        this.map.setPitch(0);
                        this.map.setBearing(0);
                    }
                }
            });
        }

        controls.querySelector('[data-action="open"]')?.addEventListener('click', () => {
            window.open('https://www.openstreetmap.org/#map=16/' + lat + '/' + lon, '_blank');
        });
    }

    renderData(d) {
        const sections = [];

        // File section
        const fileRows = [];
        fileRows.push(this.row('Name', d.name));
        fileRows.push(this.row('Path', d.path));
        fileRows.push(this.row('Size', formatSize(d.size)));
        fileRows.push(this.colorBadgeRow('Format', (d.format || '').toUpperCase(), this.formatColor(d.format)));
        fileRows.push(this.row('Modified', this.formatDate(d.modified)));
        sections.push(this.section('File', fileRows));

        if (!d.exif) return sections.join('');

        const tags = d.exif.tags || {};
        const used = new Set();

        // Location section (placed early so the map is visible without scrolling)
        if (d.exif.latitude != null && d.exif.longitude != null) {
            const lat = d.exif.latitude.toFixed(6);
            const lon = d.exif.longitude.toFixed(6);
            const locRows = [];
            locRows.push('<div id="info-map" class="info-map-container" data-lat="' + lat + '" data-lon="' + lon + '"></div>');
            locRows.push('<div class="info-map-controls">' +
                '<div class="info-map-style-wrap"></div>' +
                '<button class="btn btn-sm" data-action="open">\u2197 Open</button>' +
            '</div>');
            locRows.push(this.row('Latitude', lat));
            locRows.push(this.row('Longitude', lon));
            sections.push(this.section('Location', locRows));
        }
        // Mark GPS tags as used
        ['GPSLatitude', 'GPSLatitudeRef', 'GPSLongitude', 'GPSLongitudeRef',
         'GPSAltitude', 'GPSAltitudeRef', 'GPSTimeStamp', 'GPSDateStamp',
         'GPSVersionID'].forEach(t => used.add(t));

        // Image section
        const imageRows = [];
        if (d.exif.width && d.exif.height) {
            imageRows.push(this.row('Dimensions', d.exif.width + ' \u00d7 ' + d.exif.height));
            const arLabel = this._aspectRatioLabel(d.exif.width, d.exif.height);
            if (arLabel) {
                const icon = this._aspectRatioIcon(arLabel, 'rgba(60,50,40,0.6)');
                imageRows.push(this.row('Aspect Ratio', `<span style="display:inline-flex;align-items:center;gap:4px">${icon}${arLabel}</span>`));
            }
        }
        this.addTag(imageRows, tags, used, 'Orientation', 'Orientation', this.decodeOrientation);
        this.addTag(imageRows, tags, used, 'ColorSpace', 'Color Space', this.decodeColorSpace);
        if (imageRows.length) sections.push(this.section('Image', imageRows));

        // Camera section
        const cameraRows = [];
        this.addTag(cameraRows, tags, used, 'Make', 'Make');
        this.addTag(cameraRows, tags, used, 'Model', 'Model');
        this.addTag(cameraRows, tags, used, 'LensModel', 'Lens');
        if (tags['FilmSimulation'] != null) {
            used.add('FilmSimulation');
            const sim = this.stripQuotes(tags['FilmSimulation']);
            cameraRows.push(this.colorBadgeRow('Film Simulation', sim, this.filmSimColor(sim)));
        }
        this.addTag(cameraRows, tags, used, 'Software', 'Software');
        if (cameraRows.length) sections.push(this.section('Camera', cameraRows));

        // Exposure section
        const expRows = [];
        this.addTag(expRows, tags, used, 'ExposureTime', 'Shutter Speed', v => this.decodeExposureTime(v));
        this.addTag(expRows, tags, used, 'FNumber', 'Aperture', v => this.decodeFNumber(v));
        this.addTag(expRows, tags, used, 'ISOSpeedRatings', 'ISO');
        this.addTag(expRows, tags, used, 'ExposureBiasValue', 'Exposure Bias', v => this.decodeExposureBias(v));
        this.addTag(expRows, tags, used, 'FocalLength', 'Focal Length', v => this.decodeFocalLength(v));
        this.addTag(expRows, tags, used, 'MeteringMode', 'Metering', this.decodeMeteringMode);
        this.addTag(expRows, tags, used, 'ExposureProgram', 'Program', this.decodeExposureProgram);
        this.addTag(expRows, tags, used, 'Flash', 'Flash', this.decodeFlash);
        this.addTag(expRows, tags, used, 'WhiteBalance', 'White Balance', this.decodeWhiteBalance);
        if (expRows.length) sections.push(this.section('Exposure', expRows));

        // Dates section — use pre-parsed structured fields when available
        const dateRows = [];
        // Mark raw date and offset tags as used so they don't appear in Other
        ['DateTimeOriginal','DateTimeDigitized','DateTime',
         'OffsetTimeOriginal','OffsetTimeDigitized','OffsetTime'].forEach(t => used.add(t));
        if (d.exif.dateTaken)     dateRows.push(this.row('Original',  this.formatExifDate(d.exif.dateTaken)));
        if (d.exif.dateDigitized) dateRows.push(this.row('Digitized', this.formatExifDate(d.exif.dateDigitized)));
        if (d.exif.dateModified)  dateRows.push(this.row('Modified',  this.formatExifDate(d.exif.dateModified)));
        if (dateRows.length) sections.push(this.section('Dates', dateRows));

        // Also mark dimension tags as used
        ['PixelXDimension', 'PixelYDimension', 'ImageWidth', 'ImageLength'].forEach(t => used.add(t));

        // Other section — remaining tags
        const otherRows = [];
        const sortedKeys = Object.keys(tags).sort();
        for (const key of sortedKeys) {
            if (!used.has(key)) {
                otherRows.push(this.row(key, this.stripQuotes(tags[key])));
            }
        }
        if (otherRows.length) sections.push(this.section('Other', otherRows));

        if (this._metaContext) {
            const pubSect = this._renderPublicationsSection();
            if (pubSect) sections.push(pubSect);
            sections.push(this._renderMetaSection());
        }

        return sections.join('');
    }

    _humanizeChannelSlug(slug) {
        return slug.replace(/[-_]/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
    }

    _renderPublicationsSection() {
        const ctx = this._metaContext;
        if (!ctx || !ctx.entries) return '';

        const primaryPubs = ctx.entries.filter(e =>
            e.key.startsWith('published:') && !e.key.slice('published:'.length).includes(':')
        );
        if (primaryPubs.length === 0) return '';

        const cards = primaryPubs.map(e => {
            const slug = e.key.slice('published:'.length);
            const channelName = this._humanizeChannelSlug(slug);
            const date = this.formatDate(e.value);
            const titleEntry = ctx.entries.find(te => te.key === `published:${slug}:title`);
            const galleryTitle = titleEntry ? escapeHtml(titleEntry.value) : '';

            return `<div class="info-pub-card">` +
                `<div class="info-pub-card-header">` +
                    `<span class="info-pub-channel">${escapeHtml(channelName)}</span>` +
                    `<button class="info-pub-del btn-icon" title="Remove publication" data-key="${escapeHtml(e.key)}">×</button>` +
                `</div>` +
                `<div class="info-pub-date">${escapeHtml(date)}</div>` +
                (galleryTitle ? `<div class="info-pub-title">${galleryTitle}</div>` : '') +
            `</div>`;
        });

        return this.section('Publications', cards);
    }

    _renderMetaSection() {
        const ctx = this._metaContext;
        const rows = [];

        for (const e of (ctx.entries || []).filter(e => !e.key.startsWith('published:'))) {
            rows.push(
                `<div class="info-meta-row" data-meta-key="${escapeHtml(e.key)}">` +
                    `<span class="info-label">${escapeHtml(e.key)}</span>` +
                    `<span class="info-meta-val info-value" contenteditable="true" data-key="${escapeHtml(e.key)}">${escapeHtml(e.value)}</span>` +
                    `<button class="info-meta-del" title="Remove" data-key="${escapeHtml(e.key)}">\u00d7</button>` +
                `</div>`
            );
        }

        rows.push(
            `<div class="info-meta-add">` +
                `<input class="info-meta-key-input" type="text" placeholder="Key" autocomplete="off">` +
                `<input class="info-meta-val-input" type="text" placeholder="Value" autocomplete="off">` +
                `<button class="btn btn-sm info-meta-add-btn">Add</button>` +
            `</div>`
        );

        return this.section('Meta', rows);
    }

    section(title, rows) {
        const isCollapsed = this.collapsedSections.has(title);
        const cls = 'info-section' + (isCollapsed ? ' collapsed' : '');
        const chevron = isCollapsed ? '\u25b8' : '\u25be';
        return '<div class="' + cls + '">' +
            '<div class="info-section-title" data-section="' + title + '">' +
                '<span>' + title + '</span>' +
                '<span class="info-chevron">' + chevron + '</span>' +
            '</div>' +
            '<div class="info-section-body">' + rows.join('') + '</div>' +
            '</div>';
    }

    row(label, value) {
        return '<div class="info-row">' +
            '<span class="info-label">' + label + '</span>' +
            '<span class="info-value">' + (value || '\u2014') + '</span>' +
            '</div>';
    }

    colorBadgeRow(label, value, color) {
        if (!value || !color) return this.row(label, value);
        return '<div class="info-row">' +
            '<span class="info-label">' + label + '</span>' +
            '<span class="info-value"><span class="info-color-badge" style="background:' + color + '">' + value + '</span></span>' +
            '</div>';
    }

    formatColor(format) {
        const colors = {
            jpeg: '#c27833', jpg: '#c27833',
            heif: '#4a8c5c', heic: '#4a8c5c', hif: '#4a8c5c',
            png: '#4a6fa5',
            gif: '#8c6b4a',
            webp: '#7b5299',
        };
        return colors[(format || '').toLowerCase()] || null;
    }

    filmSimColor(sim) {
        const colors = {
            'Provia': '#3a7ca5', 'Astia': '#5a9ab5',
            'Velvia': '#b5443a',
            'Classic Chrome': '#8a7d3a', 'Classic Neg.': '#b07040',
            'Eterna': '#3a8a8a',
            'Nostalgic Neg.': '#a05050', 'Reala Ace': '#3a8a5a',
            'Pro Neg. Std': '#6a6a7a', 'Pro Neg. Hi': '#7a6a8a',
            'Bleach Bypass': '#8a8a8a',
            'Monochrome': '#404040', 'Monochrome + R': '#5a3030',
            'Monochrome + Ye': '#5a5a30', 'Monochrome + G': '#305a30',
            'Acros': '#333333', 'Acros + R': '#4a2828',
            'Acros + Ye': '#4a4a28', 'Acros + G': '#284a28',
            'Sepia': '#6a5038',
        };
        return colors[sim] || '#6a6a7a';
    }

    _aspectRatioLabel(w, h) {
        if (!w || !h) return '';
        const knownRatios = [
            [1, 2], [9, 16], [2, 3], [3, 4], [4, 5],
            [1, 1],
            [5, 4], [4, 3], [3, 2], [7, 5], [16, 10], [5, 3], [16, 9], [2, 1], [21, 9],
        ];
        const ratio = w / h;
        const tol = 0.015;
        for (const [rw, rh] of knownRatios) {
            const known = rw / rh;
            if (Math.abs(ratio - known) / known < tol) return rw + ':' + rh;
        }
        return 'Custom Crop';
    }

    _aspectRatioIcon(ratioStr, strokeColor) {
        const isCustom = ratioStr === 'Custom Crop';
        let ratio = 1;
        if (!isCustom) {
            const parts = ratioStr.split(':');
            if (parts.length === 2) ratio = parseFloat(parts[0]) / parseFloat(parts[1]);
        }
        let rw, rh;
        if (ratio > 14 / 10) { rw = 14; rh = 14 / ratio; }
        else { rh = 10; rw = 10 * ratio; }
        const x = ((16 - rw) / 2).toFixed(1);
        const y = ((12 - rh) / 2).toFixed(1);
        const dash = isCustom ? ' stroke-dasharray="2 1"' : '';
        return `<svg width="16" height="12" viewBox="0 0 16 12" fill="none" stroke="${strokeColor}" stroke-width="1.5"${dash}><rect x="${x}" y="${y}" width="${rw.toFixed(1)}" height="${rh.toFixed(1)}" rx="0.5"/></svg>`;
    }

    addTag(rows, tags, used, tagName, label, decoder) {
        if (tags[tagName] == null) return;
        used.add(tagName);
        let val = this.stripQuotes(tags[tagName]);
        if (decoder) val = decoder(val);
        rows.push(this.row(label, val));
    }

    stripQuotes(s) {
        if (typeof s !== 'string') return s;
        if (s.length >= 2 && s[0] === '"' && s[s.length - 1] === '"') {
            return s.slice(1, -1);
        }
        return s;
    }

    formatDate(iso) {
        if (!iso) return '\u2014';
        return iso.replace('T', ' ').replace(/Z$/, '').replace(/([+-]\d{2}:\d{2})$/, ' $1').trim();
    }

    formatExifDate(iso) {
        if (!iso) return '\u2014';
        return iso.replace('T', ' ').replace(/([+-]\d{2}:\d{2})$/, ' $1');
    }

    parseRational(s) {
        const slash = s.indexOf('/');
        if (slash >= 0) {
            const num = parseFloat(s.slice(0, slash));
            const den = parseFloat(s.slice(slash + 1));
            if (den === 0 || isNaN(num) || isNaN(den)) return null;
            return num / den;
        }
        const v = parseFloat(s);
        return isNaN(v) ? null : v;
    }

    decodeFNumber(s) {
        const v = this.parseRational(s);
        if (v === null || v <= 0) return s;
        return `f/${v.toFixed(1)}`;
    }

    decodeFocalLength(s) {
        const v = this.parseRational(s);
        if (v === null || v <= 0) return s;
        return v < 10 ? `${v.toFixed(2).replace(/\.?0+$/, '')} mm` : `${Math.round(v)} mm`;
    }

    decodeExposureTime(s) {
        const v = this.parseRational(s);
        if (v === null) return s;
        if (v >= 1) return Number.isInteger(v) ? `${v} s` : `${v.toFixed(1)} s`;
        return `1/${Math.round(1 / v)}`;
    }

    decodeExposureBias(s) {
        const v = this.parseRational(s);
        if (v === null) return s;
        if (v === 0) return '0 EV';
        const sign = v > 0 ? '+' : '';
        return `${sign}${v.toFixed(2).replace(/\.?0+$/, '')} EV`;
    }

    decodeMeteringMode(v)    { return exifLabel('MeteringMode', v)    ?? v; }
    decodeExposureProgram(v) { return exifLabel('ExposureProgram', v) ?? v; }
    decodeFlash(v)           { return exifLabel('Flash', v)           ?? v; }
    decodeWhiteBalance(v)    { return exifLabel('WhiteBalance', v)    ?? v; }
    decodeOrientation(v)     { return exifLabel('Orientation', v)     ?? v; }
    decodeColorSpace(v)      { return exifLabel('ColorSpace', v)      ?? v; }

    _attachMetaEvents() {
        const ctx = this._metaContext;

        this.container.querySelectorAll('.info-meta-val').forEach(valEl => {
            const key = valEl.dataset.key;
            let originalValue = valEl.textContent;
            valEl.addEventListener('blur', () => {
                const newVal = valEl.textContent.trim();
                if (newVal !== originalValue) {
                    ctx.onUpsert(key, newVal).catch(err => alert('Save failed: ' + err.message));
                    originalValue = newVal;
                }
            });
            valEl.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') { e.preventDefault(); valEl.blur(); }
            });
        });

        this.container.querySelectorAll('.info-meta-del').forEach(btn => {
            btn.addEventListener('click', async () => {
                const key = btn.dataset.key;
                try {
                    await ctx.onDelete(key);
                    // For published:{slug} keys the backend removes multiple related keys;
                    // refresh the full list instead of filtering out just one entry.
                    const isMainPublishKey = key.startsWith('published:') && !key.slice('published:'.length).includes(':');
                    if (isMainPublishKey && ctx.refresh) {
                        ctx.entries = await ctx.refresh();
                    } else {
                        ctx.entries = ctx.entries.filter(e => e.key !== key);
                    }
                    this.render();
                } catch (err) {
                    alert('Delete failed: ' + err.message);
                }
            });
        });

        this.container.querySelectorAll('.info-pub-del').forEach(btn => {
            btn.addEventListener('click', async () => {
                const key = btn.dataset.key;
                try {
                    await ctx.onDelete(key);
                    ctx.entries = await ctx.refresh();
                    this.render();
                } catch (err) {
                    alert('Delete failed: ' + err.message);
                }
            });
        });

        const addBtn = this.container.querySelector('.info-meta-add-btn');
        if (addBtn) {
            addBtn.addEventListener('click', async () => {
                const keyInput = this.container.querySelector('.info-meta-key-input');
                const valInput = this.container.querySelector('.info-meta-val-input');
                const key = keyInput.value.trim();
                const val = valInput.value.trim();
                if (!key || !val) return;
                try {
                    await ctx.onUpsert(key, val);
                    ctx.entries = await ctx.refresh();
                    this.render();
                } catch (err) {
                    alert('Failed to save: ' + err.message);
                }
            });
        }
    }

    // --- Folder info ---

    renderFolderData(d) {
        if (d.photoCount !== undefined) return this._renderLibraryFolderData(d);

        const sections = [];

        // Folder section
        const folderRows = [];
        folderRows.push(this.row('Name', d.name));
        folderRows.push(this.row('Path', d.path || '/'));
        folderRows.push(this.row('Modified', this.formatDate(d.modified)));
        sections.push(this.section('Folder', folderRows));

        // Contents section
        const contRows = [];
        contRows.push(this.row('Total size', formatSize(d.totalSize)));
        contRows.push(this.row('Files', d.fileCount.toLocaleString()));
        contRows.push(this.row('Subfolders', d.dirCount.toLocaleString()));
        contRows.push(this.row('Max depth', d.maxDepth + (d.maxDepth === 1 ? ' level' : ' levels')));
        sections.push(this.section('Contents', contRows));

        // Size map (treemap)
        if (d.subfolders && d.subfolders.length > 0) {
            const treemap = this._renderTreemap(d.subfolders, d.totalSize);
            sections.push(this.section('Size Map', [treemap]));
        }

        // Nesting depth histogram
        if (d.subfolders && d.subfolders.some(s => s.maxDepth > 0)) {
            sections.push(this.section('Nesting Depth', [this._renderDepthHistogram(d.subfolders)]));
        }

        // File types (browse mode — no library stats)
        if (!this.libraryStats && d.fileTypes && Object.keys(d.fileTypes).length > 0) {
            sections.push(this.section('File Types', [this._renderFileTypeChart(d.fileTypes)]));
        }

        // Library EXIF stats
        if (this.libraryStats) {
            sections.push(...this._renderLibraryStats(this.libraryStats));
        }

        return sections.join('');
    }

    _renderLibraryFolderData(d) {
        const sections = [];

        // Folder section — derive name/path from currentPath since LibraryFolderStats has no FS metadata
        const folderRows = [];
        const pathStr = this.currentPath || '';
        const name = pathStr ? pathStr.split('/').pop() : '(root)';
        folderRows.push(this.row('Name', name));
        folderRows.push(this.row('Path', pathStr || '/'));
        sections.push(this.section('Folder', folderRows));

        // Contents section
        const contRows = [];
        contRows.push(this.row('Total size', formatSize(d.totalSize)));
        contRows.push(this.row('Photos', d.photoCount.toLocaleString()));
        if (d.dateFirst && d.dateLast) {
            const first = d.dateFirst.slice(0, 10);
            const last = d.dateLast.slice(0, 10);
            contRows.push(this.row('Date range', first === last ? first : first + ' – ' + last));
        }
        sections.push(this.section('Contents', contRows));

        // Size map treemap — adapt LibSubfolder to the shape _renderTreemap expects
        if (d.subfolders && d.subfolders.length > 0) {
            const adapted = d.subfolders.map(s => ({ name: s.name, size: s.totalSize, fileCount: s.photoCount }));
            sections.push(this.section('Size Map', [this._renderTreemap(adapted, d.totalSize)]));
        }

        // File Types — only when libraryStats is not rendering formats already
        if (!this.libraryStats && d.formats && d.formats.length > 0) {
            const fileTypes = Object.fromEntries(d.formats.map(f => [f.name, f.count]));
            sections.push(this.section('File Types', [this._renderFileTypeChart(fileTypes)]));
        }

        // Library EXIF stats (cameras, focal lengths, shooting hours, etc.)
        if (this.libraryStats) {
            sections.push(...this._renderLibraryStats(this.libraryStats));
        }

        return sections.join('');
    }

    _renderTreemap(subfolders, totalSize) {
        const W = 260;
        const maxH = 200;
        const sorted = [...subfolders].sort((a, b) => b.size - a.size);
        const total = sorted.reduce((s, f) => s + f.size, 0) || 1;
        const H = Math.max(80, Math.min(maxH, Math.round(W * Math.min(total / (1024 * 1024 * 1024) + 0.5, 1))));

        const layout = this._squarify(sorted, 0, 0, W, H);
        const colors = [
            '#c27833', '#4a8c5c', '#4a6fa5', '#8c6b4a',
            '#7b5299', '#3a8a8a', '#b5443a', '#6a6a7a',
        ];

        let rects = '';
        layout.forEach((cell, i) => {
            const color = colors[i % colors.length];
            const label = cell.item.name.length > 14 ? cell.item.name.slice(0, 13) + '…' : cell.item.name;
            const sizeLabel = formatSize(cell.item.size);
            const countLabel = cell.item.fileCount + ' file' + (cell.item.fileCount !== 1 ? 's' : '');
            const showText = cell.w > 40 && cell.h > 30;
            const showCount = cell.w > 60 && cell.h > 52;
            const subPath = (this.currentPath ? this.currentPath + '/' : '') + cell.item.name;

            const tooltip = `${cell.item.name}\n${sizeLabel} · ${countLabel}`;
            rects += `<g class="folder-treemap-cell" data-path="${escapeHtml(subPath)}" style="cursor:pointer">` +
                `<title>${escapeHtml(tooltip)}</title>` +
                `<rect x="${cell.x.toFixed(1)}" y="${cell.y.toFixed(1)}" width="${cell.w.toFixed(1)}" height="${cell.h.toFixed(1)}" fill="${color}" rx="2"/>` +
                (showText ? `<text x="${(cell.x + 6).toFixed(1)}" y="${(cell.y + 16).toFixed(1)}" class="folder-treemap-name">${escapeHtml(label)}</text>` : '') +
                (showText ? `<text x="${(cell.x + 6).toFixed(1)}" y="${(cell.y + 30).toFixed(1)}" class="folder-treemap-size">${escapeHtml(sizeLabel)}</text>` : '') +
                (showCount ? `<text x="${(cell.x + 6).toFixed(1)}" y="${(cell.y + 44).toFixed(1)}" class="folder-treemap-count">${escapeHtml(countLabel)}</text>` : '') +
                '</g>';
        });

        return `<div class="folder-treemap"><svg width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">${rects}</svg></div>`;
    }

    _squarify(items, x, y, w, h) {
        if (!items.length) return [];
        const total = items.reduce((s, f) => s + (f.size || 1), 0);
        const result = [];
        this._squarifyRow(items, x, y, w, h, total, result);
        return result;
    }

    _squarifyRow(items, x, y, w, h, total, result) {
        if (!items.length) return;
        if (items.length === 1) {
            result.push({ x, y, w, h, item: items[0] });
            return;
        }

        const area = w * h;
        let row = [];
        let rowSize = 0;
        let bestWorst = Infinity;
        let i = 0;

        while (i < items.length) {
            const item = items[i];
            const size = item.size || 1;
            row.push(item);
            rowSize += size;

            const side = Math.min(w, h);
            const rowArea = (rowSize / total) * area;
            const rowLen = rowArea / side;
            let worst = 0;
            for (const r of row) {
                const rArea = ((r.size || 1) / total) * area;
                const rSide = rArea / rowLen;
                const ratio = Math.max(rowLen / rSide, rSide / rowLen);
                if (ratio > worst) worst = ratio;
            }

            if (worst >= bestWorst) {
                row.pop();
                rowSize -= size;
                break;
            }
            bestWorst = worst;
            i++;
        }

        // Lay out the row
        const rowFrac = rowSize / total;
        let offset = 0;
        const isWide = w >= h;
        const rowDim = isWide ? w * rowFrac : h * rowFrac;

        for (const r of row) {
            const frac = (r.size || 1) / rowSize;
            if (isWide) {
                const rh = h * frac;
                result.push({ x, y: y + offset, w: rowDim, h: rh, item: r });
                offset += rh;
            } else {
                const rw = w * frac;
                result.push({ x: x + offset, y, w: rw, h: rowDim, item: r });
                offset += rw;
            }
        }

        // Recurse on remaining items
        const remainingItems = items.slice(row.length);
        if (!remainingItems.length) return;
        const remainTotal = total - rowSize;
        if (isWide) {
            this._squarifyRow(remainingItems, x + rowDim, y, w - rowDim, h, remainTotal, result);
        } else {
            this._squarifyRow(remainingItems, x, y + rowDim, w, h - rowDim, remainTotal, result);
        }
    }

    _renderDepthHistogram(subfolders) {
        const maxDepth = Math.max(...subfolders.map(s => s.maxDepth), 1);
        const bars = subfolders.map(s => {
            const pct = Math.round((s.maxDepth / maxDepth) * 100);
            const label = s.name.length > 8 ? s.name.slice(0, 7) + '…' : s.name;
            return `<div class="folder-depth-col" title="${escapeHtml(s.name)}: ${s.maxDepth} level${s.maxDepth !== 1 ? 's' : ''} deep">` +
                `<div class="folder-depth-bar" style="height:${pct}%"></div>` +
                `<div class="folder-depth-label">${escapeHtml(label)}</div>` +
                '</div>';
        }).join('');
        return `<div class="folder-depth-histogram">${bars}</div>`;
    }

    _renderFileTypeChart(fileTypes) {
        const entries = Object.entries(fileTypes).sort((a, b) => b[1] - a[1]).slice(0, 10);
        const max = entries[0]?.[1] || 1;
        const bars = entries.map(([ext, count]) => {
            const pct = Math.round((count / max) * 100);
            const extUpper = ext.toUpperCase();
            return `<div class="folder-type-row">` +
                `<span class="folder-type-ext">${escapeHtml(extUpper)}</span>` +
                `<div class="folder-type-bar-wrap"><div class="folder-type-bar" style="width:${pct}%"></div></div>` +
                `<span class="folder-type-count">${count}</span>` +
                '</div>';
        }).join('');
        return `<div class="folder-type-chart">${bars}</div>`;
    }

    _renderLibraryStats(stats) {
        const sections = [];

        // Shooting date range from shootingDays
        const days = Object.keys(stats.shootingDays || {}).sort();
        if (days.length > 0) {
            const first = days[0];
            const last = days[days.length - 1];
            const totalDays = days.length;
            const rows = [];
            rows.push(this.row('First shot', first));
            rows.push(this.row('Last shot', last));
            rows.push(this.row('Active days', totalDays.toLocaleString()));
            rows.push(this.row('Total photos', (stats.totalPhotos || 0).toLocaleString()));
            sections.push(this.section('Photos', rows));
        }

        // Format breakdown
        if (stats.formats && stats.formats.length > 0) {
            const max = Math.max(...stats.formats.map(f => f.count), 1);
            const bars = stats.formats.slice(0, 8).map(f => {
                const pct = Math.round((f.count / max) * 100);
                return `<div class="folder-type-row">` +
                    `<span class="folder-type-ext">${escapeHtml(f.name.toUpperCase())}</span>` +
                    `<div class="folder-type-bar-wrap"><div class="folder-type-bar" style="width:${pct}%"></div></div>` +
                    `<span class="folder-type-count">${f.count}</span>` +
                    '</div>';
            }).join('');
            sections.push(this.section('Formats', [`<div class="folder-type-chart">${bars}</div>`]));
        }

        // Camera × lens
        if (stats.cameraLens && stats.cameraLens.length > 0) {
            const max = Math.max(...stats.cameraLens.map(cl => cl.count), 1);
            const items = stats.cameraLens.slice(0, 5).map(cl => {
                const pct = Math.round((cl.count / max) * 100);
                const cam = cl.camera || 'Unknown';
                const lens = cl.lens || '—';
                const label = cam + (lens !== '—' ? ' / ' + lens : '');
                return `<div class="folder-cam-item">` +
                    `<div class="folder-cam-bar-row">` +
                    `<span class="folder-cam-count">${cl.count}x</span>` +
                    `<div class="folder-cam-bar-wrap"><div class="folder-cam-bar" style="width:${pct}%"></div></div>` +
                    `</div>` +
                    `<div class="folder-cam-label">${escapeHtml(label)}</div>` +
                    `</div>`;
            }).join('');
            sections.push(this.section('Camera', [`<div class="folder-cam-chart">${items}</div>`]));
        }

        // Shooting hours (24-bar chart)
        if (stats.shootingHours && stats.shootingHours.some(h => h > 0)) {
            sections.push(this.section('Shooting Hours', [this._renderHoursChart(stats.shootingHours)]));
        }

        return sections;
    }

    _renderHoursChart(hours) {
        const max = Math.max(...hours, 1);
        const bars = hours.map((count, h) => {
            const pct = Math.round((count / max) * 100);
            const label = h + 'h';
            return `<div class="folder-hours-col" title="${label}: ${count}">` +
                `<div class="folder-hours-bar" style="height:${pct}%"></div>` +
                `<div class="folder-hours-label">${h % 6 === 0 ? label : ''}</div>` +
                '</div>';
        }).join('');
        return `<div class="folder-hours-chart">${bars}</div>`;
    }

    _attachFolderEvents() {
        this.container.querySelectorAll('.folder-treemap-cell').forEach(cell => {
            cell.addEventListener('click', () => {
                const path = cell.dataset.path;
                if (path && this.onDirNavigate) this.onDirNavigate(path);
            });
        });
    }
}
