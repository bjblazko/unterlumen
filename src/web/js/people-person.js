// people-person.js — A person's page on the computer, by name (ADR-0051):
// their faces in each library that has the name, managed there — cover,
// faces that are someone else, rename, take the name away — and in the
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
        this.pane._head(`${countFaces(faces)} in ${countPhotos(photos)}`);
        body.innerHTML = `
            <div class="people-page-head">${this.pane.crumb(escapeHtml(name))}
                <span class="people-head-spacer"></span>
                <a class="btn people-show" href="#libraries">Show photos</a>
            </div>
            ${bornHTML(birthdayOf(birthdays, name))}
            ${oftenWithHTML(data.with)}
            <p class="people-hint">Select faces to give them to someone else or to choose the cover. Rest the pointer on a face to see it in its photo; double-click opens the photo.</p>
            ${data.in.map(l => this._sectionHTML(l)).join('')}
            ${this._notInHTML(name, data.notIn)}
            <div class="people-bar" hidden>
                <div class="people-group-row">
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
        this._wireSections(name);
        this._wireNotIn(name);
    }

    _sectionHTML(l) {
        return `
            <section class="people-section people-in" data-library="${escapeHtml(l.library)}">
                <div class="people-in-head">
                    <button type="button" class="people-fold" aria-expanded="true">
                        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="3 4.5 6 7.5 9 4.5"/></svg>
                        <h3 class="people-section-title">${escapeHtml(l.libraryName)}</h3>
                    </button>
                    <span class="people-count">${countFaces(l.faces)} in ${countPhotos(l.photos)}</span>
                </div>
                <div class="people-in-faces"></div>
            </section>`;
    }

    // Each library's faces are a FaceGrid; a selection is in one library at
    // a time, since a face's number means something only in its own.
    _wireSections(name) {
        const bar = this.pane._body.querySelector('.people-bar');
        this._grids = new Map();
        for (const l of this.data.in) {
            const section = this.pane._body.querySelector(`.people-in[data-library="${CSS.escape(l.library)}"]`);
            const grid = new FaceGrid({
                lib: l.library, faces: l.shown, cover: l.cover,
                onChange: () => {
                    for (const [other, g] of this._grids) if (other !== l.library && g.selected.size) g.clear();
                    this._selected = { l, grid };
                    this._updateBar(name);
                },
                onOpen: (i) => openFacePhotos(l.library, l.shown, i),
            });
            this._grids.set(l.library, grid);
            section.querySelector('.people-in-faces').appendChild(grid.el);
            const fold = section.querySelector('.people-fold');
            fold.addEventListener('click', () => {
                const open = fold.getAttribute('aria-expanded') !== 'true';
                fold.setAttribute('aria-expanded', String(open));
                grid.el.hidden = !open;
            });
        }
        bar.querySelector('.people-cover').addEventListener('click', (e) => this._act(e.currentTarget, 'The cover was not changed.',
            ({ l, grid }) => PeopleAPI.chooseCover(l.library, l.person, [...grid.selected][0])));
        bar.querySelector('.people-unname').addEventListener('click', (e) => this._act(e.currentTarget, 'Nothing was changed.',
            ({ l, grid }) => PeopleAPI.unname(l.library, [...grid.selected])));
        const move = () => {
            const other = bar.querySelector('#people-instead').value.trim();
            if (!other) { bar.querySelector('#people-instead').focus(); return; }
            this._act(bar.querySelector('.people-move'), 'Nothing was changed.', ({ l, grid }) => PeopleAPI.name(l.library, [...grid.selected], other));
        };
        bar.querySelector('.people-move').addEventListener('click', move);
        bar.querySelector('#people-instead').addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); move(); } });
    }

    _updateBar(name) {
        const bar = this.pane._body.querySelector('.people-bar');
        const { l, grid } = this._selected;
        const n = grid.selected.size;
        bar.hidden = n === 0;
        bar.querySelector('.people-cover').hidden = n !== 1 || grid.selected.has(l.cover);
        bar.querySelector('.people-unname').textContent = n === 1 ? `Not ${name}` : `${formatCount(n)} are not ${name}`;
    }

    // _act runs a change on the selection and shows the page again; a
    // failure says so in the bar and changes nothing on the page.
    async _act(btn, unchanged, change) {
        const bar = this.pane._body.querySelector('.people-bar');
        const restore = Activity.button(btn, btn.textContent);
        try {
            reportSkipped(await change(this._selected));
            await this.show(this.data.name);
        } catch (err) {
            restore();
            showPeopleError(bar.querySelector('.people-group-error'), `${err.message} ${unchanged}`);
        }
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
                ${escapeHtml(l.libraryName)} <span class="people-num">${escapeHtml(countPhotos(l.photos))}</span></label>`).join('')}
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
