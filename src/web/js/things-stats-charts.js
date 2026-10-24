// things-stats-charts.js — the Things topic's charts (ADR-0054): which
// things the photos show, how that changed, which come together, and how
// many photos are blurred, too dark or too bright. They
// read /api/library/things-stats and draw with the People topic's charts,
// whose answer has the same shape: named series and the photos they are in.

const THINGS_WORDS = {
    none: t('No thing was found in these photos yet.'),
    undated: t('No photo with a thing found in it has a date.'),
    apart: t('No two things are in the same photo here.'),
    many: 'things',
};

// What a photo is like rather than what it shows: blurred, underexposed,
// overexposed (ADR-0055). Its own chart; the others leave it out.
const THINGS_QUALITY = t('Photo quality');

const QUALITY_WORDS = {
    none: t('No photo here was found blurred, underexposed or overexposed.'),
    undated: t('No blurred, underexposed or overexposed photo here has a date.'),
    apart: '',
    many: 'kinds',
};

// thingsAsSeries gives the things the People charts' shape, once per answer:
// the photo's qualities, or what it shows.
function thingsAsSeries(ts, quality = false) {
    if (!ts) return null;
    ts._series ??= {};
    if (ts._series[quality]) return ts._series[quality];
    const keep = ts.things.map((t, i) => [t, i]).filter(([t]) => (t.category === THINGS_QUALITY) === quality);
    const to = new Map(keep.map(([, old], i) => [old, i]));
    const a = ts.appearances;
    const rows = a.thing.map((t, k) => k).filter(k => to.has(a.thing[k]));
    ts._series[quality] = {
        granularity: ts.granularity,
        shown: id => ThingWords.name(id),
        people: keep.map(([t]) => ({ name: t.name, photos: t.photos })),
        appearances: {
            person: rows.map(k => to.get(a.thing[k])),
            lib: rows.map(k => a.lib[k]),
            photo: rows.map(k => a.photo[k]),
            date: rows.map(k => a.date[k]),
        },
    };
    return ts._series[quality];
}

// renderThingsQuality draws how many photos per period are blurred, too
// dark or too bright: where to look when sorting out.
function renderThingsQuality(el, ts, onPick) {
    if (ts && !ts.installed) { tlNoData(el, THINGS_WORDS.none); return; }
    renderPeopleLines(el, thingsAsSeries(ts, true), onPick, QUALITY_WORDS);
}

function renderThingsBars(el, ts, onPick) {
    if (ts && !ts.installed) { renderThingsInstall(el); return; }
    renderPeopleBars(el, thingsAsSeries(ts), onPick, THINGS_WORDS);
    if (ts?.looked) el.insertAdjacentHTML('beforeend', `<p class="stats-chart-note">${t('Of {photos} looked at for things.', { photos: countText(ts.looked, 'photo', 'photos') })}</p>`);
}

function renderThingsLines(el, ts, onPick) {
    if (ts && !ts.installed) { tlNoData(el, THINGS_WORDS.none); return; }
    renderPeopleLines(el, thingsAsSeries(ts), onPick, THINGS_WORDS);
}

function renderThingsMatrix(el, ts, onPick) {
    if (ts && !ts.installed) { tlNoData(el, THINGS_WORDS.none); return; }
    renderPeopleMatrix(el, thingsAsSeries(ts), onPick, THINGS_WORDS);
}

// Before things can be found: what it takes, and the one action to start.
function renderThingsInstall(el) {
    el.innerHTML = '<div class="things-install"><p>' + t('Loading what it takes…') + '</p></div>';
    ThingsAPI.status().then(({ plan, outdated }) => {
        const box = el.querySelector('.things-install');
        if (!plan.supported) {
            box.innerHTML = '<p>' + t('Finding things is not available on this computer: ONNX Runtime, which it runs on, has no build for this system.') + '</p>';
            return;
        }
        const mb = Math.round(plan.bytes / 1e6);
        box.innerHTML = `
            <p>${t('Things finds animals, objects and scenes in your library photos — a dog, a car, a house, the sea — and which are blurred, too dark or too bright, so you can filter, search and count by them, and search in your own words, in many languages. What it finds is written into each photo\'s sidecar as keywords, which other programs read too.')}</p>
            <p>${t('It runs on this computer. It needs Google\'s SigLIP 2 and an object detector from OpenCV, and ONNX Runtime from Microsoft unless People already downloaded it:')} <span class="things-num">${mb} MB</span>${t(', downloaded once from Hugging Face and GitHub. After that, no photo leaves this computer.')}${outdated ? ' ' + t('It replaces the model downloaded before, and every photo is looked at once again.') : ''}</p>
            <p>${t('A fast computer finds things in a few hundred milliseconds a photo; a NAS takes seconds, and a large library a day or two the first time. What one installation found, another sharing the library takes over.')}</p>
            <div class="things-actions"><button type="button" class="btn btn-accent things-install-btn">${t('Download and turn on')}</button></div>
            <div class="things-install-activity"></div>`;
        const btn = box.querySelector('.things-install-btn');
        btn.addEventListener('click', async () => {
            const restore = Activity.button(btn, t('Downloading…'));
            const activity = Activity.in(box.querySelector('.things-install-activity'), t('Downloading {mb} MB…', { mb: mb }));
            try {
                await ThingsAPI.install();
                activity.done(t('Things are on. Your libraries are being looked at now; the status line shows how far it is.'));
            } catch (err) {
                restore();
                activity.fail(`${err.message} Check the connection and try again.`);
            }
        });
    }).catch(err => {
        el.querySelector('.things-install').innerHTML = `<p>${escapeHtml(err.message)}</p>`;
    });
}

function thingCriterion(name) {
    return { subject: name, params: { thing: name } };
}
