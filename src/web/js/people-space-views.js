// people-space-views.js — the People topic's views on the point stage
// (ADR-0046, ADR-0051): the Face space, the Circle of people and Life lines.
// They read /api/library/people-stats. A person keeps their slot of the
// chart ramp in all three, by their rank in photos; those beyond the eighth
// share a light neutral, faces nobody named a dim grey.

const PEOPLE_OTHER_RGB = [0.72, 0.74, 0.78];
const PEOPLE_UNNAMED_RGB = [0.42, 0.44, 0.48];

function peopleStageColours(stageEl, count) {
    const style = getComputedStyle(stageEl);
    return Array.from({ length: count }, (_, i) => (i < 8
        ? cssColourToRGB(style.getPropertyValue(`--chart-${i + 1}`).trim() || '#888')
        : PEOPLE_OTHER_RGB));
}

function peopleLegendHTML(ps, colours, extra = '') {
    const named = ps.people.slice(0, 8).map((p, i) => {
        const [r, g, b] = colours[i].map(c => Math.round(c * 255));
        return `<span class="point-stage-legend-item"><span class="point-stage-legend-swatch" style="background:rgb(${r} ${g} ${b})" aria-hidden="true"></span>${escapeHtml(p.name)}</span>`;
    }).join('');
    const swatch = (rgb, text) => `<span class="point-stage-legend-item"><span class="point-stage-legend-swatch" style="background:rgb(${rgb.map(c => Math.round(c * 255)).join(' ')})" aria-hidden="true"></span>${text}</span>`;
    return named + (ps.people.length > 8 ? swatch(PEOPLE_OTHER_RGB, 'Everyone else') : '') + extra;
}

// A small, fixed offset per photo, so lights that share a place spread out
// and stay where they are from one visit to the next.
function peopleJitter(key, axis, size) {
    let h = 2166136261 ^ axis;
    for (let i = 0; i < key.length; i++) h = Math.imul(h ^ key.charCodeAt(i), 16777619);
    return ((h >>> 0) / 4294967295 - 0.5) * size;
}

function peopleNoStage(el, ps) {
    if (ps?.people.length) return false;
    tlNoData(el, PEOPLE_NOBODY);
    return true;
}

/* --- Face space --- */

// renderFaceSpace draws every face where what it looks like puts it.
// onPick gets { photo } or { person }.
function renderFaceSpace(el, ps, onPick, options) {
    if (!ps?.faceSpace?.face.length) { tlNoData(el, 'No faces were found in these photos yet.'); return; }
    renderPointStage(el, stageEl => faceSpaceSpec(ps, peopleStageColours(stageEl, ps.people.length)), onPick, options);
}

function faceSpaceSpec(ps, colours) {
    const f = ps.faceSpace;
    const n = f.face.length;
    const light = pointLight(n);
    const sprites = new Float32Array(n * SPRITE_FLOATS);
    const sums = ps.people.map(() => [0, 0, 0, 0]);
    for (let i = 0; i < n; i++) {
        const p = f.person[i];
        const named = p >= 0;
        sprites.set([f.x[i], f.y[i], f.z[i], ...(named ? colours[p] : PEOPLE_UNNAMED_RGB), light.radius, named ? light.alpha : light.alpha * 0.6], i * SPRITE_FLOATS);
        if (named) {
            const s = sums[p];
            s[0] += f.x[i]; s[1] += f.y[i]; s[2] += f.z[i]; s[3]++;
        }
    }
    const shown = sums.map((s, p) => ({ p, at: s[3] ? [s[0] / s[3], s[1] / s[3], s[2] / s[3]] : null })).filter(c => c.at);
    const most = Math.max(1, ...shown.map(c => ps.people[c.p].photos));
    const steps = new Float32Array(shown.flatMap(c => [...c.at, ...colours[c.p], stepRadius(ps.people[c.p].photos, most), 1]));
    const of = f.faces > n ? ` of ${formatCount(f.faces)}` : '';
    const axes = f.axes === 'people' ? 'the named people differ most' : 'the faces differ most';
    return {
        summary: `${formatCount(n)}${of} faces, each placed by what it looks like: faces close together look alike. The three axes are the directions in which ${axes}; they have no names of their own. Named people are in their colour, faces nobody named are grey.`,
        legend: peopleLegendHTML(ps, colours, `<span class="point-stage-legend-item"><span class="point-stage-legend-swatch" style="background:rgb(107 112 122)" aria-hidden="true"></span>Nobody named</span>`)
            + legendStepHTML('A person: the middle of their faces, larger with more photos.'),
        sprites, steps,
        trail: new Float32Array(0),
        lines: new StageLines().box().toArray(),
        labels: shown.filter(c => c.p < 8).map(c => ({ text: ps.people[c.p].name, pos: [c.at[0], c.at[1] + 0.09, c.at[2]], kind: 'name' })),
        tip: (kind, i) => {
            if (kind === 'step') {
                const person = ps.people[shown[i].p];
                return `<span class="point-stage-tip-title">${escapeHtml(person.name)}</span><span>${photoCount(person.photos)}</span>`;
            }
            const p = f.person[i];
            return photoTipHTML(ps, f.lib[i], f.photo[i], `
                <span class="point-stage-tip-title">${p >= 0 ? escapeHtml(ps.people[p].name) : 'Nobody named'}</span>
                <span>${escapeHtml(f.date[i] ? f.date[i].slice(0, 10) : 'No date')}</span>`);
        },
        pick: (kind, i) => (kind === 'step'
            ? { person: ps.people[shown[i].p].name }
            : { photo: { id: f.photo[i], lib: ps.libraries[f.lib[i]], date: f.date[i] } }),
    };
}

