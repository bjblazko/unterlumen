// metadata-date.js — the correction of the date taken in the metadata
// editor (ADR-0060), worked out per file from its own date: Set the day puts
// every file on one day and keeps each one's time of day; Shift moves every
// file by the same amount, so the time between them stays.

const META_DATE_MODES = [
    ['keep', 'Keep'],
    ['day', 'Set the day'],
    ['shift', 'Shift'],
];

class MetadataDate {
    constructor(el, states, { onChange }) {
        this.el = el;
        this.states = states;
        this.onChange = onChange;
        this.mode = 'keep';
        this.render();
    }

    // dateFix gives the correction to send, or null for none.
    dateFix() {
        if (this.mode === 'day') {
            const day = this.el.querySelector('#meta-day')?.value;
            if (!day) return null;
            const time = this.el.querySelector('#meta-day-time')?.value;
            return { mode: 'day', day, ...(time ? { time } : {}) };
        }
        if (this.mode === 'shift') {
            const n = (id) => Number(this.el.querySelector(id)?.value || 0);
            const seconds = ((n('#meta-shift-days') * 24 + n('#meta-shift-hours')) * 60 + n('#meta-shift-minutes')) * 60;
            return seconds ? { mode: 'shift', shiftSeconds: seconds } : null;
        }
        return null;
    }

    render() {
        const s = metaSummary(this.states, 'dateTaken');
        const now = s.same && s.value ? formatDateTime(s.value) : (s.same ? t('None') : t('Several values'));
        this.el.innerHTML = `<div class="meta-field-head">
                <span class="form-label">${t('Date taken')}</span>
                <span class="meta-source">${escapeHtml(metaSourceText(s.sources))}</span>
            </div>
            <div class="meta-date-now${s.same && s.value ? ' is-date' : ''}">${escapeHtml(now)}</div>
            <div class="meta-date-mode"></div>
            <div class="meta-date-inputs"></div>`;
        this.el.querySelector('.meta-date-mode').appendChild(segmentField({
            label: 'Change',
            options: META_DATE_MODES,
            value: this.mode,
            onChange: (mode) => { this.mode = mode; this._renderInputs(); this.onChange(); },
        }));
        this._renderInputs();
    }

    _renderInputs() {
        const box = this.el.querySelector('.meta-date-inputs');
        box.innerHTML = this.mode === 'day' ? this._dayHTML() : this.mode === 'shift' ? this._shiftHTML() : '';
        for (const input of box.querySelectorAll('input')) input.addEventListener('input', () => this.onChange());
    }

    _dayHTML() {
        const undated = this.states.filter(s => s.current && !s.current.fields?.dateTaken).length;
        const time = undated ? `<label class="form-label" for="meta-day-time">${t('Time for files without a date')}</label>
            <input type="time" step="1" id="meta-day-time" class="form-input meta-input meta-time">
            <span class="form-hint">${t('{files} have no date. Without a time they are left as they are.', { files: countText(undated, 'file') })}</span>` : '';
        return `<label class="form-label" for="meta-day">${t('Day')}</label>
            <input type="date" id="meta-day" class="form-input meta-input meta-day">
            <span class="form-hint">${t('Each file keeps its own time of day.')}</span>${time}`;
    }

    _shiftHTML() {
        const unit = (id, label) => `<label class="meta-shift-unit"><span class="form-label">${t(label)}</span>
            <input type="number" step="1" value="0" id="${id}" class="form-input meta-input meta-shift"></label>`;
        return `<div class="meta-shift-row">${unit('meta-shift-days', 'Days')}${unit('meta-shift-hours', 'Hours')}${unit('meta-shift-minutes', 'Minutes')}</div>
            <span class="form-hint">${t('Every file moves by the same amount, so the time between them stays. A minus goes back. Files without a date are left as they are.')}</span>`;
    }
}
