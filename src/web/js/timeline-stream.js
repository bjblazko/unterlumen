// TimelineStream — the timeline's photos (ADR-0040). The skeleton (a day and
// an aspect ratio per photo) is loaded once and kept in typed arrays; the
// details (library, ID, file name, date) come by index range, in pages, only
// for what is near the screen. A page used longest ago is dropped first.

const TIMELINE_PAGE = 500;
const TIMELINE_PAGES_KEPT = 40;

class TimelineStream {
    constructor() {
        this.scope = '';       // the shared filter as a query string, without its months (ADR-0050)
        this.onDetails = null; // a page of details arrived
        this.onStale = null;   // the server's stream changed; load() again
        this.adopt({ version: '', start: '', days: [], ratios: [], undated: 0 });
    }

    // load reads the skeleton and says whether it differs from the one held.
    async load() {
        const r = await fetch(this.scope ? `/api/timeline?${this.scope}` : '/api/timeline');
        if (!r.ok) throw new Error(await r.text());
        const sk = await r.json();
        if (sk.version === this.version && this.count) {
            this._staleReported = null; // a page that heard otherwise may ask again
            return false;
        }
        this.adopt(sk);
        return true;
    }

    adopt(sk) {
        this.version = sk.version;
        this.undated = sk.undated;
        this.count = sk.days.length;
        this.day = Uint32Array.from(sk.days);
        this.ratio = Float32Array.from(sk.ratios);
        this.span = this.count ? this.day[this.count - 1] + 1 : 0;
        this.calendar = this.count ? new TimelineCalendar(sk.start, this.span) : null;
        this.prefix = new Uint32Array(this.span + 1);
        for (let i = 0; i < this.count; i++) this.prefix[this.day[i] + 1]++;
        for (let d = 0; d < this.span; d++) this.prefix[d + 1] += this.prefix[d];
        this._pages = new Map();
        this._pending = new Map();
    }

    monthCount(k) {
        const c = this.calendar;
        return this.prefix[c.monthEnd(k)] - this.prefix[c.monthStart(k)];
    }

    detail(i) {
        const rows = this._pages.get(Math.floor(i / TIMELINE_PAGE));
        const row = rows && rows[i % TIMELINE_PAGE];
        return row ? { lib: row[0], id: row[1], name: row[2], taken: row[3] } : null;
    }

    // ensure loads the pages that hold the indexes from ≤ i < to.
    ensure(from, to) {
        const first = Math.floor(Math.max(0, from) / TIMELINE_PAGE);
        const last = Math.floor((Math.min(this.count, to) - 1) / TIMELINE_PAGE);
        const loads = [];
        for (let p = first; p <= last; p++) loads.push(this._page(p));
        return Promise.all(loads);
    }

    _page(p) {
        if (this._pages.has(p)) {
            const rows = this._pages.get(p);
            this._pages.delete(p);
            this._pages.set(p, rows); // most recently used last
            return Promise.resolve();
        }
        if (this._pending.has(p)) return this._pending.get(p);
        const load = this._fetchPage(p, this.version).finally(() => {
            if (this._pending.get(p) === load) this._pending.delete(p);
        });
        this._pending.set(p, load);
        return load;
    }

    async _fetchPage(p, version) {
        const url = `/api/timeline/photos?v=${encodeURIComponent(version)}&from=${p * TIMELINE_PAGE}&count=${TIMELINE_PAGE}${this.scope ? '&' + this.scope : ''}`;
        const r = await fetch(url);
        if (r.status === 409) {
            // Every page in flight hears it; the place needs to hear it once.
            if (version === this.version && this._staleReported !== version) {
                this._staleReported = version;
                this.onStale?.();
            }
            return;
        }
        if (!r.ok) throw new Error(await r.text());
        const { photos } = await r.json();
        if (version !== this.version) return;
        this._pages.set(p, photos);
        while (this._pages.size > TIMELINE_PAGES_KEPT) this._pages.delete(this._pages.keys().next().value);
        this.onDetails?.();
    }
}