/* --- Circle of people --- */

const CIRCLE_PEOPLE = 30;

// Where each person stands: pulled towards those they share photos with,
// pushed from everyone else, worked out the same way every time.
function circleLayout(count, pairs) {
    const pos = Array.from({ length: count }, (_, i) => {
        const y = count > 1 ? 1 - 2 * i / (count - 1) : 0;
        const r = Math.sqrt(1 - y * y), t = i * 2.399963;
        return [Math.cos(t) * r * 0.8, y * 0.8, Math.sin(t) * r * 0.8];
    });
    const most = Math.max(1, ...pairs.map(p => p.photos));
    for (let step = 0; step < 400; step++) {
        const move = pos.map(() => [0, 0, 0]);
        for (let i = 0; i < count; i++) {
            for (let j = i + 1; j < count; j++) {
                const d = pos[i].map((v, k) => v - pos[j][k]);
                const dist = Math.max(0.05, Math.hypot(...d));
                const push = 0.004 / (dist * dist);
                for (let k = 0; k < 3; k++) { move[i][k] += d[k] / dist * push; move[j][k] -= d[k] / dist * push; }
            }
        }
        for (const { a, b, photos } of pairs) {
            const d = pos[b].map((v, k) => v - pos[a][k]);
            const dist = Math.max(0.01, Math.hypot(...d));
            const want = 0.3 + 0.8 * (1 - Math.sqrt(photos / most));
            const pull = 0.05 * (dist - want);
            for (let k = 0; k < 3; k++) { move[a][k] += d[k] / dist * pull; move[b][k] -= d[k] / dist * pull; }
        }
        pos.forEach((p, i) => { for (let k = 0; k < 3; k++) p[k] += move[i][k] - 0.01 * p[k]; });
    }
    const reach = Math.max(1e-6, ...pos.flat().map(Math.abs));
    return pos.map(p => p.map(v => v / reach));
}

// renderPeopleCircle draws the people with the most photos as large lights,
// their photos around them, and a line between two people for the photos
// they share. onPick gets { photo }, { person } or { pair }.
function renderPeopleCircle(el, ps, onPick, options) {
    if (peopleNoStage(el, ps)) return;
    renderPointStage(el, stageEl => peopleCircleSpec(ps, peopleStageColours(stageEl, ps.people.length)), onPick, options);
}

