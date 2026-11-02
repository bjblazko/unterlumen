// stats-frame-charts.js — the Frame views drawn from the "frame" tallies
// (library/insights_frame.go): how each camera is held, the share of
// cropped photos over time, and the aspect ratio as a histogram.

const ORIENTATIONS = ['landscape', 'portrait', 'square'];
const ORIENTATION_NAMES = { landscape: 'Landscape', portrait: 'Portrait', square: 'Square' };

function renderOrientation(el, frame, onPick) {
    const rows = tallyRows(frame?.orient);
    const totals = sumBy(rows, p => p[0]);
    const cameras = [...totals.entries()].sort((a, b) => b[1] - a[1]).slice(0, 12).map(([name]) => name);
    const cell = sumBy(rows, p => p[0] + TALLY_SEP + p[1]);
    renderTallyMatrix(el, cameras, ORIENTATIONS, (cam, o) => cell.get(cam + TALLY_SEP + o) || 0, onPick,
        { colLabel: o => t(ORIENTATION_NAMES[o]) });
}

function renderCropped(el, frame, onPick) {
    renderShareLines(el, frame?.cropped, ['cropped'], onPick, { labels: { cropped: 'Cropped' } });
}

// The long side over the short one, in steps of 0.05 from square to 3:1.
function renderRatioHistogram(el, frame, onPick) {
    const m = sumBy(tallyRows(frame?.ratio), p => Number(p[0]).toFixed(2));
    const bins = [];
    for (let i = 20; i <= 60; i++) {
        const v = (i / 20).toFixed(2);
        bins.push({ ratio: Number(v), label: v, count: m.get(v) || 0 });
    }
    renderValueHistogram(el, bins, onPick);
}
