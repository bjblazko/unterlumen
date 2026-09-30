// Statistics timeline charts — how cameras, focal lengths, ISO, apertures,
// aspect ratios and resolution changed over time. Uses the helpers in
// stats-charts.js.

/* ─── Timeline helpers ──────────────────────────────────────────── */

// The cameras of a timeline are categories like any other (ADR-0034).

function tlAxisBottom(g, x, periods, iH, c) {
    const tickMod = periods.length > 24 ? Math.ceil(periods.length / 12) : 1;
    g.append('g').attr('transform', `translate(0,${iH})`)
        .call(d3.axisBottom(x).tickSizeOuter(0)
            .tickValues(periods.filter((_, i) => i % tickMod === 0)))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 9)
        .attr('transform', 'rotate(-45)').attr('text-anchor', 'end')
        .attr('dx', '-0.5em').attr('dy', '0.5em');
    g.selectAll('.domain, .tick line').attr('stroke', c.border);
}

// The server lists only the periods that have photos. A line over time needs
// the ones between as well, or three empty years look as long as one month.
// Counts per period are index-aligned and get zeros; everything else is keyed
// by its period and stays as it is.
function continuousTimeline(tl) {
    const periods = tl?.periods;
    if (!periods?.length) return tl;
    const all = periodRange(periods[0], periods[periods.length - 1], tl.granularity);
    if (!all || all.length === periods.length) return tl;
    const at = new Map(periods.map((p, i) => [p, i]));
    const spread = counts => all.map(p => (at.has(p) ? counts[at.get(p)] ?? 0 : 0));
    return {
        ...tl,
        periods: all,
        cameraUsage: (tl.cameraUsage ?? []).map(cs => ({ ...cs, counts: spread(cs.counts) })),
        aspectRatios: (tl.aspectRatios ?? []).map(a => ({ ...a, counts: spread(a.counts) })),
    };
}

// Every period from first to last: "2004" … "2026", or "2024-11" … "2025-02".
function periodRange(first, last, granularity) {
    const out = [];
    if (granularity === 'year') {
        for (let y = +first; y <= +last; y++) out.push(String(y));
        return out;
    }
    const m = /^(\d{4})-(\d{2})$/;
    const a = m.exec(first), b = m.exec(last);
    if (!a || !b) return null;
    for (let y = +a[1], mo = +a[2]; y < +b[1] || (y === +b[1] && mo <= +b[2]); mo === 12 ? (y++, mo = 1) : mo++) {
        out.push(`${y}-${String(mo).padStart(2, '0')}`);
    }
    return out;
}

// A transparent band over each period that has data, so a click on a line
// chart without its own hover shows that period's photos.
function periodBands(g, x, periods, iH, onPick) {
    if (!onPick) return;
    pickable(g.append('g').selectAll('rect').data(periods).join('rect')
        .attr('x', p => x(p)).attr('y', 0)
        .attr('width', x.bandwidth()).attr('height', iH)
        .attr('fill', 'transparent'),
    p => onPick({ period: p }), p => `Photos of ${p}`);
}

// A value per period as a line (and optionally an area under it) that breaks
// where a period has no value: a line through years without photos would
// invent the values between. Each value gets a dot, so a period between two
// gaps still shows.
function gappedLine(g, periods, xC, yOf, { color, area = null, curve = d3.curveMonotoneX }, c) {
    const defined = p => { const v = yOf(p); return v !== null && Number.isFinite(v); };
    if (area !== null) {
        g.append('path').datum(periods)
            .attr('d', d3.area().defined(defined).x(xC).y0(area).y1(yOf).curve(curve))
            .attr('fill', color).attr('opacity', 0.18);
    }
    g.append('path').datum(periods)
        .attr('d', d3.line().defined(defined).x(xC).y(yOf).curve(curve))
        .attr('fill', 'none').attr('stroke', color).attr('stroke-width', 2);
    g.append('g').selectAll('circle').data(periods.filter(defined)).join('circle')
        .attr('cx', xC).attr('cy', yOf).attr('r', 3)
        .attr('fill', color).attr('stroke', c.surface).attr('stroke-width', 1.5);
}