function peopleCircleSpec(ps, colours) {
    const count = Math.min(CIRCLE_PEOPLE, ps.people.length);
    const pairs = peopleStatsDerived(ps).pairs.filter(p => p.a < count && p.b < count);
    const pos = circleLayout(count, pairs);
    const a = ps.appearances;
    const photos = [];
    a.person.forEach((p, i) => { if (p < count) photos.push(i); });
    const light = pointLight(photos.length);
    const most = Math.max(1, ps.people[0].photos);
    const sprites = new Float32Array(photos.length * SPRITE_FLOATS);
    photos.forEach((i, n) => {
        const p = a.person[i];
        const key = `${a.lib[i]}/${a.photo[i]}`;
        const spread = 0.05 + 0.12 * Math.sqrt(ps.people[p].photos / most);
        const at = pos[p].map((v, k) => v + peopleJitter(key, k, spread * 2));
        sprites.set([...at, ...colours[p], light.radius * 0.8, light.alpha], n * SPRITE_FLOATS);
    });
    const mostShared = Math.max(1, ...pairs.map(p => p.photos));
    const people = pos.map((at, p) => [...at, ...colours[p], stepRadius(ps.people[p].photos, most), 1]);
    const links = pairs.map(({ a: x, b: y, photos: n }) => [
        ...pos[x].map((v, k) => (v + pos[y][k]) / 2), 0.9, 0.9, 0.86, 2 + 4 * Math.sqrt(n / mostShared), 0.7]);
    const steps = new Float32Array([...people, ...links].flat());
    const lines = new StageLines();
    for (const { a: x, b: y, photos: n } of pairs) lines.line(pos[x], pos[y], 0.08 + 0.5 * Math.sqrt(n / mostShared));
    return {
        summary: `The ${formatCount(count)} people with the most photos, each a large light with their photos around it. People who share photos are drawn together and joined by a line, brighter with more shared photos; where they stand otherwise means nothing.`,
        legend: peopleLegendHTML(ps, colours)
            + legendStepHTML('A person, larger with more photos; a small light on a line: the photos two people share.'),
        sprites, steps,
        distance: 4.4,
        trail: new Float32Array(0),
        lines: lines.toArray(),
        labels: pos.slice(0, 12).map((at, p) => ({ text: ps.people[p].name, pos: [at[0], at[1] + 0.1, at[2]], kind: 'name' })),
        tip: (kind, i) => {
            if (kind === 'step') {
                if (i < count) return `<span class="point-stage-tip-title">${escapeHtml(ps.people[i].name)}</span><span>${photoCount(ps.people[i].photos)}</span>`;
                const pair = pairs[i - count];
                return `<span class="point-stage-tip-title">${escapeHtml(ps.people[pair.a].name)} and ${escapeHtml(ps.people[pair.b].name)}</span><span>${photoCount(pair.photos)} together</span>`;
            }
            const j = photos[i];
            return photoTipHTML(ps, a.lib[j], a.photo[j], `
                <span class="point-stage-tip-title">${escapeHtml(ps.people[a.person[j]].name)}</span>
                <span>${escapeHtml(a.date[j] ? a.date[j].slice(0, 10) : 'No date')}</span>`);
        },
        pick: (kind, i) => {
            if (kind === 'step') {
                if (i < count) return { person: ps.people[i].name };
                const pair = pairs[i - count];
                return { pair: [ps.people[pair.a].name, ps.people[pair.b].name] };
            }
            const j = photos[i];
            return { photo: { id: a.photo[j], lib: ps.libraries[a.lib[j]], date: a.date[j] } };
        },
    };
}

/* --- Life lines --- */

const LIFE_LANES = 8;
const LIFE_FLOOR = -0.6; // the floor sits high, so the front lanes stay above the legend

// renderLifeLines draws time across and a lane for each of the people with
// the most photos: their photos on the floor of their lane, and a line over
// it as high as their photos per period. onPick gets { photo } or
// { person, period }.
function renderLifeLines(el, ps, onPick, options) {
    if (peopleNoStage(el, ps)) return;
    if (!peopleStatsDerived(ps).periods.length) { tlNoData(el, 'No photo of a named person has a date.'); return; }
    renderPointStage(el, stageEl => lifeLinesSpec(ps, peopleStageColours(stageEl, ps.people.length)), onPick, options);
}

