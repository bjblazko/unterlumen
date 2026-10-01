// stats-place.js — the Statistics place: what the library photos have in
// common and how that changed, one topic at a time (ADR-0043). The overview
// shows a card per topic; the sidebar lists the same topics as sub-entries.

class StatsPane {
    constructor(container) {
        this.container = container;
        this._route = { topic: '', library: '', path: '' };
        this._granularity = '';
        this._generation = 0;
        this._pickGeneration = 0;
        this._drawn = false;
    }

    // The route comes from the address (App keeps it); every visit reads the
    // numbers again, since a scan may have changed them. The server caches them.
    // The photos shown belong to what was on screen, so a new topic or
    // scope replaces them: with all photos of the scope while the column is
    // kept open (photoColumnStartsOpen), else the column closes.
    show(route) {
        this._route = route;
        this._setStage(false);
        this._hidePhotos();
        this._allPhotosDue = photoColumnStartsOpen(STATS_PHOTOS_KEY);
        return this.render();
    }

    async render() {
        if (!this.container.firstChild) this._buildShell();
        const generation = ++this._generation;
        const libs = await LibraryAPI.list().catch(() => null);
        if (generation !== this._generation) return;
        if (libs === null) { this._note('The libraries could not be read. Reload the page to try again.', true); return; }
        this._libs = libs;
        this._lib = libs.find(l => String(l.id) === this._route.library) || null;
        const topic = statsTopic(this._route.topic);
        this._drawHead(topic);
        if (libs.length === 0) {
            this._note(`No libraries yet. Statistics count the photos in a library — ${placeLink('library', 'libraries', 'add one under Libraries')}.`);
            return;
        }
        let data;
        try {
            data = await this._whileLoading(() => this._read(topic));
        } catch (err) {
            if (generation === this._generation) this._note(`The statistics could not be read: ${escapeHtml(err.message)}. Reload the page to try again.`, true);
            return;
        }
        if (generation !== this._generation) return;
        this._drawCount(data.snap);
        if (data.snap.totalPhotos === 0) { this._note(this._nothingCounted(data.snap)); return; }
        this._noteEl.hidden = true;
        this._main.innerHTML = '';
        this._drawIndexing(data.snap, topic && (data.colour || data.colourSpace));
        if (topic) this._drawTopic(topic, data);
        else this._drawOverview(data);
        this._labelStageButton();
        this._drawn = true;
        if (this._allPhotosDue) {
            this._allPhotosDue = false;
            this._showAllPhotos();
        }
    }

