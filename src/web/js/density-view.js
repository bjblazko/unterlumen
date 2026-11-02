// density-view.js — the Density landscape (Places): the world map as the
// floor, a column on every half degree where photos were taken, its height
// the number of photos (on a log scale), its colour their most common main
// colour. Where one returns again and again rises high. Drawn on a point
// stage from the "places" tallies (library/insights_places.go).

const DENSITY_CELL = 0.5; // degrees

// densityFloor is a place on the floor: longitude across, latitude in depth.
function densityFloor(lat, lon) {
    return [lon / 180, -lat / 180];
}

function densityCells(places) {
    const cells = new Map();
    for (const r of tallyRows(places?.density)) {
        const [la, lo, hue] = r.parts;
        const k = la + ',' + lo;
        const c = cells.get(k) || { lat: Number(la) * DENSITY_CELL, lon: Number(lo) * DENSITY_CELL, n: 0, hues: new Map() };
        c.n += r.n;
        c.hues.set(hue, (c.hues.get(hue) || 0) + r.n);
        cells.set(k, c);
    }
    return [...cells.values()];
}

// A hue bin's colour, as the Colour topic draws its twelve hues.
function hueBinRGB(bin) {
    if (bin === '-') return [0.62, 0.64, 0.68];
    const h = (Number(bin) * 30 + 15) * Math.PI / 180;
    return oklabToStageRGB(0.72, 0.13 * Math.cos(h), 0.13 * Math.sin(h));
}

function renderDensity(el, places, onPick, options) {
    const cells = densityCells(places);
    if (!cells.length) { tlNoData(el, 'No photo here has a location yet.'); return; }
    worldLines().then(world => renderPointStage(el, () => densitySpec(cells, world), onPick, options));
}

function densitySpec(cells, world) {
    const most = Math.max(...cells.map(c => c.n));
    const height = n => -1 + 1.8 * Math.log10(n + 1) / Math.log10(most + 1);
    const sprites = new Float32Array(cells.length * SPRITE_FLOATS);
    const trail = new Float32Array(cells.length * SEGMENT_FLOATS);
    cells.forEach((c, i) => {
        const [x, z] = densityFloor(c.lat, c.lon);
        const hue = [...c.hues.entries()].sort((a, b) => b[1] - a[1])[0][0];
        const rgb = hueBinRGB(hue);
        const top = height(c.n);
        sprites.set([x, top, z, ...rgb, 6, 0.9], i * SPRITE_FLOATS);
        trail.set([x, -1, z, ...rgb, x, top, z, ...rgb], i * SEGMENT_FLOATS);
    });
    return {
        summary: t('{photos} with a location on the world map: a column on every half degree where photos were taken, higher the more there are (on a log scale), in their most common main colour.', { photos: countText(cells.reduce((s, c) => s + c.n, 0), 'photo') }),
        legend: legendDotHTML(t('The top of a column; it shows the photos taken there')),
        sprites, steps: new Float32Array(0), trail, trailWidth: 3,
        distance: 3.2, pitch: 0.7,
        lines: densityLines(world),
        labels: [],
        tip: (kind, i) => `<span class="point-stage-tip-title">${escapeHtml(formatLatLon(cells[i].lat, cells[i].lon))}</span><span>${escapeHtml(photoCount(cells[i].n))}</span>`,
        pick: (kind, i) => ({ cell: cells[i] }),
    };
}

function densityLines(world) {
    const lines = new StageLines();
    const draw = (list, alpha) => {
        for (const line of list) {
            let prev = null;
            for (const [lon, lat] of line) {
                const [x, z] = densityFloor(lat, lon);
                if (prev && Math.abs(x - prev[0]) < 0.5) lines.line([prev[0], -1, prev[1]], [x, -1, z], alpha);
                prev = [x, z];
            }
        }
    };
    draw(world.coast, 0.32);
    draw(world.borders, 0.12);
    return lines.toArray();
}

function densityCriterion(cell) {
    const h = DENSITY_CELL / 2;
    return { subject: formatLatLon(cell.lat, cell.lon), params: { lat_from: cell.lat - h, lat_to: cell.lat + h, lon_from: cell.lon - h, lon_to: cell.lon + h } };
}
