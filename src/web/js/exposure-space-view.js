// exposure-space-view.js — the Exposure space topic (ADR-0046): every photo
// at its focal length (across), aperture (in depth) and ISO (up), each on a
// scale of stops, coloured by its camera; and a trail through each period's
// median settings. How one photographs shows as clusters. Drawn on a point
// stage (point-stage.js).

// The ranges of the axes; anything beyond sits on the edge.
const EXPOSURE_AXES = {
    focal: { min: 8, max: 800, ticks: [14, 24, 35, 50, 85, 135, 200, 400, 800], label: v => `${v} mm` },
    fnum: { min: 1, max: 32, ticks: [1.4, 2, 2.8, 4, 5.6, 8, 11, 16, 22], label: v => `f/${v}` },
    iso: { min: 25, max: 51200, ticks: [100, 400, 1600, 6400, 25600], label: v => `ISO ${v}` },
};

// A value on its axis, -1…1, in stops.
function exposureAxis(axis, v) {
    const { min, max } = EXPOSURE_AXES[axis];
    const t = (Math.log(v) - Math.log(min)) / (Math.log(max) - Math.log(min));
    return Math.max(-1, Math.min(1, t * 2 - 1));
}

function exposurePosition(focal, fnum, iso) {
    return [exposureAxis('focal', focal), exposureAxis('iso', iso), exposureAxis('fnum', fnum)];
}

// Settings come in stops, so many photos share one exactly. An offset of
// its own, within about a third of a stop and most often near none, lets
// each be told apart and a crowd glow as a cloud.
function exposureJitter(i, k) {
    const r = j => {
        const x = Math.sin((i + 1) * 12.9898 + (k * 2 + j) * 78.233) * 43758.5453;
        return x - Math.floor(x) - 0.5;
    };
    return (r(0) + r(1)) * 0.035;
}

// The last camera is "Other" when there are more than the chart names; it
// is grey rather than a slot of the chart ramp (ADR-0034).
function exposureCameraColours(stageEl, es) {
    const style = getComputedStyle(stageEl);
    return es.cameras.map((name, i) => name === 'Other'
        ? [0.62, 0.65, 0.7]
        : cssColourToRGB(style.getPropertyValue(`--chart-${i + 1}`).trim() || '#888'));
}

// renderExposureSpace draws es (/api/library/exposure-space); total is the
// number of photos in the scope. onPick gets { photo } or { step }.
function renderExposureSpace(el, es, total, onPick, options) {
    if (!es?.photos) {
        tlNoData(el, 'No photo here has focal length, aperture and ISO in its EXIF.');
        return;
    }
    renderPointStage(el, stageEl => exposureSpaceSpec(es, total, exposureCameraColours(stageEl, es)), onPick, options);
}

function exposureSpaceSpec(es, total, colours) {
    const p = es.points;
    const n = p.id.length;
    const light = pointLight(n);
    const sprites = new Float32Array(n * SPRITE_FLOATS);
    for (let i = 0; i < n; i++) {
        const [x, y, z] = exposurePosition(p.focal[i], p.fnum[i], p.iso[i]);
        sprites.set([x + exposureJitter(i, 0), y + exposureJitter(i, 1), z + exposureJitter(i, 2), ...colours[p.camera[i]], light.radius, light.alpha], i * SPRITE_FLOATS);
    }
    const most = Math.max(1, ...es.path.map(s => s.photos));
    const steps = new Float32Array(es.path.flatMap(s => [
        ...exposurePosition(s.focal, s.fnum, s.iso), 0.96, 0.94, 0.88, stepRadius(s.photos, most), 1,
    ]));
    return {
        summary: exposureSpaceSummary(es, total),
        legend: exposureLegendHTML(es, colours),
        sprites, steps,
        distance: 5.6, // the box's corners reach further out than a cloud
        trail: trailBetween(steps),
        lines: exposureLines(),
        labels: exposureLabels(es),
        tip: (kind, i) => exposureTip(es, kind, i),
        pick: (kind, i) => kind === 'step'
            ? { step: es.path[i] }
            : { photo: { id: p.id[i], lib: es.libraries[p.lib[i]], date: p.date[i] } },
    };
}

