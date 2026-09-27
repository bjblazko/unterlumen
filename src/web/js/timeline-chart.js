// TimelineChart — drawing the timeline's graph and calendar on a canvas
// (ADR-0040). The graph is a step shape of photos per day within each bin,
// so a short bin at the edge is not dwarfed; it has no numbers, only a shape
// (ADR-0034: one series, slot of the sequential ramp, no legend).

const TIMELINE_BINS = [1, 2, 4, 7, 14, 30, 61, 91, 182, 365];
const TIMELINE_MONO = '"IBM Plex Mono", ui-monospace, monospace';

const TimelineChart = {
    _redraws: new Set(),

    prepare(canvas, box) {
        const dpr = window.devicePixelRatio || 1, w = box.clientWidth, h = box.clientHeight;
        canvas.width = Math.max(1, Math.round(w * dpr));
        canvas.height = Math.max(1, Math.round(h * dpr));
        const ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, w, h);
        return { ctx, w, h };
    },

    colours() {
        const s = getComputedStyle(document.documentElement), v = (n) => s.getPropertyValue(n).trim();
        return { area: v('--chart-seq-2'), bar: v('--chart-seq-3'), line: v('--chart-seq-4'),
            grid: v('--chart-grid'), fg2: v('--fg-2'), fg3: v('--fg-3') };
    },

    area(ctx, stream, xOf, a, b, top, bottom, c, pxPerDay) {
        const bin = TIMELINE_BINS.find(n => n * pxPerDay >= 3) || 365;
        const bins = [];
        let max = 0;
        for (let s = a; s < b; s += bin) {
            const e = Math.min(s + bin, b), v = (stream.prefix[e] - stream.prefix[s]) / (e - s);
            bins.push([s, e, v]);
            max = Math.max(max, v);
        }
        if (!max) return;
        ctx.beginPath();
        ctx.moveTo(xOf(a), bottom);
        for (const [s, e, v] of bins) {
            const y = bottom - v / max * (bottom - top);
            ctx.lineTo(xOf(s), y);
            ctx.lineTo(xOf(e), y);
        }
        ctx.lineTo(xOf(b), bottom);
        ctx.closePath();
        ctx.fillStyle = c.area;
        ctx.fill();
        ctx.lineWidth = 1;
        ctx.strokeStyle = c.line;
        ctx.stroke();
    },

    // Year lines and labels, and month names where a month is 26 px or wider.
    calendar(ctx, stream, xOf, a, b, top, bottom, w, c, withMonths) {
        const cal = stream.calendar, monthY = bottom + 12, yearY = withMonths ? bottom + 27 : bottom + 12;
        let lastYearX = -Infinity;
        for (let k = cal.month[a]; k <= cal.month[b - 1]; k++) {
            const s = Math.max(cal.monthStart(k), a), x0 = xOf(s), mw = xOf(Math.min(cal.monthEnd(k), b)) - x0;
            if (mw < 0.5) continue;
            const isYear = k % 12 === 0 || s === a;
            if (isYear) lastYearX = this._year(ctx, k, x0, lastYearX, top, yearY, w, c);
            if (withMonths) this._month(ctx, k, x0, mw, isYear, bottom, monthY, c);
        }
        ctx.fillStyle = c.grid;
        ctx.fillRect(0, bottom, w, 1);
    },

    _year(ctx, k, x0, lastX, top, yearY, w, c) {
        ctx.fillStyle = c.grid;
        ctx.fillRect(Math.round(x0), top, 1, yearY - top - 9);
        if (x0 - lastX < 40 || x0 + 30 > w + 2) return lastX;
        ctx.fillStyle = c.fg2;
        ctx.font = `500 11px ${TIMELINE_MONO}`;
        ctx.fillText(String(Math.floor(k / 12)), x0 + 4, yearY);
        return x0;
    },

    _month(ctx, k, x0, mw, isYear, bottom, monthY, c) {
        if (mw >= 26) {
            ctx.fillStyle = c.fg3;
            ctx.font = `400 10px ${TIMELINE_MONO}`;
            ctx.fillText(TIMELINE_MONTHS[k % 12], x0 + 4, monthY);
        } else if (mw >= 5 && !isYear) {
            ctx.fillStyle = c.grid;
            ctx.fillRect(Math.round(x0), bottom, 1, 4);
        }
    },

    // Canvases do not follow CSS: they draw again when the theme or the fonts change.
    onRedraw(fn) {
        this._redraws.add(fn);
    },
};

(() => {
    const redraw = () => TimelineChart._redraws.forEach(fn => fn());
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', redraw);
    new MutationObserver(redraw).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    document.fonts?.ready.then(redraw);
})();
