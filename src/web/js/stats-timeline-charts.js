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

function tlNoData(el, msg) {
    el.innerHTML = `<div class="stats-tl-nodata">${escapeHtml(msg)}</div>`;
}

/* ─── TL 1. Camera stacked bar ──────────────────────────────────── */

function renderCameraStream(el, tlData) {
    const cameras = tlData.cameraUsage;
    const periods = tlData.periods;
    if (!cameras?.length || !periods?.length) { tlNoData(el, 'No camera data'); return; }
    const c = chartColors();
    const W = 680, H = 220;
    const m = { top: 8, right: 16, bottom: 48, left: 16 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;

    const cameraNames = cameras.map(cs => cs.camera);
    const stackData = periods.map((p, i) => {
        const obj = { period: p };
        for (const cs of cameras) obj[cs.camera] = cs.counts[i] ?? 0;
        return obj;
    });

    const stack = d3.stack().keys(cameraNames)(stackData);
    const colorScale = d3.scaleOrdinal().domain(cameraNames).range(c.cats);

    const x = d3.scaleBand().domain(periods).range([0, iW]).padding(0.08);
    const maxY = d3.max(stack[stack.length - 1], d => d[1]);
    const y = d3.scaleLinear().domain([0, maxY]).range([iH, 0]).nice();

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    tlAxisBottom(g, x, periods, iH, c);

    const layers = g.selectAll('.cam-layer').data(stack).join('g')
        .attr('class', 'cam-layer')
        .attr('fill', d => colorScale(d.key));

    layers.selectAll('rect').data(d => d).join('rect')
        .attr('x', d => x(d.data.period))
        .attr('y', d => y(d[1]))
        .attr('height', d => Math.max(0, y(d[0]) - y(d[1])))
        .attr('width', x.bandwidth());

    const legend = document.createElement('div');
    legend.className = 'stats-tl-legend';
    cameraNames.forEach(cam => {
        const item = document.createElement('div');
        item.className = 'stats-tl-legend-item';
        item.innerHTML = `<span class="stats-tl-legend-swatch" style="background:${colorScale(cam)}"></span>${escapeHtml(cam)}`;
        item.addEventListener('click', () => {
            item.classList.toggle('tl-dim');
            const dimmed = item.classList.contains('tl-dim');
            layers.filter(d => d.key === cam).attr('opacity', dimmed ? 0.12 : 1);
        });
        legend.appendChild(item);
    });
    el.appendChild(legend);
}

/* ─── TL 2. Focal length drift ──────────────────────────────────── */

function renderFocalDrift(el, tlData) {
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

    // IQR band
    g.append('path')
        .datum(validPeriods)
        .attr('d', d3.area()
            .x(p => xC(p)).y0(p => y(statsMap.get(p).p25)).y1(p => y(statsMap.get(p).p75))
            .curve(d3.curveMonotoneX))
        .attr('fill', c.cats[0]).attr('opacity', 0.18);

    // Median line
    g.append('path')
        .datum(validPeriods)
        .attr('d', d3.line().x(p => xC(p)).y(p => y(statsMap.get(p).median)).curve(d3.curveMonotoneX))
        .attr('fill', 'none').attr('stroke', c.cats[0]).attr('stroke-width', 2);
}

/* ─── TL 3. ISO evolution ───────────────────────────────────────── */

function renderISOEvolution(el, tlData) {
    const stats = tlData.isoStats;
    const periods = tlData.periods;
    if (!stats?.length || !periods?.length) { tlNoData(el, 'No ISO data'); return; }
    const c = chartColors();
    const W = 680, H = 160;
    const m = { top: 8, right: 16, bottom: 48, left: 56 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;

    const statsMap = new Map(stats.map(s => [s.period, s]));
    const validPeriods = periods.filter(p => statsMap.has(p));
    const allMedians = stats.map(s => s.median).filter(v => v >= 50);
    const isoMax = Math.max(d3.max(allMedians) * 1.5, 200);

    const x = d3.scaleBand().domain(periods).range([0, iW]).padding(0.1);
    const xC = p => x(p) + x.bandwidth() / 2;
    const y = d3.scaleLog().domain([50, isoMax]).range([iH, 0]).base(2);

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);

    // ISO grid lines
    [100, 400, 1600, 6400, 25600].filter(v => v <= isoMax).forEach(iso => {
        g.append('line').attr('x1', 0).attr('x2', iW).attr('y1', y(iso)).attr('y2', y(iso))
            .attr('stroke', c.border).attr('stroke-dasharray', '3,3');
    });

    tlAxisBottom(g, x, periods, iH, c);
    g.append('g').call(d3.axisLeft(y)
        .tickValues([100, 400, 1600, 6400, 25600].filter(v => v <= isoMax))
        .tickSizeOuter(0).tickFormat(d => d >= 1000 ? `${d/1000}K` : d))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 9);
    g.selectAll('.domain, .tick line').attr('stroke', c.border);

    g.append('path')
        .datum(validPeriods)
        .attr('d', d3.area().x(p => xC(p)).y0(iH).y1(p => y(statsMap.get(p).median)).curve(d3.curveMonotoneX))
        .attr('fill', c.cats[0]).attr('opacity', 0.18);

    g.append('path')
        .datum(validPeriods)
        .attr('d', d3.line().x(p => xC(p)).y(p => y(statsMap.get(p).median)).curve(d3.curveMonotoneX))
        .attr('fill', 'none').attr('stroke', c.cats[0]).attr('stroke-width', 2);
}

