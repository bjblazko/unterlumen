// people-person.js — A person's page on the computer, by name (ADR-0051):
// their faces from all libraries that have the name, newest photo first,
// each managed in its own library — cover, faces that are someone else,
// rename, take the name away — and in the
// libraries without the name, who there looks like them, put right in place.
//
// Address: #people/person/<name>.

class PeoplePerson {
    // pane is the PeoplePane that hosts the page.
    constructor(pane) {
        this.pane = pane;
    }

    async show(name) {
        const body = this.pane._body;
        const activity = Activity.in(body, `Reading ${name}…`, { area: true });
        let data, birthdays, everyone;
        try {
            [data, birthdays, everyone] = await Promise.all([PeopleAPI.someone(name), peopleBirthdays(), PeopleAPI.everyone()]);
        } catch (err) {
            activity.fail(`${name} could not be read: ${err.message}.`);
            return;
        }
        if (!data.in.length) {
            body.innerHTML = `<div class="people-note"><p>Nobody is named ${escapeHtml(name)} any more. ${this.pane.backLink()}</p></div>`;
            return;
        }
        this.data = data;
        name = data.name;
        const faces = data.in.reduce((n, l) => n + l.faces, 0);
        const photos = data.in.reduce((n, l) => n + l.photos, 0);
        this.pane._head(`${countHTML(faces, 'face')} in ${countHTML(photos, 'photo')}`);
        body.innerHTML = `
            <div class="people-page-head">${this.pane.crumb(escapeHtml(name))}
                <span class="people-head-spacer"></span>
                <a class="btn people-show" href="#libraries">Show photos</a>
            </div>
            ${bornHTML(birthdayOf(birthdays, name))}
            ${oftenWithHTML(data.with)}
            <p class="people-hint">Select a face to open its photo, show it in its library or make it the cover; select several to give them to someone else. Rest the pointer on a face to see it in its photo; double-click opens the photo.</p>
            ${this._facesHTML()}
            ${this._notInHTML(name, data.notIn)}
            <div class="people-bar" hidden>
                <div class="people-group-row">
                    <span class="people-bar-where" hidden></span>
                    <button type="button" class="btn people-open" hidden>Open photo</button>
                    <button type="button" class="btn people-in-library" hidden>Show in library</button>
                    <button type="button" class="btn people-cover" hidden>Use as cover</button>
                    <label class="people-instead-label" for="people-instead">Is instead</label>
                    <input class="form-input people-input" id="people-instead" type="text" list="people-names" autocomplete="off">
                    <button type="button" class="btn people-move">Move</button>
                    <button type="button" class="btn people-unname"></button>
                </div>
                <p class="people-group-error" hidden></p>
            </div>
            ${namesList(everyone.map(p => p.name).filter(n => n.toLowerCase() !== name.toLowerCase()))}`;
        const menu = new Menu({ label: 'Person options', items: () => [
            { label: 'Rename…', onSelect: () => this._rename(name) },
            { label: 'Birthday…', onSelect: () => editBirthday(name, birthdayOf(birthdays, name), () => this.show(name)) },
            { label: 'Take the name away…', onSelect: () => this._forget(name) },
        ] });
        body.querySelector('.people-show').after(menu.button);
        showPhotosLink(body.querySelector('.people-show'), { person: name });
        wireOftenWith(body, name);
        this._wireFaces(name);
        this._wireNotIn(name);
    }

    // One grid of all their faces, newest photo first; where they are in
    // several libraries, how many faces each holds.
    _facesHTML() {
        const where = this.data.in.length < 2 ? ''
            : `<p class="people-hint">${escapeHtml(sentenceList(this.data.in.map(l => `${countFaces(l.faces)} in ${l.libraryName}`)))}.</p>`;
        return `<section class="people-section people-faces">${where}</section>`;
    }

