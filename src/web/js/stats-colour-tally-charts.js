// stats-colour-tally-charts.js — the Colour views drawn from the "colour"
// tallies (library/insights_colour.go): how colourful each film simulation
// (or camera) makes the photos, and each main colour across the hours.

function renderColourfulness(el, colour, onPick) {
    const film = tallyRows(colour?.colourfulFilm);
    const rows = film.length ? film : tallyRows(colour?.colourfulCamera);
    const field = film.length ? 'FilmSimulation' : 'Model';
    const names = topParts(rows, 0, 10);
    const groups = names.map(name => ({ name, field, cells: sumBy(rows.filter(r => r.parts[0] === name), p => Number(p[1])) }));
    const values = [];
    for (let v = 0; v <= 120; v += 5) values.push(v);
    renderValueStrips(el, groups, values, v => onPick?.(v), { valueLabel: v => String(v), every: 4 });
}

function renderHueHours(el, colour, onPick) {
    const data = tallyRows(colour?.hueHour);
    const cell = sumBy(data, p => p[1] + TALLY_SEP + p[0]);
    const hues = Array.from({ length: 12 }, (_, i) => i).filter(h => data.some(r => r.parts[1] === String(h)));
    const hours = Array.from({ length: 24 }, (_, i) => i);
    renderTallyMatrix(el, hues, hours, (h, hr) => cell.get(h + TALLY_SEP + hr) || 0, onPick,
        { rowLabel: h => capitalise(hueName(h)), colLabel: hr => String(hr).padStart(2, '0'), empty: 'No colour photos with a time of day yet.' });
}
