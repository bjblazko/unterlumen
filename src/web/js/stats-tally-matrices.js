// stats-tally-matrices.js — the People and Things views drawn from the
// "people" and "things" tallies (library/insights_people.go): who has not
// been seen for a while, who where, who with what, things through the year
// and things by place. The matrices take the rows and columns with the most
// photos.

function topParts(rows, i, n) {
    return [...sumBy(rows, p => p[i]).entries()].sort((a, b) => b[1] - a[1]).slice(0, n).map(([k]) => k);
}

function renderPairMatrix(el, tally, onPick, { rows = 12, cols = 12, colLabel, rowLabel, empty } = {}) {
    const data = tallyRows(tally);
    const cell = sumBy(data, p => p[0] + TALLY_SEP + p[1]);
    renderTallyMatrix(el, topParts(data, 0, rows), topParts(data, 1, cols),
        (r, c) => cell.get(r + TALLY_SEP + c) || 0, onPick, { colLabel, rowLabel, empty });
}

// Months since each named person's last photo, the longest first.
function renderNotSeen(el, people, onPick) {
    const last = new Map();
    for (const r of tallyRows(people?.personMonth)) {
        const [name, month] = r.parts;
        if (!last.has(name) || last.get(name) < month) last.set(name, month);
    }
    const now = new Date();
    const items = [...last.entries()].map(([name, month]) => {
        const [y, m] = month.split('-').map(Number);
        return { name, month, count: Math.max(0, (now.getFullYear() - y) * 12 + now.getMonth() + 1 - m) };
    }).sort((a, b) => b.count - a.count);
    renderCountBars(el, items, onPick, {
        empty: 'No person is named yet.',
        valueText: d => t('{n} months', { n: formatCount(d.count) }),
        aria: d => t('last photo {month}, {n} months ago', { month: d.month, n: formatCount(d.count) }),
    });
}

function renderWhoWhere(el, people, onPick) {
    renderPairMatrix(el, people?.personCountry, onPick, { empty: 'No photo of a named person names a country yet.' });
}

function renderWhoWithWhat(el, people, onPick) {
    renderPairMatrix(el, people?.personThing, onPick, { colLabel: th => ThingWords.name(th), empty: 'No photo of a named person has things found yet.' });
}

function renderThingsThroughTheYear(el, things, onPick) {
    const data = tallyRows(things?.thingMonth);
    const cell = sumBy(data, p => p[0] + TALLY_SEP + p[1]);
    const months = Array.from({ length: 12 }, (_, i) => String(i + 1));
    renderTallyMatrix(el, topParts(data, 0, 12), months, (r, m) => cell.get(r + TALLY_SEP + m) || 0, onPick,
        { rowLabel: th => ThingWords.name(th), colLabel: m => t(MONTHS[Number(m) - 1]).slice(0, 3), empty: 'No things found yet.' });
}

function renderThingsByPlace(el, things, onPick) {
    renderPairMatrix(el, things?.thingCountry, onPick, { rowLabel: th => ThingWords.name(th), empty: 'No photo with things found names a country yet.' });
}
