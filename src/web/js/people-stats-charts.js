// people-stats-charts.js — the People topic's charts (ADR-0051): who is in
// the photos, when, with whom, and how many faces a photo holds. They read
// /api/library/people-stats; people are named people, put together across
// libraries by name. D3 and the chart ramp, as every chart (ADR-0034).

// What the charts derive from the appearances, worked out once per answer:
// every period from the first to the last, each person's photos per period,
// and how often two people share a photo, most first.
function peopleStatsDerived(ps) {
    if (ps._derived) return ps._derived;
    const n = ps.granularity === 'year' ? 4 : 7;
    const a = ps.appearances;
    const counts = ps.people.map(() => new Map());
    const seen = new Set();
    const byPhoto = new Map();
    a.person.forEach((person, i) => {
        const date = a.date[i];
        if (date.length >= n) {
            const period = date.slice(0, n);
            seen.add(period);
            counts[person].set(period, (counts[person].get(period) ?? 0) + 1);
        }
        const key = `${a.lib[i]}/${a.photo[i]}`;
        if (!byPhoto.has(key)) byPhoto.set(key, []);
        byPhoto.get(key).push(person);
    });
    const sorted = [...seen].sort();
    const periods = sorted.length ? (periodRange(sorted[0], sorted[sorted.length - 1], ps.granularity) ?? sorted) : [];
    const together = new Map();
    for (const people of byPhoto.values()) {
        people.sort((x, y) => x - y);
        for (let i = 0; i < people.length; i++) {
            for (let j = i + 1; j < people.length; j++) {
                const key = `${people[i]},${people[j]}`;
                together.set(key, (together.get(key) ?? 0) + 1);
            }
        }
    }
    const pairs = [...together].map(([key, photos]) => {
        const [x, y] = key.split(',').map(Number);
        return { a: x, b: y, photos };
    }).sort((p, q) => q.photos - p.photos);
    ps._derived = { periods, counts, pairs };
    return ps._derived;
}

const PEOPLE_NOBODY = 'Nobody is named in these photos. Name people under People, and they are counted here.';

/* ─── Who appears most ─────────────────────────────────────────── */

const PEOPLE_BARS = 20;

function renderPeopleBars(el, ps, onPick) {
    const data = ps?.people.slice(0, PEOPLE_BARS) ?? [];
    if (!data.length) { tlNoData(el, PEOPLE_NOBODY); return; }
    const c = chartColors();
    const W = 680, barH = 22, pad = { top: 8, left: 150, right: 60, bottom: 8 };
    const svg = svgBase(el, W, pad.top + data.length * barH + pad.bottom);
    const x = d3.scaleLinear().domain([0, data[0].photos]).range([0, W - pad.left - pad.right]);
    data.forEach((d, i) => {
        const y = pad.top + i * barH;
        svg.append('text')
            .attr('x', pad.left - 8).attr('y', y + barH / 2 + 4)
            .attr('text-anchor', 'end').attr('fill', c.text).attr('font-size', 11)
            .text(truncate(d.name, 22));
        pickable(svg.append('rect').datum(d.name)
            .attr('x', pad.left).attr('y', y + 3)
            .attr('width', Math.max(1, x(d.photos))).attr('height', barH - 6)
            .attr('rx', 2).attr('fill', c.cats[0]), onPick, `${d.name}, ${photoCount(d.photos)}`);
        svg.append('text')
            .attr('x', pad.left + x(d.photos) + 5).attr('y', y + barH / 2 + 4)
            .attr('fill', c.textSec).attr('font-size', 10)
            .text(d.photos.toLocaleString());
    });
    if (ps.people.length > PEOPLE_BARS) {
        el.insertAdjacentHTML('beforeend', `<p class="stats-chart-note">The ${PEOPLE_BARS} with the most photos of ${formatCount(ps.people.length)} people.</p>`);
    }
}

/* ─── People over time ─────────────────────────────────────────── */

// The eight with the most photos, each in their slot of the ramp.
function renderPeopleLines(el, ps, onPick) {
    if (!ps?.people.length) { tlNoData(el, PEOPLE_NOBODY); return; }
    const { periods, counts } = peopleStatsDerived(ps);
    if (!periods.length) { tlNoData(el, 'No photo of a named person has a date.'); return; }
    const c = chartColors();
    const series = ps.people.slice(0, 8).map((p, i) => ({
        name: p.name,
        values: periods.map(per => counts[i].get(per) ?? 0),
        color: c.cats[i],
    }));
    renderSeriesLines(el, periods, series, {
        yFormat: d => d.toLocaleString(),
        format: v => photoCount(v),
        onPick: onPick && (({ period, series: s }) => onPick({ period, person: s?.name ?? null })),
    });
}

