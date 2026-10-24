// people-naming.js — Waiting for a name: the faces of one library grouped by who they
// look like, waiting for a name (ADR-0051), and one group with all its faces.
//
// Addresses: #people/name/<library>, #people/name/<library>/group/<n> and
// #people/name/<library>/hidden.

class PeopleNaming {
    // pane is the PeoplePane that hosts the page.
    constructor(pane) {
        this.pane = pane;
    }

    // —— A library's groups ——

    async showLibrary(lib) {
        const body = this.pane._body;
        const activity = Activity.in(body, t('Reading the faces…'), { area: true });
        let data, toName, names;
        try {
            [data, toName, names] = await Promise.all([PeopleAPI.overview(lib), PeopleAPI.toName(), PeopleAPI.allNames()]);
        } catch (err) {
            activity.fail(t('The faces could not be read: {err}.', { err: err.message }));
            return;
        }
        this.pane._head(data.groupCount ? countHTML(data.groupCount, 'group') : '');
        body.innerHTML = `
            <div class="people-page-head">${this.pane.crumb(t('Waiting for a name in {lib}', { lib: escapeHtml(this.pane.libraryName(lib)) }))}</div>
            ${this.pane.libraries.length > 1 ? toNameHTML(toName, lib) : ''}
            ${pendingNote(data)}
            ${data.groups.length ? `
                <p class="people-hint">${t('Faces that look alike, the largest groups first. Name them, or hide faces nobody is to name: strangers, statues, posters.')}${data.groupCount > data.groups.length ? ' ' + t('These are {n} of {total}; naming them brings up the next.', { n: formatCount(data.groups.length), total: formatCount(data.groupCount) }) : ''}${hiddenLink(lib, data.hidden)}</p>
                <div class="people-groups">${data.groups.map(g => this._groupRow(lib, g)).join('')}</div>`
                : `<p class="people-empty">${data.people.length || data.pending ? t('No faces are waiting for a name here.') : t('No faces were found in the photos of this library.')}${hiddenLink(lib, data.hidden)}</p>`}
            ${namesList(names)}`;
        for (const form of body.querySelectorAll('.people-group')) this._wireRow(lib, form);
    }

    _groupRow(lib, g) {
        const href = peopleHash({ view: 'group', library: lib, id: g.id });
        const faces = g.faces.map(f => `<img src="${PeopleAPI.faceURL(lib, f)}" alt="" loading="lazy" width="64" height="64">`).join('');
        const inputId = `people-name-${g.id}`;
        return `
            <form class="people-group" data-group="${g.id}">
                <a class="people-group-faces" href="${href}" aria-label="${t('All {n} faces of this group', { n: formatCount(g.size) })}">${faces}</a>
                <div class="people-group-name">
                    <a class="people-group-count" href="${href}">${countFaces(g.size)}</a>
                    ${g.suggestion ? suggestionHTML(g.suggestion) : ''}
                    <label class="people-label" for="${inputId}">${t('Name')}</label>
                    <div class="people-group-row">
                        <input class="form-input people-input" id="${inputId}" type="text" list="people-names" autocomplete="off" required>
                        <button type="submit" class="btn">${t('verb|Name')}</button>
                        <button type="button" class="btn people-hide">${t('Hide')}</button>
                    </div>
                    <p class="people-group-error" hidden></p>
                </div>
            </form>`;
    }

    _wireRow(lib, form) {
        const grp = Number(form.dataset.group);
        const change = (fn) => this._changeGroup(lib, form, grp, fn);
        form.addEventListener('submit', (e) => {
            e.preventDefault();
            change((faces) => PeopleAPI.name(lib, faces, form.querySelector('.people-input').value));
        });
        // The name is read now: the faces come first, and by then the event
        // no longer knows its button.
        form.querySelector('.people-accept')?.addEventListener('click', (e) => {
            const name = e.currentTarget.dataset.name;
            change((faces) => PeopleAPI.name(lib, faces, name));
        });
        form.querySelector('.people-not')?.addEventListener('click', (e) => {
            const person = Number(e.currentTarget.dataset.person);
            change((faces) => PeopleAPI.not(lib, faces, person));
        });
        form.querySelector('.people-hide').addEventListener('click', () => change((faces) => PeopleAPI.hide(lib, faces)));
    }

    // _changeGroup applies change to every face of the group, then reads the
    // groups again: this one is gone, a person may be new.
    async _changeGroup(lib, form, grp, change) {
        const buttons = [...form.querySelectorAll('button')];
        buttons.forEach(b => { b.disabled = true; });
        try {
            const faces = (await PeopleAPI.groupFaces(lib, grp)).map(f => f.id);
            reportSkipped(await change(faces));
            await this.showLibrary(lib);
        } catch (err) {
            buttons.forEach(b => { b.disabled = false; });
            showPeopleError(form.querySelector('.people-group-error'), `${err.message} Nothing was changed.`);
        }
    }

