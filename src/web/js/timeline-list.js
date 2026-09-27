// TimelineList — the phone's photos (ADR-0040): justified rows, newest at the
// top, a heading per month that stays at the top while its month scrolls by.
// Only tiles near the screen exist (TimelineTiles). A tap opens the viewer.

const LIST = { rowH: 104, gap: 2, pad: 8, headH: 38 };

class TimelineList {
    constructor(stream, { onView, onOpen, onCount }) {
        Object.assign(this, { stream, onView, onOpen, onCount });
        this.el = document.createElement('div');
        this.el.className = 'timeline-list-wrap';
        this.scroller = document.createElement('div');
        this.scroller.className = 'timeline-list';
        this.scroller.setAttribute('aria-label', 'Photos by date taken, newest first');
        this.sticky = document.createElement('div');
        this.sticky.className = 'timeline-list-sticky';
        this.sticky.hidden = true;
        this.sizer = document.createElement('div');
        this.sizer.className = 'timeline-list-sizer';
        this.headsEl = document.createElement('div');
        const layer = document.createElement('div');
        this.sizer.append(this.headsEl, layer);
        this.scroller.appendChild(this.sizer);
        this.el.append(this.scroller, this.sticky);
        this.tiles = new TimelineTiles(layer, stream);
        this.blocks = []; this.raf = 0;
        this.startDay = Math.max(0, stream.span - 1);
        this.scroller.addEventListener('scroll', () => this.schedule(), { passive: true });
        this.scroller.addEventListener('click', e => { const t = e.target.closest('.timeline-tile'); if (t) this.onOpen(+t.dataset.i); });
        new ResizeObserver(() => this.relayout()).observe(this.scroller);
    }

    relayout() {
        if (!this.scroller.clientWidth) return;
        const d = this.blocks.length ? this.topDay() : this.startDay;
        this._layout();
        this.scrollToDay(d);
        this.render();
    }

    _layout() {
        const { rowH, gap, pad, headH } = LIST, W = this.scroller.clientWidth - pad * 2, cal = this.stream.calendar;
        const blocks = [];
        let y = pad, key = -1, row = [], sum = 0;
        const flush = (last) => {
            if (!row.length) return;
            const h = Math.min((W - gap * (row.length - 1)) / sum, last ? rowH : Infinity);
            blocks.push({ t: 'r', top: y, h, items: row, d: this.stream.day[row[0]] });
            y += h + gap; row = []; sum = 0;
        };
        for (let i = this.stream.count - 1; i >= 0; i--) {
            const d = this.stream.day[i];
            if (cal.month[d] !== key) {
                flush(true);
                if (key !== -1) y += gap * 3;
                blocks.push({ t: 'h', top: y, h: headH, d, label: cal.formatMonthLong(d), n: this.stream.monthCount(cal.month[d]) });
                y += headH; key = cal.month[d];
            }
            row.push(i); sum += this.stream.ratio[i];
            if (sum * rowH + gap * (row.length - 1) >= W) flush(false);
        }
        flush(true);
        this.blocks = blocks;
        this.sizer.style.height = (y + pad) + 'px';
    }

    _blockAt(y) {
        const B = this.blocks;
        let lo = 0, hi = B.length;
        while (lo < hi) { const m = (lo + hi) >> 1; if (B[m].top + B[m].h >= y) hi = m; else lo = m + 1; }
        return Math.min(lo, B.length - 1);
    }

    topDay() { return this.blocks[this._blockAt(this.scroller.scrollTop + 8)].d; }

    scrollToDay(d) {
        const B = this.blocks;
        if (!B.length) return;
        let lo = 0, hi = B.length;
        while (lo < hi) { const m = (lo + hi) >> 1; if (B[m].d <= d) hi = m; else lo = m + 1; }
        let b = Math.min(lo, B.length - 1);
        if (b > 0 && B[b - 1].t === 'h' && B[b - 1].d === B[b].d) b--;
        this.scroller.scrollTop = Math.max(0, B[b].top - LIST.pad);
        this.schedule();
    }

    schedule() {
        if (!this.raf) this.raf = requestAnimationFrame(() => { this.raf = 0; this.render(); });
    }

    render() {
        const B = this.blocks;
        if (!B.length) return;
        const st = this.scroller.scrollTop, vh = this.scroller.clientHeight;
        const items = [], heads = [];
        let lo = Infinity, hi = -1;
        for (let b = this._blockAt(st - vh * 0.5); b < B.length && B[b].top <= st + vh * 1.5; b++) {
            if (B[b].t === 'h') { heads.push(B[b]); continue; }
            this._rowItems(B[b], items);
            lo = Math.min(lo, B[b].items[B[b].items.length - 1]); hi = Math.max(hi, B[b].items[0]);
        }
        this.tiles.show(items);
        this.headsEl.innerHTML = heads.map(h => this._head(h, `top:${h.top}px;left:${LIST.pad}px`)).join('');
        if (hi >= 0) this.stream.ensure(lo, hi + 1).then(() => this.tiles.fill()).catch(() => {});
        this._sticky(st);
        const a = B[this._blockAt(st + 8)].d, z = B[this._blockAt(st + vh - 8)].d;
        this.onView(Math.min(a, z), Math.max(a, z));
        this.onCount?.(this.tiles.size);
    }

    _rowItems(block, items) {
        let x = LIST.pad;
        for (const i of block.items) {
            const w = this.stream.ratio[i] * block.h;
            items.push({ i, x, y: block.top, w, h: block.h });
            x += w + LIST.gap;
        }
    }

    _head(h, style) {
        return `<div class="timeline-list-head" style="${style}">${h.label}<span class="timeline-list-count">${formatCount(h.n)}</span></div>`;
    }

    _sticky(st) {
        const B = this.blocks;
        let h = this._blockAt(st + 1);
        while (h > 0 && B[h].t !== 'h') h--;
        const head = B[h];
        this.sticky.hidden = !(head.t === 'h' && head.top + head.h < st + 34);
        if (!this.sticky.hidden) this.sticky.innerHTML = `${head.label}<span class="timeline-list-count">${formatCount(head.n)}</span>`;
    }
}