function tlNoData(el, msg) {
    el.innerHTML = `<p class="stats-nodata">${escapeHtml(msg)}</p>`;
}

/* ─── Lines per series ──────────────────────────────────────────── */

// Several series over the same periods as lines, so no series hides another
// (a stack or a filled area did). A legend names every line; with four or
// fewer, each is also labelled at its end. Hovering shows every value of the
// period under the pointer.
//
// series: [{ name, values: [number|null per period], color }]
// format(value, series, periodIndex): the value as the tooltip says it.
// onPick({ period, series }): a click; series is the line nearest the pointer
// when it is close, else null.
function renderSeriesLines(el, periods, series, { yMax, yFormat, format, onPick, height = 200 }) {
    const c = chartColors();
    const direct = series.length <= 4;
    const W = 680, H = height;
    const m = { top: 12, right: direct ? 112 : 16, bottom: 48, left: 48 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;

    el.appendChild(seriesLegend(series));

    const x = d3.scalePoint().domain(periods).range([0, iW]).padding(0.5);
    const top = yMax ?? d3.max(series, s => d3.max(s.values)) ?? 0;
    const y = d3.scaleLinear().domain([0, top || 1]).range([iH, 0]).nice();

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    g.append('g').attr('class', 'stats-grid-lines')
        .call(d3.axisLeft(y).ticks(4).tickSize(-iW).tickFormat(yFormat))
        .call(axis => axis.select('.domain').remove())
        .call(axis => axis.selectAll('line').attr('stroke', c.border))
        .call(axis => axis.selectAll('text').attr('fill', c.textSec).attr('font-size', 9));
    tlAxisBottom(g, x, periods, iH, c);

    const line = d3.line()
        .defined(v => v !== null && v !== undefined)
        .x((_, i) => x(periods[i])).y(v => y(v));
    for (const s of series) {
        g.append('path').datum(s.values).attr('d', line)
            .attr('fill', 'none').attr('stroke', s.color).attr('stroke-width', 2)
            .attr('stroke-linejoin', 'round').attr('stroke-linecap', 'round');
    }
    if (direct) endLabels(g, series, periods, x, y, iW, c);

    seriesHover(el, g, { periods, series, x, y, iW, iH, m, W, format, onPick, c });
}

function seriesLegend(series) {
    const legend = document.createElement('div');
    legend.className = 'stats-legend';
    legend.innerHTML = series.map(s => `
        <span class="stats-legend-item"><span class="stats-legend-line" style="background:${s.color}"></span>${escapeHtml(s.name)}</span>`).join('');
    return legend;
}

// Each line's name beside its last value, moved apart where two would touch.
function endLabels(g, series, periods, x, y, iW, c) {
    const labels = series.map(s => {
        let i = s.values.length - 1;
        while (i >= 0 && (s.values[i] === null || s.values[i] === undefined)) i--;
        return i < 0 ? null : { name: s.name, y: y(s.values[i]) };
    }).filter(Boolean).sort((a, b) => a.y - b.y);
    for (let i = 1; i < labels.length; i++) {
        labels[i].y = Math.max(labels[i].y, labels[i - 1].y + 12);
    }
    for (const l of labels) {
        g.append('text').attr('x', iW + 8).attr('y', l.y + 3)
            .attr('fill', c.textSec).attr('font-size', 10)
            .text(truncate(l.name, 18));
    }
}

// A rule at the period under the pointer, a dot on every line there and a
// tooltip with the values, largest first. Arrow keys move it, Enter picks.
function seriesHover(el, g, { periods, series, x, y, iW, iH, m, W, format, onPick, c }) {
    const tooltip = d3.select(el).append('div').attr('class', 'stats-tooltip').style('display', 'none');
    const rule = g.append('line').attr('y1', 0).attr('y2', iH)
        .attr('stroke', c.axis).attr('stroke-width', 1).style('display', 'none');
    const dots = g.append('g').style('display', 'none');
    let current = -1;

    const show = (i) => {
        current = i;
        const px = x(periods[i]);
        rule.attr('x1', px).attr('x2', px).style('display', null);
        dots.style('display', null).selectAll('circle')
            .data(series.filter(s => s.values[i] !== null && s.values[i] !== undefined))
            .join('circle').attr('cx', px).attr('cy', s => y(s.values[i])).attr('r', 4)
            .attr('fill', s => s.color).attr('stroke', c.surface).attr('stroke-width', 2);
        const rows = series.filter(s => s.values[i]).sort((a, b) => b.values[i] - a.values[i])
            .map(s => `<span class="stats-legend-line" style="background:${s.color}"></span>${escapeHtml(s.name)} <span class="stats-tooltip-value">${escapeHtml(format(s.values[i], s, i))}</span>`);
        tooltip.style('display', 'block')
            .html(`<span class="stats-tooltip-value">${escapeHtml(periods[i])}</span><br>${rows.join('<br>') || 'No photos'}`);
        const drawn = g.node().ownerSVGElement.clientWidth;
        const left = (m.left + px) * drawn / W;
        tooltip.style('left', `${left + 12}px`).style('top', `${m.top * drawn / W}px`)
            .style('transform', left > drawn / 2 ? 'translateX(calc(-100% - 24px))' : null);
    };
    const hide = () => {
        current = -1;
        rule.style('display', 'none');
        dots.style('display', 'none');
        tooltip.style('display', 'none');
    };
    const nearestPeriod = (px) => {
        let best = 0;
        periods.forEach((p, i) => { if (Math.abs(x(p) - px) < Math.abs(x(periods[best]) - px)) best = i; });
        return best;
    };
    // The line nearest the pointer, when it is within 16 px of it.
    const nearestSeries = (i, py) => {
        let best = null, dist = 16;
        for (const s of series) {
            const v = s.values[i];
            if (v === null || v === undefined) continue;
            const d = Math.abs(y(v) - py);
            if (d < dist) { dist = d; best = s; }
        }
        return best;
    };

    const overlay = g.append('rect').attr('width', iW).attr('height', iH)
        .attr('fill', 'transparent').style('cursor', onPick ? 'pointer' : null)
        .on('mousemove', (event) => show(nearestPeriod(d3.pointer(event)[0])))
        .on('mouseleave', hide);
    if (!onPick) return;
    overlay.on('click', (event) => {
        const [px, py] = d3.pointer(event);
        const i = nearestPeriod(px);
        onPick({ period: periods[i], series: nearestSeries(i, py) });
    });
    const svg = d3.select(g.node().ownerSVGElement)
        .attr('tabindex', 0).attr('role', 'img')
        .attr('aria-label', 'Chart. Arrow keys move between periods, Enter shows the photos of one.');
    svg.on('keydown', (event) => {
        if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
            event.preventDefault();
            const step = event.key === 'ArrowRight' ? 1 : -1;
            show(Math.max(0, Math.min(periods.length - 1, (current < 0 ? (step > 0 ? -1 : periods.length) : current) + step)));
        } else if (event.key === 'Enter' && current >= 0) {
            event.preventDefault();
            onPick({ period: periods[current], series: null });
        }
    }).on('blur', hide);
}

