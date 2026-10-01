// Statistics colour charts — black and white against colour, the colour of
// each period, the main colours and warm against cool through the year.
// Uses the helpers in stats-charts.js and stats-timeline-charts.js.
//
// A mark filled with a photo's own colour shows data, not an interface
// signal: it is the same in both themes, has a hairline so a pale swatch
// still shows on the page, and always carries a word in its label or name
// (ADR-0030, "A photo's own colour as a mark").

/* ─── Helpers ───────────────────────────────────────────────────── */

// Named after the colours whose OKLCh hue lies in each sector: red 29°,
// orange 53°, amber 84°, yellow 110°, green 142°, emerald 152°, teal 195°,
// sky blue 226°, blue 264°, violet 294°, purple 328°, pink 352°.
const HUE_NAMES = ['red', 'orange', 'amber', 'yellow', 'green', 'emerald',
    'teal', 'sky blue', 'blue', 'violet', 'purple', 'pink'];

// The name of a 30° hue sector of OKLCh, as the index stores it (hue_bin).
function hueName(bin) {
    return HUE_NAMES[bin] ?? 'grey';
}

function hueNameOf(colour) {
    return colour.c < 0.03 ? 'grey' : hueName(Math.floor(colour.h / 30) % 12);
}

function photoColour({ l, c, h }) {
    return `oklch(${l.toFixed(3)} ${c.toFixed(3)} ${h.toFixed(1)})`;
}

const COLOUR_CLASSES = { mono: 'Black and white', tinted: 'Toned', colour: 'Colour' };

const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December'];

const pct = (n, total) => (total ? Math.round(100 * n / total) : 0);

// Every period from the first to the last, with zeros for the empty ones, as
// continuousTimeline does for the other charts over time.
function continuousColour(colour) {
    const periods = colour?.periods;
    if (!periods?.length) return colour;
    const all = periodRange(periods[0], periods[periods.length - 1], colour.granularity);
    if (!all || all.length === periods.length) return colour;
    const at = new Map(periods.map((p, i) => [p, i]));
    return {
        ...colour,
        periods: all,
        classes: colour.classes.map(cs => ({ ...cs, counts: all.map(p => (at.has(p) ? cs.counts[at.get(p)] : 0)) })),
    };
}

function noColourData(el, colour) {
    if (colour?.analysedPhotos) return false;
    tlNoData(el, 'No photo has been analysed yet. Analysis runs after each scan.');
    return true;
}

// A tooltip beside the pointer, for marks that are not lines.
function markTooltip(el) {
    const tip = d3.select(el).append('div').attr('class', 'stats-tooltip').style('display', 'none');
    return {
        show(event, html) {
            const [mx, my] = d3.pointer(event, el);
            tip.style('display', 'block').html(html).style('left', `${mx + 12}px`).style('top', `${my - 28}px`);
        },
        hide() { tip.style('display', 'none'); },
    };
}

/* ─── 1. Black and white against colour ─────────────────────────── */

// The share of each class per period, so a period with few photos counts as
// much as one with many. The classes are categories: the chart ramp, not
// photo colours.
function renderColourClasses(el, colour, onPick) {
    if (noColourData(el, colour)) return;
    const { periods, classes } = colour;
    if (!periods.length) { tlNoData(el, 'None of the analysed photos has a date.'); return; }
    const c = chartColors();
    const totals = periods.map((_, i) => d3.sum(classes, cs => cs.counts[i]));
    const series = classes.map((cs, slot) => ({
        name: COLOUR_CLASSES[cs.class],
        cls: cs.class,
        counts: cs.counts,
        values: periods.map((_, i) => (totals[i] ? 100 * cs.counts[i] / totals[i] : null)),
        color: c.cats[slot],
    }));
    renderSeriesLines(el, periods, series, {
        yMax: 100,
        yFormat: d => `${d} %`,
        format: (v, s, i) => `${Math.round(v)} % (${s.counts[i].toLocaleString()})`,
        onPick: onPick && (({ period, series: s }) => onPick({ period, cls: s?.cls ?? null, name: s?.name ?? null })),
        height: 180,
    });
}

/* ─── 2. The colour of each period ──────────────────────────────── */

