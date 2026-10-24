// info-things.js — what a library photo shows, in its info panel: the
// things found in it, each with a × that takes it away for good — the
// photo's sidecar remembers, so no later pass finds it again (ADR-0054).

class InfoThings {
    constructor(panel) {
        this._panel = panel;
        this.reset();
    }

    reset() {
        this._ref = null;
        this._things = [];
    }

    // load reads the things of a library photo; the panel draws them once
    // they are there. A photo without things shows nothing.
    async load(lib, photo) {
        this.reset();
        this._ref = { lib, photo };
        try {
            const things = await ThingsAPI.photoThings(lib, photo);
            if (this._ref?.photo !== photo) return;
            this._things = things;
        } catch {
            return;
        }
        if (this._things.length) this._panel.render();
    }

    render() {
        if (!this._things.length) return '';
        // On a phone the panel only says what is in the photo.
        const editable = !matchMedia(INFO_PANEL_PHONE).matches;
        const rows = this._things.map(f => {
            const name = ThingWords.name(f.name);
            const drop = t('There is no {name} in this photo', { name });
            return `<div class="info-thing"><span class="info-value info-thing-name">${escapeHtml(name)}</span>` +
                (editable ? `<button type="button" class="info-meta-del info-thing-remove" data-thing="${escapeHtml(f.name)}" title="${escapeHtml(drop)}" aria-label="${escapeHtml(drop)}">×</button>` : '') +
                '</div>';
        });
        return `<div class="info-things">${this._panel.section(t('Things'), rows)}</div>`;
    }

    attachEvents(container) {
        container.querySelectorAll('.info-thing-remove').forEach(btn => {
            btn.addEventListener('click', () => this._remove(btn.dataset.thing));
        });
    }

    async _remove(thing) {
        const { lib, photo } = this._ref;
        try {
            await ThingsAPI.remove(lib, photo, thing);
        } catch (err) {
            App.showToast(`${err.message}`);
        }
        await this.load(lib, photo);
        this._panel.render();
    }
}
