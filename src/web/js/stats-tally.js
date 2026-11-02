// stats-tally.js — reading the Statistics views' tallies (/api/library/
// insights: for each chart, key → photos, the key's parts joined by \x1f)
// and the few chart forms they are drawn in: bars, a histogram, a matrix, a
// row of cells per group (strips, also over time) and, from
// stats-timeline-charts.js, lines per series. Colour from the chart ramp
// (ADR-0034): one series in slot 1, magnitude in the sequential steps.

const TALLY_SEP = '\x1f';

// tallyRows are a tally's entries as { parts, n }.
function tallyRows(tally) {
    return Object.entries(tally || {}).map(([k, n]) => ({ parts: k.split(TALLY_SEP), n }));
}

// sumBy adds the photos of each row under key(parts).
function sumBy(rows, key) {
    const m = new Map();
    for (const r of rows) {
        const k = key(r.parts);
        if (k !== null && k !== undefined) m.set(k, (m.get(k) || 0) + r.n);
    }
    return m;
}

// tallyPeriods groups months ("YYYY-MM") into the periods a chart draws:
// years when they span more than three, else months, every one between the
// first and the last.
function tallyPeriods(months) {
    const sorted = [...new Set(months)].filter(m => /^\d{4}-\d{2}$/.test(m)).sort();
    if (!sorted.length) return { periods: [], of: () => null };
    const [y0, m0] = sorted[0].split('-').map(Number);
    const [y1, m1] = sorted[sorted.length - 1].split('-').map(Number);
    const years = (y1 * 12 + m1 - (y0 * 12 + m0)) > 36;
    const periods = [];
    if (years) {
        for (let y = y0; y <= y1; y++) periods.push(String(y));
    } else {
        for (let i = y0 * 12 + m0 - 1; i <= y1 * 12 + m1 - 1; i++) periods.push(`${Math.floor(i / 12)}-${String(i % 12 + 1).padStart(2, '0')}`);
    }
    return { periods, of: month => (years ? month.slice(0, 4) : month) };
}

// shareSeries turns a tally keyed (month, category) into one line per
// category of cats: its share of the period's photos, in per cent.
function shareSeries(tally, cats, { labels = {} } = {}) {
    const rows = tallyRows(tally);
    const { periods, of } = tallyPeriods(rows.map(r => r.parts[0]));
    const c = chartColors();
    const total = sumBy(rows, p => of(p[0]));
    const series = cats.map((cat, i) => {
        const n = sumBy(rows.filter(r => r.parts[1] === cat), p => of(p[0]));
        // name is what the line is called (its end label); key, the
        // category a click looks for.
        return { name: t(labels[cat] || cat), key: cat, color: c.cats[i], counts: periods.map(p => n.get(p) || 0),
            values: periods.map(p => (total.get(p) ? 100 * (n.get(p) || 0) / total.get(p) : null)) };
    });
    return { periods, series };
}

function renderShareLines(el, tally, cats, onPick, opts = {}) {
    const { periods, series } = shareSeries(tally, cats, opts);
    if (periods.length < 2) { tlNoData(el, opts.empty || 'No data'); return; }
    renderSeriesLines(el, periods, series, {
        yMax: 100, yFormat: v => `${v} %`,
        format: (v, s, i) => `${Math.round(v)} % · ${photoCount(s.counts[i])}`,
        onPick,
    });
}

// renderCountBars: items [{ name, label, count }], most first; half draws
// for a card of half the width.
function renderCountBars(el, items, onPick, { half = false, max = 12, empty = 'No data', label = d => d.label ?? d.name, valueText = d => formatCount(d.count), aria = d => photoCount(d.count) } = {}) {
    const data = items.slice(0, max);
    if (!data.length) { tlNoData(el, empty); return; }
    const c = chartColors();
    const W = half ? 340 : 680, barH = 22, pad = { top: 8, left: half ? 110 : 170, right: 44, bottom: 8 };
    const svg = svgBase(el, W, pad.top + data.length * barH + pad.bottom);
    const x = d3.scaleLinear().domain([0, d3.max(data, d => d.count)]).range([0, W - pad.left - pad.right]);
    data.forEach((d, i) => {
        const y = pad.top + i * barH;
        svg.append('text').attr('x', pad.left - 8).attr('y', y + barH / 2 + 4)
            .attr('text-anchor', 'end').attr('fill', c.text).attr('font-size', 11).text(truncate(label(d), half ? 16 : 26));
        pickable(svg.append('rect').datum(d).attr('x', pad.left).attr('y', y + 3)
            .attr('width', Math.max(1, x(d.count))).attr('height', barH - 6).attr('rx', 2).attr('fill', c.cats[0]),
        onPick, `${label(d)}, ${aria(d)}`);
        svg.append('text').attr('x', pad.left + x(d.count) + 5).attr('y', y + barH / 2 + 4)
            .attr('fill', c.textSec).attr('font-size', 10).text(valueText(d));
    });
    if (items.length > max) {
        el.insertAdjacentHTML('beforeend', `<p class="stats-chart-note">${t('The {n} with the most photos of {total}.', { n: max, total: formatCount(items.length) })}</p>`);
    }
}

