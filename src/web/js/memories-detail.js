// memories-detail.js — one memory's page: its title is the way back to
// Memories and its name; the picks larger, then every photo of the memory.

const MEMORIES_PICK_H = 320;
const MEMORIES_GRID_H = 180;

class MemoriesDetail {
    constructor(pane) {
        this.pane = pane;
    }

    async show(route) {
        const today = memoriesToday();
        this.pane.head(this._crumb(t('Memories')));
        const d = await this.pane.load(() => MemoriesAPI.detail(today, route.kind, route.key, this.pane._scope));
        if (!d || App.mode !== 'memories' || this.pane.route !== route) return;
        this.pane.head(this._crumb(escapeHtml(memoryTitle(d, today))), countHTML(d.total, 'photo'));
        const body = this.pane.body;
        body.innerHTML = '<div class="memories-picks memories-picks-large"></div>';
        const picks = body.querySelector('.memories-picks');
        const draw = () => { picks.innerHTML = memoriesRowHTML(memoriesRow(d.picks, picks.clientWidth, MEMORIES_PICK_H)); };
        new ResizeObserver(draw).observe(picks);
        picks.addEventListener('click', (e) => {
            const tile = e.target.closest('.memories-tile');
            if (tile) openMemoryPhotos(d.picks, Number(tile.dataset.index));
        });
        if (d.photos.length > d.picks.length) {
            body.insertAdjacentHTML('beforeend', `<h2 class="memories-section-title">${t('All photos')}</h2>`);
            body.appendChild(new MemoriesGrid(d.photos, MEMORIES_GRID_H).el);
        }
    }

    _crumb(here) {
        return `<nav class="gal-crumbs" aria-label="${t('Breadcrumb')}"><a class="link-btn" href="#memories" data-memories>${t('Memories')}</a><span class="gal-crumb-sep">/</span><span class="gal-crumb-here">${here}</span></nav>`;
    }
}
