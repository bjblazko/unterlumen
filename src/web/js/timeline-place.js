// timeline-place.js — the Timeline place: every dated library photo on one
// time axis, each photo once (ADR-0040). A desk gets the band in rows and a
// time bar (overview to limit, axis with the frame); a phone gets a list,
// newest first, and a scrubber. Both are built from one TimelineStream.

const TIMELINE_PHONE = '(max-width: 700px)';
const TIMELINE_VIEWER_REACH = 500;
// While a scan runs the stream changes with every photo; wait this long after
// hearing so before reading it again, so a scan is not answered with a reload
// per photo.
const TIMELINE_RELOAD_DELAY_MS = 1000;

class TimelinePane {
    constructor(container) {
        this.container = container;
        this.stream = new TimelineStream();
        this.stream.onDetails = () => this._lane?.tiles.fill();
        this.stream.onStale = () => this._reloadSoon();
        this.infoPanel = null;
        this._lane = null;
        this._parts = [];
        this._reloading = null;
        this._reloadTimer = 0;
        this._rows = readTimelineRows();
        this._phone = matchMedia(TIMELINE_PHONE);
        this._phone.addEventListener('change', () => {
            if (this._body && this.stream.count) this._build(this._currentMs());
        });
    }

    // Reads the stream again on every visit — a scan may have added photos —
    // and stays at the date it showed.
    async render() {
        if (!this.container.firstChild) this._buildShell();
        else this._filter.sync();
        this.stream.scope = ScopeState.query({ time: false });
        await this._reload();
    }

    _buildShell() {
        this.container.innerHTML = `
            <div class="timeline-place">
                <div class="timeline-split">
                    <div class="place-main">
                ${placeHeaderHTML({
                    title: t('Timeline'),
                    meta: '<span class="timeline-count"></span> <span class="timeline-undated"></span>',
                    actions: '<span class="timeline-actions"></span>',
                    lede: placeLede(t('Every dated photo from all {libraries} on one time axis. A folder no library catalogs does not appear.', { libraries: placeLink('library', 'libraries', t('libraries')) })),
                })}
                    <div class="timeline-body"></div>
                    <div class="timeline-note" hidden></div>
                    </div>
                </div>
            </div>`;
        this._body = this.container.querySelector('.timeline-body');
        this._noteEl = this.container.querySelector('.timeline-note');
        // The Timeline's one setting stands in its head: a ⋯ for one entry hid it.
        this._rowsField = segmentField({
            label: t('Rows'), options: [2, 3, 4].map(n => [n, String(n)]), value: this._rows,
            onChange: (v) => this._setRows(Number(v)),
        });
        // The shared filter (ADR-0050). Its months are not asked of the
        // server: the stream is read whole and its axis and band show the
        // span, so a change of span only moves them. Its column and the info
        // panel share the right edge.
        this._filter = new ScopeFilter({
            onOpen: () => { if (this.infoPanel?.expanded) this.infoPanel._setExpanded(false); },
        });
        this._infoBtn = panelToggleButton('Info');
        this.container.querySelector('.timeline-actions').replaceWith(this._filter.el, this._rowsField);
        // The filter and the info panel stand beside the place's whole
        // height, as the tool strip does.
        this._infoEl = document.createElement('div');
        this._infoEl.className = 'lib-info-panel-container timeline-info';
        this.container.querySelector('.timeline-split').append(this._filter.panel, this._infoEl, toolStrip(this._filter.toggle, this._infoBtn));
        ScopeState.onChange(() => {
            if (App.mode !== 'timeline') return;
            const scope = ScopeState.query({ time: false });
            if (scope === this.stream.scope) { this._applySpan(); return; }
            this.stream.scope = scope;
            this._reloadSoon();
        });
    }

    _setRows(n) {
        this._rows = n;
        writeTimelineRows(n);
        this._lane?.setRows?.(n);
    }

    _reloadSoon() {
        clearTimeout(this._reloadTimer);
        this._reloadTimer = setTimeout(() => this._reload(), TIMELINE_RELOAD_DELAY_MS);
    }

