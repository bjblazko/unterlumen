// timeline-place.js — the Timeline place: every dated library photo on one
// time axis, each photo once (ADR-0040). A desk gets the band in rows and a
// time bar (overview to limit, axis with the frame); a phone gets a list,
// newest first, and a scrubber. Both are built from one TimelineStream.

const TIMELINE_PHONE = '(max-width: 700px)';
const TIMELINE_VIEWER_REACH = 500;

class TimelinePane {
    constructor(container) {
        this.container = container;
        this.stream = new TimelineStream();
        this.stream.onDetails = () => this._lane?.tiles.fill();
        this.stream.onStale = () => this._reload();
        this.infoPanel = null;
        this._lane = null;
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
        await this._reload();
    }

    _buildShell() {
        this.container.innerHTML = `
            <div class="timeline-place">
                <div class="timeline-head">
                    <h1 class="timeline-title">Timeline</h1>
                    <span class="timeline-count"></span>
                    <span class="timeline-undated"></span>
                    <span class="timeline-head-spacer"></span>
                </div>
                <div class="timeline-body"></div>
                <div class="timeline-note" hidden></div>
            </div>`;
        this._body = this.container.querySelector('.timeline-body');
        this._noteEl = this.container.querySelector('.timeline-note');
        this._menu = new Menu({ label: 'Timeline options', items: () => this._menuItems() });
        this.container.querySelector('.timeline-head').appendChild(this._menu.button);
    }

    _menuItems() {
        return [2, 3, 4].map(n => ({
            label: `${n} rows`,
            disabled: n === this._rows,
            title: n === this._rows ? 'Shown now' : undefined,
            onSelect: () => {
                this._rows = n;
                writeTimelineRows(n);
                this._lane?.setRows?.(n);
            },
        }));
    }

    async _reload() {
        const at = this._currentMs();
        if (!this._lane) {
            this._noteEl.hidden = false;
            Activity.in(this._noteEl, 'Reading the timeline…', { area: true });
        }
        try {
            await this.stream.load();
        } catch (err) {
            this._noteEl.hidden = false;
            Activity.in(this._noteEl, '', { area: true })
                .fail(`The timeline could not be read: ${err.message}. Reload the page to try again.`);
            return;
        }
        this._noteEl.hidden = true;
        this._build(at);
    }

    // The date shown now, as UTC milliseconds, so it survives a new skeleton.
    _currentMs() {
        const cal = this.stream.calendar;
        if (!cal || !this._lane) return null;
        return cal.msOf(this._phone.matches ? this._lane.topDay() : this._lane.leftDay());
    }

    _head() {
        const s = this.stream, cal = s.calendar;
        this.container.querySelector('.timeline-count').textContent = s.count
            ? `${formatCount(s.count)} photos · ${cal.formatMonth(0)} – ${cal.formatMonth(s.span - 1)}` : '';
        this.container.querySelector('.timeline-undated').textContent = s.undated
            ? `${formatCount(s.undated)} without a date are not shown` : '';
        this._menu.button.hidden = this._phone.matches || !s.count;
    }

    _build(atMs) {
        this._lane?.tiles.clear();
        this._lane = null;
        this.infoPanel = null;
        this._body.innerHTML = '';
        this._head();
        if (!this.stream.count) { this._explainEmpty(); return; }
        const last = this.stream.span - 1;
        const day = atMs == null ? last : tlClamp(this.stream.calendar.dayAt(atMs), 0, last);
        if (this._phone.matches) this._buildPhone(day); else this._buildDesk(day);
    }

    _explainEmpty() {
        this._noteEl.hidden = false;
        this._noteEl.textContent = this.stream.undated
            ? 'None of the photos in the libraries has a date taken, so there is nothing to place on the timeline.'
            : 'There are no photos in any library yet. Make a library in Libraries, and its dated photos appear here.';
    }

    _buildDesk(day) {
        const grid = document.createElement('div');
        grid.className = 'timeline-desk';
        grid.innerHTML = `
            <div class="timeline-main"><div class="timeline-band-wrap"></div><div class="timeline-bar"></div></div>
            <div class="lib-info-panel-container timeline-info"></div>`;
        this._body.appendChild(grid);
        this.infoPanel = new InfoPanel(grid.querySelector('.timeline-info'));
        const band = new TimelineBand(this.stream, {
            rows: this._rows,
            onView: (a, b) => axis.setView(a, b),
            onSelect: (i) => this._showInfo(i),
            onOpen: (i) => this._open(i),
        });
        band.startDay = day;
        const axis = new TimelineAxis(this.stream, {
            height: 92,
            onSeek: (d) => band.scrollToDay(d),
            onRelease: () => band.render(),
        });
        grid.querySelector('.timeline-band-wrap').appendChild(band.el);
        grid.querySelector('.timeline-bar').append(...this._barRows(band, axis), axis.el);
        this._lane = band;
    }

    _barRows(band, axis) {
        const cal = this.stream.calendar;
        const text = document.createElement('span');
        text.className = 'timeline-range-text';
        const range = new TimelineRange(this.stream, {
            height: 40,
            onLimit: (a, b) => { axis.setDomain(a, b); band.setRange(a, b); },
            onText: (a, b) => { text.textContent = `${cal.formatMonth(a)} – ${cal.formatMonth(b)}`; },
        });
        const all = document.createElement('button');
        all.type = 'button';
        all.className = 'btn btn-sm';
        all.textContent = 'Show all';
        all.addEventListener('click', () => range.set(0, this.stream.span - 1));
        const label = document.createElement('span');
        label.className = 'timeline-bar-label';
        label.textContent = 'Shown';
        const shown = document.createElement('div');
        shown.className = 'timeline-bar-row';
        shown.append(label, text, all);
        return [range.el, shown];
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
    }

    _showInfo(i) {
        const p = this.stream.detail(i);
        if (!p || !this.infoPanel) return;
        if (!this.infoPanel.expanded) this.infoPanel.toggle();
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
            imageURLFn: (k) => LibraryAPI.photoURL(byKey.get(k).lib, byKey.get(k).id),
            thumbURLFn: (k) => LibraryAPI.thumbURL(byKey.get(k).lib, byKey.get(k).id),
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