    // —— A group ——

    // A group is often mixed: most of it one person, a face or two someone
    // else. Its page starts from the suggestion, names all or the selected
    // faces, and stays while faces are left, so the rest is named here too.
    async showGroup(lib, grp) {
        const body = this.pane._body;
        const activity = Activity.in(body, t('Reading the group…'), { area: true });
        let faces, names, suggestion;
        try {
            [faces, names, { suggestion }] = await Promise.all([
                PeopleAPI.groupFaces(lib, grp), PeopleAPI.allNames(), PeopleAPI.groupSuggestion(lib, grp)]);
        } catch (err) {
            activity.fail(t('The group could not be read: {err}.', { err: err.message }));
            return;
        }
        const back = `<a class="people-back" href="${peopleHash({ view: 'name', library: lib })}">${t('Waiting for a name in {lib}', { lib: escapeHtml(this.pane.libraryName(lib)) })}</a>`;
        if (!faces.length) {
            body.innerHTML = `<div class="people-note"><p>${t('This group is gone: its faces were named, or the faces were grouped anew since.')} ${back}</p></div>`;
            return;
        }
        this.pane._head(countHTML(faces.length, 'face'));
        const grid = new FaceGrid({ lib, faces, onChange: () => update(), onOpen: (i) => openFacePhotos(lib, faces, i) });
        body.innerHTML = `
            <div class="people-page-head">${this.pane.crumb(t('Who is this?'), back)}</div>
            ${suggestion ? suggestionHTML(suggestion) : ''}
            <p class="people-hint">${t('Name all of them, or select the ones to name first; the others stay here. Rest the pointer on a face to see it in its photo; double-click opens the photo.')}</p>
            <div class="people-grid-wrap"></div>
            ${this._groupBarHTML(suggestion)}
            ${namesList(names)}`;
        body.querySelector('.people-grid-wrap').appendChild(grid.el);
        const update = this._wireGroupBar(lib, grp, faces, grid);
        update();
    }

    _groupBarHTML(suggestion) {
        return `
            <form class="people-bar">
                <label class="people-label" for="people-group-name">${t('Name')}</label>
                <div class="people-group-row">
                    <input class="form-input people-input" id="people-group-name" type="text" list="people-names" autocomplete="off" required
                        value="${suggestion ? escapeHtml(suggestion.name) : ''}">
                    <button type="button" class="btn people-hide"></button>
                    <button type="submit" class="btn btn-accent people-submit"></button>
                </div>
                <p class="people-group-error" hidden></p>
            </form>`;
    }

    // _wireGroupBar makes the bar and the suggestion act on the chosen faces
    // — the selection, or all — and returns what redraws the buttons.
    _wireGroupBar(lib, grp, faces, grid) {
        const bar = this.pane._body.querySelector('.people-bar');
        const input = bar.querySelector('.people-input');
        const update = () => {
            const n = grid.count;
            const who = input.value.trim();
            const count = formatCount(n || faces.length);
            const several = n ? n > 1 : faces.length > 1;
            bar.querySelector('.people-submit').textContent = n
                ? (who ? t('Name {n} {who}', { n: count, who }) : t('Name {n}', { n: count }))
                : !several ? (who ? t('Name {who}', { who }) : t('verb|Name'))
                    : (who ? t('Name all {n} {who}', { n: count, who }) : t('Name all {n}', { n: count }));
            bar.querySelector('.people-hide').textContent = n ? t('Hide {n}', { n: count })
                : several ? t('Hide all {n}', { n: count }) : t('Hide');
        };
        const chosen = () => (grid.count ? grid.selectedFaces().map(f => f.id) : faces.map(f => f.id));
        this._groupActions(lib, grp, (change) => this._applyToGroup(lib, grp, () => change(chosen())));
        input.addEventListener('input', update);
        input.focus();
        input.select();
        return update;
    }

    // _groupActions wires Name, Hide, and the suggestion's Yes and No to
    // apply, which hands each change the faces chosen.
    _groupActions(lib, grp, apply) {
        const body = this.pane._body;
        const bar = body.querySelector('.people-bar');
        bar.addEventListener('submit', (e) => {
            e.preventDefault();
            const name = bar.querySelector('.people-input').value;
            apply((ids) => PeopleAPI.name(lib, ids, name));
        });
        bar.querySelector('.people-hide').addEventListener('click', () => apply((ids) => PeopleAPI.hide(lib, ids)));
        body.querySelector('.people-suggest .people-accept')?.addEventListener('click', (e) => {
            const name = e.currentTarget.dataset.name;
            apply((ids) => PeopleAPI.name(lib, ids, name));
        });
        body.querySelector('.people-suggest .people-not')?.addEventListener('click', (e) => {
            const person = Number(e.currentTarget.dataset.person);
            apply((ids) => PeopleAPI.not(lib, ids, person));
        });
    }

