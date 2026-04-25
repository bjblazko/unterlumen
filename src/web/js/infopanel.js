// Info side panel — displays file metadata and EXIF data

class InfoPanel {
    constructor(container) {
        this.container = container;
        this.expanded = false;
        this.data = null;
        this.error = null;
        this.currentPath = null;
        this.loading = false;
        this.collapsedSections = new Set();
        this.onToggle = null;
        this._metaContext = null;
        this.render();
    }

    toggle() {
        this.expanded = !this.expanded;
        this.render();
        if (this.onToggle) this.onToggle();
    }

    async loadInfo(path) {
        if (path === this.currentPath && this.data) {
            this.render();
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

    clear() {
        this.data = null;
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
                    '<button class="info-toggle-btn" title="Show info (I)">i</button>' +
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

        controls.addEventListener('click', (e) => {
            const btn = e.target.closest('button');
            if (!btn) return;
            const action = btn.dataset.action;
            if (action === '2d') {
                this.mapStyle = '2d';
                this.map.setPitch(0);
                this.map.setBearing(0);
            } else if (action === '3d') {
                this.mapStyle = '3d';
                this.map.setPitch(60);
            } else if (action === 'open') {
                window.open('https://www.openstreetmap.org/#map=16/' + lat + '/' + lon, '_blank');
                return;
            }
            controls.querySelectorAll('button[data-action="2d"], button[data-action="3d"]').forEach(b => {
                b.classList.toggle('active', b.dataset.action === this.mapStyle);
            });
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
                '<button class="btn btn-sm active" data-action="2d">2D</button>' +
                '<button class="btn btn-sm" data-action="3d">3D</button>' +
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
        this.addTag(expRows, tags, used, 'ExposureTime', 'Shutter Speed');
        this.addTag(expRows, tags, used, 'FNumber', 'Aperture');
        this.addTag(expRows, tags, used, 'ISOSpeedRatings', 'ISO');
        this.addTag(expRows, tags, used, 'ExposureBiasValue', 'Exposure Bias');
        this.addTag(expRows, tags, used, 'FocalLength', 'Focal Length');
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

        if (this._metaContext) sections.push(this._renderMetaSection());

        return sections.join('');
    }

    _renderMetaSection() {
        const ctx = this._metaContext;
        const rows = [];

        for (const e of (ctx.entries || [])) {
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

    decodeMeteringMode(v) {
        const modes = { '0': 'Unknown', '1': 'Average', '2': 'Center-weighted', '3': 'Spot',
            '4': 'Multi-spot', '5': 'Multi-segment', '6': 'Partial' };
        return modes[v] || v;
    }

    decodeExposureProgram(v) {
        const progs = { '0': 'Unknown', '1': 'Manual', '2': 'Program AE', '3': 'Aperture Priority',
            '4': 'Shutter Priority', '5': 'Creative', '6': 'Action', '7': 'Portrait', '8': 'Landscape' };
        return progs[v] || v;
    }

    decodeFlash(v) {
        const val = parseInt(v);
        if (isNaN(val)) return v;
        return (val & 1) ? 'Fired' : 'No flash';
    }

    decodeWhiteBalance(v) {
        return v === '0' ? 'Auto' : v === '1' ? 'Manual' : v;
    }

    decodeOrientation(v) {
        const map = {
            '1': 'Normal',
            '2': 'Flipped horizontally',
            '3': 'Rotated 180°',
            '4': 'Flipped vertically',
            '5': 'Transposed (flip H + 270° CW)',
            '6': 'Rotated 90° CW',
            '7': 'Transverse (flip H + 90° CW)',
            '8': 'Rotated 270° CW',
        };
        return map[v] || v;
    }

    decodeColorSpace(v) {
        return v === '1' ? 'sRGB' : v === '65535' ? 'Uncalibrated' : v;
    }

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
                    ctx.entries = ctx.entries.filter(e => e.key !== key);
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
}