/* ─── TL 1. Camera usage ────────────────────────────────────────── */

// The server sends the five cameras with the most photos, in that order, and
// the rest summed as "Other", which is drawn in the axis grey.
function renderCameraLines(el, tlData, onPick) {
    const cameras = tlData?.cameraUsage;
    const periods = tlData?.periods;
    if (!cameras?.length || !periods?.length) { tlNoData(el, 'No camera data'); return; }
    const c = chartColors();
    let slot = 0;
    const series = cameras.map(cs => ({
        name: cs.camera,
        camera: cs.camera === 'Other' ? null : cs.camera,
        values: periods.map((_, i) => cs.counts[i] ?? 0),
        color: cs.camera === 'Other' ? c.axis : c.cats[slot++],
    }));
    renderSeriesLines(el, periods, series, {
        yFormat: d => d.toLocaleString(),
        format: v => `${v.toLocaleString()} ${v === 1 ? 'photo' : 'photos'}`,
        onPick: onPick && (({ period, series: s }) => onPick({ period, camera: s?.camera ?? null })),
    });
}

/* ─── TL 2. Focal length drift ──────────────────────────────────── */

function renderFocalDrift(el, tlData, onPick) {
    const stats = tlData.focalStats;
    const periods = tlData.periods;
    if (!stats?.length || !periods?.length) { tlNoData(el, 'No focal length data'); return; }
    const c = chartColors();
    const W = 680, H = 180;
    const m = { top: 8, right: 16, bottom: 48, left: 52 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;

    const statsMap = new Map(stats.map(s => [s.period, s]));
    const validPeriods = periods.filter(p => statsMap.has(p));

    const x = d3.scaleBand().domain(periods).range([0, iW]).padding(0.1);
    const xC = p => x(p) + x.bandwidth() / 2;
    const allVals = stats.flatMap(s => [s.p25, s.p75]);
    const y = d3.scaleLinear().domain([0, d3.max(allVals) * 1.1]).range([iH, 0]).nice();

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    tlAxisBottom(g, x, periods, iH, c);
    g.append('g').call(d3.axisLeft(y).ticks(5).tickSizeOuter(0).tickFormat(d => `${d}mm`))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 9);
    g.selectAll('.domain, .tick line').attr('stroke', c.border);

    // The middle half of the photos as a band, broken where a period has none.
    g.append('path')
        .datum(periods)
        .attr('d', d3.area().defined(p => statsMap.has(p))
            .x(p => xC(p)).y0(p => y(statsMap.get(p).p25)).y1(p => y(statsMap.get(p).p75))
            .curve(d3.curveMonotoneX))
        .attr('fill', c.cats[0]).attr('opacity', 0.18);
    gappedLine(g, periods, xC, p => (statsMap.has(p) ? y(statsMap.get(p).median) : null), { color: c.cats[0] }, c);
    periodBands(g, x, validPeriods, iH, onPick);
}

