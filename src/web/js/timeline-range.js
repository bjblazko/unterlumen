// TimelineRange — the desktop's overview (ADR-0040): the whole span as a
// small graph with year labels, and two bracket handles that limit the axis
// and the band to a range of whole months. Dragging the space between the
// handles moves the range. The handle being moved is orange.

const RANGE_LABELS_H = 14;
const RANGE_SETTLE_MS = 140;

class TimelineRange {
    constructor(stream, { height, onLimit, onText }) {
        this.stream = stream;
        this.onLimit = onLimit;
        this.onText = onText;
        this.el = document.createElement('div');
        this.el.className = 'timeline-range';
        this.el.style.height = height + 'px';
        this.chartH = height - RANGE_LABELS_H;
        this.canvas = document.createElement('canvas');
        this.outL = this._part('timeline-range-out');
        this.outR = this._part('timeline-range-out');
        this.sel = this._part('timeline-range-sel');
        this.hA = this._handle('min', 'From');
        this.hB = this._handle('max', 'Until');
        this.el.prepend(this.canvas);
        this.a = 0;
        this.b = Math.max(0, stream.span - 1);
        this._timer = 0;
        this.el.addEventListener('pointerdown', e => this._down(e));
        this.hA.addEventListener('keydown', e => this._key(e, 'a'));
        this.hB.addEventListener('keydown', e => this._key(e, 'b'));
        this._resize = new ResizeObserver(() => this.redraw());
        this._resize.observe(this.el);
        this._unredraw = TimelineChart.onRedraw(() => this.redraw());
    }

    dispose() {
        clearTimeout(this._timer);
        this._resize.disconnect();
        this._unredraw();
    }

    _part(cls) {
        const el = document.createElement('div');
        el.className = cls;
        el.style.height = this.chartH + 'px';
        this.el.appendChild(el);
        return el;
    }

    _handle(side, label) {
        const h = document.createElement('button');
        h.type = 'button';
        h.className = `timeline-range-h ${side}`;
        h.setAttribute('role', 'slider');
        h.setAttribute('aria-label', label);
        h.style.height = this.chartH + 'px';
        this.el.appendChild(h);
        return h;
    }

    x(d) { return d / this.stream.span * this.w; }
    dayAt(x) { return tlClamp(Math.floor(x / this.w * this.stream.span), 0, this.stream.span - 1); }

    redraw() {
        this.w = this.el.clientWidth;
        if (!this.w || !this.stream.count) return;
        const { ctx, w } = TimelineChart.prepare(this.canvas, this.el), c = TimelineChart.colours(), span = this.stream.span;
        TimelineChart.area(ctx, this.stream, d => this.x(d), 0, span, 3, this.chartH, c, w / span);
        TimelineChart.calendar(ctx, this.stream, d => this.x(d), 0, span, 2, this.chartH, w, c, false);
        this._place();
    }

    // set takes any two days and snaps them outwards to whole months.
    set(a, b) {
        const cal = this.stream.calendar;
        this.a = cal.monthStart(cal.month[a]);
        this.b = Math.min(this.stream.span, cal.monthEnd(cal.month[b])) - 1;
        this._place();
        this._emit(true);
    }

    _place() {
        if (!this.w) return;
        const xa = this.x(this.a), xb = this.x(this.b + 1), cal = this.stream.calendar;
        Object.assign(this.outL.style, { left: '0px', width: xa + 'px' });
        Object.assign(this.outR.style, { left: xb + 'px', width: (this.w - xb) + 'px' });
        Object.assign(this.sel.style, { left: xa + 'px', width: (xb - xa) + 'px' });
        this.hA.style.left = (xa - 7) + 'px';
        this.hB.style.left = xb + 'px';
        this.hA.setAttribute('aria-valuetext', cal.formatMonth(this.a));
        this.hB.setAttribute('aria-valuetext', cal.formatMonth(this.b));
        this.onText?.(this.a, this.b);
    }

    _emit(now) {
        clearTimeout(this._timer);
        const fire = () => this.onLimit(this.a, this.b);
        if (now) fire(); else this._timer = setTimeout(fire, RANGE_SETTLE_MS);
    }

    _down(e) {
        if (e.button !== 0) return;
        const r = this.el.getBoundingClientRect(), px = e.clientX - r.left;
        const mode = this._modeAt(e.target, px), handle = { a: this.hA, b: this.hB }[mode];
        const start = { a: this.a, b: this.b, d: this.dayAt(px) };
        handle?.classList.add('is-adjusting');
        this.el.setPointerCapture(e.pointerId);
        const move = ev => { this._drag(mode, this.dayAt(ev.clientX - r.left), start); this._place(); this._emit(false); };
        const up = () => {
            handle?.classList.remove('is-adjusting');
            this.el.removeEventListener('pointermove', move);
            this.el.removeEventListener('pointerup', up);
            this.el.removeEventListener('pointercancel', up);
            this._emit(true);
        };
        this.el.addEventListener('pointermove', move);
        this.el.addEventListener('pointerup', up);
        this.el.addEventListener('pointercancel', up);
        if (mode !== 'move') move(e);
        e.preventDefault();
    }

    _modeAt(target, px) {
        if (target === this.hA) return 'a';
        if (target === this.hB) return 'b';
        if (target === this.sel) return 'move';
        return Math.abs(px - this.x(this.a)) < Math.abs(px - this.x(this.b)) ? 'a' : 'b';
    }

    _drag(mode, d, start) {
        const cal = this.stream.calendar, last = this.stream.span - 1;
        if (mode === 'a') this.a = cal.monthStart(cal.month[Math.min(d, this.b)]);
        else if (mode === 'b') this.b = Math.min(last, cal.monthEnd(cal.month[Math.max(d, this.a)]) - 1);
        else {
            const span = start.b - start.a, a = tlClamp(start.a + d - start.d, 0, last - span);
            this.a = cal.monthStart(cal.month[a]);
            this.b = Math.min(last, this.a + span);
        }
    }

    _key(e, which) {
        const step = { ArrowLeft: -1, ArrowRight: 1 }[e.key];
        if (!step) return;
        e.preventDefault();
        e.stopPropagation();
        const cal = this.stream.calendar, n = step * (e.shiftKey ? 12 : 1);
        if (which === 'a') this.a = cal.monthStart(tlClamp(cal.month[this.a] + n, cal.firstMonth, cal.month[this.b]));
        else this.b = Math.min(this.stream.span, cal.monthEnd(tlClamp(cal.month[this.b] + n, cal.month[this.a], cal.lastMonth))) - 1;
        this._place();
        this._emit(false);
    }
}
