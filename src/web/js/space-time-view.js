// space-time-view.js — the Space and time topic (ADR-0046): every located
// photo by where it was taken (the floor) and when (up), in its main
// colour. The floor is the world as seen from the middle of the photos:
// direction is true and distance runs on a log scale, so the neighbourhood
// and the far journeys fit on one floor. A trail follows the days in time
// order: home stands as a column of light, journeys reach out from it.
// Drawn on a point stage (point-stage.js) over Natural Earth's coastlines
// and borders (public domain, src/web/data/natural-earth-lines.json).

const EARTH_KM = 6371;
const FARTHEST_KM = 20000; // the other side of the earth

// A day within this distance of the middle is a day at home: its step sits
// on the time axis, so the trail runs up the middle and reaches out only on
// journeys.
const HOME_KM = 30;

let worldLinesPromise = null;

// The coastlines and land borders, decoded once: each line is a list of
// [lon, lat] in degrees. The file holds hundredths of a degree, each point
// after the first as the step from the one before.
function worldLines() {
    if (!worldLinesPromise) {
        worldLinesPromise = fetch('/data/natural-earth-lines.json')
            .then(r => (r.ok ? r.json() : { coast: [], borders: [] }))
            .then(d => {
                const decode = flat => {
                    const pts = [];
                    let lon = 0, lat = 0;
                    for (let i = 0; i < flat.length; i += 2) {
                        lon += flat[i];
                        lat += flat[i + 1];
                        pts.push([lon / 100, lat / 100]);
                    }
                    return pts;
                };
                return { coast: d.coast.map(decode), borders: d.borders.map(decode) };
            })
            .catch(() => ({ coast: [], borders: [] }));
    }
    return worldLinesPromise;
}

/* --- Projection --- */

// The middle of the photos: the median latitude and longitude.
function spaceTimeCentre(st) {
    const mid = vals => {
        const s = [...vals].sort((a, b) => a - b);
        return s[Math.floor(s.length / 2)];
    };
    return { lat: mid(st.points.lat), lon: mid(st.points.lon) };
}

// Distance in km and bearing in radians from the centre, on a sphere.
function distanceAndBearing(centre, lat, lon) {
    const rad = Math.PI / 180;
    const φ1 = centre.lat * rad, φ2 = lat * rad, Δλ = (lon - centre.lon) * rad;
    const cosD = Math.sin(φ1) * Math.sin(φ2) + Math.cos(φ1) * Math.cos(φ2) * Math.cos(Δλ);
    const d = Math.acos(Math.max(-1, Math.min(1, cosD))) * EARTH_KM;
    const bearing = Math.atan2(Math.sin(Δλ) * Math.cos(φ2), Math.cos(φ1) * Math.sin(φ2) - Math.sin(φ1) * Math.cos(φ2) * Math.cos(Δλ));
    return { d, bearing };
}

// Beyond this distance the floor's log scale runs at STRETCH times the
// pace: on a plain log scale a journey of 5,000 km sat barely further out
// than one of 1,500, and the far journeys did not stand out.
const STRETCH_FROM_KM = 1000;
const STRETCH = 2;

function stretchedLog(km) {
    const u = Math.log10(1 + km), knee = Math.log10(1 + STRETCH_FROM_KM);
    return u <= knee ? u : knee + STRETCH * (u - knee);
}

// The radius of a distance on the floor, 0 in the middle, 1 at the other
// side of the earth.
function distanceRadius(km) {
    return stretchedLog(km) / stretchedLog(FARTHEST_KM);
}

// The floor: north away from the start of the view, east to the right, the
// radius growing with the logarithm of the distance, faster beyond 1,000 km.
function floorPoint(centre, lat, lon) {
    const { d, bearing } = distanceAndBearing(centre, lat, lon);
    const r = distanceRadius(d);
    return [r * Math.sin(bearing), r * -Math.cos(bearing), d];
}