/* ─── TL 3. ISO evolution ───────────────────────────────────────── */

function renderISOEvolution(el, tlData, onPick) {
    const stats = tlData.isoStats;
    const periods = tlData.periods;
    if (!stats?.length || !periods?.length) { tlNoData(el, 'No ISO data'); return; }
    const c = chartColors();
    const W = 680, H = 160;
    const m = { top: 8, right: 16, bottom: 48, left: 56 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;

    const statsMap = new Map(stats.map(s => [s.period, s]));
    const validPeriods = periods.filter(p => statsMap.has(p));
    // A log scale has no place for 0; below 50 it starts under the lowest
    // median (some phones report ISO 20), so no line runs below the axis.
    const medians = stats.map(s => s.median).filter(v => v > 0);
    const isoMin = Math.min(50, d3.min(medians) ?? 50) / 1.25;
    const isoMax = Math.max((d3.max(medians) ?? 0) * 1.5, 200);
    const isoTicks = [25, 100, 400, 1600, 6400, 25600].filter(v => v >= isoMin && v <= isoMax);

    const x = d3.scaleBand().domain(periods).range([0, iW]).padding(0.1);
    const xC = p => x(p) + x.bandwidth() / 2;
    const y = d3.scaleLog().domain([isoMin, isoMax]).range([iH, 0]).base(2).clamp(true);

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);

    // ISO grid lines
    isoTicks.forEach(iso => {
        g.append('line').attr('x1', 0).attr('x2', iW).attr('y1', y(iso)).attr('y2', y(iso))
            .attr('stroke', c.border).attr('stroke-dasharray', '3,3');
    });

    tlAxisBottom(g, x, periods, iH, c);
    g.append('g').call(d3.axisLeft(y)
        .tickValues(isoTicks)
        .tickSizeOuter(0).tickFormat(d => d >= 1000 ? `${d/1000}K` : d))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 9);
    g.selectAll('.domain, .tick line').attr('stroke', c.border);

    const median = p => (statsMap.get(p)?.median > 0 ? y(statsMap.get(p).median) : null);
    gappedLine(g, periods, xC, median, { color: c.cats[0], area: iH }, c);
    periodBands(g, x, validPeriods, iH, onPick);
}