/* ─── Photos by age ────────────────────────────────────────────── */

// Each person with a birthday (People, a person's ⋯ menu), by how old they
// were in their photos: the eight with the most photos, in the slot of the
// ramp their rank gives them everywhere in this topic.
function renderPeopleAges(el, ps, onPick) {
    if (!ps?.people.length) { tlNoData(el, PEOPLE_NOBODY); return; }
    const births = ps.people.map(p => birthdayOf(ps.birthdays ?? {}, p.name));
    const shown = ps.people.map((p, i) => i).filter(i => births[i]).slice(0, 8);
    if (!shown.length) { tlNoData(el, 'Nobody here has a birthday yet. Give one on a person\'s page under People, and their photos are counted by age here.'); return; }
    const a = ps.appearances;
    const byAge = new Map(shown.map(i => [i, new Map()]));
    let oldest = 0;
    a.person.forEach((p, k) => {
        if (!byAge.has(p)) return;
        const age = ageAt(births[p], a.date[k]);
        if (age === null) return;
        byAge.get(p).set(age, (byAge.get(p).get(age) ?? 0) + 1);
        oldest = Math.max(oldest, age);
    });
    if (![...byAge.values()].some(m => m.size)) { tlNoData(el, 'No photo of someone with a birthday has a date.'); return; }
    const c = chartColors();
    const ages = Array.from({ length: oldest + 1 }, (_, i) => i);
    const periods = ages.map(age => ageText(age, { short: true }));
    const series = shown.map(i => ({
        name: ps.people[i].name,
        values: ages.map(age => byAge.get(i).get(age) ?? 0),
        color: i < 8 ? c.cats[i] : c.axis,
    }));
    renderSeriesLines(el, periods, series, {
        yFormat: d => d.toLocaleString(),
        format: v => photoCount(v),
        onPick: onPick && (({ period, series: s }) => s && onPick({ person: s.name, age: ages[periods.indexOf(period)], birth: births[ps.people.findIndex(p => p.name === s.name)] })),
    });
    el.insertAdjacentHTML('beforeend', '<p class="stats-chart-note">Age in whole years, across.</p>');
}

/* ─── Who with whom ────────────────────────────────────────────── */

const PEOPLE_MATRIX = 12;

// Of the people with the most photos, how many photos each two share: one
// cell per pair, darker with more photos.
function renderPeopleMatrix(el, ps, onPick) {
    if (!ps?.people.length) { tlNoData(el, PEOPLE_NOBODY); return; }
    const { pairs } = peopleStatsDerived(ps);
    const shown = ps.people.slice(0, PEOPLE_MATRIX).map((p, i) => ({ ...p, i }));
    const shared = new Map(pairs.filter(p => p.a < shown.length && p.b < shown.length).map(p => [`${p.a},${p.b}`, p.photos]));
    if (!shared.size) { tlNoData(el, 'No two named people are in the same photo here.'); return; }
    const c = chartColors();
    const most = Math.max(...shared.values());
    const cell = 28;
    const m = { top: 8, right: 16, bottom: 110, left: 150 };
    const iW = (shown.length - 1) * cell, iH = (shown.length - 1) * cell;
    const svg = svgBase(el, m.left + iW + m.right, m.top + iH + m.bottom);
    const g = svg.append('g').attr('transform', `translate(${m.left},${m.top})`);
    const tooltip = d3.select(el).append('div').attr('class', 'stats-tooltip').style('display', 'none');
    // Rows from the second person, columns to the second last: each pair once.
    shown.slice(1).forEach((row, ri) => {
        g.append('text').attr('x', -8).attr('y', ri * cell + cell * 0.62)
            .attr('text-anchor', 'end').attr('fill', c.text).attr('font-size', 11).text(truncate(row.name, 22));
    });
    shown.slice(0, -1).forEach((col, ci) => {
        g.append('text')
            .attr('transform', `translate(${ci * cell + cell / 2},${iH + 8}) rotate(-45)`)
            .attr('text-anchor', 'end').attr('fill', c.text).attr('font-size', 11).text(truncate(col.name, 18));
    });
    shown.slice(1).forEach((row, ri) => {
        shown.slice(0, ri + 1).forEach((col, ci) => {
            const photos = shared.get(`${col.i},${row.i}`) ?? 0;
            const rect = g.append('rect')
                .datum({ a: col.name, b: row.name })
                .attr('x', ci * cell).attr('y', ri * cell)
                .attr('width', cell - 2).attr('height', cell - 2).attr('rx', 2)
                .attr('fill', photos ? seqStep(c, photos, most) : c.border)
                .attr('opacity', photos ? 1 : 0.3)
                .on('mouseover', () => {
                    tooltip.style('display', 'block').text(`${col.name} and ${row.name}: ${photoCount(photos)}`);
                })
                .on('mousemove', (event) => {
                    const [mx, my] = d3.pointer(event, el);
                    tooltip.style('left', `${mx + 12}px`).style('top', `${my - 28}px`);
                })
                .on('mouseout', () => tooltip.style('display', 'none'));
            if (photos) pickable(rect, onPick, `${col.name} and ${row.name}, ${photoCount(photos)}`);
        });
    });
}