    // _applyToGroup runs a change; what is left of the group stays on
    // screen, and an empty group leads back to the list.
    async _applyToGroup(lib, grp, change) {
        const body = this.pane._body;
        const buttons = [...body.querySelectorAll('.people-bar button, .people-suggest button')];
        buttons.forEach(b => { b.disabled = true; });
        try {
            reportSkipped(await change());
            const left = await PeopleAPI.groupFaces(lib, grp);
            if (left.length) await this.showGroup(lib, grp);
            else this.pane.go({ view: 'name', library: lib }, { replace: true });
        } catch (err) {
            buttons.forEach(b => { b.disabled = false; });
            showPeopleError(body.querySelector('.people-bar .people-group-error'), `${err.message} Nothing was changed.`);
        }
    }

    // —— Hidden ——

    // The faces hidden in a library, or with no library in all of them: one
    // hidden by mistake is shown again, and waits for a name once more.
    async showHidden(lib) {
        const body = this.pane._body;
        const activity = Activity.in(body, t('Reading the hidden faces…'), { area: true });
        let faces;
        try {
            faces = lib ? await PeopleAPI.hidden(lib) : await PeopleAPI.hiddenEverywhere();
        } catch (err) {
            activity.fail(t('The hidden faces could not be read: {err}.', { err: err.message }));
            return;
        }
        const back = lib
            ? `<a class="people-back" href="${peopleHash({ view: 'name', library: lib })}">${t('Waiting for a name in {lib}', { lib: escapeHtml(this.pane.libraryName(lib)) })}</a>`
            : this.pane.backLink();
        this.pane._head(faces.length ? countHTML(faces.length, 'face') : '');
        if (!faces.length) {
            body.innerHTML = `<div class="people-page-head">${this.pane.crumb(t('Hidden'), back)}</div><p class="people-empty">No face is hidden${lib ? ' here' : ''}.</p>`;
            return;
        }
        const grid = new FaceGrid({ lib, faces, onChange: () => this._hiddenBar(grid), onOpen: (i) => openFacePhotos(lib, faces, i) });
        body.innerHTML = `
            <div class="people-page-head">${this.pane.crumb(t('Hidden'), back)}</div>
            <p class="people-hint">${lib ? t('Faces hidden so that nobody is asked to name them, the newest photo first. Select the ones to show again: they wait for a name once more.') : t('Faces hidden so that nobody is asked to name them, in all libraries, the newest photo first. Select the ones to show again: they wait for a name once more.')}</p>
            <div class="people-grid-wrap"></div>
            <div class="people-bar">
                <div class="people-group-row"><button type="button" class="btn btn-accent people-show-again" disabled>${t('Show again')}</button></div>
                <p class="people-group-error" hidden></p>
            </div>`;
        body.querySelector('.people-grid-wrap').appendChild(grid.el);
        body.querySelector('.people-show-again').addEventListener('click', (e) => this._showAgain(lib, grid, e.currentTarget));
    }

    // _showAgain shows the selected faces again, each in its own library,
    // where its number means it.
    async _showAgain(lib, grid, again) {
        const restore = Activity.button(again, again.textContent);
        const by = new Map();
        for (const f of grid.selectedFaces()) {
            if (!by.has(f.library)) by.set(f.library, []);
            by.get(f.library).push(f.id);
        }
        try {
            for (const [l, ids] of by) await PeopleAPI.showAgain(l, ids);
            await this.showHidden(lib);
        } catch (err) {
            restore();
            showPeopleError(this.pane._body.querySelector('.people-group-error'), `${err.message} Faces of other libraries may have been shown again.`);
        }
    }

    _hiddenBar(grid) {
        const again = this.pane._body.querySelector('.people-show-again');
        again.disabled = !grid.count;
        again.textContent = grid.count ? t('Show {n} again', { n: formatCount(grid.count) }) : t('Show again');
    }
}

// hiddenLink leads to the faces hidden in a library, when there are any.
function hiddenLink(lib, n) {
    return n ? ` <a class="people-hidden-link" href="${peopleHash({ view: 'hidden', library: lib })}">${escapeHtml(t('{faces} hidden', { faces: countFaces(n) }))}</a>` : '';
}

function pendingNote(d) {
    if (!d.pending) return '';
    const photos = countPhotos(d.pending);
    return `<p class="people-pending">${d.finding
        ? `Looking for faces: <span class="people-num">${photos}</span> still to go. This page fills in when it is done.`
        : `<span class="people-num">${photos}</span> have not been looked at yet. That happens after the library's next scan.`}</p>`;
}

// namesList offers the names given so far, in any library, to every name
// field of the page.
function namesList(names) {
    return `<datalist id="people-names">${names.map(n => `<option value="${escapeHtml(n)}">`).join('')}</datalist>`;
}