// renderValueHistogram: bins [{ label, count }] in order, as columns.
function renderValueHistogram(el, bins, onPick, { half = false, unit = '' } = {}) {
    if (!bins.some(b => b.count)) { el.textContent = t('No data'); return; }
    const c = chartColors();
    const W = half ? 340 : 680, H = 190, m = { top: 10, right: 10, bottom: 36, left: 40 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;
    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    const x = d3.scaleBand().domain(bins.map((_, i) => i)).range([0, iW]).padding(0.12);
    const y = d3.scaleLinear().domain([0, d3.max(bins, b => b.count)]).range([iH, 0]).nice();
    g.append('g').call(d3.axisLeft(y).ticks(4).tickSize(-iW)).call(a => a.select('.domain').remove())
        .call(a => a.selectAll('line').attr('stroke', c.border))
        .call(a => a.selectAll('text').attr('fill', c.textSec).attr('font-size', 9));
    const every = Math.max(1, Math.ceil(bins.length / (half ? 6 : 12)));
    bins.forEach((b, i) => {
        if (b.count) {
            pickable(g.append('rect').datum(b).attr('x', x(i)).attr('y', y(b.count))
                .attr('width', x.bandwidth()).attr('height', iH - y(b.count)).attr('rx', 1).attr('fill', c.cats[0]),
            onPick, `${b.label}${unit}, ${photoCount(b.count)}`);
        }
        if (i % every === 0) {
            g.append('text').attr('x', x(i) + x.bandwidth() / 2).attr('y', iH + 14).attr('text-anchor', 'middle')
                .attr('fill', c.textSec).attr('font-size', 9).attr('font-family', 'var(--font-mono)').text(b.label);
        }
    });
}

// renderTallyMatrix: a cell per row and column, in the sequential ramp.
function renderTallyMatrix(el, rows, cols, count, onPick, { rowLabel = r => r, colLabel = c => c, empty = 'No data' } = {}) {
    const max = d3.max(rows.flatMap(r => cols.map(col => count(r, col)))) || 0;
    if (!max) { tlNoData(el, empty); return; }
    const c = chartColors();
    const left = 130, top = 70, cell = Math.max(14, Math.min(26, Math.floor((680 - left - 8) / cols.length)));
    // Room on the right for the last column's slanted name.
    const W = left + cols.length * cell + 60, H = top + rows.length * cell + 4;
    const svg = svgBase(el, W, H);
    cols.forEach((col, j) => {
        svg.append('text').attr('transform', `translate(${left + j * cell + cell / 2},${top - 6}) rotate(-45)`)
            .attr('fill', c.textSec).attr('font-size', 10).text(truncate(colLabel(col), 14));
    });
    rows.forEach((r, i) => {
        svg.append('text').attr('x', left - 6).attr('y', top + i * cell + cell / 2 + 4).attr('text-anchor', 'end')
            .attr('fill', c.text).attr('font-size', 11).text(truncate(rowLabel(r), 20));
        cols.forEach((col, j) => {
            const n = count(r, col);
            const rect = svg.append('rect').datum({ row: r, col })
                .attr('x', left + j * cell).attr('y', top + i * cell).attr('width', cell - 2).attr('height', cell - 2).attr('rx', 2)
                .attr('fill', n ? seqStep(c, n, max) : c.border).attr('opacity', n ? 1 : 0.4);
            if (!n) return;
            const label = `${rowLabel(r)} · ${colLabel(col)}, ${photoCount(n)}`;
            rect.append('title').text(label);
            pickable(rect, onPick, label);
        });
    });
}

// renderValueStrips: a row of cells per group along values (a lens's focal
// lengths, a camera's months), each cell shaded by its share of the row's
// largest, so every row reads on its own.
function renderValueStrips(el, groups, values, onPick, { valueLabel = v => v, every = 0 } = {}) {
    if (!groups.length || !values.length) { el.textContent = t('No data'); return; }
    const c = chartColors();
    const left = 170, top = 20, rowH = 20;
    const cellW = Math.max(3, (680 - left - 8) / values.length);
    const W = left + values.length * cellW + 8, H = top + groups.length * rowH + 4;
    const svg = svgBase(el, W, H);
    const step = every || Math.max(1, Math.ceil(values.length / 10));
    values.forEach((v, j) => {
        if (j % step) return;
        svg.append('text').attr('x', left + j * cellW).attr('y', top - 6).attr('fill', c.textSec)
            .attr('font-size', 9).attr('font-family', 'var(--font-mono)').text(valueLabel(v));
    });
    groups.forEach((g, i) => {
        const y = top + i * rowH;
        const most = Math.max(...values.map(v => g.cells.get(v) || 0));
        svg.append('text').attr('x', left - 6).attr('y', y + rowH / 2 + 4).attr('text-anchor', 'end')
            .attr('fill', c.text).attr('font-size', 11).text(truncate(g.label ?? g.name, 26));
        values.forEach((v, j) => {
            const n = g.cells.get(v) || 0;
            if (!n) return;
            const rect = svg.append('rect').datum({ group: g, value: v })
                .attr('x', left + j * cellW).attr('y', y + 3).attr('width', Math.max(1, cellW - 1)).attr('height', rowH - 6)
                .attr('fill', seqStep(c, n, most));
            const label = `${g.label ?? g.name} · ${valueLabel(v)}, ${photoCount(n)}`;
            rect.append('title').text(label);
            pickable(rect, onPick, label);
        });
    });
}