    // One reload at a time: a second caller waits for the one under way.
    _reload() {
        this._reloading ??= this._readStream().finally(() => { this._reloading = null; });
        return this._reloading;
    }

    async _readStream() {
        const at = this._currentMs();
        if (!this._lane) {
            this._noteEl.hidden = false;
            Activity.in(this._noteEl, t('Reading the timeline…'), { area: true });
        }
        let changed;
        try {
            changed = await this.stream.load();
        } catch (err) {
            this._noteEl.hidden = false;
            Activity.in(this._noteEl, '', { area: true })
                .fail(t('The timeline could not be read: {err}. Reload the page to try again.', { err: err.message }));
            return;
        }
        this._noteEl.innerHTML = '';
        this._noteEl.hidden = true;
        if (changed || !this._lane) this._build(at);
        else this._lane.render(); // asks again for the pages a 409 turned away
    }

    // The date shown now, as UTC milliseconds, so it survives a new skeleton.
    _currentMs() {
        const cal = this.stream.calendar;
        if (!cal || !this._lane) return null;
        return cal.msOf(this._lane.shownDay());
    }

    _head() {
        const s = this.stream, cal = s.calendar;
        this.container.querySelector('.timeline-count').innerHTML = s.count
            ? `${countHTML(s.count, 'photo')} <span class="nowrap">· ${cal.formatMonth(0)} – ${cal.formatMonth(s.span - 1)}</span>` : '';
        this.container.querySelector('.timeline-undated').textContent = s.undated
            ? `${formatCount(s.undated)} without a date are not shown` : '';
        this._rowsField.hidden = this._phone.matches || !s.count;
    }

    _build(atMs) {
        this._parts.forEach(part => part.dispose());
        this._parts = [];
        this._lane = null;
        this.infoPanel = null;
        this._axis = null;
        if (this._infoEl) this._infoEl.innerHTML = '';
        this._body.innerHTML = '';
        this._head();
        if (!this.stream.count) { this._infoBtn.hidden = true; this._explainEmpty(); return; }
        const last = this.stream.span - 1;
        const day = atMs == null ? last : tlClamp(this.stream.calendar.dayAt(atMs), 0, last);
        if (this._phone.matches) this._buildPhone(day); else this._buildDesk(day);
        // Info belongs to the desk's band; without one there is nothing to describe.
        this._infoBtn.hidden = !this.infoPanel;
    }

    _explainEmpty() {
        this._noteEl.hidden = false;
        this._noteEl.innerHTML = ScopeState.narrowed()
            ? t('No dated photo matches the filter. Widen it, or reset it.')
            : this.stream.undated
            ? t('None of the photos in the libraries has a date taken, so there is nothing to place on the timeline.')
            : t('There are no photos in any library yet. Make a library in {libraries}, and its dated photos appear here.', { libraries: placeLink('library', 'libraries', t('Libraries')) });
    }

    _buildDesk(day) {
        const grid = document.createElement('div');
        grid.className = 'timeline-desk';
        grid.innerHTML = `
            <div class="timeline-main"><div class="timeline-band-wrap"></div><div class="timeline-bar"></div></div>`;
        this._body.appendChild(grid);
        this.infoPanel = new InfoPanel(this._infoEl);
        this.infoPanel.attachToggle(this._infoBtn);
        const band = new TimelineBand(this.stream, {
            rows: this._rows,
            onView: (a, b) => axis.setView(a, b),
            onSelect: (i) => this._showInfo(i),
            onOpen: (i) => this._open(i),
        });
        band.startDay = day;
        this.infoPanel.onToggle = () => {
            if (!this.infoPanel.expanded) return;
            this._filter.close();
            if (band.selected >= 0) this._showInfo(band.selected);
        };
        const axis = new TimelineAxis(this.stream, {
            height: 92,
            onSeek: (d) => band.scrollToDay(d),
            onRelease: () => band.render(),
        });
        grid.querySelector('.timeline-band-wrap').appendChild(band.el);
        grid.querySelector('.timeline-bar').append(axis.el);
        this._lane = band;
        this._axis = axis;
        this._parts.push(band, axis);
        // A span set before keeps the band where it was unless it limits it.
        if (ScopeState.value.from || ScopeState.value.until) this._applySpan();
    }

