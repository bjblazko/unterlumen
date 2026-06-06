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
    SubjectDistanceRange: { '0': 'Unknown', '1': 'Macro', '2': 'Close', '3': 'Distant' },
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
    if (typeof decoder === 'function') return decoder(rawValue) ?? null;
    return decoder[String(rawValue)] ?? null;
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