    // A change applies in the library of the selection: the grid keeps the
    // selection in one library at a time.
    _wireFaces(name) {
        const bar = this.pane._body.querySelector('.people-bar');
        const faces = this.data.faces;
        this._grid = new FaceGrid({
            faces, cover: this.data.cover,
            onChange: () => this._updateBar(name),
            onOpen: (i) => openFacePhotos(null, faces, i),
        });
        this.pane._body.querySelector('.people-faces').appendChild(this._grid.el);
        const one = () => this._grid.selectedFaces()[0];
        bar.querySelector('.people-open').addEventListener('click', () => openFacePhotos(null, faces, this._grid.selectedIndexes()[0]));
        bar.querySelector('.people-in-library').addEventListener('click', () => showPhotoInLibrary({ lib: one().library, id: one().photoId }));
        bar.querySelector('.people-cover').addEventListener('click', (e) => this._act(e.currentTarget, 'The cover was not changed.',
            ({ l, ids }) => PeopleAPI.chooseCover(l.library, l.person, ids[0])));
        bar.querySelector('.people-unname').addEventListener('click', (e) => this._act(e.currentTarget, 'Nothing was changed.',
            ({ l, ids }) => PeopleAPI.unname(l.library, ids)));
        const move = () => {
            const other = bar.querySelector('#people-instead').value.trim();
            if (!other) { bar.querySelector('#people-instead').focus(); return; }
            this._act(bar.querySelector('.people-move'), 'Nothing was changed.', ({ l, ids }) => PeopleAPI.name(l.library, ids, other));
        };
        bar.querySelector('.people-move').addEventListener('click', move);
        bar.querySelector('#people-instead').addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); move(); } });
    }

    // _byLibrary groups the selected faces by their library: a face's
    // number, and the person, mean something only there.
    _byLibrary() {
        const by = new Map();
        for (const f of this._grid.selectedFaces()) {
            const key = String(f.library);
            if (!by.has(key)) by.set(key, { l: this.data.in.find(l => String(l.library) === key), ids: [] });
            by.get(key).ids.push(f.id);
        }
        return [...by.values()];
    }

    _updateBar(name) {
        const bar = this.pane._body.querySelector('.people-bar');
        const n = this._grid.count;
        const libs = this._byLibrary();
        const first = this._grid.selectedFaces()[0];
        const cover = this.data.cover;
        bar.hidden = n === 0;
        if (!n) return;
        const where = bar.querySelector('.people-bar-where');
        where.hidden = this.data.in.length < 2;
        where.textContent = libs.length === 1 ? `In ${libs[0].l.libraryName}` : `In ${countLibraries(libs.length)}`;
        bar.querySelector('.people-open').hidden = n !== 1;
        bar.querySelector('.people-in-library').hidden = n !== 1;
        bar.querySelector('.people-cover').hidden = n !== 1 || (String(cover.library) === String(first.library) && cover.face === first.id);
        bar.querySelector('.people-unname').textContent = n === 1 ? `Not ${name}` : `${formatCount(n)} are not ${name}`;
    }

    // _act runs a change on the selected faces of each library and shows
    // the page again. A library where it fails is named; when nothing
    // changed anywhere, the page stays as it is and says so in the bar.
    async _act(btn, unchanged, change) {
        const bar = this.pane._body.querySelector('.people-bar');
        const restore = Activity.button(btn, btn.textContent);
        const groups = this._byLibrary();
        const failed = [];
        let skipped = 0;
        for (const g of groups) {
            try {
                skipped += (await change(g))?.skipped || 0;
            } catch (err) {
                failed.push(groups.length > 1 ? `In ${g.l.libraryName}: ${err.message}` : err.message);
            }
        }
        reportSkipped({ skipped });
        if (!failed.length) {
            await this.show(this.data.name);
            return;
        }
        if (failed.length < groups.length) {
            await this.show(this.data.name);
            App.showToast(`${failed.join(' ')} Nothing was changed there; the other libraries were.`);
            return;
        }
        restore();
        showPeopleError(bar.querySelector('.people-group-error'), `${failed.join(' ')} ${unchanged}`);
    }

    // —— Not named in ——

    // Only the libraries where someone looks like the person get a section
    // of their own; the others are one line.
    _notInHTML(name, notIn) {
        const suggested = notIn.filter(l => l.looksLike || l.group);
        const others = notIn.filter(l => !l.looksLike && !l.group);
        if (!notIn.length) return '';
        return `
            <section class="people-section">
                ${suggested.length ? `<h2 class="people-section-title">Not named in</h2>${suggested.map(l => this._suggestionHTML(name, l)).join('')}` : ''}
                ${others.length ? `<p class="people-hint">Not in ${escapeHtml(sentenceList(others.map(l => l.libraryName)))}.</p>` : ''}
            </section>`;
    }

    _suggestionHTML(name, l) {
        const lib = escapeHtml(l.libraryName);
        const n = escapeHtml(name);
        if (l.looksLike) {
            const other = l.looksLike;
            return `
                <div class="people-elsewhere" data-library="${escapeHtml(l.library)}">
                    <div class="people-group-faces"><img src="${PeopleAPI.faceURL(l.library, other.cover)}" alt="" loading="lazy" width="64" height="64"></div>
                    <div class="people-elsewhere-text">
                        <p>In ${lib}, ${escapeHtml(other.name)} looks like ${n}. If it is the same person, give them this name there; it goes into the sidecars of their photos.</p>
                        <button type="button" class="btn people-rename-there" data-person="${other.id}">Rename to ${n} in ${lib} (${escapeHtml(countPhotos(other.photos))})</button>
                        <a href="${peopleHash({ view: 'person', id: other.name })}">${escapeHtml(other.name)}</a>
                        <p class="people-group-error" hidden></p>
                    </div>
                </div>`;
        }
        const faces = l.group.faces.map(f => `<img src="${PeopleAPI.faceURL(l.library, f)}" alt="" loading="lazy" width="64" height="64">`).join('');
        return `
            <div class="people-elsewhere" data-library="${escapeHtml(l.library)}">
                <div class="people-group-faces">${faces}</div>
                <div class="people-elsewhere-text">
                    <p>In ${lib}, ${escapeHtml(countFaces(l.group.size))} waiting for a name look like ${n}.</p>
                    <button type="button" class="btn people-name-there" data-group="${l.group.id}">Name them ${n}</button>
                    <a href="${peopleHash({ view: 'group', library: l.library, id: l.group.id })}">See the faces</a>
                    <p class="people-group-error" hidden></p>
                </div>
            </div>`;
    }

    _wireNotIn(name) {
        for (const box of this.pane._body.querySelectorAll('.people-elsewhere')) {
            const lib = box.dataset.library;
            const run = async (btn, change) => {
                const restore = Activity.button(btn, btn.textContent);
                try {
                    reportSkipped(await change());
                    await this.show(name);
                } catch (err) {
                    restore();
                    showPeopleError(box.querySelector('.people-group-error'), `${err.message} Nothing was changed.`);
                }
            };
            box.querySelector('.people-rename-there')?.addEventListener('click', (e) => {
                const person = Number(e.currentTarget.dataset.person);
                run(e.currentTarget, () => PeopleAPI.rename(lib, person, name));
            });
            box.querySelector('.people-name-there')?.addEventListener('click', (e) => {
                const grp = Number(e.currentTarget.dataset.group);
                run(e.currentTarget, async () => PeopleAPI.name(lib, (await PeopleAPI.groupFaces(lib, grp)).map(f => f.id), name));
            });
        }
    }

    // —— Rename, take the name away ——

    // librariesHTML lists the libraries with the name, each ticked: an
    // untick leaves that library as it is — for two people of one name.
    _librariesHTML() {
        if (this.data.in.length < 2) return '';
        return `<fieldset class="people-libraries"><legend class="library-dialog-label">In</legend>${this.data.in.map(l => `
            <label class="people-library-choice"><input type="checkbox" value="${escapeHtml(l.library)}" checked>
                ${escapeHtml(l.libraryName)}<span class="count">${formatCount(l.photos)}</span></label>`).join('')}
            <span class="form-hint">Untick a library where someone else has this name.</span></fieldset>`;
    }

    _chosenLibraries(d) {
        if (this.data.in.length < 2) return this.data.in;
        const ticked = new Set([...d.el.querySelectorAll('.people-library-choice input:checked')].map(i => i.value));
        return this.data.in.filter(l => ticked.has(l.library));
    }

    _rename(name) {
        const dlg = new Dialog({
            title: 'Rename',
            subtitle: name,
            size: 'sm',
            className: 'library-dialog',
            body: `
                <label class="library-dialog-label" for="people-rename">Name</label>
                <input class="library-dialog-input" id="people-rename" type="text" autocomplete="off" value="${escapeHtml(name)}">
                <span class="form-hint">The name goes into the sidecars of their photos. Where someone already has the new name, the two become one person.</span>
                ${this._librariesHTML()}
                <p class="people-group-error" hidden></p>`,
            actions: [
                { label: 'Cancel', value: null },
                { label: 'Rename', kind: 'accent', onClick: async (d) => {
                    const to = d.el.querySelector('#people-rename').value.trim();
                    if (!to) return;
                    await this._eachLibrary(d, (l) => PeopleAPI.rename(l.library, l.person, to), 'The name was not changed there.', { view: 'person', id: to });
                } },
            ],
        });
        dlg.open().querySelector('#people-rename').select();
    }

    _forget(name) {
        new Dialog({
            title: `Take the name ${name} away?`,
            size: 'sm',
            className: 'library-dialog',
            body: `
                <p>Their faces go back to the faces waiting for a name, and the name leaves the photos' sidecars, also for your other installation.</p>
                ${this._librariesHTML()}
                <p class="people-group-error" hidden></p>`,
            actions: [
                { label: 'Cancel', value: null },
                { label: 'Take the name away', kind: 'accent', onClick: async (d) => {
                    await this._eachLibrary(d, (l) => PeopleAPI.forget(l.library, l.person), 'Nothing was changed there.', null);
                } },
            ],
        }).open();
    }

    // _eachLibrary makes the change in each library chosen in the dialog d,
    // one after the other, then goes to then, or shows this page again. A
    // library where it fails is named, and the dialog stays.
    async _eachLibrary(d, change, unchanged, then) {
        const failed = [];
        for (const l of this._chosenLibraries(d)) {
            try {
                reportSkipped(await change(l));
            } catch (err) {
                failed.push(`${l.libraryName}: ${err.message}`);
            }
        }
        if (failed.length) {
            showPeopleError(d.el.querySelector('.people-group-error'), `${failed.join(' ')} ${unchanged}`);
            return;
        }
        d.close(true);
        if (then) this.pane.go(then, { replace: true }); else await this.show(this.data.name);
    }
}