/* ─── TL 4. Aperture heatmap ────────────────────────────────────── */

function renderApertureHeat(el, tlData) {
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
            g.append('rect')
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
        });
    });
}

/* ─── TL 5. Aspect ratio river ──────────────────────────────────── */

// Aspect ratios are a fixed set, so each keeps its slot in the ramp whatever
// the library holds (ADR-0034).
const TL_ASPECT_SLOTS = { '3:2': 0, '4:3': 1, '16:9+': 2, '1:1': 3, 'other': 4 };
const aspectColor = (c, ratio) => c.cats[TL_ASPECT_SLOTS[ratio] ?? 4];

function renderAspectRiver(el, tlData) {
    const aspects = tlData.aspectRatios;
    const periods = tlData.periods;
    if (!aspects?.length || !periods?.length) { tlNoData(el, 'No aspect ratio data'); return; }
    const c = chartColors();
    const W = 680, H = 160;
    const m = { top: 8, right: 16, bottom: 48, left: 44 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;

    const ratioNames = aspects.map(a => a.ratio);
    const stackData = periods.map((p, i) => {
        const obj = { period: p };
        for (const as of aspects) obj[as.ratio] = as.counts[i] ?? 0;
        return obj;
    });

    const stack = d3.stack().keys(ratioNames).offset(d3.stackOffsetExpand)(stackData);

    const x = d3.scaleBand().domain(periods).range([0, iW]).padding(0.05);
    const xC = p => x(p) + x.bandwidth() / 2;
    const y = d3.scaleLinear().domain([0, 1]).range([iH, 0]);

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    tlAxisBottom(g, x, periods, iH, c);
    g.append('g').call(d3.axisLeft(y).ticks(4).tickSizeOuter(0).tickFormat(d => `${(d*100).toFixed(0)}%`))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 9);
    g.selectAll('.domain, .tick line').attr('stroke', c.border);

    g.selectAll('.aspect-layer').data(stack).join('path')
        .attr('class', 'aspect-layer')
        .attr('fill', d => aspectColor(c, d.key))
        .attr('opacity', 0.85)
        .attr('d', d3.area().x(d => xC(d.data.period)).y0(d => y(d[0])).y1(d => y(d[1])).curve(d3.curveMonotoneX));

    const legend = document.createElement('div');
    legend.className = 'stats-tl-legend';
    ratioNames.forEach(ratio => {
        const item = document.createElement('div');
        item.className = 'stats-tl-legend-item';
        item.innerHTML = `<span class="stats-tl-legend-swatch" style="background:${aspectColor(c, ratio)}"></span>${escapeHtml(ratio)}`;
        legend.appendChild(item);
    });
    el.appendChild(legend);
}

/* ─── TL 6. Megapixel timeline ──────────────────────────────────── */

function renderMegapixelTimeline(el, tlData) {
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
        .datum(validPeriods)
        .attr('d', d3.line().x(p => xC(p)).y(p => y(byPeriod.get(p).avg)).curve(d3.curveMonotoneX))
        .attr('fill', 'none').attr('stroke', c.textSec).attr('stroke-width', 1.5).attr('stroke-dasharray', '4,3');

    // Max line (step)
    g.append('path')
        .datum(validPeriods)
        .attr('d', d3.line().x(p => xC(p)).y(p => y(byPeriod.get(p).max)).curve(d3.curveStepAfter))
        .attr('fill', 'none').attr('stroke', c.cats[0]).attr('stroke-width', 2);

    // Mark significant max jumps (>20%)
    for (let i = 1; i < validPeriods.length; i++) {
        const prev = byPeriod.get(validPeriods[i - 1]);
        const curr = byPeriod.get(validPeriods[i]);
        if (curr.max > prev.max * 1.2) {
            g.append('circle').attr('cx', xC(validPeriods[i])).attr('cy', y(curr.max))
                .attr('r', 4).attr('fill', c.cats[0]).attr('stroke', c.surface).attr('stroke-width', 2);
        }
    }

    const legend = document.createElement('div');
    legend.className = 'stats-tl-legend';
    legend.innerHTML = `
        <div class="stats-tl-legend-item">
            <svg width="20" height="10" style="flex-shrink:0"><line x1="0" y1="5" x2="20" y2="5" stroke="${c.cats[0]}" stroke-width="2"/></svg>
            Max MP
        </div>
        <div class="stats-tl-legend-item">
            <svg width="20" height="10" style="flex-shrink:0"><line x1="0" y1="5" x2="20" y2="5" stroke="${c.textSec}" stroke-width="1.5" stroke-dasharray="4,3"/></svg>
            Avg MP
        </div>`;
    el.appendChild(legend);
}
