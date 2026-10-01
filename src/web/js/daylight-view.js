// daylight-view.js — the Daylight topic (ADR-0046): every photo by when it
// was taken and how bright it is. The day of the year runs around a ring,
// the hour from midnight in the middle outward, and the photo's brightness
// up; each light in the photo's main colour. A trail goes round the twelve
// months, all years together. Drawn on a point stage (point-stage.js) from
// the Colour space's points, which carry date and brightness.

const DAYLIGHT_FLOOR = -0.8;

// The day of the year 0…1 (January 1st at 0) and the hour 0…24 of a date
// taken as stored ("2024-07-01T14:32:00"), or null without a time.
function daylightWhen(date) {
    if (!date || date.length < 16) return null;
    const y = +date.slice(0, 4), m = +date.slice(5, 7), d = +date.slice(8, 10);
    const hour = +date.slice(11, 13) + +date.slice(14, 16) / 60;
    if (!y || !m || !d || Number.isNaN(hour)) return null;
    const day = (Date.UTC(y, m - 1, d) - Date.UTC(y, 0, 1)) / 86400000;
    return { year: day / 365.25, hour };
}

function daylightPosition(yearFraction, hour, lum) {
    const angle = yearFraction * 2 * Math.PI - Math.PI / 2;
    const r = 0.18 + 0.82 * hour / 24;
    return [r * Math.cos(angle), DAYLIGHT_FLOOR + lum * 1.6, r * Math.sin(angle)];
}

// renderDaylight draws the Colour space's points (cs) by date and
// brightness. onPick gets { photo } or { month } (1…12).
function renderDaylight(el, cs, onPick, options) {
    if (noColourData(el, cs)) return;
    const timed = daylightPhotos(cs);
    if (!timed.length) {
        tlNoData(el, 'No analysed photo here has a time of day.');
        return;
    }
    renderPointStage(el, () => daylightSpec(cs, timed), onPick, options);
}

// The analysed photos that have a time, with when they were taken.
function daylightPhotos(cs) {
    const p = cs.points;
    const out = [];
    for (let i = 0; i < p.id.length; i++) {
        const when = daylightWhen(p.date[i]);
        if (when) out.push({ i, ...when });
    }
    return out;
}

function daylightSpec(cs, timed) {
    const p = cs.points;
    const light = pointLight(timed.length);
    const sprites = new Float32Array(timed.length * SPRITE_FLOATS);
    timed.forEach(({ i, year, hour }, k) => {
        sprites.set([...daylightPosition(year, hour, p.lum[i]), ...oklabToStageRGB(p.l[i], p.a[i], p.b[i]), light.radius, light.alpha], k * SPRITE_FLOATS);
    });
    const months = daylightMonths(cs, timed);
    const most = Math.max(1, ...months.map(m => m.photos));
    const steps = new Float32Array(months.flatMap(m => [
        ...daylightPosition((m.month - 0.5) / 12, m.hour, m.lum), ...oklabToStageRGB(m.l, m.a, m.b, { boost: TRAIL_BOOST }), stepRadius(m.photos, most), 1,
    ]));
    return {
        summary: `${formatCount(timed.length)} photos by when they were taken: the day of the year around the ring, the hour from midnight in the middle outward, and how bright the photo is upward. Each is in its main colour. The trail goes through the twelve months, all years together.`,
        legend: legendDotHTML('One photo, in its main colour')
            + legendStepHTML('One month of the year, all years together: the average hour, brightness and colour of its photos, larger with more photos. The line goes round the year.'),
        sprites, steps,
        distance: 4.4,
        trail: trailBetween(steps, { closed: true }),
        lines: daylightLines(),
        labels: daylightLabels(),
        tip: (kind, k) => daylightTip(cs, timed, months, kind, k),
        pick: (kind, k) => kind === 'step'
            ? { month: months[k].month }
            : { photo: { id: p.id[timed[k].i], lib: cs.libraries[p.lib[timed[k].i]], date: p.date[timed[k].i] } },
    };
}

// Each month of the year that has photos: the mean hour, brightness and
// OKLab colour of its photos.
function daylightMonths(cs, timed) {
    const p = cs.points;
    const sums = new Map();
    for (const { i, hour } of timed) {
        const month = +p.date[i].slice(5, 7);
        const s = sums.get(month) ?? { month, photos: 0, hour: 0, lum: 0, l: 0, a: 0, b: 0 };
        s.photos++;
        s.hour += hour;
        s.lum += p.lum[i];
        s.l += p.l[i];
        s.a += p.a[i];
        s.b += p.b[i];
        sums.set(month, s);
    }
    return [...sums.values()].sort((x, y) => x.month - y.month).map(s => {
        const k = s.photos;
        return { month: s.month, photos: k, hour: s.hour / k, lum: s.lum / k, l: s.l / k, a: s.a / k, b: s.b / k };
    });
}

function formatHour(hour) {
    const h = Math.floor(hour), m = Math.round((hour - h) * 60);
    return `${String(h).padStart(2, '0')}:${String(m === 60 ? 59 : m).padStart(2, '0')}`;
}

function daylightTip(cs, timed, months, kind, k) {
    if (kind === 'step') {
        const m = months[k];
        return `<span class="point-stage-tip-title">${escapeHtml(MONTHS[m.month - 1])}, all years</span>
            <span>${formatCount(m.photos)} ${m.photos === 1 ? 'photo' : 'photos'}, on average</span>
            <span class="point-stage-tip-values">${formatHour(m.hour)} · brightness ${m.lum.toFixed(2)}</span>`;
    }
    const p = cs.points;
    const { i, hour } = timed[k];
    return photoTipHTML(cs, p.lib[i], p.id[i], `
        <span class="point-stage-tip-title">${escapeHtml(p.date[i].slice(0, 10))}</span>
        <span class="point-stage-tip-values">${formatHour(hour)} · brightness ${p.lum[i].toFixed(2)}</span>
        ${colourValuesHTML(labToLCh(p.l[i], p.a[i], p.b[i]))}`);
}

// The floor: rings at midnight, 6:00, noon, 18:00 and midnight again, a
// spoke at the start of every month, and a scale of brightness standing
// where the year begins.
function daylightLines() {
    const lines = new StageLines();
    for (const h of [0, 6, 12, 18, 24]) lines.ring(0.18 + 0.82 * h / 24, DAYLIGHT_FLOOR, h % 12 === 0 ? 0.24 : 0.14);
    for (let m = 0; m < 12; m++) lines.line(daylightPosition(m / 12, 0, 0), daylightPosition(m / 12, 24, 0), 0.1);
    const base = daylightPosition(0, 24, 0);
    lines.line(base, daylightPosition(0, 24, 1), 0.35);
    for (const lum of [0.25, 0.5, 0.75, 1]) {
        const [x, y, z] = daylightPosition(0, 24, lum);
        lines.line([x - 0.02, y, z], [x + 0.02, y, z], 0.35);
    }
    return lines.toArray();
}

function daylightLabels() {
    const labels = MONTHS.map((name, m) => ({ text: name.slice(0, 3), pos: daylightPosition((m + 0.5) / 12, 26, 0), kind: 'hue' }));
    for (const h of [0, 6, 12, 18]) labels.push({ text: `${String(h).padStart(2, '0')}:00`, pos: daylightPosition(0.5, h, 0), kind: 'axis' });
    labels.push({ text: 'Dark', pos: daylightPosition(0, 24, 0.02), kind: 'axis' });
    labels.push({ text: 'Bright', pos: daylightPosition(0, 24, 1), kind: 'axis' });
    return labels;
}