/* ─── Alone or in a group ──────────────────────────────────────── */

function renderCrowdBars(el, ps, onPick) {
    const crowd = ps?.crowd;
    if (!crowd?.photos) { tlNoData(el, 'No photo here has been looked at for faces yet.'); return; }
    const c = chartColors();
    // Half a row wide, so drawn smaller than the full-width charts.
    const W = 400, barH = 26, pad = { top: 8, left: 90, right: 80, bottom: 8 };
    const svg = svgBase(el, W, pad.top + crowd.buckets.length * barH + pad.bottom);
    const x = d3.scaleLinear().domain([0, Math.max(...crowd.total)]).range([0, W - pad.left - pad.right]);
    crowd.buckets.forEach((b, i) => {
        const y = pad.top + i * barH;
        const n = crowd.total[i];
        svg.append('text')
            .attr('x', pad.left - 8).attr('y', y + barH / 2 + 4)
            .attr('text-anchor', 'end').attr('fill', c.text).attr('font-size', 11).text(b.label);
        const bar = svg.append('rect').datum(b)
            .attr('x', pad.left).attr('y', y + 4)
            .attr('width', Math.max(n ? 1 : 0, x(n))).attr('height', barH - 8)
            .attr('rx', 2).attr('fill', c.cats[0]);
        if (n) pickable(bar, onPick, `${b.label}, ${photoCount(n)}`);
        svg.append('text')
            .attr('x', pad.left + x(n) + 5).attr('y', y + barH / 2 + 4)
            .attr('fill', c.textSec).attr('font-size', 10)
            .text(`${n.toLocaleString()} · ${Math.round(100 * n / crowd.photos)} %`);
    });
}

// The share of each number of faces per period, so a period with few
// photos counts as much as one with many; a period without photos is a gap.
function renderCrowdLines(el, ps, onPick) {
    const crowd = ps?.crowd;
    if (!crowd?.periods.length) { tlNoData(el, 'No photo looked at for faces has a date.'); return; }
    const c = chartColors();
    const all = periodRange(crowd.periods[0], crowd.periods[crowd.periods.length - 1], ps.granularity) ?? crowd.periods;
    const at = new Map(crowd.periods.map((p, i) => [p, crowd.counts[i]]));
    const totals = all.map(p => (at.get(p) ?? []).reduce((s, n) => s + n, 0));
    const series = crowd.buckets.map((b, bi) => ({
        name: b.label,
        bucket: b,
        counts: all.map(p => at.get(p)?.[bi] ?? 0),
        values: all.map((p, i) => (totals[i] ? 100 * (at.get(p)?.[bi] ?? 0) / totals[i] : null)),
        color: c.cats[bi],
    }));
    renderSeriesLines(el, all, series, {
        yMax: 100,
        yFormat: d => `${d} %`,
        format: (v, s, i) => `${Math.round(v)} % (${s.counts[i].toLocaleString()})`,
        onPick: onPick && (({ period, series: s }) => onPick({ period, bucket: s?.bucket ?? null })),
        height: 180,
    });
}

/* ─── Criteria ─────────────────────────────────────────────────── */

function personCriterion(name) {
    return { subject: name, params: { person: name } };
}

function crowdParams(b) {
    return b.max < 0 ? { faces_min: b.min } : { faces_min: b.min, faces_max: b.max };
}
