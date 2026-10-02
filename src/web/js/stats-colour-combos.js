// stats-colour-combos.js — colour combinations in the Colour topic (ADR-0049):
// the well-known ones as chords across the hue circle, and any two or three
// hues chosen on a wheel. A photo has a hue when its swatches of it cover at
// least a tenth of the frame; the server counts the photos per set of hues
// (colour.hueSets), so every combination is counted here, without asking.

const COMBO_SHARE_MIN_PERCENT = 10;

// The combinations photographers name, in the hue sectors of OKLCh.
const COLOUR_COMBOS = [
    { name: 'orange & teal', hues: [1, 6] },
    { name: 'blue & orange', hues: [1, 8] },
    { name: 'blue & yellow', hues: [3, 8] },
    { name: 'violet & yellow', hues: [3, 9] },
    { name: 'red & green', hues: [0, 4] },
    { name: 'pink & green', hues: [4, 11] },
    { name: 'red, yellow & blue', hues: [0, 3, 8] },
    { name: 'orange, green & violet', hues: [1, 4, 9] },
];

// The photos that have every hue of hues.
function comboCount(colour, hues) {
    let n = 0;
    for (const set of colour.hueSets || []) {
        if (hues.every(h => set.hues.includes(h))) n += set.photos;
    }
    return n;
}

// A combination's name: as photographers call it when it is a well-known one,
// else its hues in the order of the circle.
function comboName(hues) {
    const key = [...hues].sort((a, b) => a - b).join(',');
    const known = COLOUR_COMBOS.find(c => [...c.hues].sort((a, b) => a - b).join(',') === key);
    if (known) return known.name;
    const names = hues.map(hueName);
    return names.length > 2 ? `${names.slice(0, -1).join(', ')} & ${names[names.length - 1]}` : names.join(' & ');
}

// What a combination's photos are, for the photo column.
function comboCriterion(hues) {
    return {
        subject: `${capitalise(comboName(hues))} · each at least ${COMBO_SHARE_MIN_PERCENT} % of the frame`,
        params: { mono: 'colour', hues: hues.join(',') },
    };
}

// A sector's colour: the library's own for that hue where it has one, else
// the middle of the sector at a moderate lightness and chroma.
function sectorColour(colour, bin) {
    const own = (colour.hueWheel || []).find(s => s.bin === bin);
    return photoColour(own ? own.colour : { l: 0.66, c: 0.12, h: bin * 30 + 15 });
}

/* ─── Colour combinations: chords across the circle ─────────────── */

function renderColourCombos(el, colour, onPick) {
    if (noColourData(el, colour)) return;
    if (!colour.hueSets?.length) { tlNoData(el, 'There are no colour photos here.'); return; }
    const combos = COLOUR_COMBOS.map(c => ({ ...c, photos: comboCount(colour, c.hues) }));
    const max = d3.max(combos, c => c.photos) || 1;
    const c = chartColors();
    const W = 260, H = 260, R = 112, ring = 12;
    const step = Math.PI / 6;
    const at = (bin, r) => d3.pointRadial((bin + 0.5) * step, r);

    const wrap = d3.select(el).append('div').attr('class', 'combo-chart');
    const svg = wrap.append('svg').attr('viewBox', `0 0 ${W} ${H}`).attr('class', 'stats-svg combo-wheel')
        .attr('width', W).attr('height', H).style('max-width', `${Math.round(W * 1.3)}px`);
    const g = svg.append('g').attr('transform', `translate(${W / 2},${H / 2})`);
    g.append('g').selectAll('path').data(d3.range(12)).join('path')
        .attr('d', d3.arc().innerRadius(R - ring).outerRadius(R).startAngle(b => b * step).endAngle(b => (b + 1) * step).padAngle(0.02))
        .attr('fill', b => sectorColour(colour, b));

    // A pair is a chord bending toward the middle, a triad a triangle; as
    // thick as its photos are many, in the interface's ink.
    const width = d => (d.photos ? 1.5 + 9 * Math.sqrt(d.photos / max) : 0);
    const shape = d => {
        const pts = d.hues.map(b => at(b, R - ring - 4));
        if (pts.length === 2) return `M${pts[0]}Q0,0 ${pts[1]}`;
        return `M${pts.join('L')}Z`;
    };
    const tip = markTooltip(el);
    const chords = g.append('g').attr('fill', 'none').selectAll('path').data(combos).join('path')
        .attr('d', shape).attr('stroke', c.textSec).attr('stroke-width', width)
        .attr('stroke-linejoin', 'round').attr('stroke-linecap', 'round').attr('opacity', 0.75)
        .on('mousemove', (event, d) => tip.show(event, `${escapeHtml(capitalise(d.name))} <span class="stats-tooltip-value">${photoCount(d.photos)}</span>`))
        .on('mouseleave', tip.hide);

    const list = wrap.append('ol').attr('class', 'combo-list');
    const rows = list.selectAll('li').data(combos).join('li').attr('class', 'combo-row');
    const item = rows.append(d => document.createElement(d.photos && onPick ? 'button' : 'span'))
        .attr('class', 'combo-item').attr('type', 'button');
    item.append('span').attr('class', 'combo-swatches').selectAll('span').data(d => d.hues).join('span')
        .attr('class', 'combo-swatch').style('background', b => sectorColour(colour, b));
    item.append('span').attr('class', 'combo-name').text(d => capitalise(d.name));
    item.append('span').attr('class', 'combo-count').text(d => d.photos.toLocaleString());
    if (onPick) item.filter(d => d.photos > 0).on('click', (event, d) => onPick(d.hues));

    // Pointing at a row or a chord shows which chord it is.
    const focus = name => chords.attr('stroke', d => (d.name === name ? c.text : c.textSec))
        .attr('opacity', d => (name === null || d.name === name ? 0.9 : 0.25));
    rows.on('mouseenter focusin', (event, d) => focus(d.name)).on('mouseleave focusout', () => focus(null));
    chords.on('mouseenter.focus', (event, d) => focus(d.name)).on('mouseleave.focus', () => focus(null));
    focus(null);
}

