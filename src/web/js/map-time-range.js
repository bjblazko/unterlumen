// MapTimeRange — a from–to range of months over the dates the photos were
// taken, drawn as a RangeSlider (ADR-0039). The shared filter of the Map and
// the Statistics holds it (ADR-0050).
//
// Months are counted as year * 12 + month index, read from the ISO date
// text itself, so a photo's month never shifts with the browser's time zone.

const MAP_TIME_DEBOUNCE_MS = 120;

function monthOf(iso) {
    const m = /^(\d{4})-(\d{2})/.exec(iso || '');
    return m ? Number(m[1]) * 12 + Number(m[2]) - 1 : null;
}

function formatMonth(month) {
    return new Date(Date.UTC(Math.floor(month / 12), month % 12))
        .toLocaleDateString('en', { month: 'short', year: 'numeric', timeZone: 'UTC' });
}

class MapTimeRange {
    // first, last: the months of the oldest and newest photo.
    // value: { from, until } to start at, or null for all of them.
    // onChange(range): range is { from, until } in months, or null for all
    // of them; called once the handles rest for a moment.
    constructor({ first, last, value = null, onChange }) {
        this._first = first;
        this._span = last - first;
        this._onChange = onChange;
        this._timer = null;

        this.el = document.createElement('div');
        this.el.className = 'map-time';
        this.el.innerHTML = '<span class="map-time-label">Taken</span><span class="map-time-value"></span>';
        this._value = this.el.querySelector('.map-time-value');

        this._slider = new RangeSlider({
            label: 'Taken',
            step: 1 / this._span,
            valueText: (pos) => formatMonth(this._monthAt(pos)),
            onInput: (minPos, maxPos) => this._input(minPos, maxPos),
        });
        this.el.insertBefore(this._slider.el, this._value);
        const from = Math.max(first, Math.min(last, value?.from ?? first));
        const until = Math.max(from, Math.min(last, value?.until ?? last));
        this._slider.setPositions((from - first) / this._span, (until - first) / this._span);
        this._show(from, until);
    }

    _monthAt(pos) {
        return this._first + Math.round(pos * this._span);
    }

    _input(minPos, maxPos) {
        const from = this._monthAt(minPos);
        const until = this._monthAt(maxPos);
        this._show(from, until);
        clearTimeout(this._timer);
        const all = from === this._first && until === this._first + this._span;
        this._timer = setTimeout(() => this._onChange(all ? null : { from, until }), MAP_TIME_DEBOUNCE_MS);
    }

    _show(from, until) {
        this._value.textContent = from === until
            ? formatMonth(from)
            : `${formatMonth(from)} – ${formatMonth(until)}`;
    }
}
