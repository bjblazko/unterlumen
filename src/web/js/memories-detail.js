// memories-detail.js — one memory's page: its title is the way back to
// Memories and its name; its picks as one collage (memories-sheet.js), then
// every photo of the memory in rows.

const MEMORIES_GRID_H = 180;

class MemoriesDetail {
    constructor(pane) {
        this.pane = pane;
        this._grid = null;
        this._sheet = null;
    }

    clear() {
        this._grid?.destroy();
        this._grid = null;
        this._sheet = null;
    }

    async show(route, current) {
        const today = memoriesToday();
        this.pane.head(this._crumb(t('Memories')));
        const d = await this.pane.load(() => MemoriesAPI.detail(today, route.kind, route.key, this.pane._scope));
        if (!d || !current()) return;
        this.pane.head(this._crumb(escapeHtml(memoryTitle(d, today))), countHTML(d.total, 'photo'));
        const body = this.pane.body;
        body.innerHTML = '<div class="memories-picks"></div>';
        const picks = body.querySelector('.memories-picks');
        // One block, its shape chance's, the photos in their time order.
        this._sheet = new MemoriesSheet(picks, [{ ...d, photos: d.picks }], today, { captions: false });
        this.pane._sheet = this._sheet;
        if (d.photos.length > d.picks.length) {
            body.insertAdjacentHTML('beforeend', `<h2 class="memories-section-title">${t('All photos')}</h2>`);
            this._grid = new MemoriesGrid(d.photos, MEMORIES_GRID_H);
            body.appendChild(this._grid.el);
        }
    }

    _crumb(here) {
        return `<nav class="gal-crumbs" aria-label="${t('Breadcrumb')}"><a class="link-btn" href="#memories" data-memories>${t('Memories')}</a><span class="gal-crumb-sep">/</span><span class="gal-crumb-here">${here}</span></nav>`;
    }
}
