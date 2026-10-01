// stats-topics.js — what the Statistics place shows, by topic (ADR-0043).
// The place, its sidebar entries and the overview's cards are all built from
// this list, so a new topic is one entry here and its charts. A card shows
// its topic's first charts small (overviewCharts in stats-place.js).
//
// A chart reads the snapshot (`snap`, /api/library/statistics) or the
// development over time (`tl`, /api/library/timeline). A chart reports the
// value a click hit; its entry here turns that into a criterion, { subject,
// params }, and `pick` shows the photos that meet it: params are those of
// /api/library/search.

const STATS_TOPICS = [
    {
        id: 'equipment',
        label: 'Equipment',
        blurb: 'Cameras, lenses, file formats and film simulations, and how their use changed.',
        charts: [
            { title: 'Camera and lens', source: 'snap', full: true,
              render: (el, d, pick) => renderCameraLensTreemap(el, d.snap.cameraLens, d.snap.totalPhotos, v => pick(cameraCriterion(v))) },
            { title: 'Camera usage', subtitle: 'Photos per camera and period', source: 'tl', full: true,
              render: (el, d, pick) => renderCameraLines(el, d.tl, v => pick(v.camera
                  ? { subject: `${v.camera} · ${v.period}`, params: { Model: v.camera, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: 'Film simulation', source: 'snap', full: true,
              when: d => d.snap.filmSims?.some(s => s.name !== 'None'),
              render: (el, d, pick) => renderFilmSimBar(el, d.snap.filmSims, f => pick({ subject: f, params: { FilmSimulation: f } })) },
            { title: 'Resolution', subtitle: 'Largest and average megapixels per period', source: 'tl', full: true,
              render: (el, d, pick) => renderMegapixelTimeline(el, d.tl, v => pick(periodCriterion(v))) },
            { title: 'Format', source: 'snap',
              render: (el, d, pick) => renderFormatDonut(el, d.snap.formats, f => pick({ subject: f.toUpperCase(), params: { ext: f } })) },
        ],
    },
    {
        id: 'exposure',
        label: 'Exposure',
        blurb: 'Focal lengths, apertures and ISO: every photo in the space of settings, coloured by camera, and how they drifted over time.',
        charts: [
            { title: 'Exposure space', subtitle: 'Focal length, aperture and ISO of each photo, in stops and coloured by camera, with a trail through the median settings of each period', source: 'exposureSpace', full: true, stage: true,
              preview: (el, d) => renderExposureSpace(el, d.exposureSpace, d.snap.totalPhotos, null, { preview: true }),
              render: (el, d, pick) => renderExposureSpace(el, d.exposureSpace, d.snap.totalPhotos, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: `All photos · ${v.step.period}`, params: periodDates(v.step.period) })) },
            { title: 'Focal length', source: 'snap', full: true, coverage: d => d.snap.focalLengths,
              render: (el, d, pick) => renderFocalHistogram(el, expandValues(d.snap.focalLengths), expandValues(d.snap.focalLengths35),
                  v => pick({ subject: `${v.min}–${v.max} mm${v.field === 'FocalLength35' ? ' (35 mm)' : ''}`, params: binRange(v.field, v) })) },
            { title: 'Aperture', source: 'snap', coverage: d => d.snap.apertures,
              render: (el, d, pick) => renderApertureHistogram(el, expandValues(d.snap.apertures),
                  v => pick({ subject: `f/${v.min} – f/${v.max}`, params: binRange('FNumber', v) })) },
            { title: 'ISO', source: 'snap', coverage: d => d.snap.isos,
              render: (el, d, pick) => renderISOHistogram(el, expandValues(d.snap.isos),
                  v => pick({ subject: `ISO ${v.min} – ${v.max}`, params: binRange('ISOSpeedRatings', v) })) },
            { title: 'Focal length over time', subtitle: 'Median 35 mm equivalent, with the middle half of all photos as a band', source: 'tl', full: true,
              render: (el, d, pick) => renderFocalDrift(el, d.tl, v => pick(periodCriterion(v))) },
            { title: 'ISO over time', subtitle: 'Median ISO per period', source: 'tl', full: true,
              render: (el, d, pick) => renderISOEvolution(el, d.tl, v => pick(periodCriterion(v))) },
            { title: 'Aperture over time', subtitle: 'Share of each f-stop per period', source: 'tl', full: true,
              render: (el, d, pick) => renderApertureHeat(el, d.tl, v => pick({
                  subject: `${v.bucket} · ${v.period}`,
                  params: { ...apertureBucketRange(v.bucket), ...periodDates(v.period) },
              })) },
        ],
    },
    {
        id: 'time',
        label: 'Time',
        blurb: 'The hours and days on which the photos were taken, and in what light.',
        charts: [
            { title: 'Daylight', subtitle: 'Day of the year around, hour outward, brightness up; each photo in its main colour, with a trail through the months of the year', source: 'colourSpace', periods: false, full: true, stage: true,
              preview: (el, d) => renderDaylight(el, d.colourSpace, null, { preview: true }),
              render: (el, d, pick) => renderDaylight(el, d.colourSpace, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: `${MONTHS[v.month - 1]} · all years`, params: { month: v.month } })) },
            { title: 'Time of day', source: 'snap',
              render: (el, d, pick) => renderShootingClock(el, d.snap.shootingHours, h => pick({
                  subject: `${String(h).padStart(2, '0')}:00 – ${String(h + 1).padStart(2, '0')}:00`,
                  params: { hour: h },
              })) },
            { title: 'Shooting calendar', source: 'snap', full: true,
              render: (el, d, pick) => renderCalendarHeatmap(el, d.snap.shootingDays, day => pick({
                  subject: day, params: { date_taken_min: day, date_taken_max: day },
              })) },
        ],
    },
    {
        id: 'frame',
        label: 'Frame',
        blurb: 'The shape of the pictures, and how it changed.',
        charts: [
            { title: 'Aspect ratio', subtitle: 'Share of each frame shape per period', source: 'tl', full: true,
              render: (el, d, pick) => renderAspectLines(el, d.tl, v => pick(v.aspect
                  ? { subject: `${v.aspect} · ${v.period}`, params: { aspect: v.aspect, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
        ],
    },
    {
        id: 'colour',
        label: 'Colour',
        blurb: 'Every photo at its main colour and by its character, black and white or colour, the main colours, and warm against cool through the year.',
        charts: [
            { title: 'Colour space', subtitle: 'Each photo at its main colour in OKLab, with a trail through the mean colour of each period', source: 'colourSpace', full: true, stage: true,
              preview: (el, d) => renderColourSpace(el, d.colourSpace, null, { preview: true }),
              render: (el, d, pick) => renderColourSpace(el, d.colourSpace, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: `Colour photos · ${v.step.period}`, params: { mono: 'colour', ...periodDates(v.step.period) } })) },
            { title: 'Character', subtitle: 'Brightness across, contrast in depth, colourfulness up; each photo in its main colour, with a trail through the average of each period', source: 'colourSpace', full: true, stage: true,
              preview: (el, d) => renderCharacter(el, d.colourSpace, null, { preview: true }),
              render: (el, d, pick) => renderCharacter(el, d.colourSpace, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: `All photos · ${v.step.period}`, params: periodDates(v.step.period) })) },
            { title: 'Black and white', subtitle: 'Share of black-and-white, toned and colour photos per period', source: 'colour', full: true,
              render: (el, d, pick) => renderColourClasses(el, d.colour, v => pick(v.cls
                  ? { subject: `${v.name} · ${v.period}`, params: { mono: v.cls, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: 'Colour of each period', subtitle: 'The average hue of the colour photos\' main colours, at their most colourful', source: 'colour', full: true,
              render: (el, d, pick) => renderColourStrip(el, d.colour, v => pick({
                  subject: `Colour photos · ${v.period}`, params: { mono: 'colour', ...periodDates(v.period) },
              })) },
            { title: 'Main colours', subtitle: 'Part of the colour photos\' area in each hue; each hue at its most colourful', source: 'colour',
              render: (el, d, pick) => renderHueWheel(el, d.colour, bin => pick({
                  subject: `${capitalise(hueName(bin))} · at least ${HUE_SHARE_MIN_PERCENT} % of the frame`,
                  params: { mono: 'colour', hue_bin: bin },
              })) },
            { title: 'Warm and cool through the year', subtitle: 'Share of warm and of cool colour photos per month, in their warm or cool colours', source: 'colour', full: true,
              render: (el, d, pick) => renderWarmCool(el, d.colour, b => pick({
                  subject: `${b.warmth === 'warm' ? 'Warm' : 'Cool'} · ${MONTHS[b.month - 1]}${b.year ? ' ' + b.year : ''}`,
                  params: { mono: 'colour', warmth: b.warmth, month: b.month, ...(b.year ? periodDates(b.year) : {}) },
              })) },
        ],
    },
    {
        id: 'places',
        label: 'Places',
        blurb: 'Where and when the photos with a location were taken: home as a column of light, journeys as trails out into the world.',
        charts: [
            { title: 'Space and time', subtitle: 'Where on the floor, around the middle of your photos with distance on a log scale; when, upward; a trail through the days', source: 'spaceTime', periods: false, full: true, stage: true,
              preview: (el, d) => renderSpaceTime(el, d.spaceTime, null, { preview: true }),
              render: (el, d, pick) => renderSpaceTime(el, d.spaceTime, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: v.day, params: { date_taken_min: v.day, date_taken_max: v.day } })) },
        ],
    },
];

// A chart may be a stage, a 3D view (ADR-0046): it comes after the topic's
// other charts, takes the width of the page, has a preview of its own for the
// overview's card, and a Full view button shows it,
// and the photos beside it, over the whole window in the dark. A chart with
// periods: false has no use for the Periods select, though its source has.

// The topics the 3D views had on their own for a day; their addresses lead
// to the topics that hold them now.
const STATS_TOPIC_MOVED = {
    'colour-space': 'colour', 'character': 'colour', 'exposure-space': 'exposure',
    'daylight': 'time', 'space-time': 'places',
};

// The swatch a hue must cover to be a main colour of a photo; the server's
// HueShareMin.
const HUE_SHARE_MIN_PERCENT = 20;

const capitalise = s => s.charAt(0).toUpperCase() + s.slice(1);

/* --- Criteria --- */

// A camera, or a camera and lens. Photos without a lens tag cannot be looked
// for as such, so that cell shows all of the camera's photos and says so.
function cameraCriterion({ camera, lens }) {
    if (!lens || lens === '(no lens)' || lens === '(unknown)') return { subject: camera, params: { Model: camera } };
    return { subject: `${camera} · ${lens}`, params: { Model: camera, LensModel: lens } };
}

// One photo, picked as a light on a 3D stage.
function photoCriterion({ id, date }) {
    return { subject: `One photo · ${date ? date.slice(0, 10) : 'no date'}`, params: { photo_id: id } };
}

function periodCriterion({ period }) {
    return { subject: period, params: periodDates(period) };
}

// A period's first and last day: "2024" or "2024-05".
function periodDates(period) {
    if (/^\d{4}$/.test(period)) return { date_taken_min: `${period}-01-01`, date_taken_max: `${period}-12-31` };
    const [y, m] = period.split('-').map(Number);
    const last = new Date(Date.UTC(y, m, 0)).getUTCDate();
    return { date_taken_min: `${period}-01`, date_taken_max: `${period}-${String(last).padStart(2, '0')}` };
}

// A histogram bar holds its lower bound and not its upper one, as the search
// holds both: the upper one moves in by a hair.
function binRange(field, { min, max }) {
    return { [`${field}_min`]: min, [`${field}_max`]: max - 0.001 };
}

// The f-stop buckets of the aperture chart, as the server draws them
// (tlApertureMap): each holds its upper bound and not its lower one.
const APERTURE_BUCKETS = {
    'f/1': [0, 1.2], 'f/1.4': [1.2, 1.6], 'f/2': [1.6, 2.3], 'f/2.8': [2.3, 3.3], 'f/4': [3.3, 4.7],
    'f/5.6': [4.7, 6.5], 'f/8': [6.5, 9.5], 'f/11': [9.5, 13], 'f/16+': [13, 1000],
};

function apertureBucketRange(bucket) {
    const [lo, hi] = APERTURE_BUCKETS[bucket];
    return { FNumber_min: lo ? lo + 0.001 : 0, FNumber_max: hi };
}

function statsTopic(id) {
    return STATS_TOPICS.find(t => t.id === id) || null;
}

/* --- Addresses --- */

// #statistics[/<topic>][?library=<id>&path=<folder relative to the library>]
// The folder is relative, never absolute: the same address must work on the
// NAS and on the Mac (CLAUDE.md, "two views of the same photos").

function statsRouteFromHash(hash) {
    const [pathPart, query = ''] = hash.replace(/^#/, '').split('?');
    const named = pathPart.split('/')[1] || '';
    const topic = STATS_TOPIC_MOVED[named] ?? named;
    const q = new URLSearchParams(query);
    return {
        topic: statsTopic(topic) ? topic : '',
        library: q.get('library') || '',
        path: q.get('path') || '',
    };
}

function statsHash(route) {
    const q = new URLSearchParams();
    if (route.library) q.set('library', route.library);
    if (route.library && route.path) q.set('path', route.path);
    const qs = q.toString();
    return '#statistics' + (route.topic ? '/' + route.topic : '') + (qs ? '?' + qs : '');
}
