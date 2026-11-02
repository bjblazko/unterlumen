// stats-places-charts.js — Countries and Cities, in the Places topic: photos
// per value of the IPTC fields (ADR-0061), as bars. A bar shows its photos.
// Without any value the chart says where they come from.

const PLACE_BARS = 12;

function renderPlaceBars(el, items, onPick) {
    const data = (items || []).slice(0, PLACE_BARS);
    if (!data.length) {
        el.innerHTML = `<p class="stats-nodata">${t('No photo has one yet. Fill it in {metadata} — for many photos at once from their GPS position.', { metadata: placeLink('metadata', 'metadata', t('Metadata')) })}</p>`;
        return;
    }
    const c = chartColors();
    // Drawn for a half-width card, beside its sibling.
    const W = 340, barH = 22, pad = { top: 8, left: 110, right: 40, bottom: 8 };
    const svg = svgBase(el, W, pad.top + data.length * barH + pad.bottom);
    const x = d3.scaleLinear().domain([0, d3.max(data, d => d.count)]).range([0, W - pad.left - pad.right]);
    data.forEach((d, i) => {
        const y = pad.top + i * barH;
        svg.append('text').attr('x', pad.left - 8).attr('y', y + barH / 2 + 4)
            .attr('text-anchor', 'end').attr('fill', c.text).attr('font-size', 11)
            .text(truncate(d.name, 16));
        pickable(svg.append('rect').datum(d.name)
            .attr('x', pad.left).attr('y', y + 3)
            .attr('width', Math.max(1, x(d.count))).attr('height', barH - 6)
            .attr('rx', 2).attr('fill', c.cats[0]), onPick, `${d.name}, ${photoCount(d.count)}`);
        svg.append('text').attr('x', pad.left + x(d.count) + 5).attr('y', y + barH / 2 + 4)
            .attr('fill', c.textSec).attr('font-size', 10).text(formatCount(d.count));
    });
    if (items.length > PLACE_BARS) {
        el.insertAdjacentHTML('beforeend', `<p class="stats-chart-note">${t('The {n} with the most photos of {total}.', { n: PLACE_BARS, total: formatCount(items.length) })}</p>`);
    }
}

/* --- Journeys (the "places" tallies) --- */

// A day counts as away when the middle of its photos lies this far from
// home, which is the tenth-of-a-degree cell with the most photos; away days
// with at most JOURNEY_GAP days between them are one journey.
const JOURNEY_KM = 100;
const JOURNEY_GAP = 2;

function findJourneys(places) {
    const rows = tallyRows(places?.days);
    if (!rows.length) return [];
    const cells = sumBy(rows, p => p[1] + ',' + p[2]);
    const [homeKey] = [...cells.entries()].sort((a, b) => b[1] - a[1])[0];
    const home = homeKey.split(',').map(v => Number(v) / 10);
    const days = new Map();
    for (const r of rows) {
        const d = days.get(r.parts[0]) || { day: r.parts[0], n: 0, lat: 0, lon: 0 };
        d.n += r.n;
        d.lat += r.n * Number(r.parts[1]) / 10;
        d.lon += r.n * Number(r.parts[2]) / 10;
        days.set(r.parts[0], d);
    }
    const away = [...days.values()].map(d => ({ ...d, km: kmBetween(home, [d.lat / d.n, d.lon / d.n]) }))
        .filter(d => d.km > JOURNEY_KM).sort((a, b) => (a.day < b.day ? -1 : 1));
    const countries = tallyRows(places?.countryDays);
    const journeys = [];
    for (const d of away) {
        const last = journeys[journeys.length - 1];
        if (last && daysBetween(last.end, d.day) <= JOURNEY_GAP + 1) {
            last.end = d.day; last.photos += d.n; last.km = Math.max(last.km, d.km);
        } else {
            journeys.push({ start: d.day, end: d.day, photos: d.n, km: d.km });
        }
    }
    for (const j of journeys) {
        const named = sumBy(countries.filter(r => r.parts[0] >= j.start && r.parts[0] <= j.end), p => p[1]);
        j.country = [...named.entries()].sort((a, b) => b[1] - a[1])[0]?.[0] || '';
    }
    return journeys;
}

function kmBetween([lat1, lon1], [lat2, lon2]) {
    const r = Math.PI / 180;
    const a = Math.sin((lat2 - lat1) * r / 2) ** 2 + Math.cos(lat1 * r) * Math.cos(lat2 * r) * Math.sin((lon2 - lon1) * r / 2) ** 2;
    return 12742 * Math.asin(Math.sqrt(a));
}

function daysBetween(a, b) {
    return Math.round((Date.parse(b) - Date.parse(a)) / 86400000);
}

// A row per year, the journeys as bars across its days.
function renderJourneys(el, places, onPick) {
    const journeys = findJourneys(places);
    if (!journeys.length) { tlNoData(el, 'No day far enough from home yet — photos need a GPS position.'); return; }
    const c = chartColors();
    const years = [...new Set(journeys.map(j => j.start.slice(0, 4)))].sort().reverse();
    const left = 48, top = 20, rowH = 22, W = 680, dayW = (W - left - 8) / 366;
    const svg = svgBase(el, W, top + years.length * rowH + 4);
    MONTHS.forEach((m, i) => {
        const x = left + daysBetween('2024-01-01', `2024-${String(i + 1).padStart(2, '0')}-01`) * dayW;
        svg.append('text').attr('x', x).attr('y', top - 6).attr('fill', c.textSec).attr('font-size', 9).text(t(m).slice(0, 3));
    });
    years.forEach((y, i) => {
        const ry = top + i * rowH;
        svg.append('text').attr('x', left - 6).attr('y', ry + rowH / 2 + 4).attr('text-anchor', 'end')
            .attr('fill', c.text).attr('font-size', 11).attr('font-family', 'var(--font-mono)').text(y);
        svg.append('rect').attr('x', left).attr('y', ry + 9).attr('width', 366 * dayW).attr('height', 2).attr('fill', c.border);
        for (const j of journeys.filter(j => j.start.startsWith(y))) {
            const x = left + daysBetween(`${y}-01-01`, j.start) * dayW;
            const w = Math.max(3, (daysBetween(j.start, j.end) + 1) * dayW);
            const label = `${j.start === j.end ? j.start : `${j.start} – ${j.end}`}${j.country ? ' · ' + j.country : ''}, ${photoCount(j.photos)}, ${t('up to {km} km from home', { km: formatCount(Math.round(j.km)) })}`;
            const bar = svg.append('rect').datum(j).attr('x', x).attr('y', ry + 3).attr('width', w).attr('height', rowH - 6).attr('rx', 2).attr('fill', c.cats[0]);
            bar.append('title').text(label);
            pickable(bar, onPick, label);
        }
    });
}
