// TimelineScrubber — the phone's time axis (ADR-0040): a column down the
// right edge, newest at the top, with year marks and a bar per month for how
// many photos it has. A finger put down anywhere on it and dragged moves the
// list; a bubble shows the month. The thumb being dragged is orange.

const SCRUB_PAD = 14;
const SCRUB_BAR_MAX = 22;

class TimelineScrubber {
    constructor(stream, { onSeek, onRelease }) {
        Object.assign(this, { stream, onSeek, onRelease });
        this.el = document.createElement('div');
        this.el.className = 'timeline-scrubber';
        this.el.tabIndex = 0;
        this.el.setAttribute('role', 'slider');
        this.el.setAttribute('aria-label', 'Jump to a date');
        this.canvas = document.createElement('canvas');
        this.thumb = document.createElement('div');
        this.thumb.className = 'timeline-scrubber-thumb';
        this.bubble = document.createElement('div');
        this.bubble.className = 'timeline-scrubber-bubble';
        this.bubble.hidden = true;
        this.el.append(this.canvas, this.thumb, this.bubble);
        this.day = Math.max(0, stream.span - 1);
        this.dragging = false;
        this.el.addEventListener('pointerdown', e => this._down(e));
        this.el.addEventListener('keydown', e => this._key(e));
        new ResizeObserver(() => this.redraw()).observe(this.el);
        TimelineChart.onRedraw(() => this.redraw());
    }

    y(d) { return SCRUB_PAD + (this.stream.span - d) / this.stream.span * (this.h - SCRUB_PAD * 2); }
    dayAt(y) { return tlClamp(Math.floor(this.stream.span - (y - SCRUB_PAD) / (this.h - SCRUB_PAD * 2) * this.stream.span), 0, this.stream.span - 1); }

    redraw() {
        this.h = this.el.clientHeight;
        if (!this.h || !this.stream.count) return;
        const { ctx, w } = TimelineChart.prepare(this.canvas, this.el), c = TimelineChart.colours();
        this._bars(ctx, w, c);
        this._years(ctx, w, c);
        this._place();
    }

    _bars(ctx, w, c) {
        const cal = this.stream.calendar;
        let max = 0;
        for (let k = cal.firstMonth; k <= cal.lastMonth; k++) max = Math.max(max, this.stream.monthCount(k));
        ctx.fillStyle = c.bar;
        for (let k = cal.firstMonth; k <= cal.lastMonth; k++) {
            const n = this.stream.monthCount(k);
            if (!n) continue;
            const y0 = this.y(cal.monthEnd(k)), y1 = this.y(cal.monthStart(k)), bw = Math.max(1, n / max * SCRUB_BAR_MAX);
            ctx.fillRect(w - 4 - bw, y0, bw, Math.max(0.8, y1 - y0));
        }
    }

    _years(ctx, w, c) {
        const cal = this.stream.calendar;
        ctx.font = `500 10px ${TIMELINE_MONO}`;
        ctx.textBaseline = 'middle';
        let last = -Infinity;
        for (let yr = cal.year[this.stream.span - 1]; yr >= cal.year[0]; yr--) {
            const yy = this.y(cal.yearStart(yr));
            ctx.fillStyle = c.grid;
            ctx.fillRect(2, Math.round(yy), w - 4, 1);
            if (yy - last < 16) continue;
            ctx.fillStyle = c.fg2;
            ctx.fillText('’' + String(yr).slice(2), 4, yy - 7);
            last = yy;
        }
    }

    setDay(d) { if (!this.dragging) { this.day = d; this._place(); } }

    _place() {
        if (!this.h) return;
        this.thumb.style.top = this.y(this.day + 1) + 'px';
        this.el.setAttribute('aria-valuetext', this.stream.calendar.formatMonth(this.day));
    }

    _down(e) {
        const r = this.el.getBoundingClientRect();
        this.dragging = true;
        this.el.classList.add('is-adjusting');
        this.el.setPointerCapture(e.pointerId);
        const move = ev => this._drag(tlClamp(ev.clientY - r.top, SCRUB_PAD, this.h - SCRUB_PAD));
        const up = () => {
            this.dragging = false;
            this.el.classList.remove('is-adjusting');
            this.bubble.hidden = true;
            this.el.removeEventListener('pointermove', move);
            this.el.removeEventListener('pointerup', up);
            this.el.removeEventListener('pointercancel', up);
            this.onRelease?.();
        };
        this.el.addEventListener('pointermove', move);
        this.el.addEventListener('pointerup', up);
        this.el.addEventListener('pointercancel', up);
        move(e);
        e.preventDefault();
    }

    _drag(y) {
        this.day = this.dayAt(y);
        this._place();
        this.bubble.hidden = false;
        this.bubble.style.top = y + 'px';
        this.bubble.textContent = this.stream.calendar.formatMonth(this.day);
        this.onSeek(this.day);
    }

    _key(e) {
        const step = { ArrowUp: 30, ArrowDown: -30 }[e.key];
        if (!step) return;
        e.preventDefault();
        e.stopPropagation();
        this.onSeek(tlClamp(this.day + step, 0, this.stream.span - 1));
    }
}
