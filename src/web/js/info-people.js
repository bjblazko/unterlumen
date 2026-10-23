// info-people.js — who is in a library photo, in its info panel: the faces
// found in it, the names given, a field to name a face and a × to take a
// name away (ADR-0051). Resting on a face shows it framed in the photo. In
// the viewer, Mark a face draws one that was not found (FaceMarkTool).

class InfoPeople {
    constructor(panel) {
        this._panel = panel;
        this.reset();
    }

    reset() {
        this._ref = null;
        this._faces = [];
        this._names = [];
        this._peek?.hide();
    }

    // load reads the faces of a library photo; the panel draws them once they
    // are there. A photo whose faces were not looked for shows nothing.
    async load(lib, photo) {
        this.reset();
        this._ref = { lib, photo };
        try {
            const r = await PeopleAPI.photoFaces(lib, photo);
            if (this._ref?.photo !== photo) return;
            this._faces = r.faces.filter(f => !f.hidden);
            this._names = r.names;
        } catch {
            return; // no faces to show: the section stays away
        }
        this._panel.render();
    }

    render() {
        if (matchMedia(INFO_PANEL_PHONE).matches) return this._faces.length ? this._renderToRead() : '';
        const markable = !!(this._ref && this._panel.onMarkFace);
        if (!this._faces.length && !markable) return '';
        const rows = this._faces.map((f, i) => {
            const pic = `<button type="button" class="info-face-pic" data-index="${i}" aria-label="Face ${i + 1}">` +
                `<img src="${PeopleAPI.faceURL(this._ref.lib, f.id)}" alt="" width="40" height="40" loading="lazy"></button>`;
            // A face marked by hand is there only as someone: × takes it away.
            const drop = f.manual ? `Remove ${f.name}'s frame` : `This is not ${f.name}`;
            const who = f.name
                ? `<span class="info-value info-face-name">${escapeHtml(f.name)}${f.age !== undefined ? `<span class="info-face-age"> · ${ageText(f.age)}</span>` : ''}</span>` +
                  `<button type="button" class="info-meta-del info-face-unname" data-face="${f.id}" title="${escapeHtml(drop)}" aria-label="${escapeHtml(drop)}">×</button>`
                : `<input class="info-face-input" type="text" list="info-face-names" autocomplete="off" data-face="${f.id}" aria-label="Who is face ${i + 1}?">`;
            return `<div class="info-face">${pic}${who}</div>`;
        });
        if (this._faces.some(f => !f.name)) rows.push('<p class="info-label info-face-hint">Type a name beside a face and press Return.</p>');
        if (markable) rows.push('<button type="button" class="btn btn-sm info-face-mark" title="Draw a frame around a face that was not found">Mark a face</button>');
        const options = this._names.map(n => `<option value="${escapeHtml(n)}">`).join('');
        rows.push(`<datalist id="info-face-names">${options}</datalist>`);
        return `<div class="info-people">${this._panel.section('People', rows)}</div>`;
    }

    // On a phone the panel only says who is in the photo: naming is work
    // for the computer.
    _renderToRead() {
        const named = this._faces.map((f, i) => ({ f, i })).filter(({ f }) => f.name);
        if (!named.length) return '';
        const rows = named.map(({ f, i }) => `<div class="info-face">` +
            `<button type="button" class="info-face-pic" data-index="${i}" aria-label="${escapeHtml(f.name)}">` +
            `<img src="${PeopleAPI.faceURL(this._ref.lib, f.id)}" alt="" width="40" height="40" loading="lazy"></button>` +
            `<span class="info-value info-face-name">${escapeHtml(f.name)}${f.age !== undefined ? `<span class="info-face-age"> · ${ageText(f.age)}</span>` : ''}</span></div>`);
        return `<div class="info-people">${this._panel.section('People', rows)}</div>`;
    }

    attachEvents(container) {
        const box = container.querySelector('.info-people');
        if (!box) return;
        this._peek = new FacePeek(this._ref.lib, this._faces, box, '.info-face-pic');
        box.querySelectorAll('.info-face-input').forEach(input => {
            input.addEventListener('keydown', (e) => {
                if (e.key !== 'Enter' || !input.value.trim()) return;
                e.preventDefault();
                this._change(() => PeopleAPI.name(this._ref.lib, [Number(input.dataset.face)], input.value.trim()));
            });
        });
        box.querySelector('.info-face-mark')?.addEventListener('click', () => {
            this._panel.onMarkFace(this._ref, this._faces, this._names);
        });
        box.querySelectorAll('.info-face-unname').forEach(btn => {
            btn.addEventListener('click', () => this._change(() => PeopleAPI.unname(this._ref.lib, [Number(btn.dataset.face)])));
        });
    }

    async _change(change) {
        const { lib, photo } = this._ref;
        try {
            reportSkipped(await change());
        } catch (err) {
            App.showToast(`${err.message} Nothing was changed.`);
        }
        await this.load(lib, photo);
    }
}
