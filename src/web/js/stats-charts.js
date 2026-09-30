// Statistics charts — the snapshot of a library: formats, film simulations,
// lenses, exposure, when photos were taken. D3, colour from the chart ramp
// (ADR-0034). Shared chart helpers live here too.

/* ─── helpers ──────────────────────────────────────────────────── */

function expandValues(valueCounts) {
    const arr = [];
    for (const {value, count} of (valueCounts ?? [])) {
        for (let i = 0; i < count; i++) arr.push(value);
    }
    return arr;
}

function cssVar(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function resolveColor(cssValue) {
    const el = document.createElement('div');
    el.style.color = cssValue;
    el.style.display = 'none';
    document.body.appendChild(el);
    const rgb = getComputedStyle(el).color;
    document.body.removeChild(el);
    return rgb;
}

// Every colour a chart draws with comes from the tokens (ADR-0034), so the
// two themes are designed rather than flipped, and the eight categorical
// slots keep their order: slot N is the same entity in light and in dark.
// Magnitude reads as a step on one hue, not as an opacity guess: five steps,
// light to dark in light mode and the other way round in dark (ADR-0034).
function seqStep(c, value, max) {
    if (!(max > 0) || !(value > 0)) return null;
    const i = Math.min(c.seq.length - 1, Math.floor(Math.sqrt(value / max) * c.seq.length));
    return c.seq[i];
}

function chartColors() {
    return {
        text:    cssVar('--fg'),
        textSec: cssVar('--fg-2'),
        border:  cssVar('--chart-grid'),
        axis:    cssVar('--chart-axis'),
        surface: cssVar('--bg'),
        accent:  cssVar('--accent'),
        accentRgb: resolveColor(cssVar('--accent')),
        cats: Array.from({ length: 8 }, (_, i) => cssVar(`--chart-${i + 1}`)),
        seq:  Array.from({ length: 5 }, (_, i) => cssVar(`--chart-seq-${i + 1}`)),
    };
}

// A chart is drawn at a fixed size and shrinks with a narrow card. It grows
// by at most a third, which brings its 9–11 px labels to the page's own text
// size; any more and they would outgrow it.
function svgBase(el, w, h) {
    return d3.select(el).append('svg')
        .attr('width', w).attr('height', h)
        .attr('viewBox', `0 0 ${w} ${h}`)
        .attr('class', 'stats-svg')
        .style('max-width', `${Math.round(w * 1.3)}px`);
}

// Marks that show their photos when clicked, or with Enter or Space: a pointer,
// a place in the tab order and a name. Without onPick (the overview's
// previews) they stay pictures.
function pickable(selection, onPick, label) {
    if (!onPick) return selection;
    return selection.classed('stats-pickable', true)
        .attr('tabindex', 0).attr('role', 'button')
        .attr('aria-label', label)
        .on('click', (event, d) => onPick(d))
        .on('keydown', (event, d) => {
            if (event.key !== 'Enter' && event.key !== ' ') return;
            event.preventDefault();
            onPick(d);
        });
}

const photoCount = n => `${n.toLocaleString()} ${n === 1 ? 'photo' : 'photos'}`;

/* ─── 1. Format donut ───────────────────────────────────────────── */

function renderFormatDonut(el, formats, onPick) {
    if (!formats?.length) { el.textContent = 'No data'; return; }
    const c = chartColors();
    const W = 320, H = 220, R = 80, r = 44;
    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${W/2},${H/2})`);

    const total = d3.sum(formats, d => d.count);
    const pie = d3.pie().value(d => d.count).sort((a,b) => b.count - a.count);
    const arc = d3.arc().innerRadius(r).outerRadius(R);
    const arcHover = d3.arc().innerRadius(r).outerRadius(R+6);

    const color = d3.scaleOrdinal()
        .domain(formats.map(d => d.name))
        .range(c.cats);

    const tooltip = d3.select(el).append('div').attr('class','stats-tooltip').style('display','none');

    const slices = g.selectAll('path')
        .data(pie(formats))
        .join('path')
        .attr('d', arc)
        .attr('fill', d => color(d.data.name))
        .attr('stroke', c.surface).attr('stroke-width', 2)
        .on('mouseover', function(event, d) {
            d3.select(this).attr('d', arcHover);
            const pct = ((d.data.count / total) * 100).toFixed(1);
            tooltip.style('display','block').html(`<strong>${d.data.name.toUpperCase()}</strong><br>${d.data.count.toLocaleString()} (${pct}%)`);
        })
        .on('mousemove', function(event) {
            const [mx,my] = d3.pointer(event, el);
            tooltip.style('left', mx+12+'px').style('top', my-28+'px');
        })
        .on('mouseout', function() {
            d3.select(this).attr('d', arc);
            tooltip.style('display','none');
        });
    pickable(slices, onPick && (d => onPick(d.data.name)), d => `${d.data.name.toUpperCase()}, ${photoCount(d.data.count)}`);

    // Centre label
    g.append('text').attr('text-anchor','middle').attr('dy','-0.2em')
        .attr('fill', c.text).attr('font-size', 18).attr('font-weight', 500)
        .text(total.toLocaleString());
    g.append('text').attr('text-anchor','middle').attr('dy','1.2em')
        .attr('fill', c.textSec).attr('font-size', 11)
        .text('photos');

    // Legend
    const leg = svg.append('g').attr('transform', `translate(${W/2+R+14},${H/2 - formats.length*9})`);
    formats.slice(0,6).forEach((d,i) => {
        const row = leg.append('g').attr('transform', `translate(0,${i*18})`);
        row.append('rect').attr('width',10).attr('height',10).attr('rx',2).attr('fill', color(d.name));
        row.append('text').attr('x',14).attr('y',9).attr('fill',c.textSec).attr('font-size',11)
            .text(d.name.toUpperCase());
    });
}

/* ─── 2. Film simulation bar ────────────────────────────────────── */

// A film simulation's colour shows the look it is named after — Velvia is
// punchy, Acros is grey — so this is a domain palette, not an identity ramp
// (ADR-0034). It is legible only because every bar carries its name; colour
// alone never has to carry the difference here.
const FILMSIM_TOKENS = {
    'Provia':         '--film-provia',
    'Velvia':         '--film-velvia',
    'Astia':          '--film-astia',
    'Classic Chrome': '--film-classic-chrome',
    'Pro Neg. Hi':    '--film-pro-neg-hi',
    'Pro Neg. Std':   '--film-pro-neg-std',
    'Eterna':         '--film-eterna',
    'Classic Neg.':   '--film-classic-neg',
    'Nostalgic Neg.': '--film-nostalgic-neg',
    'Reala ACE':      '--film-reala-ace',
    'Acros':          '--film-acros',
    'Monochrome':     '--film-monochrome',
    'Sepia':          '--film-sepia',
    'None':           '--film-none',
};

function filmSimColor(name) {
    const token = FILMSIM_TOKENS[name];
    return token ? cssVar(token) : null;
}

function renderFilmSimBar(el, filmSims, onPick) {
    if (!filmSims?.length) { el.textContent = 'No data'; return; }

    let showNone = false;

    const toggleRow = document.createElement('div');
    toggleRow.className = 'stats-filmsim-toggle-row';
    toggleRow.innerHTML =
        '<span class="stats-filmsim-toggle-label">Untagged photos</span>' +
        '<button class="toggle" role="switch" aria-checked="false" data-state="off">' +
        '<span class="toggle-label toggle-label-on">Counted</span>' +
        '<span class="toggle-track"><span class="toggle-thumb"></span></span>' +
        '<span class="toggle-label toggle-label-off">Left out</span>' +
        '</button>';
    el.appendChild(toggleRow);

    const btn = toggleRow.querySelector('.toggle');
    btn.addEventListener('click', () => {
        showNone = !showNone;
        btn.dataset.state = showNone ? 'on' : 'off';
        btn.setAttribute('aria-checked', String(showNone));
        svgEl.remove();
        svgEl = drawSVG();
    });

    let svgEl = drawSVG();

    function drawSVG() {
        const data = (showNone ? filmSims : filmSims.filter(s => s.name !== 'None')).slice(0, 12);
        const c = chartColors();
        const W = 680, barH = 22, pad = { top: 8, left: 130, right: 50, bottom: 8 };
        const H = pad.top + data.length * barH + pad.bottom;
        const svg = svgBase(el, W, H);

        const maxVal = d3.max(data, d => d.count);
        const x = d3.scaleLinear().domain([0, maxVal]).range([0, W - pad.left - pad.right]);

        data.forEach((d, i) => {
            const y = pad.top + i * barH;
            const name = cleanExif(d.name);
            const barColor = filmSimColor(name) ?? c.cats[i % c.cats.length];
            svg.append('text')
                .attr('x', pad.left - 8).attr('y', y + barH/2 + 4)
                .attr('text-anchor','end').attr('fill', c.text).attr('font-size', 11)
                .text(name);
            const bar = svg.append('rect').datum(name)
                .attr('x', pad.left).attr('y', y + 3)
                .attr('width', x(d.count)).attr('height', barH - 6)
                .attr('rx', 2).attr('fill', barColor);
            // Untagged photos have no value to look for.
            if (name !== 'None') pickable(bar, onPick, `${name}, ${photoCount(d.count)}`);
            svg.append('text')
                .attr('x', pad.left + x(d.count) + 5).attr('y', y + barH/2 + 4)
                .attr('fill', c.textSec).attr('font-size', 10)
                .text(d.count.toLocaleString());
        });
        return svg.node();
    }
}

/* ─── 3. Focal length histogram (with 35mm toggle) ─────────────── */

function renderFocalHistogram(el, focalLengths, focalLengths35, onPick) {
    const has35 = focalLengths35?.length > 0;
    let show35 = false;

    if (has35) {
        const wrap = document.createElement('div');
        wrap.className = 'stats-focal-toggle';
        Toggle.create(wrap, {
            initial: false,
            labelOn: '35mm',
            labelOff: 'Native',
            onChange: (on) => {
                show35 = on;
                svgEl.remove();
                svgEl = drawFocalSVG(show35 ? focalLengths35 : focalLengths);
            }
        });
        el.appendChild(wrap);
    }

    let svgEl = drawFocalSVG(focalLengths);

    function drawFocalSVG(data) {
        if (!data?.length) {
            const d = document.createElement('div');
            d.className = 'stats-nodata';
            d.textContent = 'No data';
            el.appendChild(d);
            return d;
        }
        const W = 600, H = 160;
        const margin = { top: 12, right: 20, bottom: 28, left: 44 };
        const iW = W - margin.left - margin.right;
        const iH = H - margin.top - margin.bottom;

        const svg = svgBase(el, W, H);
        const g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);
        const c = chartColors();

        const x = d3.scaleLinear().domain([0, d3.max(data, d => d) * 1.05]).range([0, iW]).nice();
        const bins = d3.histogram().domain(x.domain()).thresholds(x.ticks(30))(data);
        const y = d3.scaleLinear().domain([0, d3.max(bins, b => b.length)]).range([iH, 0]).nice();

        g.append('g').attr('class','stats-axis').attr('transform',`translate(0,${iH})`)
            .call(d3.axisBottom(x).tickSizeOuter(0).tickFormat(d => `${d}mm`))
            .selectAll('text').attr('fill', c.textSec).attr('font-size', 10);
        g.append('g').attr('class','stats-axis')
            .call(d3.axisLeft(y).ticks(4).tickSizeOuter(0))
            .selectAll('text').attr('fill', c.textSec).attr('font-size', 10);
        g.selectAll('.stats-axis path, .stats-axis line').attr('stroke', c.border);

        const bars = g.selectAll('rect').data(bins).join('rect')
            .attr('x', d => x(d.x0) + 1)
            .attr('y', d => y(d.length))
            .attr('width', d => Math.max(0, x(d.x1) - x(d.x0) - 2))
            .attr('height', d => iH - y(d.length))
            .attr('fill', c.cats[0]).attr('rx', 1);
        const field = data === focalLengths35 ? 'FocalLength35' : 'FocalLength';
        pickable(bars.filter(d => d.length > 0), onPick && (d => onPick({ field, min: d.x0, max: d.x1 })),
            d => `${d.x0}–${d.x1} mm, ${photoCount(d.length)}`);

        return svg.node();
    }
}

/* ─── 4. Aperture histogram ─────────────────────────────────────── */

function renderApertureHistogram(el, apertures, onPick) {
    if (!apertures?.length) { el.textContent = 'No data'; return; }
    const c = chartColors();
    const W = 280, H = 160;
    const margin = { top: 12, right: 16, bottom: 28, left: 40 };
    const iW = W - margin.left - margin.right, iH = H - margin.top - margin.bottom;
    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);

    // f-stop thresholds
    const stops = [1, 1.4, 2, 2.8, 4, 5.6, 8, 11, 16, 22, 32];
    const x = d3.scaleLog().domain([0.9, 35]).range([0, iW]).base(2);
    const bins = d3.histogram().domain([0.9, 35]).thresholds(stops)(apertures);
    const y = d3.scaleLinear().domain([0, d3.max(bins, b => b.length)]).range([iH, 0]).nice();

    g.append('g').attr('class','stats-axis').attr('transform',`translate(0,${iH})`)
        .call(d3.axisBottom(x).tickValues([1.4,2,2.8,4,5.6,8,11,16]).tickFormat(d => `f/${d}`).tickSizeOuter(0))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 9);
    g.append('g').attr('class','stats-axis')
        .call(d3.axisLeft(y).ticks(4).tickSizeOuter(0))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 10);
    g.selectAll('.stats-axis path, .stats-axis line').attr('stroke', c.border);

    const bars = g.selectAll('rect').data(bins).join('rect')
        .attr('x', d => x(Math.max(0.9, d.x0)) + 1)
        .attr('y', d => y(d.length))
        .attr('width', d => Math.max(0, x(Math.max(0.9, d.x1)) - x(Math.max(0.9, d.x0)) - 2))
        .attr('height', d => iH - y(d.length))
        .attr('fill', c.cats[0]).attr('rx', 1);
    pickable(bars.filter(d => d.length > 0), onPick && (d => onPick({ min: d.x0, max: d.x1 })),
        d => `f/${d.x0} to f/${d.x1}, ${photoCount(d.length)}`);
}

/* ─── 5. ISO histogram (log scale) ─────────────────────────────── */

function renderISOHistogram(el, isos, onPick) {
    if (!isos?.length) { el.textContent = 'No data'; return; }
    const c = chartColors();
    const W = 280, H = 160;
    const margin = { top: 12, right: 16, bottom: 28, left: 40 };
    const iW = W - margin.left - margin.right, iH = H - margin.top - margin.bottom;
    const svg = svgBase(el, W, H);
    const g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);

    const isoStops = [50, 100, 200, 400, 800, 1600, 3200, 6400, 12800, 25600, 51200];
    const filteredISOs = isos.filter(v => v >= 50);
    if (!filteredISOs.length) { el.textContent = 'No data'; return; }
    const x = d3.scaleLog().domain([50, 55000]).range([0, iW]).base(2);
    const bins = d3.histogram().domain([50, 55000]).thresholds(isoStops)(filteredISOs);
    const y = d3.scaleLinear().domain([0, d3.max(bins, b => b.length)]).range([iH, 0]).nice();

    g.append('g').attr('class','stats-axis').attr('transform',`translate(0,${iH})`)
        .call(d3.axisBottom(x).tickValues([100,400,1600,6400,25600]).tickFormat(d => d >= 1000 ? `${d/1000}K` : d).tickSizeOuter(0))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 9);
    g.append('g').attr('class','stats-axis')
        .call(d3.axisLeft(y).ticks(4).tickSizeOuter(0))
        .selectAll('text').attr('fill', c.textSec).attr('font-size', 10);
    g.selectAll('.stats-axis path, .stats-axis line').attr('stroke', c.border);

    const bars = g.selectAll('rect').data(bins).join('rect')
        .attr('x', d => x(Math.max(50, d.x0)) + 1)
        .attr('y', d => y(d.length))
        .attr('width', d => Math.max(0, x(Math.max(50, d.x1)) - x(Math.max(50, d.x0)) - 2))
        .attr('height', d => iH - y(d.length))
        .attr('fill', c.cats[0]).attr('rx', 1);
    pickable(bars.filter(d => d.length > 0), onPick && (d => onPick({ min: d.x0, max: d.x1 })),
        d => `ISO ${d.x0} to ${d.x1}, ${photoCount(d.length)}`);
}

/* ─── 6. Camera × lens treemap ──────────────────────────────────── */

function renderCameraLensTreemap(el, cameraLens, totalPhotos, onPick) {
    if (!cameraLens?.length) { el.textContent = 'No data'; return; }
    const c = chartColors();
    const W = 600, H = 220;
    const svg = svgBase(el, W, H);

    // Build hierarchy: root → cameras → lenses
    const cameraMap = new Map();
    for (const clc of cameraLens) {
        const cam = cleanExif(clc.camera);
        const lens = cleanExif(clc.lens) || '(unknown)';
        if (!cameraMap.has(cam)) cameraMap.set(cam, []);
        cameraMap.get(cam).push({ name: lens, value: clc.count });
    }
    const children = [];
    for (const [cam, lenses] of cameraMap) {
        children.push({ name: cam, children: lenses });
    }
    const root = d3.hierarchy({ name: 'root', children })
        .sum(d => d.value)
        .sort((a,b) => b.value - a.value);

    d3.treemap().size([W, H]).padding(2).paddingTop(18)(root);

    const cameraNames = [...cameraMap.keys()];
    const camColor = d3.scaleOrdinal().domain(cameraNames)
        .range(c.cats);

    const tooltip = d3.select(el).append('div').attr('class','stats-tooltip').style('display','none');

    // Camera-level cells (for label background)
    svg.selectAll('.cam-cell')
        .data(root.children ?? [])
        .join('g').attr('class','cam-cell')
        .each(function(d) {
            const g = d3.select(this);
            const cell = g.append('rect')
                .attr('x', d.x0).attr('y', d.y0)
                .attr('width', d.x1 - d.x0).attr('height', d.y1 - d.y0)
                .attr('fill', camColor(d.data.name)).attr('opacity', 0.15).attr('rx', 2);
            pickable(cell, onPick && (() => onPick({ camera: d.data.name })), `${d.data.name}, ${photoCount(d.value)}`);
            const w = d.x1 - d.x0;
            if (w > 40) {
                g.append('text')
                    .attr('x', d.x0 + 4).attr('y', d.y0 + 12)
                    .attr('fill', c.text).attr('font-size', 10).attr('font-weight', 500)
                    .text(truncate(d.data.name, Math.floor(w / 6)));
            }
        });

    // Leaf cells (lens level)
    svg.selectAll('.lens-cell')
        .data(root.leaves())
        .join('g').attr('class','lens-cell')
        .each(function(d) {
            const g = d3.select(this);
            const w = d.x1 - d.x0, h = d.y1 - d.y0;
            const cell = g.append('rect')
                .attr('x', d.x0 + 1).attr('y', d.y0 + 1)
                .attr('width', Math.max(0, w - 2)).attr('height', Math.max(0, h - 2))
                .attr('fill', camColor(d.parent.data.name)).attr('opacity', 0.55).attr('rx', 2);
            pickable(cell, onPick && (() => onPick({ camera: d.parent.data.name, lens: d.data.name })),
                `${d.parent.data.name}, ${d.data.name}, ${photoCount(d.value)}`);
            if (w > 50 && h > 18) {
                g.append('text')
                    .attr('x', d.x0 + 4).attr('y', d.y0 + h/2 + 4)
                    .attr('fill', c.text).attr('font-size', 9)
                    .text(truncate(d.data.name, Math.floor(w / 5.5)));
            }
        })
        .on('mouseover', function(event, d) {
            const pct = totalPhotos ? ((d.value / totalPhotos) * 100).toFixed(1) : '?';
            tooltip.style('display','block')
                .html(`<strong>${d.parent.data.name}</strong><br>${d.data.name}<br>${d.value.toLocaleString()} shots (${pct}%)`);
        })
        .on('mousemove', function(event) {
            const [mx,my] = d3.pointer(event, el);
            tooltip.style('left', mx+12+'px').style('top', my-28+'px');
        })
        .on('mouseout', () => tooltip.style('display','none'));
}

function truncate(s, maxLen) {
    return s.length > maxLen ? s.slice(0, Math.max(3, maxLen-1)) + '…' : s;
}

// Strip surrounding quotes stored as artifacts in some EXIF index values.
function cleanExif(s) {
    return s?.startsWith('"') && s.endsWith('"') ? s.slice(1, -1) : (s ?? '');
}

/* ─── 7. Shooting-time radial clock ─────────────────────────────── */

function renderShootingClock(el, shootingHours, onPick) {
    if (!shootingHours || shootingHours.every(h => h === 0)) { el.textContent = 'No data'; return; }
    const c = chartColors();
    const W = 260, H = 260, cx = W/2, cy = H/2;
    const innerR = 40, outerR = 100;
    const svg = svgBase(el, W, H);

    const maxVal = d3.max(shootingHours);
    const rScale = d3.scaleLinear().domain([0, maxVal]).range([innerR, outerR]);
    const slice = (2 * Math.PI) / 24;

    // Daytime shading: 6am (angle π/2) clockwise to 6pm (angle 3π/2)
    // Midnight is at top (angle 0), hours run clockwise.
    svg.append('path')
        .attr('d', d3.arc()({
            innerRadius: innerR, outerRadius: outerR + 8,
            startAngle: Math.PI / 2, endAngle: 3 * Math.PI / 2
        }))
        .attr('transform', `translate(${cx},${cy})`)
        .attr('fill', c.border).attr('opacity', 0.25);

    // Hour bars — D3 arc angle 0 = top (midnight), increasing clockwise
    for (let h = 0; h < 24; h++) {
        const n = shootingHours[h];
        if (n === 0) continue;
        const startAngle = h * slice;
        const endAngle = startAngle + slice * 0.85;
        const arc = d3.arc()({ innerRadius: innerR, outerRadius: rScale(n), startAngle, endAngle });
        const bar = svg.append('path').datum(h).attr('d', arc)
            .attr('transform', `translate(${cx},${cy})`)
            .attr('fill', seqStep(c, n, maxVal) ?? c.seq[0]);
        pickable(bar, onPick, `${String(h).padStart(2, '0')}:00 to ${String(h + 1).padStart(2, '0')}:00, ${photoCount(n)}`);
    }

    // Clock face labels: midnight top, 6am right, noon bottom, 6pm left
    // Using sin/cos with midnight-at-top convention: x = cx + r*sin(a), y = cy - r*cos(a)
    [0, 6, 12, 18].forEach(h => {
        const a = h * slice;
        const r = outerR + 14;
        svg.append('text')
            .attr('x', cx + r * Math.sin(a))
            .attr('y', cy - r * Math.cos(a) + 4)
            .attr('text-anchor', 'middle').attr('fill', c.textSec).attr('font-size', 10)
            .text(h + 'h');
    });

    // Inner ring
    svg.append('circle').attr('cx', cx).attr('cy', cy).attr('r', innerR)
        .attr('fill', 'none').attr('stroke', c.border).attr('stroke-width', 1);
}

/* ─── 8. Calendar heatmap (paginated by year) ───────────────────── */

function renderCalendarHeatmap(el, shootingDays, onPick) {
    if (!shootingDays || Object.keys(shootingDays).length === 0) { el.textContent = 'No data'; return; }

    const c = chartColors();
    const cellSize = 13, cellGap = 2, step = cellSize + cellGap;

    const years = [...new Set(Object.keys(shootingDays).map(d => d.slice(0, 4)))].sort();
    if (!years.length) { el.textContent = 'No data'; return; }

    const maxCount = d3.max(Object.values(shootingDays)) || 1;

    let yearIdx = years.length - 1;

    const controls = document.createElement('div');
    controls.className = 'stats-cal-controls';
    const chevron = d => `<svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="${d}"/></svg>`;
    const prevBtn = document.createElement('button');
    prevBtn.className = 'btn btn-sm stats-cal-nav';
    prevBtn.setAttribute('aria-label', 'Year before');
    prevBtn.innerHTML = chevron('M7 2L3 6l4 4');
    const yearLabel = document.createElement('span');
    yearLabel.className = 'stats-cal-year';
    const nextBtn = document.createElement('button');
    nextBtn.className = 'btn btn-sm stats-cal-nav';
    nextBtn.setAttribute('aria-label', 'Year after');
    nextBtn.innerHTML = chevron('M5 2l4 4-4 4');
    controls.appendChild(prevBtn);
    controls.appendChild(yearLabel);
    controls.appendChild(nextBtn);
    el.appendChild(controls);

    const svgContainer = document.createElement('div');
    el.appendChild(svgContainer);

    function renderYear(yr) {
        svgContainer.innerHTML = '';
        yearLabel.textContent = yr;
        prevBtn.disabled = yearIdx === 0;
        nextBtn.disabled = yearIdx === years.length - 1;

        // Week columns covering the full calendar year
        const jan1 = new Date(parseInt(yr), 0, 1);
        const dec31 = new Date(parseInt(yr), 11, 31);
        const startDate = new Date(jan1);
        startDate.setDate(startDate.getDate() - startDate.getDay()); // back to Sunday
        const endDate = new Date(dec31);
        endDate.setDate(endDate.getDate() + (6 - endDate.getDay())); // forward to Saturday

        const weeks = [];
        let cur = new Date(startDate);
        while (cur <= endDate) {
            const week = [];
            for (let d = 0; d < 7; d++) { week.push(new Date(cur)); cur.setDate(cur.getDate() + 1); }
            weeks.push(week);
        }

        const labelWidth = 16;
        const monthLabelHeight = 18;
        const W = labelWidth + weeks.length * step + 4;
        const H = monthLabelHeight + 7 * step + 4;
        const svg = svgBase(svgContainer, W, H);

        const tooltip = d3.select(el).append('div').attr('class','stats-tooltip').style('display','none');

        // Day-of-week labels (Mon, Wed, Fri only to avoid crowding)
        ['S','M','T','W','T','F','S'].forEach((lbl, di) => {
            if (di % 2 === 1) {
                svg.append('text')
                    .attr('x', labelWidth - 3)
                    .attr('y', monthLabelHeight + di * step + cellSize - 2)
                    .attr('text-anchor', 'end').attr('fill', c.textSec).attr('font-size', 9)
                    .text(lbl);
            }
        });

        // Month labels
        let lastMonth = -1;
        weeks.forEach((week, wi) => {
            const marker = week.find(d => d.getFullYear() === parseInt(yr) && d.getDate() <= 7);
            if (marker && marker.getMonth() !== lastMonth) {
                lastMonth = marker.getMonth();
                svg.append('text')
                    .attr('x', labelWidth + wi * step)
                    .attr('y', monthLabelHeight - 4)
                    .attr('fill', c.textSec).attr('font-size', 9)
                    .text(marker.toLocaleDateString('en', { month: 'short' }));
            }
        });

        // Day cells
        weeks.forEach((week, wi) => {
            week.forEach((date, di) => {
                const key = `${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,'0')}-${String(date.getDate()).padStart(2,'0')}`;
                const inYear = date.getFullYear() === parseInt(yr);
                const n = inYear ? (shootingDays[key] ?? 0) : 0;
                if (!inYear) return; // skip overflow days from adjacent years
                const rect = svg.append('rect')
                    .attr('x', labelWidth + wi * step)
                    .attr('y', monthLabelHeight + di * step)
                    .attr('width', cellSize).attr('height', cellSize)
                    .attr('rx', 2)
                    .attr('fill', n > 0 ? seqStep(c, n, maxCount) : c.border)
                    .attr('opacity', n > 0 ? 1 : 0.4);
                if (n > 0) {
                    pickable(rect.datum(key), onPick, `${key}, ${photoCount(n)}`);
                    rect.on('mouseover', function(event) {
                            tooltip.style('display','block').html(`${key}<br>${n} photo${n !== 1 ? 's':''}`);
                        })
                        .on('mousemove', function(event) {
                            const [mx,my] = d3.pointer(event, el);
                            tooltip.style('left', mx+12+'px').style('top', my-28+'px');
                        })
                        .on('mouseout', () => tooltip.style('display','none'));
                }
            });
        });
    }

    prevBtn.addEventListener('click', () => { if (yearIdx > 0) { yearIdx--; renderYear(years[yearIdx]); } });
    nextBtn.addEventListener('click', () => { if (yearIdx < years.length - 1) { yearIdx++; renderYear(years[yearIdx]); } });

    renderYear(years[yearIdx]);
}
