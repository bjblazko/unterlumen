// stats-week-hours.js — Weekday and hour, in the Time topic: photos per
// weekday (Monday first) and hour of the day, as one grid of 7 × 24 cells in
// the sequential ramp (ADR-0034). Flat on purpose: as 3D columns the cells
// would hide each other. A cell shows its photos.

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

function renderWeekHours(el, weekHours, onPick) {
    const max = d3.max((weekHours || []).flat()) || 0;
    if (!max) { el.textContent = t('No data'); return; }
    const c = chartColors();
    const left = 36, top = 18, cellW = 26, cellH = 22, gap = 2;
    const W = left + 24 * cellW + 4, H = top + 7 * cellH + 4;
    const svg = svgBase(el, W, H);
    for (let h = 0; h < 24; h += 3) {
        svg.append('text').attr('x', left + h * cellW).attr('y', top - 6)
            .attr('fill', c.textSec).attr('font-size', 10).attr('font-family', 'var(--font-mono)')
            .text(String(h).padStart(2, '0'));
    }
    weekHours.forEach((hours, wd) => {
        svg.append('text').attr('x', left - 6).attr('y', top + wd * cellH + cellH / 2 + 4)
            .attr('text-anchor', 'end').attr('fill', c.textSec).attr('font-size', 10)
            .text(t(WEEKDAYS[wd]));
        hours.forEach((n, hr) => {
            const cell = svg.append('rect').datum({ wd, hr })
                .attr('x', left + hr * cellW).attr('y', top + wd * cellH)
                .attr('width', cellW - gap).attr('height', cellH - gap).attr('rx', 2)
                .attr('fill', n ? seqStep(c, n, max) : c.border)
                .attr('opacity', n ? 1 : 0.4);
            if (!n) return;
            const label = `${t(WEEKDAYS[wd])} ${weekHourSpan(hr)}, ${photoCount(n)}`;
            cell.append('title').text(label);
            pickable(cell, onPick, label);
        });
    });
}

// weekHourSpan is "10:00–11:00".
function weekHourSpan(hr) {
    const hh = h => String(h).padStart(2, '0') + ':00';
    return `${hh(hr)}–${hh(hr + 1)}`;
}
