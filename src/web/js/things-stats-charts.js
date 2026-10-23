// things-stats-charts.js — the Things topic's charts (ADR-0054): which
// things the photos show, how that changed, and which come together. They
// read /api/library/things-stats and draw with the People topic's charts,
// whose answer has the same shape: named series and the photos they are in.

const THINGS_WORDS = {
    none: 'No thing was found in these photos yet.',
    undated: 'No photo with a thing found in it has a date.',
    apart: 'No two things are in the same photo here.',
    many: 'things',
};

// thingsAsSeries gives the things the People charts' shape, once per answer.
function thingsAsSeries(ts) {
    if (!ts) return null;
    ts._series ??= {
        granularity: ts.granularity,
        people: ts.things.map(t => ({ name: t.name, photos: t.photos })),
        appearances: { person: ts.appearances.thing, lib: ts.appearances.lib, photo: ts.appearances.photo, date: ts.appearances.date },
    };
    return ts._series;
}

function renderThingsBars(el, ts, onPick) {
    if (ts && !ts.installed) { renderThingsInstall(el); return; }
    renderPeopleBars(el, thingsAsSeries(ts), onPick, THINGS_WORDS);
    if (ts?.looked) el.insertAdjacentHTML('beforeend', `<p class="stats-chart-note">Of ${countText(ts.looked, 'photo', 'photos')} looked at for things.</p>`);
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
    el.innerHTML = '<div class="things-install"><p>Loading what it takes…</p></div>';
    ThingsAPI.status().then(({ plan }) => {
        const box = el.querySelector('.things-install');
        if (!plan.supported) {
            box.innerHTML = '<p>Finding things is not available on this computer: ONNX Runtime, which it runs on, has no build for this system.</p>';
            return;
        }
        const mb = Math.round(plan.bytes / 1e6);
        box.innerHTML = `
            <p>Things finds animals, objects and scenes in your library photos — a dog, a car, a house, the sea — so you can filter, search and count by them. What it finds is written into each photo's sidecar as keywords, which other programs read too.</p>
            <p>It runs on this computer. It needs OpenAI's CLIP and an object detector from OpenCV, and ONNX Runtime from Microsoft unless People already downloaded it: <span class="things-num">${mb} MB</span>, downloaded once from Hugging Face and GitHub. After that, no photo leaves this computer.</p>
            <div class="things-actions"><button type="button" class="btn btn-accent things-install-btn">Download and turn on</button></div>
            <div class="things-install-activity"></div>`;
        const btn = box.querySelector('.things-install-btn');
        btn.addEventListener('click', async () => {
            const restore = Activity.button(btn, 'Downloading…');
            const activity = Activity.in(box.querySelector('.things-install-activity'), `Downloading ${mb} MB…`);
            try {
                await ThingsAPI.install();
                activity.done('Things are on. Your libraries are being looked at now; the status line shows how far it is.');
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