    _buildShell() {
        this.container.innerHTML = `
            <div class="stats-place">
                <div class="stats-head"></div>
                <div class="stats-body">
                    <div class="stats-main"></div>
                    <div class="stats-note" hidden></div>
                </div>
            </div>`;
        this._head = this.container.querySelector('.stats-head');
        this._main = this.container.querySelector('.stats-main');
        this._noteEl = this.container.querySelector('.stats-note');
        this._photos = new PhotoColumn({
            onOpen: (photos, index) => openLibraryPhotos(photos, index),
            onClose: () => this.closePhotos(),
            label: 'Photos of the value picked in a chart',
            emptyText: 'No photos match. Some charts count a value the photos only carry in part.',
        });
        this._photos.el.id = 'stats-photos';
        this._photos.el.hidden = true;
        this.container.querySelector('.stats-body').appendChild(this._photos.el);
        // Cards, the way back and the links in a note are addresses; go there
        // as the sidebar does, without a reload.
        this.container.addEventListener('click', (e) => {
            const link = e.target.closest('a[data-stats-topic]');
            if (!link || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
            e.preventDefault();
            App.openStatistics({ ...this._route, topic: link.dataset.statsTopic });
        });
    }

    /* --- Head: where we are, what is counted --- */

    _drawHead(topic) {
        const hasTimeline = topic ? topic.charts.some(c => ['tl', 'colour', 'colourSpace'].includes(c.source)) : false;
        this._head.innerHTML = `
            ${topic ? `<a class="btn btn-sm stats-back" href="${statsHash({ ...this._route, topic: '' })}" data-stats-topic="">
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M7 2L3 6l4 4"/></svg>
                Statistics</a>` : ''}
            <h1 class="stats-title">${topic ? escapeHtml(topic.label) : 'Statistics'}</h1>
            <span class="stats-count"></span>
            <span class="stats-head-spacer"></span>
            <div class="stats-scope">
                ${this._libs.length ? this._librarySelectHTML() : ''}
                ${this._lib && this._route.path ? this._folderHTML() : ''}
            </div>
            ${hasTimeline ? this._granularitySelectHTML() : ''}
            ${placeLede(`What the photos in your ${placeLink('library', 'libraries', 'libraries')} have in common, and how that changed. Click a bar, a slice or a point to see its photos.`)}`;
        this._head.querySelector('.stats-library')?.addEventListener('change', (e) => {
            App.openStatistics({ ...this._route, library: e.target.value, path: '' });
        });
        this._head.querySelector('.stats-whole-library')?.addEventListener('click', () => {
            App.openStatistics({ ...this._route, path: '' });
        });
        this._head.querySelector('.stats-granularity')?.addEventListener('change', (e) => {
            this._granularity = e.target.value;
            this.render();
        });
    }

    _librarySelectHTML() {
        const options = this._libs.map(l =>
            `<option value="${escapeHtml(String(l.id))}"${l === this._lib ? ' selected' : ''}>${escapeHtml(l.name)}</option>`).join('');
        return `<select class="btn btn-sm select-btn stats-library" aria-label="Library">
            <option value="">All libraries</option>${options}</select>`;
    }

    _folderHTML() {
        return `<span class="stats-folder"><span class="stats-folder-label">Folder</span>
            <span class="stats-folder-path">${escapeHtml(this._route.path)}</span></span>
            <button class="btn btn-sm stats-whole-library">Whole library</button>`;
    }

    _granularitySelectHTML() {
        const opts = [['', 'Auto'], ['month', 'Months'], ['year', 'Years']].map(([v, label]) =>
            `<option value="${v}"${v === this._granularity ? ' selected' : ''}>${label}</option>`).join('');
        return `<label class="stats-period"><span class="stats-period-label">Periods</span>
            <select class="btn btn-sm select-btn stats-granularity">${opts}</select></label>`;
    }

    _drawCount(snap) {
        const n = snap.totalPhotos;
        this._head.querySelector('.stats-count').textContent = `${formatCount(n)} ${n === 1 ? 'photo' : 'photos'}`;
    }

    _nothingCounted(snap) {
        if (snap.indexingPhotos > 0) return `The ${formatCount(snap.indexingPhotos)} photos here are still being read. Their statistics appear once the scan is done.`;
        if (this._route.path) return 'This folder holds no photos.';
        return this._lib ? 'This library holds no photos yet. Scan it under Libraries to count them.' : 'Your libraries hold no photos yet.';
    }

    // colour: the Colour topic's numbers, which only count analysed photos.
    _drawIndexing(snap, colour) {
        const lines = [];
        if (snap.indexingPhotos > 0) lines.push(`${formatCount(snap.indexingPhotos)} photos are still being read, so these numbers are not complete yet.`);
        if (colour?.analysedPhotos > 0 && colour.unanalysedPhotos > 0) lines.push(`${formatCount(colour.unanalysedPhotos)} photos have not been analysed yet, so the colours are not complete.`);
        lines.push(...(snap.warnings ?? []));
        if (!lines.length) return;
        const p = document.createElement('p');
        p.className = 'stats-incomplete';
        p.textContent = lines.join(' ');
        this._main.appendChild(p);
    }

    /* --- Reading --- */

    // The scope as the server takes it: library ids and an absolute folder.
    _scopeParams() {
        const params = {};
        if (!this._lib) return params;
        params.ids = String(this._lib.id);
        const root = this._lib.sourcePath.replace(/\/$/, '');
        params.pathPrefix = this._route.path ? `${root}/${this._route.path}` : root;
        return params;
    }

    // The overview's cards and every topic with a development over time need
    // the timeline, the colours or the colour space as well.
    async _read(topic) {
        const scope = this._scopeParams();
        const over = { ...scope, ...(this._granularity ? { granularity: this._granularity } : {}) };
        const needs = source => !topic || topic.charts.some(c => c.source === source);
        const [snap, tl, colour, colourSpace] = await Promise.all([
            this._fetch('/api/library/statistics', scope),
            needs('tl') ? this._fetch('/api/library/timeline', over) : null,
            needs('colour') ? this._fetch('/api/library/colour', over) : null,
            needs('colourSpace') ? this._fetch('/api/library/colour-space', over) : null,
        ]);
        return { snap, tl: continuousTimeline(tl), colour: continuousColour(colour), colourSpace };
    }

    async _fetch(url, params) {
        const qs = new URLSearchParams(params).toString();
        const r = await fetch(qs ? `${url}?${qs}` : url);
        if (!r.ok) throw new Error((await r.text()).trim() || r.statusText);
        return r.json();
    }

    // Words only after 400 ms (Activity), and only while nothing is shown
    // yet; a revisit refreshes behind what is already there.
    async _whileLoading(read) {
        if (this._drawn) return read();
        this._noteEl.hidden = false;
        Activity.in(this._noteEl, 'Counting the photos…', { area: true });
        const result = await read();
        this._noteEl.hidden = true;
        this._noteEl.innerHTML = '';
        return result;
    }

    _note(html, isError = false) {
        this._main.innerHTML = '';
        this._drawn = false;
        this._noteEl.hidden = false;
        this._noteEl.classList.toggle('stats-note-error', isError);
        this._noteEl.innerHTML = `<p>${html}</p>`;
    }

    /* --- Overview and topics --- */

    _drawOverview(data) {
        const cards = document.createElement('div');
        cards.className = 'stats-cards';
        for (const topic of STATS_TOPICS) {
            const card = document.createElement('a');
            card.className = 'stats-card';
            card.href = statsHash({ ...this._route, topic: topic.id });
            card.dataset.statsTopic = topic.id;
            card.innerHTML = `
                <span class="stats-card-title">${escapeHtml(topic.label)}</span>
                <span class="stats-card-blurb">${escapeHtml(topic.blurb)}</span>
                <span class="stats-card-preview" aria-hidden="true"></span>`;
            cards.appendChild(card);
            // A preview fills its card: it is a picture of the topic, not read.
            drawSafely(card.querySelector('.stats-card-preview'), el => {
                topic.preview(el, data);
                el.querySelector('.stats-svg')?.style.removeProperty('max-width');
            });
        }
        this._main.appendChild(cards);
    }

    _drawTopic(topic, data) {
        const grid = document.createElement('div');
        grid.className = 'stats-grid' + (topic.wide ? ' stats-grid--wide' : '');
        for (const chart of topic.charts) {
            if (chart.when && !chart.when(data)) continue;
            grid.appendChild(this._chartCard(chart, data));
        }
        this._main.appendChild(grid);
    }

    _chartCard(chart, data) {
        const card = document.createElement('section');
        card.className = 'stats-chart' + (chart.full ? ' stats-chart--full' : '');
        const coverage = chart.coverage ? coverageLine(chart.coverage(data), data.snap.totalPhotos) : '';
        const sub = [chart.subtitle, coverage].filter(Boolean).join(' · ');
        const title = `<h2 class="stats-chart-title">${escapeHtml(chart.title)}</h2>`;
        card.innerHTML = `
            ${chart.stage ? `<div class="stats-chart-head">${title}<button class="btn btn-sm stats-stage-btn"></button></div>` : title}
            ${sub ? `<p class="stats-chart-subtitle">${escapeHtml(sub)}</p>` : ''}
            <div class="stats-chart-content"></div>`;
        card.querySelector('.stats-stage-btn')?.addEventListener('click', () => this._setStage(!this._stage));
        drawSafely(card.querySelector('.stats-chart-content'), el => chart.render(el, data, c => this._pick(c)));
        return card;
    }

    /* --- Full view --- */

    // A stage chart and the photo column over the whole window, dark in
    // both themes (ADR-0046); the head and the other charts step aside.
    _setStage(on) {
        this._stage = on;
        const place = this.container.querySelector('.stats-place');
        if (!place) return;
        place.classList.toggle('stats-place--stage', on);
        if (on) place.dataset.theme = 'dark';
        else delete place.dataset.theme;
        this._labelStageButton();
    }

    _labelStageButton() {
        const btn = this._main?.querySelector('.stats-stage-btn');
        if (btn) btn.textContent = this._stage ? 'Leave full view' : 'Full view';
    }

    // Escape leaves the full view before it closes the photos; returns
    // whether there was one to leave.
    leaveStage() {
        if (!this._stage) return false;
        this._setStage(false);
        return true;
    }

    /* --- The photos of a picked value --- */

    // Before a value is picked, the column holds every photo in the scope.
    _showAllPhotos() {
        const where = this._route.path ? `in ${this._route.path}` : this._lib ? `in ${this._lib.name}` : 'in all libraries';
        return this._showPhotos({ subject: `All photos ${where}`, params: {} });
    }

    // A value picked in a chart opens the column, and keeps it open on the
    // next visit.
    _pick(criterion) {
        keepPhotoColumnOpen(STATS_PHOTOS_KEY, true);
        return this._showPhotos(criterion);
    }

    // A criterion is { subject, params }: what the photos have in common, and
    // the search that finds them, within the scope shown.
    async _showPhotos({ subject, params }) {
        const generation = ++this._pickGeneration;
        const query = { ...params, ...this._scopeParams() };
        const read = async (offset) => {
            const page = await LibraryAPI.search({ ...query, offset, limit: STATS_PHOTOS_PAGE });
            return {
                total: page.total,
                photos: page.results.map(r => ({ lib: r.libraryID, id: r.id, name: r.filename, taken: r.dateTaken })),
            };
        };
        let first;
        try {
            first = await read(0);
        } catch (err) {
            App.showToast(`The photos could not be read: ${err.message}`);
            return;
        }
        if (generation !== this._pickGeneration) return;
        this._photos.el.hidden = false;
        this._photos.show(first.photos, {
            subject,
            total: first.total,
            more: async (offset) => (await read(offset)).photos,
        });
    }

    // Escape or Done closes the column, and it stays closed on the next visit;
    // returns whether there was one to close.
    closePhotos() {
        if (!this._hidePhotos()) return false;
        keepPhotoColumnOpen(STATS_PHOTOS_KEY, false);
        return true;
    }

    _hidePhotos() {
        this._pickGeneration++;
        if (!this._photos || this._photos.el.hidden) return false;
        this._photos.el.hidden = true;
        return true;
    }
}

// Where this browser keeps whether the column is open.
const STATS_PHOTOS_KEY = 'stats-photos-open';

// Photos read at a time for the column; more follow as it scrolls.
const STATS_PHOTOS_PAGE = 200;

// A chart that cannot draw its data says so instead of taking the page down.
function drawSafely(el, draw) {
    try {
        draw(el);
    } catch (err) {
        console.error(err);
        el.innerHTML = '<p class="stats-nodata">This chart could not be drawn.</p>';
    }
}

// "412 of 1,000 photos" when a chart only knows some of the photos.
function coverageLine(valueCounts, total) {
    const n = (valueCounts ?? []).reduce((s, v) => s + v.count, 0);
    return n < total ? `${formatCount(n)} of ${formatCount(total)} photos` : '';
}
