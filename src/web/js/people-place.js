// people-place.js — People: everyone named in your libraries, the faces
// waiting for a name in each library, and a page for each person (ADR-0051).
// This file is the place's frame and its addresses; the pages are
// people-everyone.js (everyone), people-naming.js (to name) and
// people-person.js (a person).
//
// Addresses: #people, #people/person/<name>, #people/name/<library>,
// #people/name/<library>/group/<n>, #people/name/<library>/hidden and
// #people/hidden (every library). Groups are rebuilt after every pass, so
// a group's address is only good until the next one. Older addresses lead
// to their new place (peopleRouteFromHash).

// peopleRouteFromHash reads an address into { view, library, id }. view is
// '' (everyone), 'person' (id: the name), 'name' (a library's groups),
// 'group' (id: the group), 'hidden' (a library's hidden faces) or 'person-of' (an older address: id is a person's
// number in library, shown under their name).
function peopleRouteFromHash(hash) {
    const parts = hash.replace(/^#/, '').split(/[?]/)[0].split('/').slice(1).map(decodeURIComponent);
    const [a = '', b = '', c = '', d = ''] = parts;
    if (a === 'person' && b) return { view: 'person', id: b, library: '' };
    if (a === 'hidden') return { view: 'hidden', library: '', id: '' };
    if (a === 'name' && b && c === 'hidden') return { view: 'hidden', library: b, id: '' };
    if (a === 'name' && b) return c === 'group' && d !== '' ? { view: 'group', library: b, id: d } : { view: 'name', library: b, id: '' };
    if (a === 'all' || !a) return b === 'person' && c ? { view: 'person', id: c, library: '' } : { view: '', library: '', id: '' };
    if (b === 'group' && c !== '') return { view: 'group', library: a, id: c };
    if (b === 'person' && c !== '') return { view: 'person-of', library: a, id: c };
    return { view: 'name', library: a, id: '' };
}

function peopleHash({ view = '', library = '', id = '' } = {}) {
    const enc = encodeURIComponent;
    if (view === 'person') return `#people/person/${enc(id)}`;
    if (view === 'name') return `#people/name/${enc(library)}`;
    // Groups are numbered from 0, so an id of 0 is an id.
    if (view === 'group') return `#people/name/${enc(library)}/group/${enc(id)}`;
    if (view === 'hidden') return library ? `#people/name/${enc(library)}/hidden` : '#people/hidden';
    if (view === 'person-of') return `#people/${enc(library)}/person/${enc(id)}`;
    return '#people';
}

class PeoplePane {
    constructor(container) {
        this.container = container;
        this.route = { view: '' };
        this.libraries = [];
        this._everyone = new PeopleEveryone(this);
        this._naming = new PeopleNaming(this);
        this._person = new PeoplePerson(this);
        this._phone = new PeoplePhone(this);
        App.statusLine?.onFinished((job) => {
            if (job.place === 'people' && App.mode === 'people') this.show(this.route);
        });
    }

    async show(route) {
        if (!this.container.firstChild) this._buildShell();
        this.route = { view: '', library: '', id: '', ...route };
        let status;
        try {
            [status, this.libraries] = await Promise.all([PeopleAPI.status(), LibraryAPI.list()]);
        } catch (err) {
            this._fail(`People could not be read: ${err.message}. Reload the page to try again.`);
            return;
        }
        this._head('');
        this._lede(false);
        if (!status.installed) { this._showInstall(status.plan); return; }
        if (!this.libraries.length) { this._showNoLibraries(); return; }
        const { view, library, id } = this.route;
        if (view === 'person-of') { await this._showPersonOf(library, Number(id)); return; }
        if (view === 'person') await (peopleOnPhone() ? this._phone.showPerson(id) : this._person.show(id));
        else if (view === 'name' && !peopleOnPhone()) await this._naming.showLibrary(this._known(library));
        else if (view === 'group' && !peopleOnPhone()) await this._naming.showGroup(this._known(library), Number(id));
        else if (view === 'hidden' && !peopleOnPhone()) await this._naming.showHidden(library ? this._known(library) : null);
        else await (peopleOnPhone() ? this._phone.showOverview() : this._everyone.show());
    }

    // _known is the library asked for when there is one of that id, else the
    // first: an address kept from a library since removed still leads somewhere.
    _known(id) {
        return this.libraries.some(l => String(l.id) === id) ? id : String(this.libraries[0].id);
    }

    // An older address names a person by their number in one library; they
    // are shown under their name now.
    async _showPersonOf(library, id) {
        try {
            const { people } = await PeopleAPI.overview(library);
            const person = people.find(p => p.id === id);
            if (person) { this.go({ view: 'person', id: person.name }, { replace: true }); return; }
        } catch { /* falls through to everyone */ }
        this.go({}, { replace: true });
    }

    _buildShell() {
        this.container.innerHTML = `
            <div class="people-place">
                ${placeHeaderHTML({ title: 'People', below: '<div class="people-lede"></div>' })}
                <div class="people-body"></div>
            </div>`;
        this._body = this.container.querySelector('.people-body');
    }

    _head(count = '') {
        this.container.querySelector('.place-meta').innerHTML = count;
    }

    // What People is and what it writes shows until someone is named; after
    // that the owner knows, and the faces come first.
    _lede(shown) {
        this.container.querySelector('.people-lede').innerHTML = shown
            ? placeLede(`The faces found in your libraries' photos, grouped by who they look like. Names you give are written into each photo's sidecar, where your other installation and programs such as Lightroom read them.`) : '';
    }

    _fail(text) {
        Activity.in(this._body, '', { area: true }).fail(text);
    }

    go(route, { replace = false } = {}) {
        App.openPeople(route, { replace });
    }

    libraryName(id) {
        return this.libraries.find(l => String(l.id) === String(id))?.name || 'this library';
    }

    // —— Not installed ——

    _showInstall(plan) {
        if (!plan.supported) {
            this._body.innerHTML = `<div class="people-note"><p>Face recognition is not available on this computer: ONNX Runtime, which it runs on, has no build for this system.</p></div>`;
            return;
        }
        const mb = Math.round(plan.bytes / 1e6);
        this._body.innerHTML = `
            <div class="people-note">
                <p>People finds the faces in your library photos and groups the ones that look alike, so you can name them.</p>
                <p>Face recognition runs on this computer. It needs ONNX Runtime from Microsoft and two face models from OpenCV, <span class="people-num">${mb} MB</span>, downloaded once from GitHub and Hugging Face. After that, no photo and no face leaves this computer.</p>
                <div class="people-actions"><button type="button" class="btn btn-accent people-install">Download and turn on</button></div>
                <div class="people-install-activity"></div>
            </div>`;
        const btn = this._body.querySelector('.people-install');
        btn.addEventListener('click', async () => {
            const restore = Activity.button(btn, 'Downloading…');
            const activity = Activity.in(this._body.querySelector('.people-install-activity'), `Downloading ${mb} MB…`);
            try {
                await PeopleAPI.install();
                activity.done('Face recognition is on. Your libraries are being looked at now.');
                setTimeout(() => this.show(this.route), 1500);
            } catch (err) {
                restore();
                activity.fail(`${err.message} Check the connection and try again.`);
            }
        });
    }

    _showNoLibraries() {
        this._body.innerHTML = `<div class="people-note"><p>People works on libraries. Make one in ${placeLink('library', 'libraries', 'Libraries')} first; its photos are looked at as soon as it is read.</p></div>`;
    }

    // —— Shared by the pages ——

    backLink() {
        return `<a class="people-back" href="${peopleHash()}">People</a>`;
    }

    // crumb is the way back to everyone and the page's title.
    crumb(title, back = this.backLink()) {
        return `${back}<span class="people-crumb-sep" aria-hidden="true">/</span><h2 class="people-page-title">${title}</h2>`;
    }
}

// openFacePhotos opens the photos of faces, each once, in the viewer, at
// the photo of faces[index]; a face without a library is in lib.
function openFacePhotos(lib, faces, index) {
    const key = (f) => `${f.library || lib}/${f.photoId}`;
    const seen = new Map();
    for (const f of faces) if (!seen.has(key(f))) seen.set(key(f), { lib: f.library || lib, id: f.photoId, name: f.file || f.photoId });
    openLibraryPhotos([...seen.values()], [...seen.keys()].indexOf(key(faces[index])));
}

// suggestionHTML asks whether a face or a group is the person it looks
// like, as a sentence with its answers beside it: Yes takes the name, No
// says it is not them, which keeps the question from coming back.
function suggestionHTML(person) {
    const n = escapeHtml(person.name);
    return `<div class="people-suggest">
        <span class="people-suggest-question">Is this <strong>${n}</strong>?</span>
        <span class="people-suggest-answers">
            <button type="button" class="btn btn-sm people-accept" data-name="${n}" title="Yes, this is ${n}" aria-label="Yes, this is ${n}">Yes</button>
            <button type="button" class="btn btn-sm people-not" data-person="${person.id}" title="No, this is not ${n}" aria-label="No, this is not ${n}">No</button>
        </span>
    </div>`;
}

// reportSkipped says when names could not go into some sidecars.
function reportSkipped(result) {
    if (!result?.skipped) return;
    const photos = result.skipped === 1 ? '1 photo keeps its' : `${formatCount(result.skipped)} photos keep their`;
    App.showToast(`${photos} names in this library only: another program keeps the faces in the sidecar.`);
}

// showPeopleError puts a sentence into a page's error line.
function showPeopleError(el, text) {
    el.hidden = false;
    el.textContent = text;
}
