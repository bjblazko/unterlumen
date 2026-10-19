// people-place.js — People: the faces found in a library's photos, grouped
// by who they look like, and the people their owner named (ADR-0051).
//
// Addresses: #people, #people/<library>, #people/<library>/group/<n> and
// #people/<library>/person/<n>. Groups are rebuilt after every pass, so a
// group's address is only good until the next one. #people/all is everyone
// in all libraries (people-everyone.js).

const PEOPLE_LIBRARY_KEY = 'people-library';

function peopleRouteFromHash(hash) {
    const [, library = '', view = '', id = ''] = hash.replace(/^#/, '').split('/');
    return { library: decodeURIComponent(library), view, id: decodeURIComponent(id) };
}

function peopleHash({ library = '', view = '', id = '' } = {}) {
    if (!library) return '#people';
    // Groups are numbered from 0, so an id of 0 is an id.
    return `#people/${encodeURIComponent(library)}` + (view && String(id) !== '' ? `/${view}/${encodeURIComponent(id)}` : '');
}

class PeoplePane {
    constructor(container) {
        this.container = container;
        this.route = { library: '' };
        this.libraries = [];
        this.overviewData = null;
        this._everyone = new PeopleEveryone(this);
        this._phone = new PeoplePhone(this);
        App.statusLine?.onFinished((job) => {
            if (job.place === 'people' && App.mode === 'people') this.show(this.route);
        });
    }

    async show(route) {
        if (!this.container.firstChild) this._buildShell();
        let status;
        try {
            [status, this.libraries] = await Promise.all([PeopleAPI.status(), LibraryAPI.list()]);
        } catch (err) {
            this._fail(`People could not be read: ${err.message}. Reload the page to try again.`);
            return;
        }
        this.route = { ...route, library: this._chooseLibrary(route.library) };
        this._head();
        if (!status.installed) { this._showInstall(status.plan); return; }
        if (!this.route.library) { this._showNoLibraries(); return; }
        if (peopleOnPhone()) { await this._showOnPhone(route); return; }
        try { localStorage.setItem(PEOPLE_LIBRARY_KEY, this.route.library); } catch { /* private window */ }
        if (this.route.library === PEOPLE_ALL) {
            if (this.route.view === 'person') await this._everyone.showPerson(this.route.id);
            else await this._everyone.showOverview();
        } else if (this.route.view === 'group') await this._showGroup(Number(this.route.id));
        else if (this.route.view === 'person') await this._showPerson(Number(this.route.id));
        else await this._showOverview();
    }

    // A phone finds people and shows their photos; everything else about
    // People is work for the computer (people-phone.js).
    async _showOnPhone(route) {
        if (route.view === 'person' && route.library === PEOPLE_ALL) await this._phone.showPerson(route.id);
        else if (route.view === 'person' && route.library) await this._phone.showPersonOf(route.library, Number(route.id));
        else await this._phone.showOverview();
    }

    // The library asked for, else the one shown last, else the first. All
    // libraries is a choice once there are two.
    _chooseLibrary(asked) {
        const ids = this.libraries.map(l => String(l.id));
        if (ids.length > 1) ids.push(PEOPLE_ALL);
        let last = '';
        try { last = localStorage.getItem(PEOPLE_LIBRARY_KEY) || ''; } catch { /* private window */ }
        return [asked, last, ids[0] || ''].find(id => id && ids.includes(id)) || '';
    }

    _buildShell() {
        this.container.innerHTML = `
            <div class="people-place">
                <div class="people-head">
                    <h1 class="people-title">People</h1>
                    <span class="people-count"></span>
                    <span class="people-head-spacer"></span>
                    <label class="people-library-label" hidden>Library
                        <select class="btn btn-sm select-btn people-library"></select>
                    </label>
                    ${placeLede(`The faces found in a library's photos, grouped by who they look like. Names you give are written into each photo's sidecar, where your other installation and programs such as Lightroom read them.`)}
                </div>
                <div class="people-body"></div>
            </div>`;
        this._body = this.container.querySelector('.people-body');
        this._select = this.container.querySelector('.people-library');
        this._select.addEventListener('change', () => App.openPeople({ library: this._select.value }));
    }

    _head(count = '') {
        const label = this.container.querySelector('.people-library-label');
        label.hidden = this.libraries.length < 2;
        this._select.innerHTML = `<option value="${PEOPLE_ALL}">All libraries</option>` + this.libraries.map(l =>
            `<option value="${escapeHtml(String(l.id))}">${escapeHtml(l.name)}</option>`).join('');
        this._select.value = this.route.library;
        this.container.querySelector('.people-count').textContent = count;
    }

    _fail(text) {
        Activity.in(this._body, '', { area: true }).fail(text);
    }

    _go(route, { replace = false } = {}) {
        App.openPeople({ library: this.route.library, ...route }, { replace });
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

    // —— Overview ——

    async _showOverview() {
        const activity = Activity.in(this._body, 'Reading the faces…', { area: true });
        let data;
        try {
            data = await PeopleAPI.overview(this.route.library);
        } catch (err) {
            activity.fail(`The faces could not be read: ${err.message}.`);
            return;
        }
        this.overviewData = data;
        this._head(this._overviewCount(data));
        this._body.innerHTML = `
            ${this._pendingNote(data)}
            <section class="people-section">
                <h2 class="people-section-title">Named</h2>
                ${data.people.length ? `<div class="people-grid">${data.people.map(p => this._personCard(p)).join('')}</div>`
                    : `<p class="people-empty">Nobody is named yet. Give a group below a name.</p>`}
            </section>
            <section class="people-section">
                <h2 class="people-section-title">Who is this?</h2>
                ${data.groupCount > data.groups.length ? `<p class="people-hint">The ${formatCount(data.groups.length)} largest of ${formatCount(data.groupCount)} groups. Naming them brings up the next.</p>` : ''}
                ${data.groups.length ? data.groups.map(g => this._groupRow(g)).join('')
                    : `<p class="people-empty">${data.people.length || data.pending ? 'No faces are waiting for a name.' : 'No faces were found in the photos of this library.'}</p>`}
            </section>
            <datalist id="people-names">${data.people.map(p => `<option value="${escapeHtml(p.name)}">`).join('')}</datalist>`;
        this._wireGroups();
    }

    _overviewCount(d) {
        const parts = [];
        if (d.people.length) parts.push(d.people.length === 1 ? '1 person' : `${formatCount(d.people.length)} people`);
        if (d.groupCount) parts.push(d.groupCount === 1 ? '1 group to name' : `${formatCount(d.groupCount)} groups to name`);
        return parts.join(' · ');
    }

    _pendingNote(d) {
        if (!d.pending) return '';
        const photos = d.pending === 1 ? '1 photo' : `${formatCount(d.pending)} photos`;
        return `<p class="people-pending">${d.finding
            ? `Looking for faces: <span class="people-num">${photos}</span> still to go. This page fills in when it is done.`
            : `<span class="people-num">${photos}</span> have not been looked at yet. That happens after the library's next scan.`}</p>`;
    }

    _personCard(p) {
        const photos = p.photos === 1 ? '1 photo' : `${formatCount(p.photos)} photos`;
        return `
            <a class="people-card" href="${peopleHash({ library: this.route.library, view: 'person', id: p.id })}" data-person="${p.id}">
                <img src="${PeopleAPI.faceURL(this.route.library, p.cover)}" alt="" loading="lazy" width="96" height="96">
                <span class="people-card-name">${escapeHtml(p.name)}</span>
                <span class="people-card-count">${photos}</span>
            </a>`;
    }

    _groupRow(g) {
        const href = peopleHash({ library: this.route.library, view: 'group', id: g.id });
        const faces = g.faces.map(f => `<img src="${PeopleAPI.faceURL(this.route.library, f)}" alt="" loading="lazy" width="64" height="64">`).join('');
        const inputId = `people-name-${g.id}`;
        const suggestion = g.suggestion
            ? `<button type="button" class="btn people-accept" data-name="${escapeHtml(g.suggestion.name)}">This is ${escapeHtml(g.suggestion.name)}</button>` : '';
        return `
            <form class="people-group" data-group="${g.id}">
                <a class="people-group-faces" href="${href}" aria-label="All ${g.size} faces of this group">${faces}</a>
                <span class="people-group-count"><a href="${href}">${g.size === 1 ? '1 face' : `${formatCount(g.size)} faces`}</a></span>
                <div class="people-group-name">
                    <label class="people-label" for="${inputId}">Name</label>
                    <div class="people-group-row">
                        <input class="form-input people-input" id="${inputId}" type="text" list="people-names" autocomplete="off" required>
                        <button type="submit" class="btn">Name</button>
                        ${suggestion}
                        <button type="button" class="btn people-hide" title="Faces nobody is to name: strangers, statues, posters">Hide</button>
                    </div>
                    <p class="people-group-error" hidden></p>
                </div>
            </form>`;
    }

    _wireGroups() {
        for (const form of this._body.querySelectorAll('.people-group')) {
            const grp = Number(form.dataset.group);
            form.addEventListener('submit', (e) => {
                e.preventDefault();
                this._changeGroup(form, grp, (faces) => PeopleAPI.name(this.route.library, faces, form.querySelector('.people-input').value));
            });
            form.querySelector('.people-accept')?.addEventListener('click', (e) =>
                this._changeGroup(form, grp, (faces) => PeopleAPI.name(this.route.library, faces, e.target.dataset.name)));
            form.querySelector('.people-hide').addEventListener('click', () =>
                this._changeGroup(form, grp, (faces) => PeopleAPI.hide(this.route.library, faces)));
        }
    }

    // _changeGroup applies change to every face of the group, then reads
    // the overview again: the group is gone, a person may be new.
    async _changeGroup(form, grp, change) {
        const error = form.querySelector('.people-group-error');
        const buttons = [...form.querySelectorAll('button')];
        buttons.forEach(b => { b.disabled = true; });
        try {
            const faces = (await PeopleAPI.groupFaces(this.route.library, grp)).map(f => f.id);
            reportSkipped(await change(faces));
            await this._showOverview();
        } catch (err) {
            buttons.forEach(b => { b.disabled = false; });
            error.hidden = false;
            error.textContent = `${err.message} Nothing was changed.`;
        }
    }

    // —— A group ——

    async _showGroup(grp) {
        const activity = Activity.in(this._body, 'Reading the group…', { area: true });
        let faces;
        try {
            faces = await PeopleAPI.groupFaces(this.route.library, grp);
        } catch (err) {
            activity.fail(`The group could not be read: ${err.message}.`);
            return;
        }
        if (!faces.length) {
            this._body.innerHTML = `<div class="people-note"><p>This group is gone: its faces were named, or the faces were grouped anew since. ${this._backLink()}</p></div>`;
            return;
        }
        this._head(`${formatCount(faces.length)} faces`);
        const grid = new FaceGrid({
            lib: this.route.library, faces, selected: faces.map(f => f.id), dimOff: true,
            onChange: () => update(),
            onOpen: (i) => this._openPhotos(faces, i),
        });
        this._body.innerHTML = `
            <div class="people-page-head">${this._crumb('Who is this?')}</div>
            <p class="people-hint">Switch off the faces that are someone else; they stay unnamed. Rest the pointer on a face to see it in its photo; double-click to open the photo.</p>
            <div class="people-grid-wrap"></div>
            <form class="people-bar">
                <label class="people-label" for="people-group-name">Name</label>
                <div class="people-group-row">
                    <input class="form-input people-input" id="people-group-name" type="text" list="people-names" autocomplete="off" required>
                    <button type="button" class="btn people-hide">Hide</button>
                    <button type="submit" class="btn btn-accent people-submit">Name</button>
                </div>
                <p class="people-group-error" hidden></p>
            </form>
            <datalist id="people-names">${(this.overviewData?.people || []).map(p => `<option value="${escapeHtml(p.name)}">`).join('')}</datalist>`;
        this._body.querySelector('.people-grid-wrap').appendChild(grid.el);
        const bar = this._body.querySelector('.people-bar');
        const submit = bar.querySelector('.people-submit');
        const hide = bar.querySelector('.people-hide');
        const update = () => {
            const n = grid.selected.size;
            submit.textContent = n === 1 ? 'Name 1 face' : `Name ${formatCount(n)} faces`;
            hide.textContent = n === 1 ? 'Hide 1 face' : `Hide ${formatCount(n)} faces`;
            submit.disabled = hide.disabled = n === 0;
        };
        update();
        bar.addEventListener('submit', (e) => {
            e.preventDefault();
            this._changeSelected(bar, grid, (ids) => PeopleAPI.name(this.route.library, ids, bar.querySelector('.people-input').value));
        });
        hide.addEventListener('click', () => this._changeSelected(bar, grid, (ids) => PeopleAPI.hide(this.route.library, ids)));
        bar.querySelector('.people-input').focus();
    }

    async _changeSelected(bar, grid, change) {
        const buttons = [...bar.querySelectorAll('button')];
        buttons.forEach(b => { b.disabled = true; });
        try {
            reportSkipped(await change([...grid.selected]));
            this._go({}, { replace: true });
        } catch (err) {
            buttons.forEach(b => { b.disabled = false; });
            const error = bar.querySelector('.people-group-error');
            error.hidden = false;
            error.textContent = `${err.message} Nothing was changed.`;
        }
    }

    // —— A person ——

    async _showPerson(id) {
        const activity = Activity.in(this._body, 'Reading the faces…', { area: true });
        let faces, person, everyone, birthdays;
        try {
            [faces, this.overviewData, everyone, birthdays] = await Promise.all([
                PeopleAPI.personFaces(this.route.library, id), PeopleAPI.overview(this.route.library),
                this.libraries.length > 1 ? PeopleAPI.everyone() : [], peopleBirthdays()]);
            person = this.overviewData.people.find(p => p.id === id);
        } catch (err) {
            activity.fail(`The faces could not be read: ${err.message}.`);
            return;
        }
        if (!person) {
            this._body.innerHTML = `<div class="people-note"><p>Nobody of this library has this address any more. ${this._backLink()}</p></div>`;
            return;
        }
        const photos = person.photos === 1 ? '1 photo' : `${formatCount(person.photos)} photos`;
        this._head(`${formatCount(faces.length)} faces in ${photos}`);
        const elsewhere = this._namedElsewhere(everyone, person.name);
        const grid = new FaceGrid({
            lib: this.route.library, faces, cover: person.cover,
            onChange: () => update(),
            onOpen: (i) => this._openPhotos(faces, i),
        });
        const menu = new Menu({ label: 'Person options', items: () => [
            { label: 'Rename…', onSelect: () => this._rename(person, elsewhere) },
            { label: 'Birthday…', onSelect: () => editBirthday(person.name, birthdayOf(birthdays, person.name), () => this.show(this.route)) },
            { label: 'Take the name away…', onSelect: () => this._forget(person, faces.length, elsewhere) },
        ] });
        this._body.innerHTML = `
            <div class="people-page-head">${this._crumb(escapeHtml(person.name))}
                <span class="people-head-spacer"></span>
                <a class="btn people-show" href="#libraries">Show photos</a>
            </div>
            ${bornHTML(birthdayOf(birthdays, person.name))}
            <p class="people-hint">Switch on a face that is someone else, then take it away from ${escapeHtml(person.name)}; switch on one face to use it as the cover. Rest the pointer on a face to see it in its photo; Show photos finds ${escapeHtml(person.name)} in every library.${this._elsewhereNote(person.name, elsewhere)}</p>
            <div class="people-grid-wrap"></div>
            <div class="people-bar" hidden>
                <div class="people-group-row">
                    <button type="button" class="btn people-cover" hidden>Use as cover</button>
                    <button type="button" class="btn people-unname"></button>
                </div>
                <p class="people-group-error" hidden></p>
            </div>`;
        this._body.querySelector('.people-show').after(menu.button);
        this._body.querySelector('.people-grid-wrap').appendChild(grid.el);
        showPhotosLink(this._body.querySelector('.people-show'), { person: person.name });
        const bar = this._body.querySelector('.people-bar');
        const unname = bar.querySelector('.people-unname');
        const cover = bar.querySelector('.people-cover');
        const update = () => {
            const n = grid.selected.size;
            bar.hidden = n === 0;
            cover.hidden = n !== 1 || grid.selected.has(person.cover);
            unname.textContent = n === 1 ? `This is not ${person.name}` : `These ${formatCount(n)} are not ${person.name}`;
        };
        cover.addEventListener('click', async () => {
            cover.disabled = true;
            try {
                await PeopleAPI.chooseCover(this.route.library, person.id, [...grid.selected][0]);
                this.show(this.route);
            } catch (err) {
                cover.disabled = false;
                const error = bar.querySelector('.people-group-error');
                error.hidden = false;
                error.textContent = `${err.message} The cover was not changed.`;
            }
        });
        unname.addEventListener('click', async () => {
            unname.disabled = true;
            try {
                reportSkipped(await PeopleAPI.unname(this.route.library, [...grid.selected]));
                this.show(this.route);
            } catch (err) {
                unname.disabled = false;
                const error = bar.querySelector('.people-group-error');
                error.hidden = false;
                error.textContent = `${err.message} Nothing was changed.`;
            }
        });
    }

    // The libraries other than this one where the name is given, from the
    // list of everyone.
    _namedElsewhere(everyone, name) {
        const someone = everyone.find(p => p.name.toLowerCase() === name.toLowerCase());
        return (someone?.libraries || []).filter(l => l.library !== this.route.library);
    }

    _elsewhereNote(name, elsewhere) {
        if (!elsewhere.length) return '';
        const links = elsewhere.map(l =>
            `<a href="${peopleHash({ library: l.library, view: 'person', id: l.person })}">${escapeHtml(l.libraryName)}</a>`);
        const all = `<a href="${peopleHash({ library: PEOPLE_ALL, view: 'person', id: name })}">${escapeHtml(name)} in all libraries</a>`;
        return ` ${escapeHtml(name)} is also named in ${sentenceList(links)}; see ${all}.`;
    }

    _libraryName() {
        return this.libraries.find(l => String(l.id) === this.route.library)?.name || 'this library';
    }

    _rename(person, elsewhere) {
        const dlg = new Dialog({
            title: 'Rename',
            subtitle: person.name,
            size: 'sm',
            className: 'library-dialog',
            body: `
                <label class="library-dialog-label" for="people-rename">Name</label>
                <input class="library-dialog-input" id="people-rename" type="text" autocomplete="off" value="${escapeHtml(person.name)}">
                <span class="form-hint">If someone else already has this name, the two become one person.</span>
                ${elsewhere.length ? `<span class="form-hint">This changes the name in ${escapeHtml(this._libraryName())} only. ${escapeHtml(person.name)} is also named in ${escapeHtml(sentenceList(elsewhere.map(l => l.libraryName)))}: rename there too, so they stay one person.</span>` : ''}
                <p class="people-group-error" hidden></p>`,
            actions: [
                { label: 'Cancel', value: null },
                { label: 'Rename', kind: 'accent', onClick: async (d) => {
                    const name = d.el.querySelector('#people-rename').value.trim();
                    if (!name) return;
                    try {
                        reportSkipped(await PeopleAPI.rename(this.route.library, person.id, name));
                        d.close(true);
                        this._go({}, { replace: true });
                    } catch (err) {
                        const error = d.el.querySelector('.people-group-error');
                        error.hidden = false;
                        error.textContent = `${err.message} The name was not changed.`;
                    }
                } },
            ],
        });
        dlg.open().querySelector('#people-rename').select();
    }

    // _forget takes the person's name from all their faces, which go back to
    // the groups waiting for a name.
    _forget(person, count, elsewhere) {
        const faces = count === 1 ? '1 face goes' : `${formatCount(count)} faces go`;
        new Dialog({
            title: `Take the name ${person.name} away?`,
            size: 'sm',
            className: 'library-dialog',
            body: `
                <p>Its ${faces} back to Who is this?, and the name leaves the photos' sidecars, also for your other installation.</p>
                ${elsewhere.length ? `<p>This is ${escapeHtml(this._libraryName())} only: the name stays in ${escapeHtml(sentenceList(elsewhere.map(l => l.libraryName)))}.</p>` : ''}
                <p class="people-group-error" hidden></p>`,
            actions: [
                { label: 'Cancel', value: null },
                { label: 'Take the name away', kind: 'accent', onClick: async (d) => {
                    try {
                        reportSkipped(await PeopleAPI.forget(this.route.library, person.id));
                        d.close(true);
                        this._go({}, { replace: true });
                    } catch (err) {
                        const error = d.el.querySelector('.people-group-error');
                        error.hidden = false;
                        error.textContent = `${err.message} Nothing was changed.`;
                    }
                } },
            ],
        }).open();
    }

    _backLink() {
        return `<a class="people-back" href="${peopleHash({ library: this.route.library })}">People</a>`;
    }

    // _crumb is the way back to the overview and the page's title.
    _crumb(title) {
        return `${this._backLink()}<span class="people-crumb-sep" aria-hidden="true">/</span><h2 class="people-page-title">${title}</h2>`;
    }

    // The photos of the faces, each once, in the viewer.
    _openPhotos(faces, index) {
        const seen = new Map();
        for (const f of faces) if (!seen.has(f.photoId)) seen.set(f.photoId, seen.size);
        const photos = [...seen.keys()].map(id => ({ lib: this.route.library, id, name: id }));
        openLibraryPhotos(photos, seen.get(faces[index].photoId));
    }
}

// reportSkipped says when names could not go into some sidecars.
function reportSkipped(result) {
    if (!result?.skipped) return;
    const photos = result.skipped === 1 ? '1 photo keeps its' : `${formatCount(result.skipped)} photos keep their`;
    App.showToast(`${photos} names in this library only: another program keeps the faces in the sidecar.`);
}
