// people-naming.js — Waiting for a name: the faces of one library grouped by who they
// look like, waiting for a name (ADR-0051), and one group with all its faces.
//
// Addresses: #people/name/<library> and #people/name/<library>/group/<n>.

class PeopleNaming {
    // pane is the PeoplePane that hosts the page.
    constructor(pane) {
        this.pane = pane;
    }

    // —— A library's groups ——

    async showLibrary(lib) {
        const body = this.pane._body;
        const activity = Activity.in(body, 'Reading the faces…', { area: true });
        let data, toName, names;
        try {
            [data, toName, names] = await Promise.all([PeopleAPI.overview(lib), PeopleAPI.toName(), PeopleAPI.allNames()]);
        } catch (err) {
            activity.fail(`The faces could not be read: ${err.message}.`);
            return;
        }
        this.pane._head(data.groupCount ? `${formatCount(data.groupCount)} ${data.groupCount === 1 ? 'group' : 'groups'}` : '');
        body.innerHTML = `
            <div class="people-page-head">${this.pane.crumb(`Waiting for a name in ${escapeHtml(this.pane.libraryName(lib))}`)}</div>
            ${this.pane.libraries.length > 1 ? toNameHTML(toName, lib) : ''}
            ${pendingNote(data)}
            ${data.groups.length ? `
                <p class="people-hint">Faces that look alike, the largest groups first. Name them, or hide faces nobody is to name: strangers, statues, posters.${data.groupCount > data.groups.length ? ` These are ${formatCount(data.groups.length)} of ${formatCount(data.groupCount)}; naming them brings up the next.` : ''}</p>
                <div class="people-groups">${data.groups.map(g => this._groupRow(lib, g)).join('')}</div>`
                : `<p class="people-empty">${data.people.length || data.pending ? 'No faces are waiting for a name here.' : 'No faces were found in the photos of this library.'}</p>`}
            ${namesList(names)}`;
        for (const form of body.querySelectorAll('.people-group')) this._wireRow(lib, form);
    }

    _groupRow(lib, g) {
        const href = peopleHash({ view: 'group', library: lib, id: g.id });
        const faces = g.faces.map(f => `<img src="${PeopleAPI.faceURL(lib, f)}" alt="" loading="lazy" width="64" height="64">`).join('');
        const inputId = `people-name-${g.id}`;
        return `
            <form class="people-group" data-group="${g.id}">
                <a class="people-group-faces" href="${href}" aria-label="All ${formatCount(g.size)} faces of this group">${faces}</a>
                <div class="people-group-name">
                    <a class="people-group-count" href="${href}">${countFaces(g.size)}</a>
                    ${g.suggestion ? `<p class="people-suggest">Is this <button type="button" class="btn btn-sm people-accept" data-name="${escapeHtml(g.suggestion.name)}">${escapeHtml(g.suggestion.name)}</button><span class="people-suggest-mark">?</span></p>` : ''}
                    <label class="people-label" for="${inputId}">Name</label>
                    <div class="people-group-row">
                        <input class="form-input people-input" id="${inputId}" type="text" list="people-names" autocomplete="off" required>
                        <button type="submit" class="btn">Name</button>
                        <button type="button" class="btn people-hide">Hide</button>
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

    async showGroup(lib, grp) {
        const body = this.pane._body;
        const activity = Activity.in(body, 'Reading the group…', { area: true });
        let faces, names;
        try {
            [faces, names] = await Promise.all([PeopleAPI.groupFaces(lib, grp), PeopleAPI.allNames()]);
        } catch (err) {
            activity.fail(`The group could not be read: ${err.message}.`);
            return;
        }
        const back = `<a class="people-back" href="${peopleHash({ view: 'name', library: lib })}">Waiting for a name in ${escapeHtml(this.pane.libraryName(lib))}</a>`;
        if (!faces.length) {
            body.innerHTML = `<div class="people-note"><p>This group is gone: its faces were named, or the faces were grouped anew since. ${back}</p></div>`;
            return;
        }
        this.pane._head(countFaces(faces.length));
        const grid = new FaceGrid({ lib, faces, onChange: () => update(), onOpen: (i) => openFacePhotos(lib, faces, i) });
        body.innerHTML = `
            <div class="people-page-head">${this.pane.crumb('Who is this?', back)}</div>
            <p class="people-hint">Name all of them, or select the ones to name first. Rest the pointer on a face to see it in its photo; double-click opens the photo.</p>
            <div class="people-grid-wrap"></div>
            <form class="people-bar">
                <label class="people-label" for="people-group-name">Name</label>
                <div class="people-group-row">
                    <input class="form-input people-input" id="people-group-name" type="text" list="people-names" autocomplete="off" required>
                    <button type="button" class="btn people-hide"></button>
                    <button type="submit" class="btn btn-accent people-submit"></button>
                </div>
                <p class="people-group-error" hidden></p>
            </form>
            ${namesList(names)}`;
        body.querySelector('.people-grid-wrap').appendChild(grid.el);
        const bar = body.querySelector('.people-bar');
        const submit = bar.querySelector('.people-submit');
        const hide = bar.querySelector('.people-hide');
        // Nothing selected is the whole group; a selection is only those.
        const chosen = () => (grid.selected.size ? [...grid.selected] : faces.map(f => f.id));
        const update = () => {
            const n = grid.selected.size;
            submit.textContent = n ? `Name ${formatCount(n)}` : `Name all ${formatCount(faces.length)}`;
            hide.textContent = n ? `Hide ${formatCount(n)}` : `Hide all ${formatCount(faces.length)}`;
        };
        update();
        const apply = async (change) => {
            const buttons = [...bar.querySelectorAll('button')];
            buttons.forEach(b => { b.disabled = true; });
            try {
                reportSkipped(await change(chosen()));
                this.pane.go({ view: 'name', library: lib }, { replace: true });
            } catch (err) {
                buttons.forEach(b => { b.disabled = false; });
                showPeopleError(bar.querySelector('.people-group-error'), `${err.message} Nothing was changed.`);
            }
        };
        bar.addEventListener('submit', (e) => {
            e.preventDefault();
            apply((ids) => PeopleAPI.name(lib, ids, bar.querySelector('.people-input').value));
        });
        hide.addEventListener('click', () => apply((ids) => PeopleAPI.hide(lib, ids)));
        bar.querySelector('.people-input').focus();
    }
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