function renderColourStrip(el, colour, onPick) {
    if (noColourData(el, colour)) return;
    const { periods, periodColours } = colour;
    if (!periodColours.length) { tlNoData(el, 'There are no colour photos with a date here.'); return; }
    const c = chartColors();
    const line = cssVar('--line');
    const W = 680, H = 112;
    const m = { top: 8, right: 16, bottom: 48, left: 16 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;
    const x = d3.scaleBand().domain(periods).range([0, iW]);
    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    tlAxisBottom(g, x, periods, iH, c);
    const tip = markTooltip(el);
    const label = d => `${d.period}, ${hueNameOf(d.colour)}, ${photoCount(d.photos)}`;
    const swatches = g.append('g').selectAll('rect').data(periodColours).join('rect')
        .attr('x', d => x(d.period)).attr('y', 0)
        .attr('width', Math.max(1, x.bandwidth())).attr('height', iH)
        .attr('fill', d => photoColour(d.colour))
        .attr('stroke', line).attr('stroke-width', 0.5)
        .on('mousemove', (event, d) => tip.show(event,
            `<span class="stats-tooltip-value">${escapeHtml(d.period)}</span><br>${escapeHtml(hueNameOf(d.colour))}, ${photoCount(d.photos)}`))
        .on('mouseleave', tip.hide);
    pickable(swatches, onPick, label);
}

/* ─── 3. The main colours ───────────────────────────────────────── */

// Twelve sectors of 30° in the order of the hue circle; a sector's length
// grows with the root of its share, so its area does with the share. The
// neutrals sit in the middle, in the interface's grey: they have no hue.
function renderHueWheel(el, colour, onPick) {
    if (noColourData(el, colour)) return;
    const sectors = colour.hueWheel;
    if (!sectors.length) { tlNoData(el, 'There are no colour photos here.'); return; }
    const c = chartColors();
    const line = cssVar('--line');
    const W = 360, H = 320, r0 = 40, R = 112;
    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${W / 2},${H / 2})`);
    const maxShare = d3.max(sectors, s => s.share);
    const outer = s => r0 + (R - r0) * Math.sqrt(s.share / maxShare);
    const step = Math.PI / 6;
    const arc = d3.arc().innerRadius(r0).outerRadius(outer)
        .startAngle(s => s.bin * step).endAngle(s => (s.bin + 1) * step).padAngle(0.01);
    const share = s => `${Math.round(100 * s.share)} %`;
    const tip = markTooltip(el);

    const marks = g.append('g').selectAll('path').data(sectors).join('path')
        .attr('d', arc).attr('fill', s => photoColour(s.colour))
        .attr('stroke', line).attr('stroke-width', 0.5)
        .on('mousemove', (event, s) => tip.show(event,
            `${escapeHtml(hueName(s.bin))} <span class="stats-tooltip-value">${share(s)}</span><br>Main colour of ${photoCount(s.photos)}`))
        .on('mouseleave', tip.hide);
    // A sector shows the photos it is a main colour of; a hue that is only
    // ever a small part of a photo has none to show.
    pickable(marks.filter(s => s.photos > 0), onPick && (s => onPick(s.bin)),
        s => `${hueName(s.bin)}, ${share(s)} of the colours, main colour of ${photoCount(s.photos)}`);

    const labelled = sectors.filter(s => s.share >= 0.01);
    const at = (s, r) => d3.pointRadial((s.bin + 0.5) * step, r);
    const labels = g.append('g').selectAll('text').data(labelled).join('text')
        .attr('text-anchor', s => { const [px] = at(s, 1); return Math.abs(px) < 0.3 ? 'middle' : px > 0 ? 'start' : 'end'; })
        .attr('transform', s => `translate(${at(s, outer(s) + 10)})`)
        .attr('font-size', 10);
    labels.append('tspan').attr('x', 0).attr('fill', c.textSec).text(s => hueName(s.bin));
    labels.append('tspan').attr('x', 0).attr('dy', '1.2em').attr('fill', c.text)
        .attr('font-family', 'var(--font-mono)').text(share);

    g.append('circle').attr('r', r0 - 6).attr('fill', cssVar('--bg-3'));
    g.append('text').attr('text-anchor', 'middle').attr('dy', '-0.3em')
        .attr('fill', c.textSec).attr('font-size', 10).text('Neutral');
    g.append('text').attr('text-anchor', 'middle').attr('dy', '1em')
        .attr('fill', c.text).attr('font-size', 11).attr('font-family', 'var(--font-mono)')
        .text(`${Math.round(100 * colour.neutralShare)} %`);
}

/* ─── 4. Warm and cool through the year ─────────────────────────── */

// Each month of all years together: the share of warm colour photos above
// the line, of cool ones below it, each bar in the mean colour of its photos.
// Above and below say warm and cool as well, so the chart reads without
// its colours. A select narrows it to one year; a bar then reports its year.
function renderWarmCool(el, colour, onPick) {
    if (noColourData(el, colour)) return;
    if (!d3.sum(colour.seasons, s => s.photos)) { tlNoData(el, 'There are no colour photos with a date here.'); return; }
    const years = [...(colour.seasonsByYear ?? [])].reverse();
    const row = document.createElement('div');
    row.className = 'stats-chart-controls';
    row.innerHTML = `<label class="stats-period"><span class="stats-period-label">Year</span>
        <select class="btn btn-sm select-btn stats-season-year">
            <option value="">All years</option>
            ${years.map(y => `<option value="${escapeHtml(y.year)}">${escapeHtml(y.year)}</option>`).join('')}
        </select></label>`;
    el.appendChild(row);
    const plot = document.createElement('div');
    el.appendChild(plot);
    const draw = (year) => {
        plot.innerHTML = '';
        const months = year ? years.find(y => y.year === year).seasons : colour.seasons;
        drawWarmCool(plot, months, year, onPick);
    };
    row.querySelector('select').addEventListener('change', e => draw(e.target.value));
    draw('');
}

function drawWarmCool(el, months, year, onPick) {
    const c = chartColors();
    const line = cssVar('--line');
    const W = 680, H = 240;
    const m = { top: 20, right: 16, bottom: 24, left: 48 };
    const iW = W - m.left - m.right, iH = H - m.top - m.bottom;
    const bars = months.flatMap(s => [
        { year, month: s.month, warmth: 'warm', n: s.warm, photos: s.photos, colour: s.warmColour },
        { year, month: s.month, warmth: 'cool', n: s.cool, photos: s.photos, colour: s.coolColour },
    ]).filter(b => b.n > 0);
    if (!bars.length) { tlNoData(el, 'None of the colour photos of this year is clearly warm or cool.'); return; }
    const top = d3.max(bars, b => pct(b.n, b.photos)) || 1;
    const x = d3.scaleBand().domain(months.map(s => s.month)).range([0, iW]).padding(0.25);
    const y = d3.scaleLinear().domain([-top, top]).range([iH, 0]).nice();

    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    g.append('g').attr('class', 'stats-grid-lines')
        .call(d3.axisLeft(y).ticks(4).tickSize(-iW).tickFormat(d => `${Math.abs(d)} %`))
        .call(axis => axis.select('.domain').remove())
        .call(axis => axis.selectAll('line').attr('stroke', c.border))
        .call(axis => axis.selectAll('text').attr('fill', c.textSec).attr('font-size', 9));
    g.append('line').attr('x1', 0).attr('x2', iW).attr('y1', y(0)).attr('y2', y(0)).attr('stroke', c.axis);
    g.append('text').attr('x', -m.left).attr('y', -8).attr('fill', c.textSec).attr('font-size', 10).text('Warm');
    g.append('text').attr('x', -m.left).attr('y', iH + 16).attr('fill', c.textSec).attr('font-size', 10).text('Cool');
    g.append('g').selectAll('text').data(months).join('text')
        .attr('x', s => x(s.month) + x.bandwidth() / 2).attr('y', iH + 16)
        .attr('text-anchor', 'middle').attr('fill', c.textSec).attr('font-size', 9)
        .text(s => MONTHS[s.month - 1].slice(0, 3));

    const tip = markTooltip(el);
    const label = b => `${b.warmth === 'warm' ? 'Warm' : 'Cool'} in ${MONTHS[b.month - 1]}${b.year ? ' ' + b.year : ''}: ${pct(b.n, b.photos)} % of ${photoCount(b.photos)}`;
    const marks = g.append('g').selectAll('rect').data(bars).join('rect')
        .attr('x', b => x(b.month)).attr('width', x.bandwidth())
        .attr('y', b => (b.warmth === 'warm' ? y(pct(b.n, b.photos)) : y(0)))
        .attr('height', b => Math.abs(y(pct(b.n, b.photos)) - y(0)))
        // Warm or cool photos whose main colours are all greys have no hue to show.
        .attr('fill', b => (b.colour ? photoColour(b.colour) : cssVar('--bg-3'))).attr('stroke', line).attr('stroke-width', 0.5)
        .on('mousemove', (event, b) => tip.show(event, escapeHtml(label(b))))
        .on('mouseleave', tip.hide);
    pickable(marks, onPick, label);
    g.append('g').selectAll('text').data(bars).join('text')
        .attr('x', b => x(b.month) + x.bandwidth() / 2)
        .attr('y', b => (b.warmth === 'warm' ? y(pct(b.n, b.photos)) - 4 : y(-pct(b.n, b.photos)) + 11))
        .attr('text-anchor', 'middle').attr('fill', c.textSec).attr('font-size', 9)
        .attr('font-family', 'var(--font-mono)')
        .text(b => `${pct(b.n, b.photos)} %`);
}
