// character-view.js — the Character topic (ADR-0046): every analysed photo
// by how bright (across), how contrasty (in depth) and how colourful (up)
// it is, in its main colour; a trail through each period's mean. The moods
// of the pictures show as clusters: dark and moody, bright and airy, flat
// and foggy, punchy and vivid. Drawn on a point stage (point-stage.js) from
// the Colour space's points, which carry these measurements (ADR-0044).

const CHARACTER_AXES = {
    lum: { max: 1, ticks: [0, 0.25, 0.5, 0.75, 1], ends: ['Dark', 'Bright'] },
    contrast: { max: 0.4, ticks: [0, 0.1, 0.2, 0.3, 0.4], ends: ['Flat', 'Contrasty'] },
    colourful: { max: 120, ticks: [0, 30, 60, 90, 120], ends: ['Muted', 'Colourful'] },
};

// A measurement on its axis, -1…1; anything beyond sits on the edge.
function characterAxis(axis, v) {
    return Math.max(-1, Math.min(1, v / CHARACTER_AXES[axis].max * 2 - 1));
}

function characterPosition(lum, contrast, colourful) {
    return [characterAxis('lum', lum), characterAxis('colourful', colourful), characterAxis('contrast', contrast)];
}

// renderCharacter draws the Colour space's points (cs) by brightness,
// contrast and colourfulness. onPick gets { photo } or { step }.
function renderCharacter(el, cs, onPick, options) {
    if (noColourData(el, cs)) return;
    renderPointStage(el, () => characterSpec(cs), onPick, options);
}

function characterSpec(cs) {
    const p = cs.points;
    const n = p.id.length;
    const light = pointLight(n);
    const sprites = new Float32Array(n * SPRITE_FLOATS);
    for (let i = 0; i < n; i++) {
        sprites.set([...characterPosition(p.lum[i], p.contrast[i], p.colourful[i]), ...oklabToStageRGB(p.l[i], p.a[i], p.b[i]), light.radius, light.alpha], i * SPRITE_FLOATS);
    }
    const periods = characterPeriods(cs);
    const most = Math.max(1, ...periods.map(s => s.photos));
    const steps = new Float32Array(periods.flatMap(s => [
        ...characterPosition(s.lum, s.contrast, s.colourful), ...oklabToStageRGB(s.l, s.a, s.b, { boost: TRAIL_BOOST }), stepRadius(s.photos, most), 1,
    ]));
    const unit = periodUnit(cs.granularity);
    return {
        summary: `${formatCount(n)} photos by how bright they are (across), how contrasty (in depth) and how colourful (up), each in its main colour.${periods.length > 1 ? ` The trail joins each ${unit}'s average${pathSpan(periods)}.` : ''}`,
        legend: legendDotHTML('One photo, in its main colour')
            + legendStepHTML(`One ${unit}: the average brightness, contrast and colourfulness of its photos, larger with more photos. The line joins the ${unit}s in time order${pathSpan(periods)}.`),
        sprites, steps,
        distance: 5.6,
        trail: trailBetween(steps),
        lines: characterLines(),
        labels: characterLabels(periods),
        tip: (kind, i) => characterTip(cs, periods, kind, i),
        pick: (kind, i) => kind === 'step'
            ? { step: periods[i] }
            : { photo: { id: p.id[i], lib: cs.libraries[p.lib[i]], date: p.date[i] } },
    };
}