    // The filter's Taken limits the axis and the band to its months; without
    // it they show every day there is.
    _applySpan() {
        if (!this._axis || !this.stream.count) return;
        const [a, b] = this._span();
        this._axis.setDomain(a, b);
        this._lane.setRange(a, b);
    }

    _span() {
        const v = ScopeState.value, cal = this.stream.calendar, last = this.stream.span - 1;
        const from = monthNumber(v.from), until = monthNumber(v.until);
        const a = from === null || from <= cal.firstMonth ? 0 : from > cal.lastMonth ? last : cal.monthStart(from);
        const b = until === null || until >= cal.lastMonth ? last : until < cal.firstMonth ? 0 : cal.monthEnd(until) - 1;
        return a <= b ? [a, b] : [0, last];
    }

    _buildPhone(day) {
        const wrap = document.createElement('div');
        wrap.className = 'timeline-phone';
        this._body.appendChild(wrap);
        const list = new TimelineList(this.stream, {
            onView: (a, b) => scrub.setDay(b),
            onOpen: (i) => this._open(i),
        });
        list.startDay = day;
        const scrub = new TimelineScrubber(this.stream, {
            onSeek: (d) => list.scrollToDay(d),
            onRelease: () => list.render(),
        });
        wrap.append(list.el, scrub.el);
        this._lane = list;
        this._parts.push(list, scrub);
    }

    // The panel shows the selected photo while it is open; it does not open
    // by itself, since opening narrows the band and moves the tiles away from
    // under a double click.
    _showInfo(i) {
        const p = this.stream.detail(i);
        if (!p || !this.infoPanel?.expanded) return;
        this.infoPanel.loadFromURL(`/api/library/${p.lib}/photo/${p.id}/info`, `lib:${p.lib}:${p.id}`);
    }

    // The viewer gets the photos around the one opened, read-only as from
    // the Map: 100 000 keys would be too many to hand over at once.
    async _open(i) {
        const lo = Math.max(0, i - TIMELINE_VIEWER_REACH), hi = Math.min(this.stream.count, i + TIMELINE_VIEWER_REACH);
        await this.stream.ensure(lo, hi);
        const opened = this.stream.detail(i);
        if (!opened) return;
        const keyOf = (p) => `${p.lib}/${p.id}/${p.name}`;
        const byKey = new Map();
        for (let j = lo; j < hi; j++) {
            const p = this.stream.detail(j);
            if (p) byKey.set(keyOf(p), p);
        }
        const keys = [...byKey.keys()];
        if (this._phone.matches) keys.reverse();
        App.showViewer(keyOf(opened), keys, {
            readOnly: true,
            libraryRef: (k) => ({ lib: byKey.get(k).lib, id: byKey.get(k).id }),
            imageURLFn: (k) => LibraryAPI.photoURL(byKey.get(k).lib, byKey.get(k).id),
            thumbURLFn: (k) => LibraryAPI.thumbURL(byKey.get(k).lib, byKey.get(k).id),
            previewURLFn: (k) => LibraryAPI.thumbURL(byKey.get(k).lib, byKey.get(k).id),
            infoLoadFn: (k, panel) => {
                const p = byKey.get(k);
                panel.loadFromURL(`/api/library/${p.lib}/photo/${p.id}/info`, `lib:${p.lib}:${p.id}`);
            },
        });
    }
}

// The number of rows is remembered per browser; without storage it is 3.
function readTimelineRows() {
    try {
        const n = Number(localStorage.getItem('timeline-rows'));
        return [2, 3, 4].includes(n) ? n : 3;
    } catch {
        return 3;
    }
}

function writeTimelineRows(n) {
    try { localStorage.setItem('timeline-rows', String(n)); } catch { /* per browser only */ }
}
