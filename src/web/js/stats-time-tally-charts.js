// stats-time-tally-charts.js — the Time views beside the snapshot's: year
// against year (from the days of the snapshot) and sessions (the "time"
// tallies, library/insights_time.go).

// A line per year — the eight latest — through the twelve months.
function renderYearAgainstYear(el, shootingDays, onPick) {
    const byYearMonth = new Map();
    for (const [day, n] of Object.entries(shootingDays || {})) {
        const k = day.slice(0, 7);
        byYearMonth.set(k, (byYearMonth.get(k) || 0) + n);
    }
    const years = [...new Set([...byYearMonth.keys()].map(k => k.slice(0, 4)))].sort().slice(-8);
    if (years.length < 2) { tlNoData(el, 'Photos of two years are needed to compare them.'); return; }
    const c = chartColors();
    const months = MONTHS.map(m => t(m).slice(0, 3));
    // A month without photos is 0, not a gap; only months still to come are.
    const now = new Date().toISOString().slice(0, 7);
    const series = years.map((y, i) => ({ name: y, color: c.cats[i],
        values: months.map((_, mi) => {
            const k = `${y}-${String(mi + 1).padStart(2, '0')}`;
            return k > now ? null : (byYearMonth.get(k) || 0);
        }) }));
    renderSeriesLines(el, months, series, {
        format: v => photoCount(v),
        onPick: v => onPick?.({ month: months.indexOf(v.period) + 1, year: v.series?.name }),
    });
}

function renderSessionSizes(el, time, onPick) {
    const m = sumBy(tallyRows(time?.sessionSizes), p => p[0]);
    const bins = ['1', '2–5', '6–20', '21–50', '51–200', '201+'].map(label => ({ label, count: m.get(label) || 0 }));
    renderValueHistogram(el, bins, null, { half: true });
}

// The largest sessions, by their first moment; a bar shows its photos.
function renderLargestSessions(el, time, onPick) {
    const items = tallyRows(time?.sessions).map(r => ({ first: r.parts[0], last: r.parts[1], count: r.n,
        label: sessionLabel(r.parts[0], r.parts[1]) })).sort((a, b) => b.count - a.count);
    renderCountBars(el, items, onPick, { max: 8 });
}

function sessionLabel(first, last) {
    const day = first.slice(0, 10);
    return last.slice(0, 10) === day ? `${day} ${first.slice(11, 16)}–${last.slice(11, 16)}` : `${day} – ${last.slice(0, 10)}`;
}
