// TimelineCalendar — the days of a timeline, counted from its first photo's
// day (ADR-0040). Day d is start + d days in UTC, so a photo never moves to
// another day with the browser's time zone. Months are keyed year * 12 + month.

const TIMELINE_DAY_MS = 864e5;
const TIMELINE_MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const TIMELINE_MONTHS_LONG = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const TIMELINE_WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

function tlClamp(v, a, b) {
    return Math.max(a, Math.min(b, v));
}

class TimelineCalendar {
    // start: the first day, 'YYYY-MM-DD'; span: how many days, at least 1.
    constructor(start, span) {
        const [y, m, d] = start.split('-').map(Number);
        this.epoch = Date.UTC(y, m - 1, d);
        this.span = span;
        this.year = new Uint16Array(span);
        this.month = new Uint32Array(span);
        this.date = new Uint8Array(span);
        for (let i = 0; i < span; i++) {
            const t = new Date(this.msOf(i));
            this.year[i] = t.getUTCFullYear();
            this.month[i] = t.getUTCFullYear() * 12 + t.getUTCMonth();
            this.date[i] = t.getUTCDate();
        }
        this.firstMonth = this.month[0];
        this.lastMonth = this.month[span - 1];
        this._monthStarts = [];
        for (let k = this.firstMonth; k <= this.lastMonth + 1; k++) {
            this._monthStarts.push(tlClamp(this.dayAt(Date.UTC(Math.floor(k / 12), k % 12, 1)), 0, span));
        }
    }

    msOf(d) { return this.epoch + d * TIMELINE_DAY_MS; }
    dayAt(ms) { return Math.round((ms - this.epoch) / TIMELINE_DAY_MS); }
    monthStart(k) { return this._monthStarts[k - this.firstMonth]; }
    monthEnd(k) { return this._monthStarts[k - this.firstMonth + 1]; }
    yearStart(y) { return tlClamp(this.dayAt(Date.UTC(y, 0, 1)), 0, this.span); }

    formatMonth(d) { return `${TIMELINE_MONTHS[this.month[d] % 12]} ${this.year[d]}`; }
    formatMonthLong(d) { return `${TIMELINE_MONTHS_LONG[this.month[d] % 12]} ${this.year[d]}`; }
    formatDay(d) {
        const weekday = TIMELINE_WEEKDAYS[new Date(this.msOf(d)).getUTCDay()];
        return `${weekday} ${this.date[d]} ${TIMELINE_MONTHS[this.month[d] % 12]} ${this.year[d]}`;
    }
}
