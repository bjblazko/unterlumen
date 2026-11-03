// memories-place.js — Memories (ADR-0064): photos of this day in earlier
// years, and a trip, a person, this month back then and photos rediscovered,
// chosen by who and what they show, never by stars. This file is the
// place's frame, its addresses and the overview; memories-detail.js is a
// memory's page.
//
// Addresses: #memories, #memories/<kind>/<key> — kind is day (key: the
// date), trip (its first day), person (the name), month (01…12) or
// rediscovered (today's date).

const MEMORIES_ROW_H = 160;

function memoriesRouteFromHash(hash) {
    const [kind = '', key = ''] = hash.replace(/^#/, '').split('?')[0].split('/').slice(1).map(decodeURIComponent);
    return kind && key ? { kind, key } : {};
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
            return ago === 1 ? t('A year ago in {country}', { country: m.country }) : t('{year} in {country}', { year: m.start.slice(0, 4), country: m.country });
        case 'person':
            return m.birthday ? t('{name} turns {age} on {date}', { name: m.name, age: m.age, date: m.birthday }) : t('{name} over the years', { name: m.name });
        case 'month': return t('{month} in earlier years', { month: t(MONTHS[m.month - 1]) });
        case 'rediscovered': return t('Rediscovered');
    }
    return '';
}

class MemoriesPane {
    constructor(container) {
        this.container = container;
        this.route = {};
        this._detail = new MemoriesDetail(this);
    }

    async render() {
        if (!this.container.firstChild) this._buildShell();
        else this._filter.sync();
        this.route = App._memoriesRoute || {};
        this._scope = ScopeState.query({ time: false });
        await (this.route.kind ? this._detail.show(this.route) : this._showOverview());
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
        // Rows and cards are addresses; go there as the sidebar does.
        this.container.addEventListener('click', (e) => {
            const link = e.target.closest('a[data-memories]');
            if (!link || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
            e.preventDefault();
            App.openMemories(memoriesRouteFromHash(link.getAttribute('href')));
        });
    }

    // head draws the place's head: its title (on a memory's page the way
    // back and the memory's name) and the filter.
    head(title, meta = '') {
        this._headEl.innerHTML = placeHeaderHTML({
            title,
            meta,
            actions: '<span class="memories-actions"></span>',
            lede: this.route.kind ? '' : placeLede(t('Photos from this day in earlier years, and more to find again — chosen by who and what they show, never by stars.')),
        });
        this._headEl.querySelector('.memories-actions').replaceWith(this._filter.el);
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

    async _showOverview() {
        this.head(t('Memories'));
        const today = memoriesToday();
        const o = await this.load(() => MemoriesAPI.overview(today, this._scope));
        if (!o || App.mode !== 'memories' || this.route.kind) return;
        this.body.innerHTML = '';
        if (!o.years.length) {
            this.body.insertAdjacentHTML('beforeend', `<p class="memories-empty">${t('No photos from this day in earlier years.')}</p>`);
        }
        for (const y of o.years) this.body.appendChild(this._yearEl(y, today));
        if (o.cards.length) this.body.appendChild(this._cardsEl(o.cards, today));
    }

    _yearEl(y, today) {
        const el = document.createElement('section');
        el.className = 'memories-year';
        const route = { kind: 'day', key: y.date };
        el.innerHTML = `<a class="memories-year-head" href="${memoriesHash(route)}" data-memories>
                <h2 class="memories-year-title">${escapeHtml(memoryTitle({ ...route }, today))}</h2>
                <span class="memories-data">${formatDay(y.date)} · ${countText(y.total, 'photo')}</span>
                <span class="memories-more" aria-hidden="true">›</span>
            </a>
            <div class="memories-picks"></div>`;
        const picks = el.querySelector('.memories-picks');
        const draw = () => { picks.innerHTML = memoriesRowHTML(memoriesRow(y.picks, picks.clientWidth, MEMORIES_ROW_H)); };
        new ResizeObserver(draw).observe(picks);
        picks.addEventListener('click', (e) => {
            const tile = e.target.closest('.memories-tile');
            if (tile) openMemoryPhotos(y.picks, Number(tile.dataset.index));
        });
        return el;
    }

    _cardsEl(cards, today) {
        const el = document.createElement('section');
        el.className = 'memories-cards-section';
        el.innerHTML = `<h2 class="memories-section-title">${t('More memories')}</h2>
            <div class="memories-cards">${cards.map(c => this._cardHTML(c, today)).join('')}</div>`;
        return el;
    }

    _cardHTML(c, today) {
        const data = [c.kind === 'trip' ? countText(c.days, 'day') : '', countText(c.total, 'photo')].filter(Boolean).join(' · ');
        return `<a class="memories-card" href="${memoriesHash(c)}" data-memories>
            <img class="memories-card-cover" src="${LibraryAPI.thumbURL(c.cover.lib, c.cover.id)}" alt="" loading="lazy" decoding="async">
            <span class="memories-card-title">${escapeHtml(memoryTitle(c, today))}</span>
            <span class="memories-data">${data}</span>
        </a>`;
    }
}
