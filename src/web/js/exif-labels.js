// exif-labels.js — human-readable EXIF value decoders
// Shared by infopanel.js and library-filter.js so both show identical labels.

const EXIF_LABELS = {
    Orientation: {
        '1': 'Normal',
        '2': 'Flipped horizontally',
        '3': 'Rotated 180°',
        '4': 'Flipped vertically',
        '5': 'Transposed',
        '6': 'Rotated 90° CW',
        '7': 'Transverse',
        '8': 'Rotated 270° CW',
    },
    // Flash is a bitmask — bit 0 = fired; higher bits = strobe return / mode / red-eye.
    Flash: v => {
        const n = parseInt(v);
        if (isNaN(n)) return null;
        return (n & 1) ? 'Fired' : 'No flash';
    },
    WhiteBalance: { '0': 'Auto', '1': 'Manual' },
    MeteringMode: {
        '0': 'Unknown', '1': 'Average', '2': 'Center-weighted',
        '3': 'Spot', '4': 'Multi-spot', '5': 'Multi-segment', '6': 'Partial',
    },
    ExposureProgram: {
        '0': 'Unknown', '1': 'Manual', '2': 'Program AE',
        '3': 'Aperture priority', '4': 'Shutter priority',
        '5': 'Creative', '6': 'Action', '7': 'Portrait', '8': 'Landscape',
    },
    ExposureMode: { '0': 'Auto', '1': 'Manual', '2': 'Auto bracket' },
    ColorSpace: { '1': 'sRGB', '65535': 'Uncalibrated' },
    SceneCaptureType: { '0': 'Standard', '1': 'Landscape', '2': 'Portrait', '3': 'Night' },
    GainControl: { '0': 'None', '1': 'Low gain up', '2': 'High gain up', '3': 'Low gain down', '4': 'High gain down' },
    Contrast: { '0': 'Normal', '1': 'Soft', '2': 'Hard' },
    Saturation: { '0': 'Normal', '1': 'Low', '2': 'High' },
    Sharpness: { '0': 'Normal', '1': 'Soft', '2': 'Hard' },
    SubjectDistanceRange: { '0': 'Unknown', '1': 'Macro', '2': 'distance|Close', '3': 'Distant' },
    SensingMethod: {
        '1': 'Undefined', '2': 'One-chip color area', '3': 'Two-chip color area',
        '4': 'Three-chip color area', '5': 'Color sequential area',
        '7': 'Trilinear', '8': 'Color sequential linear',
    },
    LightSource: {
        '0': 'Unknown', '1': 'Daylight', '2': 'Fluorescent', '3': 'Tungsten',
        '4': 'Flash', '9': 'Fine weather', '10': 'Cloudy', '11': 'Shade',
        '12': 'Daylight fluorescent', '13': 'Day white fluorescent',
        '14': 'Cool white fluorescent', '15': 'White fluorescent',
        '17': 'Standard A', '18': 'Standard B', '19': 'Standard C',
        '20': 'D55', '21': 'D65', '22': 'D75', '23': 'D50',
        '24': 'ISO studio tungsten', '255': 'Other',
    },
    CustomRendered: { '0': 'Normal', '1': 'Custom' },
    FocalLengthIn35mmFilm: v => v ? `${v} mm` : null,
};

// Returns the human-readable label for a raw EXIF value, or null if no mapping exists.
function exifLabel(field, rawValue) {
    const decoder = EXIF_LABELS[field];
    if (!decoder) return null;
    const label = typeof decoder === 'function' ? decoder(rawValue) : decoder[String(rawValue)];
    return label == null ? null : t(label);
}

// Applies exifLabel to an array of raw values, deduplicates by label, and returns
// either plain strings (no decoder) or [{label, value}] pairs (decoder exists).
function labeledExifValues(field, rawValues) {
    const decoder = EXIF_LABELS[field];
    if (!decoder) return rawValues;
    const seen = new Map(); // label → first rawValue that produced it
    for (const raw of rawValues) {
        const label = exifLabel(field, raw) ?? raw;
        if (!seen.has(label)) seen.set(label, raw);
    }
    return Array.from(seen.entries()).map(([label, value]) => ({ label, value }));
}

// exifText is a tag's value as text: exiftool quotes some strings.
function exifText(s) {
    if (typeof s !== 'string') return s;
    return s.length >= 2 && s[0] === '"' && s[s.length - 1] === '"' ? s.slice(1, -1) : s;
}

// parseRational reads "28/10" or "2.8"; null when it is neither.
function parseRational(s) {
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

// An aperture of 0 (written 0/1) means the lens did not say.
function decodeFNumber(s) {
    const v = parseRational(s);
    if (v === null) return s;
    if (v <= 0) return null;
    return `f/${v.toFixed(1)}`;
}

function decodeFocalLength(s) {
    const v = parseRational(s);
    if (v === null || v <= 0) return s;
    return v < 10 ? `${v.toFixed(2).replace(/\.?0+$/, '')} mm` : `${Math.round(v)} mm`;
}

function decodeExposureTime(s) {
    const v = parseRational(s);
    if (v === null) return s;
    if (v >= 1) return Number.isInteger(v) ? `${v} s` : `${v.toFixed(1)} s`;
    return `1/${Math.round(1 / v)}`;
}

function decodeExposureBias(s) {
    const v = parseRational(s);
    if (v === null) return s;
    if (v === 0) return '0 EV';
    const sign = v > 0 ? '+' : '';
    return `${sign}${v.toFixed(2).replace(/\.?0+$/, '')} EV`;
}

// aspectRatioLabel names a frame's shape, "3:2", when it is a known one.
function aspectRatioLabel(w, h) {
    if (!w || !h) return '';
    const known = [
        [1, 2], [9, 16], [2, 3], [3, 4], [4, 5],
        [1, 1],
        [5, 4], [4, 3], [3, 2], [7, 5], [16, 10], [5, 3], [16, 9], [2, 1], [21, 9],
    ];
    const ratio = w / h;
    for (const [rw, rh] of known) {
        if (Math.abs(ratio - rw / rh) / (rw / rh) < 0.015) return rw + ':' + rh;
    }
    return t('Custom Crop');
}

// aspectRatioIcon draws a frame of that shape; a custom crop's is dashed.
function aspectRatioIcon(ratioStr, stroke) {
    const isCustom = !ratioStr.includes(':');
    const [a, b] = isCustom ? [1, 1] : ratioStr.split(':').map(parseFloat);
    const ratio = a / b || 1;
    const [rw, rh] = ratio > 14 / 10 ? [14, 14 / ratio] : [10 * ratio, 10];
    const x = ((16 - rw) / 2).toFixed(1);
    const y = ((12 - rh) / 2).toFixed(1);
    const dash = isCustom ? ' stroke-dasharray="2 1"' : '';
    return `<svg width="16" height="12" viewBox="0 0 16 12" fill="none" stroke="${stroke}" stroke-width="1.5"${dash}><rect x="${x}" y="${y}" width="${rw.toFixed(1)}" height="${rh.toFixed(1)}" rx="0.5"/></svg>`;
}
