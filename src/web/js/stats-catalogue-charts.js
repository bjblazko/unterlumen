// stats-catalogue-charts.js — Completeness, in the Cataloguing topic: for
// each field the metadata editor fills, the photos that have it and those
// that lack it (library/completeness.go). The missing part is the one to
// act on: a click shows exactly those photos, to select and take to
// Metadata.

const COMPLETENESS_NAMES = {
    title: 'Title', description: 'Description', keywords: 'Keywords', location: 'GPS position',
    country: 'Country', people: 'People named', rating: 'Stars',
};

function renderCompleteness(el, completeness, total, onPick) {
    if (!total || !completeness?.length) { el.textContent = t('No data'); return; }
    const c = chartColors();
    const W = 680, rowH = 30, pad = { top: 8, left: 130, right: 150, bottom: 8 };
    const svg = svgBase(el, W, pad.top + completeness.length * rowH + pad.bottom);
    const x = d3.scaleLinear().domain([0, total]).range([0, W - pad.left - pad.right]);
    completeness.forEach((f, i) => {
        const y = pad.top + i * rowH;
        const name = t(COMPLETENESS_NAMES[f.name] || f.name);
        const missing = total - f.count;
        svg.append('text').attr('x', pad.left - 8).attr('y', y + rowH / 2 + 4)
            .attr('text-anchor', 'end').attr('fill', c.text).attr('font-size', 11).text(name);
        // The track is all photos; the filled part those that have the field.
        svg.append('rect').attr('x', pad.left).attr('y', y + 6)
            .attr('width', x(total)).attr('height', rowH - 12).attr('rx', 2).attr('fill', c.border).attr('opacity', 0.5);
        if (f.count) {
            svg.append('rect').attr('x', pad.left).attr('y', y + 6)
                .attr('width', Math.max(1, x(f.count))).attr('height', rowH - 12).attr('rx', 2).attr('fill', c.cats[0]);
        }
        const pct = Math.round(100 * f.count / total);
        svg.append('text').attr('x', pad.left + x(total) + 8).attr('y', y + rowH / 2 + 4)
            .attr('fill', c.textSec).attr('font-size', 10).attr('font-family', 'var(--font-mono)')
            .text(`${pct} %`);
        if (!missing) return;
        const label = t('{field}: {n} without', { field: name, n: photoCount(missing) });
        // Text answers a click only on its letters; the link's whole box does.
        const lx = pad.left + x(total) + 44;
        const hit = svg.append('rect').datum(f.name).attr('x', lx).attr('y', y + 4)
            .attr('width', pad.right - 48).attr('height', rowH - 8).attr('fill', 'transparent');
        hit.append('title').text(label);
        pickable(hit, onPick, label);
        svg.append('text').attr('x', lx + 4).attr('y', y + rowH / 2 + 4)
            .attr('fill', c.text).attr('font-size', 10).attr('text-decoration', 'underline')
            .attr('pointer-events', 'none')
            .text(t('{n} without', { n: formatCount(missing) }));
        // The empty part of the track leads there too.
        const gapRect = svg.append('rect').datum(f.name).attr('x', pad.left + x(f.count)).attr('y', y + 6)
            .attr('width', x(missing)).attr('height', rowH - 12).attr('fill', 'transparent');
        pickable(gapRect, onPick, label);
    });
}

/* --- Stars, keywords and own fields (the "catalogue" tallies) --- */

const STAR_SERIES = ['0', '1', '2', '3', '4', '5'];

function renderStarsOverTime(el, cat, onPick) {
    const labels = { 0: 'Not rated' };
    for (let n = 1; n <= 5; n++) labels[n] = '★'.repeat(n);
    renderShareLines(el, cat?.stars, STAR_SERIES, onPick, { labels });
}

function renderKeywordBars(el, cat, onPick) {
    const items = [...sumBy(tallyRows(cat?.keywords), p => p[0]).entries()]
        .map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count);
    renderCountBars(el, items, onPick, { half: true, empty: 'No photo has a keyword yet.' });
}

function renderOwnFieldBars(el, cat, onPick) {
    const items = tallyRows(cat?.fields).map(r => ({ key: r.parts[0], value: r.parts[1], label: `${r.parts[0]} = ${r.parts[1]}`, count: r.n }))
        .sort((a, b) => b.count - a.count);
    renderCountBars(el, items, onPick, { half: true, empty: 'No photo has an own field yet.' });
}