/* ─── TL 4. Aperture heatmap ────────────────────────────────────── */

function renderApertureHeat(el, tlData, onPick) {
    const heat = tlData.apertureHeat;
    const periods = tlData.periods;
    if (!heat?.length || !periods?.length) { tlNoData(el, 'No aperture data'); return; }
    const c = chartColors();
    const bucketOrder = ['f/1', 'f/1.4', 'f/2', 'f/2.8', 'f/4', 'f/5.6', 'f/8', 'f/11', 'f/16+'];
    const cellW = Math.max(4, Math.min(22, Math.floor(600 / periods.length)));
    const cellH = 17;
    const m = { top: 4, right: 16, bottom: 48, left: 44 };
    const iW = periods.length * cellW, iH = bucketOrder.length * cellH;
    const W = m.left + iW + m.right, H = m.top + iH + m.bottom;

    const rowMap = new Map(heat.map(r => [r.period, r.buckets]));
    const totals = new Map(heat.map(r => [r.period, Object.values(r.buckets).reduce((a, b) => a + b, 0)]));
    const cellOpacity = norm => norm > 0 ? (0.15 + 0.85 * Math.sqrt(norm)) : 0;

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);

    bucketOrder.forEach((b, bi) => {
        g.append('text').attr('x', -4).attr('y', bi * cellH + cellH * 0.72)
            .attr('text-anchor', 'end').attr('fill', c.textSec).attr('font-size', 9).text(b);
    });

    const tickMod = periods.length > 30 ? Math.ceil(periods.length / 15) : 1;
    periods.forEach((p, pi) => {
        if (pi % tickMod !== 0) return;
        g.append('text')
            .attr('transform', `translate(${pi * cellW + cellW / 2},${iH + 6}) rotate(-45)`)
            .attr('text-anchor', 'end').attr('fill', c.textSec).attr('font-size', 9).text(p);
    });

    const tooltip = d3.select(el).append('div').attr('class', 'stats-tooltip').style('display', 'none');

    periods.forEach((p, pi) => {
        const buckets = rowMap.get(p) ?? {};
        const total = totals.get(p) ?? 1;
        bucketOrder.forEach((b, bi) => {
            const count = buckets[b] ?? 0;
            const norm = count / total;
            const cell = g.append('rect')
                .attr('x', pi * cellW).attr('y', bi * cellH)
                .attr('width', cellW - 1).attr('height', cellH - 1).attr('rx', 1)
                .attr('fill', count > 0 ? seqStep(c, norm, 1) : c.border)
                .attr('opacity', count > 0 ? 1 : 0.3)
                .on('mouseover', function(event) {
                    if (!count) return;
                    tooltip.style('display', 'block')
                        .html(`${p} · ${b}<br>${count.toLocaleString()} shots (${(norm*100).toFixed(1)}%)`);
                })
                .on('mousemove', function(event) {
                    const [mx, my] = d3.pointer(event, el);
                    tooltip.style('left', mx + 12 + 'px').style('top', my - 28 + 'px');
                })
                .on('mouseout', () => tooltip.style('display', 'none'));
            if (count) pickable(cell, onPick && (() => onPick({ period: p, bucket: b })), `${p}, ${b}, ${count.toLocaleString()} photos`);
        });
    });
}

/* ─── TL 5. Aspect ratio ────────────────────────────────────────── */

// Aspect ratios are a fixed set, so each keeps its slot in the ramp whatever
// the library holds (ADR-0034).
const TL_ASPECT_SLOTS = { '3:2': 0, '4:3': 1, '16:9+': 2, '1:1': 3, 'other': 4 };
const aspectColor = (c, ratio) => c.cats[TL_ASPECT_SLOTS[ratio] ?? 4];