// Each period of the Colour space's granularity that has dated photos: the
// mean of each measurement and of the colour.
function characterPeriods(cs) {
    const p = cs.points;
    const n = cs.granularity === 'year' ? 4 : 7;
    const sums = new Map();
    for (let i = 0; i < p.id.length; i++) {
        if (!p.date[i] || p.date[i].length < 7) continue;
        const period = p.date[i].slice(0, n);
        const s = sums.get(period) ?? { period, photos: 0, lum: 0, contrast: 0, colourful: 0, l: 0, a: 0, b: 0 };
        s.photos++;
        s.lum += p.lum[i];
        s.contrast += p.contrast[i];
        s.colourful += p.colourful[i];
        s.l += p.l[i];
        s.a += p.a[i];
        s.b += p.b[i];
        sums.set(period, s);
    }
    return [...sums.values()].sort((x, y) => (x.period < y.period ? -1 : 1)).map(s => {
        const k = s.photos;
        return { period: s.period, photos: k, lum: s.lum / k, contrast: s.contrast / k, colourful: s.colourful / k, l: s.l / k, a: s.a / k, b: s.b / k };
    });
}

function characterValuesHTML(lum, contrast, colourful) {
    return `<span class="point-stage-tip-values">brightness ${lum.toFixed(2)} · contrast ${contrast.toFixed(2)} · colourfulness ${Math.round(colourful)}</span>`;
}

function characterTip(cs, periods, kind, i) {
    if (kind === 'step') {
        const s = periods[i];
        return `<span class="point-stage-tip-title">${escapeHtml(s.period)}</span>
            <span>${formatCount(s.photos)} ${s.photos === 1 ? 'photo' : 'photos'}, on average</span>
            ${characterValuesHTML(s.lum, s.contrast, s.colourful)}`;
    }
    const p = cs.points;
    return photoTipHTML(cs, p.lib[i], p.id[i], `
        <span class="point-stage-tip-title">${escapeHtml(p.date[i] ? p.date[i].slice(0, 10) : 'No date')}</span>
        ${characterValuesHTML(p.lum[i], p.contrast[i], p.colourful[i])}`);
}

// The box, with a line across its floor at every brightness and contrast
// tick, and colourfulness ticks up the corner where the axes meet.
function characterLines() {
    const lines = new StageLines().box();
    for (const v of CHARACTER_AXES.lum.ticks) {
        const x = characterAxis('lum', v);
        lines.line([x, -1, -1], [x, -1, 1], 0.1);
    }
    for (const v of CHARACTER_AXES.contrast.ticks) {
        const z = characterAxis('contrast', v);
        lines.line([-1, -1, z], [1, -1, z], 0.1);
    }
    for (const v of CHARACTER_AXES.colourful.ticks) {
        const y = characterAxis('colourful', v);
        lines.line([-1, y, -1], [-0.96, y, -1], 0.35).line([-1, y, -1], [-1, y, -0.96], 0.35);
    }
    return lines.toArray();
}

// Each axis's ends in words (colourfulness only at the top: its foot is
// where the other axes meet), its ticks in numbers, and the first and last
// period of the trail.
function characterLabels(periods) {
    const { lum, contrast, colourful } = CHARACTER_AXES;
    const labels = [];
    for (const v of lum.ticks) labels.push({ text: String(v), pos: [characterAxis('lum', v), -1, 1.08], kind: 'tick' });
    for (const v of contrast.ticks) labels.push({ text: String(v), pos: [-1.1, -1, characterAxis('contrast', v)], kind: 'tick' });
    for (const v of colourful.ticks) labels.push({ text: String(v), pos: [-1, characterAxis('colourful', v), -1], kind: 'axis' });
    labels.push({ text: lum.ends[0], pos: [-1, -1, 1.3], kind: 'name' }, { text: lum.ends[1], pos: [1, -1, 1.3], kind: 'name' });
    labels.push({ text: contrast.ends[0], pos: [-1.35, -1, -1], kind: 'name' }, { text: contrast.ends[1], pos: [-1.35, -1, 1], kind: 'name' });
    labels.push({ text: colourful.ends[1], pos: [-1.12, 1.1, -1.12], kind: 'name' });
    if (periods.length > 1) {
        for (const s of [periods[0], periods[periods.length - 1]]) labels.push({ text: s.period, pos: characterPosition(s.lum, s.contrast, s.colourful), kind: 'period' });
    }
    return labels;
}
