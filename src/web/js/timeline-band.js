// TimelineBand — the desktop's photos (ADR-0040): tiles of one height in 2–4
// rows, time running left to right, as the axis below it does. Each photo
// goes into the row that ends furthest left; a month starts a new column
// under its label. Only tiles near the screen exist (TimelineTiles).

const BAND = { gap: 4, pad: 16, labelH: 28, monthGap: 20, scrollbar: 14, margin: 900 };

class TimelineBand {
    constructor(stream, { rows, onView, onSelect, onOpen, onCount }) {
        Object.assign(this, { stream, rows, onView, onSelect, onOpen, onCount });
        this.el = document.createElement('div');
        this.el.className = 'timeline-band';
        this.el.tabIndex = 0;
        this.el.setAttribute('aria-label', 'Photos by date taken');
        this.sizer = document.createElement('div');
        this.sizer.className = 'timeline-band-sizer';
        this.labelsEl = document.createElement('div');
        const layer = document.createElement('div');
        this.sizer.append(this.labelsEl, layer);
        this.el.appendChild(this.sizer);
        this.tiles = new TimelineTiles(layer, stream);
        this.lo = 0; this.hi = stream.count; this.n = 0; this.raf = 0; this.selected = -1;
        this.startDay = Math.max(0, stream.span - 1);
        this._listen();
        new ResizeObserver(() => this.relayout()).observe(this.el);
    }

    _listen() {
        this.el.addEventListener('scroll', () => this.schedule(), { passive: true });
        this.el.addEventListener('wheel', e => {
            if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return;
            this.el.scrollLeft += e.deltaY;
            e.preventDefault();
        }, { passive: false });
        this.el.addEventListener('click', e => { const t = e.target.closest('.timeline-tile'); if (t) this.select(+t.dataset.i); });
        this.el.addEventListener('dblclick', e => { const t = e.target.closest('.timeline-tile'); if (t) this.onOpen(+t.dataset.i); });
        this.el.addEventListener('keydown', e => this._key(e));
    }

    setRange(a, b) {
        this.lo = this.stream.prefix[a];
        this.hi = this.stream.prefix[b + 1];
        this._layout();
        this.scrollToDay(a);
        this.render();
    }

    setRows(n) { this.rows = n; this.relayout(); }

    relayout() {
        if (!this.el.clientHeight) return;
        const d = this.n ? this.leftDay() : this.startDay;
        this._layout();
        this.scrollToDay(d);
        this.render();
    }

    _layout() {
        const { gap, pad, labelH, monthGap, scrollbar } = BAND;
        const rowH = (this.el.clientHeight - scrollbar - pad - labelH - gap * (this.rows - 1)) / this.rows;
        const n = this.hi - this.lo, month = this.stream.calendar?.month;
        this.xs = new Float32Array(n); this.ws = new Float32Array(n); this.rows_ = new Uint8Array(n);
        this.labels = [];
        const ends = new Float64Array(this.rows).fill(pad);
        let key = -1;
        for (let j = 0; j < n; j++) {
            const i = this.lo + j, d = this.stream.day[i];
            if (month[d] !== key) {
                const x = Math.max(...ends) + (key === -1 ? 0 : monthGap);
                ends.fill(x);
                this.labels.push({ x, d, j });
                key = month[d];
            }
            let r = 0;
            for (let q = 1; q < this.rows; q++) if (ends[q] < ends[r] - 0.5) r = q;
            this.xs[j] = ends[r]; this.ws[j] = this.stream.ratio[i] * rowH; this.rows_[j] = r;
            ends[r] += this.ws[j] + gap;
        }
        this.rowH = rowH; this.n = n;
        this.sizer.style.width = (Math.max(...ends) + pad) + 'px';
    }

    leftDay() {
        const j = this._firstEndingAfter(this.el.scrollLeft);
        return this.stream.day[this.lo + Math.min(j, this.n - 1)];
    }

    _firstEndingAfter(x) {
        let lo = 0, hi = this.n;
        while (lo < hi) { const m = (lo + hi) >> 1; if (this.xs[m] + this.ws[m] >= x) hi = m; else lo = m + 1; }
        return lo;
    }

    _firstStartingAfter(x) {
        let lo = 0, hi = this.n;
        while (lo < hi) { const m = (lo + hi) >> 1; if (this.xs[m] > x) hi = m; else lo = m + 1; }
        return lo;
    }

    scrollToDay(d) {
        if (!this.n) return;
        const j = tlClamp(this.stream.prefix[tlClamp(d, 0, this.stream.span)] - this.lo, 0, this.n - 1);
        const label = this.labels.find(l => l.j === j);
        this.el.scrollLeft = (label ? label.x : this.xs[j]) - BAND.pad;
        this.schedule();
    }

    schedule() {
        if (!this.raf) this.raf = requestAnimationFrame(() => { this.raf = 0; this.render(); });
    }

    render() {
        if (!this.n) { this.tiles.show([]); this.labelsEl.innerHTML = ''; return; }
        const sl = this.el.scrollLeft, vw = this.el.clientWidth;
        const j0 = this._firstStartingAfter(sl - vw * 0.5 - BAND.margin), j1 = this._firstStartingAfter(sl + vw * 1.5);
        this.tiles.show(this._items(j0, j1));
        this._labels(sl, vw);
        this.stream.ensure(this.lo + j0, this.lo + j1).then(() => this.tiles.fill()).catch(() => {});
        const last = tlClamp(this._firstStartingAfter(sl + vw) - 1, 0, this.n - 1);
        this.onView(this.leftDay(), this.stream.day[this.lo + last]);
        this.onCount?.(this.tiles.size);
    }

    _items(j0, j1) {
        const items = [], top0 = BAND.pad + BAND.labelH - 8;
        for (let j = j0; j < j1; j++) {
            items.push({ i: this.lo + j, x: this.xs[j], y: top0 + this.rows_[j] * (this.rowH + BAND.gap), w: this.ws[j], h: this.rowH });
        }
        return items;
    }

    _labels(sl, vw) {
        const cal = this.stream.calendar;
        this.labelsEl.innerHTML = this.labels
            .filter(l => l.x > sl - vw && l.x < sl + vw * 2)
            .map(l => `<div class="timeline-band-label" style="left:${l.x}px;top:${BAND.pad - 6}px">${cal.formatMonth(l.d)}</div>`)
            .join('');
    }

    select(i) {
        this.selected = i;
        this.tiles.select(i);
        this.onSelect(i);
    }

    _key(e) {
        if (this.selected < 0) return;
        if (e.key === 'Enter') { e.preventDefault(); this.onOpen(this.selected); return; }
        const step = { ArrowRight: 1, ArrowLeft: -1 }[e.key];
        if (!step) return;
        e.preventDefault();
        const i = tlClamp(this.selected + step, this.lo, this.hi - 1), x = this.xs[i - this.lo];
        const sl = this.el.scrollLeft, vw = this.el.clientWidth;
        if (x < sl || x > sl + vw - 100) this.el.scrollLeft = x - vw / 2;
        this.select(i);
    }
}
