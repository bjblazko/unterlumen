// info-exif.js — the photo's own details in the info panel: the short list
// that tells two frames apart, where it was taken, and — folded away —
// everything else its file records. Every tag shown is noted in `used`, so
// "All metadata" lists only the rest.

const INFO_GPS_TAGS = ['GPSLatitude', 'GPSLatitudeRef', 'GPSLongitude', 'GPSLongitudeRef',
    'GPSAltitude', 'GPSAltitudeRef', 'GPSTimeStamp', 'GPSDateStamp', 'GPSVersionID'];
const INFO_DATE_TAGS = ['DateTimeOriginal', 'DateTimeDigitized', 'DateTime',
    'OffsetTimeOriginal', 'OffsetTimeDigitized', 'OffsetTime'];
const INFO_SIZE_TAGS = ['PixelXDimension', 'PixelYDimension', 'ImageWidth', 'ImageLength'];
// Fujifilm's maker notes are why a group of their own exists: a film
// simulation says more about a frame than most EXIF tags.
const INFO_FUJI_TAG = /^(Fuji|DynamicRange|GrainEffect|ColorChrome|Highlight|Shadow|NoiseReduction|Sharpness|Saturation|WhiteBalanceFineTune)/i;

class InfoExif {
    constructor(panel) {
        this._panel = panel;
    }

    // The short list: what you need to tell two frames apart.
    shortList(d, tags, used) {
        const p = this._panel;
        const rows = [p.row('Name', d.name)];
        // A date its file's name gave says so: no camera recorded it.
        const named = tags.DateTakenFrom ? ` · ${t('from the file name')}` : '';
        used.add('DateTakenFrom');
        rows.push(d.exif?.dateTaken ? p.row('Taken', p.formatDate(d.exif.dateTaken) + named) : p.row('Modified', p.formatDate(d.modified)));
        this._tag(rows, tags, used, 'Model', 'Camera');
        this._tag(rows, tags, used, 'LensModel', 'Lens');
        const exposure = exposureLine(tags);
        if (exposure) rows.push(p.row('Exposure', exposure));
        if (tags.Duration) {
            used.add('Duration');
            rows.push(p.row('Length', formatDuration(tags.Duration)));
        }
        if (d.exif?.width && d.exif?.height) {
            const ratio = aspectRatioLabel(d.exif.width, d.exif.height);
            rows.push(p.row('Size', d.exif.width + ' × ' + d.exif.height + (ratio ? ' · ' + ratio : '')));
        }
        rows.push(p.row('File', formatSize(d.size)));
        rows.push(p.badgeRow('Format', (d.format || '').toUpperCase()));
        return p.section(isVideoName(d.name) ? 'Video' : 'Photo', rows);
    }

    // Where it was taken, with the map right there — or an honest "None".
    location(d, used) {
        const p = this._panel;
        INFO_GPS_TAGS.forEach(tag => used.add(tag));
        if (!d.exif || d.exif.latitude == null || d.exif.longitude == null) {
            return p.section('Location', [p.row('Coordinates', t('None'))]);
        }
        const lat = d.exif.latitude.toFixed(6);
        const lon = d.exif.longitude.toFixed(6);
        return p.section('Location', [
            '<div id="info-map" class="info-map-container" data-lat="' + lat + '" data-lon="' + lon + '"></div>',
            '<div class="info-map-controls">' +
                '<div class="info-map-style-wrap"></div>' +
                '<button class="btn btn-sm" data-action="open">↗ ' + t('Open') + '</button>' +
            '</div>',
            p.row('Latitude', lat),
            p.row('Longitude', lon),
        ]);
    }

    // Everything else, folded away and grouped, so the panel stays short but
    // loses nothing.
    allMetadata(d, tags, used, open) {
        const groups = [
            this._group('Camera', this._cameraRows(tags, used)),
            this._group('Exposure', this._exposureRows(tags, used)),
            this._group('Fujifilm', this._fujiRows(tags, used)),
            this._group('File', this._fileRows(d, used)),
            this._group('XMP and other tags', Object.keys(tags).sort().filter(k => !used.has(k))
                .map(k => this._panel.row(k, exifText(tags[k])))),
        ];
        return '<details class="info-all-meta"' + (open ? ' open' : '') + '>' +
            '<summary>' + t('All metadata') + '</summary>' +
            '<div class="info-all-meta-body">' + groups.join('') + '</div>' +
            '</details>';
    }

