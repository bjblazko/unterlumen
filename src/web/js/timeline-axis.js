// TimelineAxis — the desktop's detail axis (ADR-0040): the chosen range
// stretched across the width, its graph and calendar, and the frame marking
// the days the band shows. Dragging the frame or clicking the axis asks the
// band to go there (onSeek); while dragging, the band's reports do not move
// the frame. The frame being dragged is the one orange thing on screen.

const AXIS_LABELS_H = 32;

class TimelineAxis {
    constructor(stream, { height, onSeek, onRelease }) {
        this.stream = stream;
        this.onSeek = onSeek;
        this.onRelease = onRelease;
        this.el = document.createElement('div');
        this.el.className = 'timeline-axis';
        this.el.style.height = height + 'px';
        this.canvas = document.createElement('canvas');
        this.frame = document.createElement('div');
        this.frame.className = 'timeline-frame';
        this.frame.tabIndex = 0;
        this.frame.setAttribute('role', 'slider');
        this.frame.setAttribute('aria-label', 'Time shown');
        this.frame.style.height = (height - AXIS_LABELS_H) + 'px';
        this.tip = document.createElement('div');
        this.tip.className = 'timeline-tip';
        this.tip.hidden = true;
        this.el.append(this.canvas, this.frame, this.tip);
        this.a = 0;
        this.b = Math.max(0, stream.span - 1);
        this.view = [this.b, this.b];
        this.dragging = false;
        this.el.addEventListener('pointerdown', e => this._down(e));
        this.frame.addEventListener('keydown', e => this._key(e));
        new ResizeObserver(() => this.redraw()).observe(this.el);
        TimelineChart.onRedraw(() => this.redraw());
    }

    setDomain(a, b) { this.a = a; this.b = b; this.redraw(); }
    setView(a, b) { if (this.dragging) return; this.view = [a, b]; this._place(); }

    x(d) { return (tlClamp(d, this.a, this.b + 1) - this.a) / (this.b + 1 - this.a) * this.w; }
    dayAt(x) { return tlClamp(Math.floor(this.a + x / this.w * (this.b + 1 - this.a)), this.a, this.b); }

    redraw() {
        this.w = this.el.clientWidth;
        if (!this.w || !this.stream.count) return;
        const { ctx, w, h } = TimelineChart.prepare(this.canvas, this.el), c = TimelineChart.colours();
        const bottom = h - AXIS_LABELS_H, xOf = d => this.x(d), end = this.b + 1;
        TimelineChart.area(ctx, this.stream, xOf, this.a, end, 6, bottom, c, w / (end - this.a));
        TimelineChart.calendar(ctx, this.stream, xOf, this.a, end, 6, bottom, w, c, true);
        this._place();
    }

    _place() {
        if (!this.w) return;
        const [a, b] = this.view, cal = this.stream.calendar;
        const x0 = this.x(a), width = Math.max(8, this.x(b + 1) - x0);
        this.frame.style.left = Math.min(x0, this.w - width) + 'px';
        this.frame.style.width = width + 'px';
        this.frame.setAttribute('aria-valuetext', a === b ? cal.formatMonth(a) : `${cal.formatMonth(a)} to ${cal.formatMonth(b)}`);
    }

    _down(e) {
        if (e.button !== 0) return;
        const r = this.el.getBoundingClientRect(), onFrame = this.frame.contains(e.target);
        const span = this.view[1] - this.view[0], x0 = this.x(this.view[0]);
        const grab = onFrame ? e.clientX - r.left - x0 : (this.x(this.view[1] + 1) - x0) / 2;
        this.dragging = true;
        this.frame.classList.add('is-adjusting');
        this.el.setPointerCapture(e.pointerId);
        const move = ev => this._drag(ev.clientX - r.left - grab, span);
        const up = () => {
            this.dragging = false;
            this.frame.classList.remove('is-adjusting');
            this.tip.hidden = true;
            this.el.removeEventListener('pointermove', move);
            this.el.removeEventListener('pointerup', up);
            this.el.removeEventListener('pointercancel', up);
            this.onRelease?.();
        };
        this.el.addEventListener('pointermove', move);
        this.el.addEventListener('pointerup', up);
        this.el.addEventListener('pointercancel', up);
        if (!onFrame) move(e);
        e.preventDefault();
        this.frame.focus({ preventScroll: true });
    }

    _drag(x, span) {
        const d = tlClamp(this.dayAt(x), this.a, Math.max(this.a, this.b - span));
        this.view = [d, d + span];
        this._place();
        this.tip.hidden = false;
        this.tip.textContent = this.stream.calendar.formatMonth(d);
        this.tip.style.left = tlClamp(this.x(d), 30, this.w - 30) + 'px';
        this.onSeek(d);
    }

    _key(e) {
        const steps = { ArrowLeft: -30, ArrowRight: 30 };
        let d;
        if (e.key in steps) d = this.view[0] + steps[e.key] * (e.shiftKey ? 12 : 1);
        else if (e.key === 'Home') d = this.a;
        else if (e.key === 'End') d = this.b;
        else return;
        e.preventDefault();
        e.stopPropagation();
        this.onSeek(tlClamp(d, this.a, this.b));
    }
}
