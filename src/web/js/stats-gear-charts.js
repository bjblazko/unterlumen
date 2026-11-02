// stats-gear-charts.js — the Equipment and Exposure views drawn from the
// "gear" tallies (library/insights_gear.go): phone or camera, how each zoom
// is used, the life lines of cameras and lenses, the risk of shake, the
// scenes' light, exposure times, flash and exposure compensation over time,
// and white balance.

function renderDeviceShare(el, gear, onPick) {
    // The camera's share is the rest; one line says it all.
    renderShareLines(el, gear?.device, ['phone'], onPick, { labels: { phone: 'Taken with a phone' } });
}

// The lenses used at three focal lengths or more — the zooms — each as a
// strip along the focal lengths it was used at.
function renderZoomUse(el, gear, onPick) {
    const byLens = new Map();
    for (const r of tallyRows(gear?.lensFocal)) {
        const [lens, mm] = r.parts;
        if (!byLens.has(lens)) byLens.set(lens, new Map());
        byLens.get(lens).set(Number(mm), r.n);
    }
    const zooms = [...byLens.entries()].filter(([, m]) => m.size >= 3)
        .map(([name, cells]) => ({ name, cells, total: [...cells.values()].reduce((a, b) => a + b, 0) }))
        .sort((a, b) => b.total - a.total).slice(0, 10);
    if (!zooms.length) { tlNoData(el, 'No zoom lens has been used at three focal lengths or more.'); return; }
    const values = [...new Set(zooms.flatMap(z => [...z.cells.keys()]))].sort((a, b) => a - b);
    renderValueStrips(el, zooms, values, v => onPick?.(v), { valueLabel: v => `${v} mm` });
}

// A lane for each of the cameras and lenses with the most photos, its
// periods shaded by how many it took then.
function renderGearLives(el, gear, onPick) {
    const lanes = (tally, kind) => {
        const rows = tallyRows(tally);
        const totals = sumBy(rows, p => p[0]);
        return [...totals.entries()].sort((a, b) => b[1] - a[1]).slice(0, 8)
            .map(([name]) => ({ name, kind, rows: rows.filter(r => r.parts[0] === name) }));
    };
    const all = [...lanes(gear?.cameraMonth, 'camera'), ...lanes(gear?.lensMonth, 'lens')];
    const { periods, of } = tallyPeriods(all.flatMap(l => l.rows.map(r => r.parts[1])));
    const groups = all.map(l => ({ name: l.name, kind: l.kind, cells: sumBy(l.rows, p => of(p[1])) }));
    renderValueStrips(el, groups, periods, v => onPick?.(v), { every: periods[0]?.length === 4 ? 2 : 12 });
}

// Per lens, the share of photos exposed longer than 1/focal length (35 mm).
function renderShakeRisk(el, gear, onPick) {
    const rows = tallyRows(gear?.shake);
    const all = sumBy(rows, p => p[0]);
    const slow = sumBy(rows.filter(r => r.parts[1] === 'slow'), p => p[0]);
    const items = [...all.entries()].filter(([, n]) => n >= 10)
        .map(([name, n]) => ({ name, slow: slow.get(name) || 0, all: n, count: Math.round(100 * (slow.get(name) || 0) / n) }))
        .sort((a, b) => b.count - a.count);
    renderCountBars(el, items, onPick, {
        empty: 'No lens has ten photos with an exposure time and a focal length.',
        valueText: d => `${d.count} %`,
        aria: d => t('{share} % at risk, {n} of {all}', { share: d.count, n: formatCount(d.slow), all: formatCount(d.all) }),
    });
}

// The exposure value at ISO 100 of each photo: how bright its scene was.
function renderSceneLight(el, gear, onPick) {
    const m = sumBy(tallyRows(gear?.ev), p => Number(p[0]));
    if (!m.size) { el.textContent = t('No data'); return; }
    const lo = Math.max(-6, Math.min(...m.keys())), hi = Math.min(20, Math.max(...m.keys()));
    const bins = [];
    for (let v = lo; v <= hi; v++) bins.push({ ev: v, label: String(v), count: m.get(v) || 0 });
    renderValueHistogram(el, bins, onPick, { half: true, unit: ' EV' });
}

// Exposure times in whole stops, from the shortest to the longest.
function renderExposureTimes(el, gear, onPick) {
    const m = sumBy(tallyRows(gear?.exposureTime), p => Number(p[0]));
    if (!m.size) { el.textContent = t('No data'); return; }
    const lo = Math.max(-14, Math.min(...m.keys())), hi = Math.min(6, Math.max(...m.keys()));
    const bins = [];
    for (let k = lo; k <= hi; k++) bins.push({ stop: k, label: exposureStopLabel(k), count: m.get(k) || 0 });
    renderValueHistogram(el, bins, onPick, { half: true });
}

// The shutter speeds cameras name, for each whole stop below a second.
const SHUTTER_STOPS = { 1: 2, 2: 4, 3: 8, 4: 15, 5: 30, 6: 60, 7: 125, 8: 250, 9: 500, 10: 1000, 11: 2000, 12: 4000, 13: 8000, 14: 16000 };

function exposureStopLabel(k) {
    return k < 0 ? `1/${SHUTTER_STOPS[-k] ?? 2 ** -k}` : `${2 ** k} s`;
}

function renderFlashShare(el, gear, onPick) {
    renderShareLines(el, gear?.flash, ['fired'], onPick, { labels: { fired: 'Flash fired' } });
}

function renderBiasShare(el, gear, onPick) {
    renderShareLines(el, gear?.bias, ['minus', 'zero', 'plus'], onPick, { labels: { minus: 'Darker', zero: 'As metered', plus: 'Brighter' } });
}

function renderWhiteBalance(el, gear, onPick) {
    const m = sumBy(tallyRows(gear?.whiteBalance), p => p[0]);
    const items = [['0', 'Automatic'], ['1', 'Set by hand']].filter(([v]) => m.get(v))
        .map(([value, label]) => ({ name: value, label: t(label), count: m.get(value) }));
    renderCountBars(el, items, onPick, { half: true });
}