// The share of each frame shape per period, so a period with few photos
// counts as much as one with many. A period without photos is a gap.
function renderAspectLines(el, tlData, onPick) {
    const aspects = tlData?.aspectRatios;
    const periods = tlData?.periods;
    if (!aspects?.length || !periods?.length) { tlNoData(el, 'No aspect ratio data'); return; }
    const c = chartColors();
    const totals = periods.map((_, i) => d3.sum(aspects, a => a.counts[i] ?? 0));
    const ordered = [...aspects].sort((a, b) => (TL_ASPECT_SLOTS[a.ratio] ?? 4) - (TL_ASPECT_SLOTS[b.ratio] ?? 4));
    const series = ordered.map(a => ({
        name: a.ratio,
        counts: a.counts,
        values: periods.map((_, i) => totals[i] ? 100 * (a.counts[i] ?? 0) / totals[i] : null),
        color: aspectColor(c, a.ratio),
    }));
    renderSeriesLines(el, periods, series, {
        yMax: 100,
        yFormat: d => `${d} %`,
        format: (v, s, i) => `${Math.round(v)} % (${(s.counts[i] ?? 0).toLocaleString()})`,
        onPick: onPick && (({ period, series: s }) => onPick({ period, aspect: s?.name ?? null })),
        height: 180,
    });
}

/* ─── TL 6. Megapixel timeline ──────────────────────────────────── */

function renderMegapixelTimeline(el, tlData, onPick) {
    const stats = tlData.megapixelStats;
    const periods = tlData.periods;
    if (!stats?.length || !periods?.length) { tlNoData(el, 'No megapixel data'); return; }
    const c = chartColors();
    const W = 680, H = 160;
    const m = { top: 16, right: 16, bottom: 48, left: 52 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;

    const byPeriod = new Map(stats.map(s => [s.period, s]));
    const validPeriods = periods.filter(p => byPeriod.has(p));

    const x = d3.scaleBand().domain(periods).range([0, iW]).padding(0.1);
    const xC = p => x(p) + x.bandwidth() / 2;
    const maxMP = d3.max(stats, s => s.max);
    const y = d3.scaleLinear().domain([0, maxMP * 1.15]).range([iH, 0]).nice();

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    tlAxisBottom(g, x, periods, iH, c);
    g.append('g').call(d3.axisLeft(y).ticks(4).tickSizeOuter(0).tickFormat(d => `${d.toFixed(0)}MP`))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 9);
    g.selectAll('.domain, .tick line').attr('stroke', c.border);

    // Avg line (dashed)
    g.append('path')
        .datum(periods)
        .attr('d', d3.line().defined(p => byPeriod.has(p)).x(p => xC(p)).y(p => y(byPeriod.get(p).avg)).curve(d3.curveMonotoneX))
        .attr('fill', 'none').attr('stroke', c.textSec).attr('stroke-width', 1.5).attr('stroke-dasharray', '4,3');

    // Max line (step)
    g.append('path')
        .datum(periods)
        .attr('d', d3.line().defined(p => byPeriod.has(p)).x(p => xC(p)).y(p => y(byPeriod.get(p).max)).curve(d3.curveStepAfter))
        .attr('fill', 'none').attr('stroke', c.cats[0]).attr('stroke-width', 2);

    periodBands(g, x, validPeriods, iH, onPick);

    // Mark significant max jumps (>20%)
    for (let i = 1; i < validPeriods.length; i++) {
        const prev = byPeriod.get(validPeriods[i - 1]);
        const curr = byPeriod.get(validPeriods[i]);
        if (curr.max > prev.max * 1.2) {
            g.append('circle').attr('cx', xC(validPeriods[i])).attr('cy', y(curr.max))
                .attr('r', 4).attr('fill', c.cats[0]).attr('stroke', c.surface).attr('stroke-width', 2);
        }
    }

    el.insertBefore(seriesLegend([
        { name: 'Largest', color: c.cats[0] },
        { name: 'Average', color: c.textSec },
    ]), el.firstChild);
}
