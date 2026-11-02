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
        label: t('Equipment'),
        blurb: t('Cameras, lenses, file formats and film simulations, and how their use changed.'),
        charts: [
            { title: t('Camera and lens'), source: 'snap', full: true,
              render: (el, d, pick) => renderCameraLensTreemap(el, d.snap.cameraLens, d.snap.totalPhotos, v => pick(cameraCriterion(v))) },
            { title: t('Camera usage'), subtitle: t('Photos per camera and period'), source: 'tl', full: true,
              render: (el, d, pick) => renderCameraLines(el, d.tl, v => pick(v.camera
                  ? { subject: `${v.camera} · ${v.period}`, params: { Model: v.camera, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Phone or camera'), subtitle: t('Share of photos taken with a phone, per period'), source: 'in:gear', full: true,
              render: (el, d, pick) => renderDeviceShare(el, d.in.gear, v => pick(v.series
                  ? { subject: `${v.series.name} · ${v.period}`, params: { device: v.series.key, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Life lines of the gear'), subtitle: t('A lane for each camera and lens with the most photos, darker in the periods it took more'), source: 'in:gear', full: true,
              render: (el, d, pick) => renderGearLives(el, d.in.gear, v => pick({
                  subject: `${v.group.name} · ${v.value}`,
                  params: { [v.group.kind === 'lens' ? 'LensModel' : 'Model']: v.group.name, ...periodDates(v.value) },
              })) },
            { title: t('Film simulation'), source: 'snap', full: true,
              when: d => d.snap.filmSims?.some(s => s.name !== 'None'),
              render: (el, d, pick) => renderFilmSimBar(el, d.snap.filmSims, f => pick({ subject: f, params: { FilmSimulation: f } })) },
            { title: t('How each zoom is used'), subtitle: t('The focal lengths each zoom lens was used at, darker where it was used most'), source: 'in:gear', full: true,
              render: (el, d, pick) => renderZoomUse(el, d.in.gear, v => pick({
                  subject: `${v.group.name} · ${v.value} mm`,
                  params: { LensModel: v.group.name, FocalLength_min: v.value - 0.5, FocalLength_max: v.value + 0.49 },
              })) },
            { title: t('Resolution'), subtitle: t('Largest and average megapixels per period'), source: 'tl', full: true,
              render: (el, d, pick) => renderMegapixelTimeline(el, d.tl, v => pick(periodCriterion(v))) },
            { title: t('Format'), source: 'snap',
              render: (el, d, pick) => renderFormatDonut(el, d.snap.formats, f => pick({ subject: f.toUpperCase(), params: { ext: f } })) },
            { title: t('White balance'), subtitle: t('Set by the camera or by hand'), source: 'in:gear',
              render: (el, d, pick) => renderWhiteBalance(el, d.in.gear, w => pick({ subject: w.label, params: { WhiteBalance: w.name } })) },
        ],
    },
    {
        id: 'exposure',
        label: t('Exposure'),
        blurb: t('Focal lengths, apertures and ISO: every photo in the space of settings, coloured by camera, and how they drifted over time.'),
        charts: [
            { title: t('Exposure space'), subtitle: t('Focal length, aperture and ISO of each photo, in stops and coloured by camera, with a trail through the median settings of each period'), source: 'exposureSpace', full: true, stage: true,
              preview: (el, d) => renderExposureSpace(el, d.exposureSpace, d.snap.totalPhotos, null, { preview: true }),
              render: (el, d, pick) => renderExposureSpace(el, d.exposureSpace, d.snap.totalPhotos, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: t('All photos · {period}', { period: v.step.period }), params: periodDates(v.step.period) })) },
            { title: t('Focal length'), source: 'snap', full: true, coverage: d => d.snap.focalLengths,
              render: (el, d, pick) => renderFocalHistogram(el, expandValues(d.snap.focalLengths), expandValues(d.snap.focalLengths35),
                  v => pick({ subject: `${v.min}–${v.max} mm${v.field === 'FocalLength35' ? ' (35 mm)' : ''}`, params: binRange(v.field, v) })) },
            { title: t('Aperture'), source: 'snap', coverage: d => d.snap.apertures,
              render: (el, d, pick) => renderApertureHistogram(el, expandValues(d.snap.apertures),
                  v => pick({ subject: `f/${v.min} – f/${v.max}`, params: binRange('FNumber', v) })) },
            { title: t('ISO'), source: 'snap', coverage: d => d.snap.isos,
              render: (el, d, pick) => renderISOHistogram(el, expandValues(d.snap.isos),
                  v => pick({ subject: `ISO ${v.min} – ${v.max}`, params: binRange('ISOSpeedRatings', v) })) },
            { title: t('Exposure time'), subtitle: t('Photos per stop of exposure time'), source: 'in:gear',
              render: (el, d, pick) => renderExposureTimes(el, d.in.gear, b => pick({
                  subject: exposureStopLabel(b.stop), params: { ExposureTime_min: 2 ** (b.stop - 0.5), ExposureTime_max: 2 ** (b.stop + 0.5) - 1e-9 },
              })) },
            { title: t('How bright the scenes were'), subtitle: t('Exposure value at ISO 100, from aperture, time and ISO: night low, sun high'), source: 'in:gear',
              render: (el, d, pick) => renderSceneLight(el, d.in.gear, b => pick({ subject: `EV ${b.ev}`, params: { ev_from: b.ev, ev_to: b.ev } })) },
            { title: t('Risk of shake'), subtitle: t('Share of each lens’s photos exposed longer than one over the focal length (35 mm), the old rule for a sharp photo held in the hand'), source: 'in:gear', full: true,
              render: (el, d, pick) => renderShakeRisk(el, d.in.gear, l => pick({ subject: t('{lens} · at risk of shake', { lens: l.name }), params: { LensModel: l.name, slow: 1 } })) },
            { title: t('Focal length over time'), subtitle: t('Median 35 mm equivalent, with the middle half of all photos as a band'), source: 'tl', full: true,
              render: (el, d, pick) => renderFocalDrift(el, d.tl, v => pick(periodCriterion(v))) },
            { title: t('ISO over time'), subtitle: t('Median ISO per period'), source: 'tl', full: true,
              render: (el, d, pick) => renderISOEvolution(el, d.tl, v => pick(periodCriterion(v))) },
            { title: t('Aperture over time'), subtitle: t('Share of each f-stop per period'), source: 'tl', full: true,
              render: (el, d, pick) => renderApertureHeat(el, d.tl, v => pick({
                  subject: `${v.bucket} · ${v.period}`,
                  params: { ...apertureBucketRange(v.bucket), ...periodDates(v.period) },
              })) },
            { title: t('Flash'), subtitle: t('Share of photos per period for which the flash fired'), source: 'in:gear', full: true,
              render: (el, d, pick) => renderFlashShare(el, d.in.gear, v => pick(v.series
                  ? { subject: t('Flash · {period}', { period: v.period }), params: { flash: 1, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Exposure compensation'), subtitle: t('Share of photos per period exposed darker or brighter than metered, by a third of a stop or more'), source: 'in:gear', full: true,
              render: (el, d, pick) => renderBiasShare(el, d.in.gear, v => pick(periodCriterion(v))) },
        ],
    },
    {
        id: 'time',
        label: t('Time'),
        blurb: t('The hours and days on which the photos were taken, and in what light.'),
        charts: [
            { title: t('Daylight'), subtitle: t('Day of the year around, hour outward, brightness up; each photo in its main colour, with a trail through the months of the year'), source: 'colourSpace', periods: false, full: true, stage: true,
              preview: (el, d) => renderDaylight(el, d.colourSpace, null, { preview: true }),
              render: (el, d, pick) => renderDaylight(el, d.colourSpace, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: t('{month} · all years', { month: MONTHS[v.month - 1] }), params: { month: v.month } })) },
            { title: t('Time of day'), source: 'snap',
              render: (el, d, pick) => renderShootingClock(el, d.snap.shootingHours, h => pick({
                  subject: `${String(h).padStart(2, '0')}:00 – ${String(h + 1).padStart(2, '0')}:00`,
                  params: { hour: h },
              })) },
            { title: t('Sessions'), subtitle: t('Runs of photos with less than two hours between them, by their size'), source: 'in:time',
              render: (el, d) => renderSessionSizes(el, d.in.time) },
            { title: t('Weekday and hour'), subtitle: t('Photos per weekday and hour of the day'), source: 'snap', full: true,
              render: (el, d, pick) => renderWeekHours(el, d.snap.weekHours, v => pick({
                  subject: `${t(WEEKDAYS[v.wd])} · ${weekHourSpan(v.hr)}`,
                  params: { weekday: v.wd, hour: v.hr },
              })) },
            { title: t('Year against year'), subtitle: t('Photos per month, a line for each year'), source: 'snap', full: true,
              render: (el, d, pick) => renderYearAgainstYear(el, d.snap.shootingDays, v => pick(v.year
                  ? { subject: `${t(MONTHS[v.month - 1])} ${v.year}`, params: periodDates(`${v.year}-${String(v.month).padStart(2, '0')}`) }
                  : { subject: t('{month} · all years', { month: t(MONTHS[v.month - 1]) }), params: { month: v.month } })) },
            { title: t('The largest sessions'), subtitle: t('The runs of photos with the most in them'), source: 'in:time', full: true,
              render: (el, d, pick) => renderLargestSessions(el, d.in.time, x => pick({ subject: x.label, params: { taken_from: x.first, taken_to: x.last } })) },
            { title: t('Shooting calendar'), source: 'snap', full: true,
              render: (el, d, pick) => renderCalendarHeatmap(el, d.snap.shootingDays, day => pick({
                  subject: day, params: { date_taken_min: day, date_taken_max: day },
              })) },
        ],
    },
    {
        id: 'frame',
        label: t('Frame'),
        blurb: t('The shape of the pictures, and how it changed.'),
        charts: [
            { title: t('Aspect ratio'), subtitle: t('Share of each frame shape per period'), source: 'tl', full: true,
              render: (el, d, pick) => renderAspectLines(el, d.tl, v => pick(v.aspect
                  ? { subject: `${v.aspect} · ${v.period}`, params: { aspect: v.aspect, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Aspect ratio in detail'), subtitle: t('The long side over the short one, in steps of 0.05: squares at 1.00, 3:2 at 1.50, panoramas beyond 2'), source: 'in:frame', full: true,
              render: (el, d, pick) => renderRatioHistogram(el, d.in.frame, b => pick({ subject: `${b.label} : 1`, params: { ratio_from: b.ratio - 0.01, ratio_to: b.ratio + 0.01 } })) },
            { title: t('Portrait or landscape'), subtitle: t('How each camera was held'), source: 'in:frame', full: true,
              render: (el, d, pick) => renderOrientation(el, d.in.frame, v => pick({ subject: `${v.row} · ${t(ORIENTATION_NAMES[v.col])}`, params: { Model: v.row, orient: v.col } })) },
            { title: t('Cropped'), subtitle: t('Share of photos per period smaller than the largest frame of their camera — cropped, or taken smaller'), source: 'in:frame', full: true,
              render: (el, d, pick) => renderCropped(el, d.in.frame, v => pick(v.series
                  ? { subject: t('Cropped · {period}', { period: v.period }), params: { cropped: 1, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
        ],
    },
    {
        id: 'colour',
        label: t('Colour'),
        blurb: t('Every photo at its main colour and by its character, black and white or colour, the main colours, and warm against cool through the year.'),
        charts: [
            { title: t('Colour space'), subtitle: t('Each photo at its main colour in OKLab, with a trail through the mean colour of each period'), source: 'colourSpace', full: true, stage: true,
              preview: (el, d) => renderColourSpace(el, d.colourSpace, null, { preview: true }),
              render: (el, d, pick) => renderColourSpace(el, d.colourSpace, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: t('Colour photos · {period}', { period: v.step.period }), params: { mono: 'colour', ...periodDates(v.step.period) } })) },
            { title: t('Character'), subtitle: t('Brightness across, contrast in depth, colourfulness up; each photo in its main colour, with a trail through the average of each period'), source: 'colourSpace', full: true, stage: true,
              preview: (el, d) => renderCharacter(el, d.colourSpace, null, { preview: true }),
              render: (el, d, pick) => renderCharacter(el, d.colourSpace, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: t('All photos · {period}', { period: v.step.period }), params: periodDates(v.step.period) })) },
            { title: t('Black and white'), subtitle: t('Share of black-and-white, toned and colour photos per period'), source: 'colour', full: true,
              render: (el, d, pick) => renderColourClasses(el, d.colour, v => pick(v.cls
                  ? { subject: `${v.name} · ${v.period}`, params: { mono: v.cls, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Colour of each period'), subtitle: t('The average hue of the colour photos\' main colours, at their most colourful'), source: 'colour', full: true,
              render: (el, d, pick) => renderColourStrip(el, d.colour, v => pick({
                  subject: t('Colour photos · {period}', { period: v.period }), params: { mono: 'colour', ...periodDates(v.period) },
              })) },
            { title: t('Main colours'), subtitle: t('Part of the colour photos\' area in each hue; each hue at its most colourful'), source: 'colour',
              render: (el, d, pick) => renderHueWheel(el, d.colour, bin => pick({
                  subject: t('{hue} · at least {pct} % of the frame', { hue: capitalise(hueName(bin)), pct: HUE_SHARE_MIN_PERCENT }),
                  params: { mono: 'colour', hue_bin: bin },
              })) },
            { title: t('Colour combinations'), subtitle: t('Photos with each colour of a well-known combination, each at least {COMBO_SHARE_MIN_PERCENT} % of the frame', { COMBO_SHARE_MIN_PERCENT: COMBO_SHARE_MIN_PERCENT }), source: 'colour',
              render: (el, d, pick) => renderColourCombos(el, d.colour, hues => pick(comboCriterion(hues))) },
            { title: t('Your combination'), subtitle: t('Choose two or three colours on the wheel to count and show their photos'), source: 'colour', full: true,
              render: (el, d, pick) => renderComboPicker(el, d.colour, hues => pick(comboCriterion(hues))) },
            { title: t('Warm and cool through the year'), subtitle: t('Share of warm and of cool colour photos per month, in their warm or cool colours'), source: 'colour', full: true,
              render: (el, d, pick) => renderWarmCool(el, d.colour, b => pick({
                  subject: `${b.warmth === 'warm' ? t('Warm') : t('Cool')} · ${MONTHS[b.month - 1]}${b.year ? ' ' + b.year : ''}`,
                  params: { mono: 'colour', warmth: b.warmth, month: b.month, ...(b.year ? periodDates(b.year) : {}) },
              })) },
            { title: t('How colourful'), subtitle: t('Colourfulness of the colour photos of each film simulation, or of each camera without them, darker where most photos are'), source: 'in:colour', full: true,
              render: (el, d, pick) => renderColourfulness(el, d.in.colour, v => pick({ subject: v.group.name, params: { [v.group.field]: v.group.name, mono: 'colour' } })) },
            { title: t('Colours through the day'), subtitle: t('Photos per hour with each main colour'), source: 'in:colour', full: true,
              render: (el, d, pick) => renderHueHours(el, d.in.colour, v => pick({
                  subject: `${capitalise(hueName(v.row))} · ${weekHourSpan(v.col)}`, params: { mono: 'colour', hue_bin: v.row, hour: v.col },
              })) },
            { title: t('Palette space'), subtitle: t('Each photo’s three largest colours in OKLab, joined into a triangle: small for calm palettes, wide for contrasting ones'), source: 'space:palette', periods: false, full: true, stage: true, noPreview: true,
              render: (el, d, pick) => renderPaletteSpace(el, d.space.palette, v => pick(photoCriterion(v.photo))) },
        ],
    },
    {
        id: 'places',
        label: t('Places'),
        blurb: t('Where and when the photos with a location were taken: home as a column of light, journeys as trails out into the world; and the countries and cities they name.'),
        charts: [
            { title: t('Space and time'), subtitle: t('Where on the floor, around the middle of your photos with distance on a log scale, wider beyond 1,000 km; when, upward; a trail through the days'), source: 'spaceTime', periods: false, full: true, stage: true,
              preview: (el, d) => renderSpaceTime(el, d.spaceTime, null, { preview: true }),
              render: (el, d, pick) => renderSpaceTime(el, d.spaceTime, v => pick(v.photo
                  ? photoCriterion(v.photo)
                  : { subject: v.day, params: { date_taken_min: v.day, date_taken_max: v.day } })) },
            { title: t('Density landscape'), subtitle: t('The world map as the floor, a column on every half degree, higher the more photos (log scale), in their most common main colour'), source: 'in:places', periods: false, full: true, stage: true, noPreview: true,
              render: (el, d, pick) => renderDensity(el, d.in.places, v => pick(densityCriterion(v.cell))) },
            { title: t('Countries'), subtitle: t('Photos per country, as the photos name it'), source: 'snap',
              render: (el, d, pick) => renderPlaceBars(el, d.snap.countries, v => pick({ subject: v, params: { Country: v } })) },
            { title: t('Cities'), subtitle: t('Photos per city, as the photos name it'), source: 'snap',
              render: (el, d, pick) => renderPlaceBars(el, d.snap.cities, v => pick({ subject: v, params: { City: v } })) },
            { title: t('Journeys'), subtitle: t('Days whose photos were taken more than 100 km from home, joined into journeys, a row per year'), source: 'in:places', full: true,
              render: (el, d, pick) => renderJourneys(el, d.in.places, j => pick({
                  subject: `${j.start === j.end ? j.start : `${j.start} – ${j.end}`}${j.country ? ' · ' + j.country : ''}`,
                  params: { date_taken_min: j.start, date_taken_max: j.end },
              })) },
        ],
    },
    {
        id: 'cataloguing',
        label: t('Cataloguing'),
        blurb: t('How fully the photos are described: titles, descriptions, keywords, places, people and stars — and where they are still missing.'),
        charts: [
            { title: t('Completeness'), subtitle: t('Photos with each field, and those without; a click on “without” shows them, to select and take to Metadata'), source: 'snap', full: true,
              render: (el, d, pick) => renderCompleteness(el, d.snap.completeness, d.snap.totalPhotos, f => pick({
                  subject: t('Without: {field}', { field: t(COMPLETENESS_NAMES[f]) }), params: { lacks: f },
              })) },
            { title: t('Stars over time'), subtitle: t('Share of photos per period with each number of stars, and without'), source: 'in:catalogue', full: true,
              render: (el, d, pick) => renderStarsOverTime(el, d.in.catalogue, v => pick(v.series
                  ? { subject: `${v.series.name} · ${v.period}`, params: { ...(v.series.key === '0' ? { lacks: 'rating' } : { Rating: v.series.key }), ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Keywords'), subtitle: t('The keywords on most photos'), source: 'in:catalogue',
              render: (el, d, pick) => renderKeywordBars(el, d.in.catalogue, k => pick({ subject: k.name, params: { Keywords: k.name } })) },
            { title: t('Own fields'), subtitle: t('The values of own fields on most photos'), source: 'in:catalogue',
              render: (el, d, pick) => renderOwnFieldBars(el, d.in.catalogue, f => pick({ subject: f.label, params: { [`meta_${f.key}`]: f.value } })) },
        ],
    },
    {
        id: 'people',
        label: t('People'),
        blurb: t('Who is in the photos, when and with whom, how many faces a photo holds, and every face placed by what it looks like.'),
        charts: [
            { title: t('Who appears most'), subtitle: t('Photos per named person'), source: 'people', full: true,
              render: (el, d, pick) => renderPeopleBars(el, d.people, name => pick(personCriterion(name))) },
            { title: t('People over time'), subtitle: t('Photos per period of the people compared, or of the eight with the most photos'), source: 'people', full: true,
              render: (el, d, pick) => renderPeopleLines(el, d.people, v => pick(v.person
                  ? { subject: `${v.person} · ${v.period}`, params: { person: v.person, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Photos by age'), subtitle: t('Photos per year of life of the people with a birthday'), source: 'people', periods: false, full: true,
              render: (el, d, pick) => renderPeopleAges(el, d.people, v => pick({ subject: `${v.person}, ${ageText(v.age)}`, params: { person: v.person, ...ageDates(v.birth, v.age) } })) },
            { title: t('Who with whom'), subtitle: t('Photos two people share, of the people compared, or of the twelve with the most photos'), source: 'people', periods: false,
              render: (el, d, pick) => renderPeopleMatrix(el, d.people, v => pick({ subject: `${v.a} and ${v.b}`, params: { person: [v.a, v.b] } })) },
            { title: t('Alone or in a group'), subtitle: t('Photos by the faces found in them, named or not'), source: 'people', periods: false,
              render: (el, d, pick) => renderCrowdBars(el, d.people, b => pick({ subject: b.label, params: crowdParams(b) })) },
            { title: t('Faces per photo over time'), subtitle: t('Share of each number of faces per period'), source: 'people', full: true,
              render: (el, d, pick) => renderCrowdLines(el, d.people, v => pick(v.bucket
                  ? { subject: `${v.bucket.label} · ${v.period}`, params: { ...crowdParams(v.bucket), ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Not seen for a while'), subtitle: t('Months since each named person’s last photo'), source: 'in:people', periods: false, full: true,
              render: (el, d, pick) => renderNotSeen(el, d.in.people, p => pick(personCriterion(p.name))) },
            { title: t('Who where'), subtitle: t('Photos of each person in each country the photos name'), source: 'in:people', periods: false, full: true,
              render: (el, d, pick) => renderWhoWhere(el, d.in.people, v => pick({ subject: `${v.row} · ${v.col}`, params: { person: v.row, Country: v.col } })) },
            { title: t('Who with what'), subtitle: t('Photos of each person with each thing found'), source: 'in:people', periods: false, full: true,
              render: (el, d, pick) => renderWhoWithWhat(el, d.in.people, v => pick({ subject: `${v.row} · ${ThingWords.name(v.col)}`, params: { person: v.row, thing: v.col } })) },
            { title: t('Face space'), subtitle: t('Every face placed by what it looks like, in the colour of its person; the middle of each person as a larger light'), source: 'people', periods: false, full: true, stage: true,
              preview: (el, d) => renderFaceSpace(el, d.people, null, { preview: true }),
              render: (el, d, pick) => renderFaceSpace(el, d.people, v => pick(v.person ? personCriterion(v.person) : photoCriterion(v.photo))) },
            { title: t('Circle of people'), subtitle: t('People drawn together by the photos they share, with their photos around them'), source: 'people', periods: false, full: true, stage: true,
              preview: (el, d) => renderPeopleCircle(el, d.people, null, { preview: true }),
              render: (el, d, pick) => renderPeopleCircle(el, d.people, v => pick(
                  v.pair ? { subject: v.pair.join(' and '), params: { person: v.pair } }
                      : v.person ? personCriterion(v.person) : photoCriterion(v.photo))) },
            { title: t('Life lines'), subtitle: t('Time across, a lane for each of the people compared, or of the eight with the most photos, as high as their photos per period'), source: 'people', full: true, stage: true,
              preview: (el, d) => renderLifeLines(el, d.people, null, { preview: true }),
              render: (el, d, pick) => renderLifeLines(el, d.people, v => pick(v.person
                  ? { subject: `${v.person} · ${v.period}`, params: { person: v.person, ...periodDates(v.period) } }
                  : photoCriterion(v.photo))) },
        ],
    },
    {
        id: 'things',
        label: t('Things'),
        blurb: t('Animals, objects and scenes found in the photos: which come most often, how that changed, which come together, and how many photos are blurred, too dark or too bright.'),
        charts: [
            { title: t('Things met most'), subtitle: t('Photos per thing found'), source: 'things', full: true,
              render: (el, d, pick) => renderThingsBars(el, d.things, name => pick(thingCriterion(name))) },
            { title: t('Things over time'), subtitle: t('Photos per period of the eight things with the most photos'), source: 'things', full: true,
              render: (el, d, pick) => renderThingsLines(el, d.things, v => pick(v.person
                  ? { subject: `${v.person} · ${v.period}`, params: { thing: v.person, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Things together'), subtitle: t('Photos two things share, of the twelve with the most photos'), source: 'things', periods: false, full: true,
              render: (el, d, pick) => renderThingsMatrix(el, d.things, v => pick({ subject: `${v.a} and ${v.b}`, params: { thing: [v.a, v.b] } })) },
            { title: t('Photo quality over time'), subtitle: t('Photos per period that are blurred, underexposed or overexposed: where to look when sorting out'), source: 'things', full: true,
              render: (el, d, pick) => renderThingsQuality(el, d.things, v => pick(v.person
                  ? { subject: `${v.person} · ${v.period}`, params: { thing: v.person, ...periodDates(v.period) } }
                  : periodCriterion(v))) },
            { title: t('Things through the year'), subtitle: t('Photos of each thing per month of the year'), source: 'in:things', periods: false, full: true,
              render: (el, d, pick) => renderThingsThroughTheYear(el, d.in.things, v => pick({
                  subject: `${ThingWords.name(v.row)} · ${t(MONTHS[Number(v.col) - 1])}`, params: { thing: v.row, month: Number(v.col) },
              })) },
            { title: t('Things by place'), subtitle: t('Photos of each thing in each country the photos name'), source: 'in:things', periods: false, full: true,
              render: (el, d, pick) => renderThingsByPlace(el, d.in.things, v => pick({ subject: `${ThingWords.name(v.row)} · ${v.col}`, params: { thing: v.row, Country: v.col } })) },
            { title: t('Meaning space'), subtitle: t('Every photo the vision model has looked at, placed by what it shows: alike photos close together'), source: 'space:meaning', periods: false, full: true, stage: true, noPreview: true,
              render: (el, d, pick) => renderMeaningSpace(el, d.space.meaning, v => pick(photoCriterion(v.photo))) },
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

// A camera, or a camera and lens; the "(no lens)" cell, the camera's photos
// without a lens tag.
function cameraCriterion({ camera, lens }) {
    if (lens === '(no lens)') return { subject: `${camera} · ${t('no lens tag')}`, params: { Model: camera, nolens: 1 } };
    if (!lens || lens === '(unknown)') return { subject: camera, params: { Model: camera } };
    return { subject: `${camera} · ${lens}`, params: { Model: camera, LensModel: lens } };
}

// One photo, picked as a light on a 3D stage.
function photoCriterion({ id, date }) {
    return { subject: t('One photo · {date}', { date: date ? date.slice(0, 10) : t('no date') }), params: { photo_id: id } };
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