// The height of a date taken, -1 at the first photo and 1 at the last.
function timeScale(dates) {
    const ms = dates.map(d => Date.parse(d.length >= 19 ? d.slice(0, 19) + 'Z' : d.slice(0, 10) + 'T12:00:00Z'));
    const lo = Math.min(...ms), hi = Math.max(...ms);
    const span = Math.max(1, hi - lo);
    return { ms, lo, hi, y: t => (t - lo) / span * 2 - 1 };
}

/* --- The stage --- */

// renderSpaceTime draws st (/api/library/space-time). onPick gets
// { photo } or { day } ("2024-05-01").
function renderSpaceTime(el, st, onPick, options) {
    if (!st?.photos) {
        tlNoData(el, 'No photo here has a location and a date.');
        return;
    }
    worldLines().then(world => renderPointStage(el, () => spaceTimeSpec(st, world), onPick, options));
}

function spaceTimeSpec(st, world) {
    const p = st.points;
    const n = p.id.length;
    const centre = spaceTimeCentre(st);
    const time = timeScale(p.date);
    const floor = p.id.map((_, i) => floorPoint(centre, p.lat[i], p.lon[i]));
    const light = pointLight(n);
    const sprites = new Float32Array(n * SPRITE_FLOATS);
    for (let i = 0; i < n; i++) {
        sprites.set([floor[i][0], time.y(time.ms[i]), floor[i][1], ...spaceTimeColour(p, i), light.radius, light.alpha], i * SPRITE_FLOATS);
    }
    const days = spaceTimeDays(st, centre, floor);
    const most = Math.max(1, ...days.map(d => d.photos));
    const steps = new Float32Array(days.flatMap(d => [
        d.x, time.y(Date.parse(d.day + 'T12:00:00Z')), d.z, ...d.colour, d.home ? 2 : 3 + 5 * Math.sqrt(d.photos / most), d.home ? 0.35 : 0.9,
    ]));
    const first = p.date.reduce((a, b) => (a < b ? a : b)).slice(0, 4);
    const last = p.date.reduce((a, b) => (a > b ? a : b)).slice(0, 4);
    return {
        summary: `${formatCount(n)} photos with a place and a date: where they were taken on the floor, around the middle of the photos at ${formatLatLon(centre.lat, centre.lon)} and further out the further away, on a log scale of distance that widens beyond ${formatCount(STRETCH_FROM_KM)} km; and when, upward from ${first} to ${last}. Each is in its main colour.`,
        legend: legendDotHTML('One photo, where and when it was taken')
            + legendStepHTML(`One day away: where most of its photos were taken, larger with more photos. Days within ${HOME_KM} km of the middle sit on the axis. The line follows the days in time order.`),
        sprites, steps,
        distance: 4.2,
        trailWidth: 2,
        trail: trailBetween(steps),
        lines: spaceTimeLines(centre, world),
        labels: spaceTimeLabels(time),
        tip: (kind, i) => spaceTimeTip(st, days, floor, kind, i),
        pick: (kind, i) => kind === 'step'
            ? { day: days[i].day }
            : { photo: { id: p.id[i], lib: st.libraries[p.lib[i]], date: p.date[i] } },
    };
}

// A photo's main colour, or a quiet grey before it is analysed.
function spaceTimeColour(p, i) {
    return p.l[i] < 0 ? [0.55, 0.58, 0.62] : oklabToStageRGB(p.l[i], p.a[i], p.b[i]);
}