    _cameraRows(tags, used) {
        const rows = [];
        this._tag(rows, tags, used, 'Make', 'Make');
        this._tag(rows, tags, used, 'Software', 'Software');
        this._tag(rows, tags, used, 'Orientation', 'Orientation', v => exifLabel('Orientation', v) ?? v);
        this._tag(rows, tags, used, 'ColorSpace', 'Color Space', v => exifLabel('ColorSpace', v) ?? v);
        return rows;
    }

    _exposureRows(tags, used) {
        const rows = [];
        const labelled = field => v => exifLabel(field, v) ?? v;
        this._tag(rows, tags, used, 'ExposureTime', 'Shutter Speed', decodeExposureTime);
        this._tag(rows, tags, used, 'FNumber', 'Aperture', decodeFNumber);
        this._tag(rows, tags, used, 'ISOSpeedRatings', 'ISO');
        this._tag(rows, tags, used, 'ExposureBiasValue', 'Exposure Bias', decodeExposureBias);
        this._tag(rows, tags, used, 'FocalLength', 'Focal Length', decodeFocalLength);
        this._tag(rows, tags, used, 'MeteringMode', 'Metering', labelled('MeteringMode'));
        this._tag(rows, tags, used, 'ExposureProgram', 'Program', labelled('ExposureProgram'));
        this._tag(rows, tags, used, 'Flash', 'Flash', labelled('Flash'));
        this._tag(rows, tags, used, 'WhiteBalance', 'White Balance', labelled('WhiteBalance'));
        return rows;
    }

    _fujiRows(tags, used) {
        const p = this._panel;
        const rows = [];
        if (tags['FilmSimulation'] != null) {
            used.add('FilmSimulation');
            rows.push(p.badgeRow('Film Simulation', exifText(tags['FilmSimulation'])));
        }
        for (const key of Object.keys(tags).sort()) {
            if (used.has(key) || !INFO_FUJI_TAG.test(key)) continue;
            used.add(key);
            rows.push(p.row(key, exifText(tags[key])));
        }
        return rows;
    }

    _fileRows(d, used) {
        const p = this._panel;
        INFO_DATE_TAGS.forEach(tag => used.add(tag));
        INFO_SIZE_TAGS.forEach(tag => used.add(tag));
        const rows = [p.row('Path', d.path), p.row('Modified', p.formatDate(d.modified))];
        if (d.exif.dateDigitized) rows.push(p.row('Digitized', p.formatDate(d.exif.dateDigitized)));
        if (d.exif.dateModified) rows.push(p.row('EXIF modified', p.formatDate(d.exif.dateModified)));
        return rows;
    }

    // _tag adds the row of one tag, decoded, and notes it as shown.
    _tag(rows, tags, used, tagName, label, decode) {
        if (tags[tagName] == null) return;
        used.add(tagName);
        let val = exifText(tags[tagName]);
        if (decode) val = decode(val);
        if (val != null) rows.push(this._panel.row(label, val));
    }

    // A group with no rows is left out.
    _group(title, rows) {
        if (!rows.length) return '';
        return '<div class="info-meta-group">' +
            '<h4 class="info-meta-group-title">' + escapeHtml(t(title)) + '</h4>' +
            rows.join('') +
            '</div>';
    }
}

// exposureLine is a frame's exposure on one line: "1/250 · f/2.8 · ISO 200 · 23 mm".
function exposureLine(tags) {
    const tag = name => (tags[name] != null ? exifText(tags[name]) : null);
    return [
        tag('ExposureTime') != null ? decodeExposureTime(tag('ExposureTime')) : null,
        tag('FNumber') != null ? decodeFNumber(tag('FNumber')) : null,
        tag('ISOSpeedRatings') != null ? 'ISO ' + tag('ISOSpeedRatings') : null,
        tag('FocalLength') != null ? decodeFocalLength(tag('FocalLength')) : null,
    ].filter(Boolean).join(' · ');
}
