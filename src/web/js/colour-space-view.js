// colour-space-view.js — the Colour space topic (ADR-0046): every analysed
// photo as a point of light at its main colour in OKLab — lightness up, hue
// around, chroma outward — and a trail through the mean colour of each
// period's colour photos. Drawn on a point stage (point-stage.js).

// The trail is drawn more colourful than it is: a mean of many colours is
// greyer than any of them, and the lights around it show the real ones.
const TRAIL_BOOST = 2.5;

// Photos rarely reach a chroma of 0.2, so a and b are stretched against L.
function colourSpacePosition(l, a, b) {
    return [a * 5, (l - 0.5) * 2, b * 5];
}

// renderColourSpace draws cs (/api/library/colour-space). onPick gets
// { photo: { id, lib, date } } or { step } for a period of the trail.
function renderColourSpace(el, cs, onPick, options) {
    if (noColourData(el, cs)) return;
    renderPointStage(el, () => colourSpaceSpec(cs), onPick, options);
}

function colourSpaceSpec(cs) {
    const p = cs.points;
    const n = p.id.length;
    const light = pointLight(n);
    const sprites = new Float32Array(n * SPRITE_FLOATS);
    for (let i = 0; i < n; i++) {
        sprites.set([...colourSpacePosition(p.l[i], p.a[i], p.b[i]), ...oklabToStageRGB(p.l[i], p.a[i], p.b[i]), light.radius, light.alpha], i * SPRITE_FLOATS);
    }
    const most = Math.max(1, ...cs.path.map(s => s.photos));
    const steps = new Float32Array(cs.path.flatMap(s => [
        ...colourSpacePosition(s.l, s.a, s.b), ...oklabToStageRGB(s.l, s.a, s.b, { boost: TRAIL_BOOST }), stepRadius(s.photos, most), 1,
    ]));
    return {
        summary: colourSpaceSummary(cs),
        legend: colourSpaceLegendHTML(cs),
        sprites, steps,
        trail: trailBetween(steps),
        lines: colourSpaceLines(),
        labels: colourSpaceLabels(cs),
        tip: (kind, i) => colourSpaceTip(cs, kind, i),
        pick: (kind, i) => kind === 'step'
            ? { step: cs.path[i] }
            : { photo: { id: p.id[i], lib: cs.libraries[p.lib[i]], date: p.date[i] } },
    };
}

function periodUnit(granularity) {
    return granularity === 'year' ? 'year' : 'month';
}

function pathSpan(path) {
    return path.length > 1 ? `, from ${path[0].period} to ${path[path.length - 1].period}` : '';
}

function colourSpaceSummary(cs) {
    const n = cs.analysedPhotos;
    const trail = cs.path.length > 1
        ? ` The trail joins the mean colour of each ${periodUnit(cs.granularity)}'s colour photos${pathSpan(cs.path)}.`
        : '';
    return `${formatCount(n)} ${n === 1 ? 'photo' : 'photos'}, each at its main colour: lightness up, hue around, chroma outward.${trail}`;
}

function colourSpaceLegendHTML(cs) {
    const unit = periodUnit(cs.granularity);
    return legendDotHTML('One photo, at its main colour')
        + legendStepHTML(`One ${unit}: the average colour of its colour photos, larger with more photos. The line joins the ${unit}s in time order${pathSpan(cs.path)}.`);
}

// The floor at L 0: chroma rings at 0.05, 0.1, 0.15 and 0.2, a spoke at
// every 30° sector edge, and the lightness axis standing in the middle.
function colourSpaceLines() {
    const lines = new StageLines();
    const at = (l, c, deg) => colourSpacePosition(l, c * Math.cos(deg * Math.PI / 180), c * Math.sin(deg * Math.PI / 180));
    for (const c of [0.05, 0.1, 0.15, 0.2]) lines.ring(c * 5, -1, 0.22);
    for (let bin = 0; bin < 12; bin++) lines.line(at(0, 0.015, bin * 30), at(0, 0.22, bin * 30), 0.12);
    lines.line(at(0, 0, 0), at(1, 0, 0), 0.35);
    for (const l of [0.25, 0.5, 0.75, 1]) lines.line(colourSpacePosition(l, -0.008, 0), colourSpacePosition(l, 0.008, 0), 0.35);
    return lines.toArray();
}

// Hue names around the floor, lightness up the axis, and the first and last
// period of the trail.
function colourSpaceLabels(cs) {
    const labels = [];
    for (let bin = 0; bin < 12; bin++) {
        const t = (bin * 30 + 15) * Math.PI / 180;
        labels.push({ text: capitalise(hueName(bin)), pos: colourSpacePosition(0, 0.245 * Math.cos(t), 0.245 * Math.sin(t)), kind: 'hue' });
    }
    for (const l of [0, 0.5, 1]) labels.push({ text: `L ${l}`, pos: colourSpacePosition(l, 0, 0), kind: 'axis' });
    const path = cs.path;
    if (path.length > 1) {
        for (const s of [path[0], path[path.length - 1]]) labels.push({ text: s.period, pos: colourSpacePosition(s.l, s.a, s.b), kind: 'period' });
    }
    return labels;
}

function labToLCh(l, a, b) {
    const c = Math.hypot(a, b);
    const h = (Math.atan2(b, a) * 180 / Math.PI + 360) % 360;
    return { l, c, h };
}

function colourValuesHTML({ l, c, h }) {
    return `<span class="point-stage-tip-values"><span class="point-stage-swatch" style="background:${photoColour({ l, c, h })}"></span>L ${l.toFixed(2)} · C ${c.toFixed(3)} · h ${Math.round(h)}°</span>`;
}

function colourSpaceTip(cs, kind, i) {
    if (kind === 'step') {
        const s = cs.path[i];
        return `<span class="point-stage-tip-title">${escapeHtml(s.period)}</span>
            <span>${formatCount(s.photos)} colour ${s.photos === 1 ? 'photo' : 'photos'}, on average</span>
            ${colourValuesHTML(labToLCh(s.l, s.a, s.b))}`;
    }
    const p = cs.points;
    const lch = labToLCh(p.l[i], p.a[i], p.b[i]);
    return photoTipHTML(cs, p.lib[i], p.id[i], `
        <span class="point-stage-tip-title">${escapeHtml(p.date[i] ? p.date[i].slice(0, 10) : 'No date')}</span>
        <span>${escapeHtml(COLOUR_CLASSES[p.mono[i]] ?? '')}, mostly ${escapeHtml(hueNameOf(lch))}</span>
        ${colourValuesHTML(lch)}`);
}
