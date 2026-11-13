// memories-place.js — Memories (ADR-0064): photos of this day in earlier
// years, and a trip, a person, this month back then and photos rediscovered,
// chosen by who and what they show, never by stars. This file is the
// place's frame, its addresses and the overview; memories-detail.js is a
// memory's page.
//
// Addresses: #memories, #memories/<kind>/<key> — kind is day (key: the
// date), trip (its first day), person (the name), month (01…12) or
// rediscovered (today's date).


// memoriesRouteFromHash reads #memories/<kind>/<key>; an address that cannot
// be read (a broken %-escape) is the overview.
function memoriesRouteFromHash(hash) {
    try {
        const [kind = '', key = ''] = hash.replace(/^#/, '').split('?')[0].split('/').slice(1).map(decodeURIComponent);
        return kind && key ? { kind, key } : {};
    } catch {
        return {};
    }
}

function memoriesHash({ kind = '', key = '' } = {}) {
    return kind && key ? `#memories/${encodeURIComponent(kind)}/${encodeURIComponent(key)}` : '#memories';
}

// memoriesToday is the browser's date: memories follow the owner's day.
function memoriesToday() {
    return formatDay(new Date());
}

// memoryTitle names a memory, as its card and its page say it.
function memoryTitle(m, today) {
    const ago = Number(today.slice(0, 4)) - Number((m.key || '').slice(0, 4));
    switch (m.kind) {
        case 'day': return ago === 1 ? t('On this day a year ago') : t('On this day {n} years ago', { n: ago });
        case 'trip':
            if (!m.country) return t('Away from home, {date}', { date: formatDay(m.start) });
            // "A year ago" only for a trip back on its anniversary.
            if (!m.anniversary) return t('{year} in {country}', { year: m.start.slice(0, 4), country: m.country });
            return ago === 1 ? t('A year ago in {country}', { country: m.country }) : t('{n} years ago in {country}', { n: ago, country: m.country });
        case 'person':
            if (!m.birthday) return t('{name} over the years', { name: m.name });
            return m.age === 1 ? t('{name} turns 1 on {date}', { name: m.name, date: m.birthday }) : t('{name} turns {age} on {date}', { name: m.name, age: m.age, date: m.birthday });
        case 'month': return t('{month} in earlier years', { month: t(MONTHS[m.month - 1]) });
        case 'rediscovered': return t('Rediscovered');
    }
    return '';
}

// memoriesData is the line under a memory's title: its date where it has
// one, its days away, its photos.
function memoriesData(m) {
    const parts = [];
    if (m.kind === 'day') parts.push(formatDay(m.key));
    if (m.kind === 'trip') parts.push(countText(m.days, 'day'));
    parts.push(countText(m.total, 'photo'));
    return parts.join(' · ');
}

class MemoriesPane {
    constructor(container) {
        this.container = container;
        this.route = {};
        this._detail = new MemoriesDetail(this);
        this._ticket = 0;
        this._seed = 0;
        this._sheet = null;
    }

    async render() {
        if (!this.container.firstChild) this._buildShell();
        else this._filter.sync();
        this.route = App._memoriesRoute || {};
        this._scope = ScopeState.query({ time: false });
        this.clear();
        const ticket = ++this._ticket;
        const current = () => ticket === this._ticket && App.mode === 'memories';
        await (this.route.kind ? this._detail.show(this.route, current) : this._showOverview(current));
    }

    _buildShell() {
        this.container.innerHTML = `
            <div class="memories-place">
                <div class="place-main">
                    <div class="memories-head"></div>
                    <div class="memories-body"></div>
                </div>
            </div>`;
        this._headEl = this.container.querySelector('.memories-head');
        this.body = this.container.querySelector('.memories-body');
        this._filter = new ScopeFilter({});
        this.container.querySelector('.memories-place').append(this._filter.panel, toolStrip(this._filter.toggle));
        ScopeState.onChange(() => {
            if (App.mode !== 'memories' || ScopeState.query({ time: false }) === this._scope) return;
            this.render();
        });
        // Captions and the way back are addresses; go there as the sidebar does.
        this.container.addEventListener('click', (e) => {
            const link = e.target.closest('a[data-memories]');
            if (!link || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
            e.preventDefault();
            App.openMemories(memoriesRouteFromHash(link.getAttribute('href')));
        });
        this._resize = new ResizeObserver(() => this._sheet?.relayout());
        this._resize.observe(this.body);
    }

    // clear ends what the last page left: its grid's observers, its sheet.
    clear() {
        this._sheet = null;
        this._detail.clear();
    }

    // head draws the place's head: its title (on a memory's page the way
    // back and the memory's name), Shuffle on the sheet, and the filter.
    head(title, meta = '') {
        const sheet = !this.route.kind;
        this._headEl.innerHTML = placeHeaderHTML({
            title,
            meta,
            actions: `${sheet ? `<button type="button" class="btn memories-shuffle">${t('Shuffle')}</button>` : ''}<span class="memories-actions"></span>`,
            lede: sheet ? placeLede(t('Photos from this day in earlier years, and more to find again — chosen by who and what they show, never by stars.')) : '',
        });
        this._headEl.querySelector('.memories-actions').replaceWith(this._filter.el);
        this._headEl.querySelector('.memories-shuffle')?.addEventListener('click', () => this._sheet?.shuffle());
    }

    // load says the memories are being chosen while read runs, and what
    // went wrong when it fails.
    async load(read) {
        const activity = Activity.in(this.body, t('Choosing memories…'), { area: true });
        try {
            const data = await read();
            activity.clear();
            return data;
        } catch (err) {
            activity.fail(err.status === 404
                ? t('This memory is not among today\'s; its title leads back to all of them.')
                : t('The memories could not be read: {err}. Reload the page to try again.', { err: err.message }));
            return null;
        }
    }

    async _showOverview(current) {
        this.head(t('Memories'));
        const today = memoriesToday();
        const o = await this.load(() => MemoriesAPI.overview(today, this._scope));
        if (!o || !current()) return;
        const memories = [
            ...o.years.map(y => ({ kind: 'day', key: y.date, total: y.total, photos: y.picks })),
            ...o.cards.map(c => ({ ...c, photos: c.picks })),
        ];
        this.body.innerHTML = '';
        if (!o.years.length) {
            this.body.insertAdjacentHTML('beforeend', `<p class="memories-empty">${t('No photos from this day in earlier years.')}</p>`);
        }
        if (!memories.length) return;
        const el = document.createElement('div');
        this.body.appendChild(el);
        this._sheet = new MemoriesSheet(el, memories, today);
    }
}

// MemoriesSheet draws the memories as one sheet (memories-sheet.js): a
// block for each, captioned, the order and the shapes chance's. Shuffle
// lays it out again; a new width keeps the shapes' seed. A memory's page
// shows its one block without a caption.
class MemoriesSheet {
    constructor(el, memories, today, { captions = true } = {}) {
        this.el = el;
        this.el.classList.add('sheet');
        this._memories = memories;
        this._today = today;
        this._captions = captions;
        this._seed = Math.floor(Math.random() * 2 ** 31);
        this._width = 0;
        this.el.addEventListener('click', (e) => {
            const tile = e.target.closest('.sheet-tile');
            if (!tile) return;
            const block = this._blocks[Number(tile.dataset.block)];
            openMemoryPhotos(block.memory.photos, Number(tile.dataset.index));
        });
        this.relayout();
    }

    shuffle() {
        this._seed = Math.floor(Math.random() * 2 ** 31);
        this._width = 0;
        this.relayout();
    }

    relayout() {
        const width = this.el.clientWidth;
        if (!width || width === this._width) return;
        this._width = width;
        const bands = sheetLayout(this._memories, width, { phone: onPhone(), seed: this._seed });
        this._blocks = bands.flat();
        let i = 0;
        this.el.innerHTML = bands.map(band => `<div class="sheet-band">${band.map(b => this._blockHTML(b, i++)).join('')}</div>`).join('');
    }

    _blockHTML(b, index) {
        const m = b.memory;
        if (!this._captions) return `<div class="sheet-block" style="width:${b.w}px">${sheetBlockHTML(b, index)}</div>`;
        return `<figure class="sheet-block" style="width:${b.w}px">
            ${sheetBlockHTML(b, index)}
            <figcaption class="sheet-caption">
                <a class="sheet-title" href="${memoriesHash(m)}" data-memories>${escapeHtml(memoryTitle(m, this._today))}</a>
                <span class="memories-data">${memoriesData(m)}</span>
            </figcaption>
        </figure>`;
    }
}
