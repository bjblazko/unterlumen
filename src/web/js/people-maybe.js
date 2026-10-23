// people-maybe.js — "Maybe Anna" on a person's page (ADR-0051, decision
// 13): the faces nobody named that look most like them, in every library
// where they are named, the likeliest first. Selected, they are her, or
// they are not, which keeps them from being asked about again.

class PeopleMaybe {
    // page is the PeoplePerson showing name; el the section to fill.
    constructor(page, name, el) {
        this._page = page;
        this._name = name;
        this._el = el;
    }

    async load() {
        let faces;
        try {
            faces = await PeopleAPI.maybe(this._name);
        } catch {
            return; // the page is whole without it
        }
        if (!faces.length || !this._el.isConnected) return;
        const n = escapeHtml(this._name);
        this._el.hidden = false;
        this._el.innerHTML = `
            <h2 class="people-section-title">Maybe ${n}</h2>
            <p class="people-hint">Faces nobody named that look like ${n}, the likeliest first. Select the ones that are ${n}, or the ones that are not, so they are not suggested again.</p>
            <div class="people-group-row people-maybe-actions">
                <button type="button" class="btn people-maybe-yes" disabled>This is ${n}</button>
                <button type="button" class="btn people-maybe-no" disabled>Not ${n}</button>
            </div>
            <p class="people-group-error" hidden></p>`;
        this._grid = new FaceGrid({
            faces,
            onChange: () => this._update(),
            onOpen: (i) => openFacePhotos(null, faces, i),
        });
        this._el.appendChild(this._grid.el);
        this._el.querySelector('.people-maybe-yes').addEventListener('click', (e) =>
            this._act(e.currentTarget, ({ lib, ids }) => PeopleAPI.name(lib, ids, this._name)));
        this._el.querySelector('.people-maybe-no').addEventListener('click', (e) =>
            this._act(e.currentTarget, ({ lib, ids, person }) => PeopleAPI.not(lib, ids, person)));
    }

    _update() {
        const none = this._grid.count === 0;
        for (const b of this._el.querySelectorAll('.people-maybe-actions .btn')) b.disabled = none;
    }

    // _act changes the selected faces in each of their libraries, where the
    // person has an ID of their own, then shows the page again.
    async _act(btn, change) {
        const restore = Activity.button(btn, btn.textContent);
        const by = new Map();
        for (const f of this._grid.selectedFaces()) {
            if (!by.has(f.library)) by.set(f.library, { lib: f.library, person: f.person, ids: [] });
            by.get(f.library).ids.push(f.id);
        }
        try {
            for (const g of by.values()) reportSkipped(await change(g));
        } catch (err) {
            restore();
            showPeopleError(this._el.querySelector('.people-group-error'), `${err.message} Nothing was changed there.`);
            return;
        }
        await this._page.show(this._name);
    }
}