function exposureSpaceSummary(es, total) {
    const n = es.photos;
    const of = total > n ? ` of ${formatCount(total)}` : '';
    const trail = es.path.length > 1
        ? ` The trail joins the median settings of each ${periodUnit(es.granularity)}${pathSpan(es.path)}.`
        : '';
    return `${formatCount(n)}${of} photos with focal length, aperture and ISO: focal length across, aperture in depth and ISO up, each in stops, coloured by camera.${trail}`;
}

function exposureLegendHTML(es, colours) {
    const unit = periodUnit(es.granularity);
    const cameras = es.cameras.map((name, i) => {
        const [r, g, b] = colours[i].map(c => Math.round(c * 255));
        return `<span class="point-stage-legend-item"><span class="point-stage-legend-swatch" style="background:rgb(${r} ${g} ${b})" aria-hidden="true"></span>${escapeHtml(name)}</span>`;
    }).join('');
    return cameras + legendStepHTML(`One ${unit}: the median focal length, aperture and ISO of its photos. The line joins the ${unit}s in time order${pathSpan(es.path)}.`);
}

function formatFNum(v) {
    return `f/${Number(v.toFixed(1))}`;
}

function exposureSettingsHTML(focal, fnum, iso) {
    return `<span class="point-stage-tip-values">${Math.round(focal)} mm · ${formatFNum(fnum)} · ISO ${Math.round(iso)}</span>`;
}

function exposureTip(es, kind, i) {
    if (kind === 'step') {
        const s = es.path[i];
        return `<span class="point-stage-tip-title">${escapeHtml(s.period)}</span>
            <span>${formatCount(s.photos)} ${s.photos === 1 ? 'photo' : 'photos'}, the median</span>
            ${exposureSettingsHTML(s.focal, s.fnum, s.iso)}`;
    }
    const p = es.points;
    return photoTipHTML(es, p.lib[i], p.id[i], `
        <span class="point-stage-tip-title">${escapeHtml(p.date[i] ? p.date[i].slice(0, 10) : 'No date')}</span>
        <span>${escapeHtml(es.cameras[p.camera[i]])}</span>
        ${exposureSettingsHTML(p.focal[i], p.fnum[i], p.iso[i])}`);
}

// The floor of the box (ISO at its lowest) with a line at every focal
// length and aperture tick, the box's edges, and ISO ticks up one corner.
function exposureLines() {
    const lines = new StageLines();
    const floor = -1;
    for (const f of EXPOSURE_AXES.focal.ticks) {
        const x = exposureAxis('focal', f);
        lines.line([x, floor, -1], [x, floor, 1], 0.1);
    }
    for (const a of EXPOSURE_AXES.fnum.ticks) {
        const z = exposureAxis('fnum', a);
        lines.line([-1, floor, z], [1, floor, z], 0.1);
    }
    lines.box();
    for (const iso of EXPOSURE_AXES.iso.ticks) {
        const y = exposureAxis('iso', iso);
        lines.line([-1, y, -1], [-0.96, y, -1], 0.35).line([-1, y, -1], [-1, y, -0.96], 0.35);
    }
    return lines.toArray();
}

// Focal lengths along the front edge, apertures along the left one, ISO up
// the corner where they meet, and the first and last period of the trail.
function exposureLabels(es) {
    const labels = [];
    const { focal, fnum, iso } = EXPOSURE_AXES;
    for (const f of focal.ticks) labels.push({ text: focal.label(f), pos: [exposureAxis('focal', f), -1, 1.08], kind: 'tick' });
    for (const a of fnum.ticks) labels.push({ text: fnum.label(a), pos: [-1.1, -1, exposureAxis('fnum', a)], kind: 'tick' });
    for (const v of iso.ticks) labels.push({ text: iso.label(v), pos: [-1, exposureAxis('iso', v), -1], kind: 'axis' });
    labels.push({ text: 'Focal length (35 mm)', pos: [0, -1, 1.36], kind: 'name' });
    labels.push({ text: 'Aperture', pos: [-1.42, -1, 0], kind: 'name' });
    const path = es.path;
    if (path.length > 1) {
        for (const s of [path[0], path[path.length - 1]]) labels.push({ text: s.period, pos: exposurePosition(s.focal, s.fnum, s.iso), kind: 'period' });
    }
    return labels;
}