function lifeLinesSpec(ps, colours) {
    const { periods, counts } = peopleStatsDerived(ps);
    const lanes = Math.min(LIFE_LANES, ps.people.length);
    const at = new Map(periods.map((p, i) => [p, i]));
    const n = ps.granularity === 'year' ? 4 : 7;
    const width = periods.length > 1 ? 2 / (periods.length - 1) : 0;
    const x = i => (periods.length > 1 ? -1 + i * width : 0);
    const z = lane => (lanes > 1 ? -1 + 1.8 * lane / (lanes - 1) : 0);
    const most = Math.max(1, ...counts.slice(0, lanes).flatMap(m => [...m.values()]));
    const y = count => LIFE_FLOOR + 1.6 * count / most;

    const a = ps.appearances;
    const photos = [];
    a.person.forEach((p, i) => { if (p < lanes && a.date[i].length >= n) photos.push(i); });
    const light = pointLight(photos.length);
    const sprites = new Float32Array(photos.length * SPRITE_FLOATS);
    photos.forEach((i, k) => {
        const p = a.person[i];
        const key = `${a.lib[i]}/${a.photo[i]}`;
        sprites.set([
            x(at.get(a.date[i].slice(0, n))) + peopleJitter(key, 0, width * 0.8),
            LIFE_FLOOR + Math.abs(peopleJitter(key, 1, 0.08)),
            z(p) + peopleJitter(key, 2, 0.12),
            ...colours[p], light.radius * 0.8, light.alpha,
        ], k * SPRITE_FLOATS);
    });

    const stepMeta = [];
    const steps = [];
    const trail = [];
    for (let p = 0; p < lanes; p++) {
        periods.forEach((period, i) => {
            const c = counts[p].get(period) ?? 0;
            if (c) {
                stepMeta.push({ p, period, photos: c });
                steps.push(x(i), y(c), z(p), ...colours[p], 3 + 6 * Math.sqrt(c / most), 1);
            }
            if (i + 1 < periods.length) {
                const next = counts[p].get(periods[i + 1]) ?? 0;
                trail.push(x(i), y(c), z(p), ...colours[p], x(i + 1), y(next), z(p), ...colours[p]);
            }
        });
    }
    const lines = new StageLines();
    for (let p = 0; p < lanes; p++) lines.line([-1, LIFE_FLOOR, z(p)], [1, LIFE_FLOOR, z(p)], 0.18);
    const labels = ps.people.slice(0, lanes).map((person, p) => ({ text: person.name, pos: [-1.12, LIFE_FLOOR, z(p)], kind: 'name' }));
    const every = Math.max(1, Math.ceil(periods.length / 8));
    periods.forEach((period, i) => {
        if (i % every === 0) labels.push({ text: period, pos: [x(i), LIFE_FLOOR, z(0) - 0.2], kind: 'axis' });
    });
    const unit = periodUnit(ps.granularity);
    return {
        summary: `Time across, from ${periods[0]} to ${periods[periods.length - 1]}, and a lane for each of the ${formatCount(lanes)} people with the most photos: their photos on the floor of their lane, and a line above it as high as their photos in each ${unit}, up to ${formatCount(most)}.`,
        legend: peopleLegendHTML({ people: ps.people.slice(0, lanes) }, colours)
            + legendStepHTML(`A person's photos in one ${unit}, higher and larger with more.`),
        sprites,
        steps: new Float32Array(steps),
        // From the front and a little above: time runs left to right and
        // the lanes go back from the people with the most photos.
        distance: 5, yaw: 0.22, pitch: 0.42,
        trailWidth: 2,
        trail: new Float32Array(trail),
        lines: lines.toArray(),
        labels,
        tip: (kind, i) => {
            if (kind === 'step') {
                const s = stepMeta[i];
                return `<span class="point-stage-tip-title">${escapeHtml(ps.people[s.p].name)} · ${escapeHtml(s.period)}</span><span>${photoCount(s.photos)}</span>`;
            }
            const j = photos[i];
            const age = ageAt(birthdayOf(ps.birthdays ?? {}, ps.people[a.person[j]].name), a.date[j]);
            return photoTipHTML(ps, a.lib[j], a.photo[j], `
                <span class="point-stage-tip-title">${escapeHtml(ps.people[a.person[j]].name)}</span>
                <span>${escapeHtml(a.date[j].slice(0, 10))}${age !== null ? ` · ${age} years old` : ''}</span>`);
        },
        pick: (kind, i) => {
            if (kind === 'step') return { person: ps.people[stepMeta[i].p].name, period: stepMeta[i].period };
            const j = photos[i];
            return { photo: { id: a.photo[j], lib: ps.libraries[a.lib[j]], date: a.date[j] } };
        },
    };
}