/* ─── Your combination: two or three hues, chosen on a wheel ────── */

// The choice outlives a redraw: a new scope keeps it, so the counts change
// under the same colours.
let comboChoice = [1, 6];

function renderComboPicker(el, colour, onPick) {
    if (noColourData(el, colour)) return;
    if (!colour.hueSets?.length) { tlNoData(el, 'There are no colour photos here.'); return; }
    const c = chartColors();
    const W = 260, H = 260, R = 112, r0 = 62;
    const step = Math.PI / 6;
    const wrap = d3.select(el).append('div').attr('class', 'combo-chart');
    const svg = wrap.append('svg').attr('viewBox', `0 0 ${W} ${H}`).attr('class', 'stats-svg combo-wheel')
        .attr('width', W).attr('height', H).style('max-width', `${Math.round(W * 1.3)}px`);
    const g = svg.append('g').attr('transform', `translate(${W / 2},${H / 2})`);
    const arc = d3.arc().innerRadius(r0).outerRadius(R).startAngle(b => b * step).endAngle(b => (b + 1) * step).padAngle(0.02);
    const tip = markTooltip(el);
    const side = wrap.append('div').attr('class', 'combo-result');

    const sectors = g.append('g').selectAll('path').data(d3.range(12)).join('path')
        .attr('d', arc).attr('fill', b => sectorColour(colour, b))
        .attr('class', 'stats-pickable').attr('tabindex', 0).attr('role', 'button')
        .on('click', (event, b) => toggle(b))
        .on('keydown', (event, b) => {
            if (event.key !== 'Enter' && event.key !== ' ') return;
            event.preventDefault();
            toggle(b);
        })
        .on('mousemove', (event, b) => {
            const chosen = comboChoice.includes(b);
            const next = chosen ? comboChoice.filter(x => x !== b) : [...comboChoice, b];
            const hint = chosen ? 'Click to leave it out' : comboChoice.length >= 3 ? 'Three at most' : `With it: ${photoCount(comboCount(colour, next))}`;
            tip.show(event, `${escapeHtml(capitalise(hueName(b)))}<br><span class="stats-tooltip-value">${escapeHtml(hint)}</span>`);
        })
        .on('mouseleave', tip.hide);
    const centre = g.append('text').attr('text-anchor', 'middle').attr('font-family', 'var(--font-mono)')
        .attr('font-size', 18).attr('dy', '0.35em').attr('fill', c.text);

    function toggle(b) {
        if (comboChoice.includes(b)) comboChoice = comboChoice.filter(x => x !== b);
        else if (comboChoice.length < 3) comboChoice = [...comboChoice, b].sort((x, y) => x - y);
        draw();
    }

    // The chosen hues carry a frame in the interface's ink and say so to a
    // screen reader; orange stays the page's one action (ADR-0030).
    function draw() {
        const n = comboChoice.length >= 2 ? comboCount(colour, comboChoice) : null;
        sectors.attr('stroke', b => (comboChoice.includes(b) ? c.text : 'none'))
            .attr('stroke-width', b => (comboChoice.includes(b) ? 3 : 0))
            .attr('aria-pressed', b => String(comboChoice.includes(b)))
            .attr('aria-label', b => hueName(b));
        centre.text(n === null ? '' : n.toLocaleString());
        side.html('');
        if (n === null) {
            side.append('p').attr('class', 'combo-sentence').text('Choose two or three colours on the wheel.');
            return;
        }
        side.append('p').attr('class', 'combo-sentence')
            .text(`${n === 0 ? 'No photo is' : `${photoCount(n)} ${n === 1 ? 'is' : 'are'}`} ${comboName(comboChoice)}, each at least ${COMBO_SHARE_MIN_PERCENT} % of the frame.`);
        if (onPick) {
            side.append('button').attr('class', 'btn btn-sm').attr('type', 'button')
                .property('disabled', n === 0).text('Show photos')
                .on('click', () => onPick([...comboChoice]));
        }
    }
    draw();
}
