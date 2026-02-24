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
        this.render();
    }

    toggle() {
        this.expanded = !this.expanded;
        this.render();
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
        this.render();
    }

    render() {
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
    }

    renderData(d) {
        const sections = [];

        // File section
        const fileRows = [];
        fileRows.push(this.row('Name', d.name));
        fileRows.push(this.row('Path', d.path));
        fileRows.push(this.row('Size', formatSize(d.size)));
        fileRows.push(this.row('Format', (d.format || '').toUpperCase()));
        fileRows.push(this.row('Modified', this.formatDate(d.modified)));
        sections.push(this.section('File', fileRows));

        if (!d.exif) return sections.join('');

        const tags = d.exif.tags || {};
        const used = new Set();

        // Image section
        const imageRows = [];
        if (d.exif.width && d.exif.height) {
            imageRows.push(this.row('Dimensions', d.exif.width + ' \u00d7 ' + d.exif.height));
        }
        this.addTag(imageRows, tags, used, 'Orientation', 'Orientation');
        this.addTag(imageRows, tags, used, 'ColorSpace', 'Color Space', this.decodeColorSpace);
        if (imageRows.length) sections.push(this.section('Image', imageRows));

        // Camera section
        const cameraRows = [];
        this.addTag(cameraRows, tags, used, 'Make', 'Make');
        this.addTag(cameraRows, tags, used, 'Model', 'Model');
        this.addTag(cameraRows, tags, used, 'LensModel', 'Lens');
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

        // Dates section
        const dateRows = [];
        this.addTag(dateRows, tags, used, 'DateTimeOriginal', 'Original');
        this.addTag(dateRows, tags, used, 'DateTimeDigitized', 'Digitized');
        this.addTag(dateRows, tags, used, 'DateTime', 'Modified');
        if (dateRows.length) sections.push(this.section('Dates', dateRows));

        // Location section
        if (d.exif.latitude != null && d.exif.longitude != null) {
            const locRows = [];
            locRows.push(this.row('Latitude', d.exif.latitude.toFixed(6)));
            locRows.push(this.row('Longitude', d.exif.longitude.toFixed(6)));
            sections.push(this.section('Location', locRows));
        }
        // Mark GPS tags as used
        ['GPSLatitude', 'GPSLatitudeRef', 'GPSLongitude', 'GPSLongitudeRef',
         'GPSAltitude', 'GPSAltitudeRef', 'GPSTimeStamp', 'GPSDateStamp',
         'GPSVersionID'].forEach(t => used.add(t));
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

        return sections.join('');
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
        const d = new Date(iso);
        return d.toLocaleString();
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

    decodeColorSpace(v) {
        return v === '1' ? 'sRGB' : v === '65535' ? 'Uncalibrated' : v;
    }
}