// Each day with photos: where most of them were taken (the median of their
// places on the floor), or the time axis for a day at home; their number
// and their mean colour.
function spaceTimeDays(st, centre, floor) {
    const p = st.points;
    const byDay = new Map();
    p.date.forEach((date, i) => {
        const day = date.slice(0, 10);
        if (!byDay.has(day)) byDay.set(day, []);
        byDay.get(day).push(i);
    });
    const mid = vals => vals.sort((a, b) => a - b)[Math.floor(vals.length / 2)];
    return [...byDay.entries()].sort((a, b) => (a[0] < b[0] ? -1 : 1)).map(([day, idx]) => {
        const coloured = idx.filter(i => p.l[i] >= 0);
        const mean = k => coloured.reduce((s, i) => s + p[k][i], 0) / coloured.length;
        const colour = coloured.length ? oklabToStageRGB(mean('l'), mean('a'), mean('b'), { boost: TRAIL_BOOST }) : [0.75, 0.78, 0.82];
        const lat = mid(idx.map(i => p.lat[i])), lon = mid(idx.map(i => p.lon[i]));
        const [x, z, d] = floorPoint(centre, lat, lon);
        const home = d < HOME_KM;
        return { day, photos: idx.length, x: home ? 0 : x, z: home ? 0 : z, d, home, colour };
    });
}

/* --- Floor, labels, tooltip --- */

// The coastlines and borders in the floor's projection, rings at 10, 100,
// 1,000 and 10,000 km, and the time axis standing in the middle. A segment
// that jumps across the floor (near the other side of the earth) is left out.
function spaceTimeLines(centre, world) {
    const lines = new StageLines();
    const y = -1.02;
    const draw = (list, alpha) => {
        for (const line of list) {
            let prev = null;
            for (const [lon, lat] of line) {
                const [x, z] = floorPoint(centre, lat, lon);
                if (prev && Math.hypot(x - prev[0], z - prev[1]) < 0.25) lines.line([prev[0], y, prev[1]], [x, y, z], alpha);
                prev = [x, z];
            }
        }
    };
    draw(world.coast, 0.32);
    draw(world.borders, 0.14);
    for (const km of [10, 100, 1000, 10000]) lines.ring(distanceRadius(km), y, 0.1);
    lines.line([0, y, 0], [0, 1, 0], 0.3);
    return lines.toArray();
}

function spaceTimeLabels(time) {
    const labels = [];
    for (const km of [10, 100, 1000, 10000]) {
        const r = distanceRadius(km);
        labels.push({ text: `${formatCount(km)} km`, pos: [r * Math.SQRT1_2, -1.02, r * Math.SQRT1_2], kind: 'tick' });
    }
    labels.push({ text: 'North', pos: [0, -1.02, -1.05], kind: 'name' });
    const first = new Date(time.lo).getUTCFullYear(), last = new Date(time.hi).getUTCFullYear();
    const step = [1, 2, 5, 10, 20].find(s => (last - first) / s <= 8) ?? 25;
    for (let year = Math.ceil(first / step) * step; year <= last; year += step) {
        labels.push({ text: String(year), pos: [0, time.y(Date.UTC(year, 0, 1)), 0], kind: 'axis' });
    }
    return labels;
}

function formatLatLon(lat, lon) {
    return `${Math.abs(lat).toFixed(2)}° ${lat < 0 ? 'S' : 'N'}, ${Math.abs(lon).toFixed(2)}° ${lon < 0 ? 'W' : 'E'}`;
}

function formatDistance(km) {
    return km < 1 ? 'in the middle' : `${formatCount(Math.round(km))} km from the middle`;
}

function spaceTimeTip(st, days, floor, kind, i) {
    if (kind === 'step') {
        const d = days[i];
        return `<span class="point-stage-tip-title">${escapeHtml(d.day)}</span>
            <span>${formatCount(d.photos)} ${d.photos === 1 ? 'photo' : 'photos'}</span>
            <span class="point-stage-tip-values">${formatDistance(d.d)}</span>`;
    }
    const p = st.points;
    return photoTipHTML(st, p.lib[i], p.id[i], `
        <span class="point-stage-tip-title">${escapeHtml(p.date[i].slice(0, 16).replace('T', ' '))}</span>
        <span class="point-stage-tip-values">${formatLatLon(p.lat[i], p.lon[i])}</span>
        <span>${formatDistance(floor[i][2])}</span>`);
}
