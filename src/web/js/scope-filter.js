// scope-filter.js — the one filter of the Map, the Statistics and the
// Timeline (ADR-0050): libraries, a span of months, cameras and lenses, each
// optional and all of everything at first. Its state is shared, so a place
// shows what the others show; it is kept in the browser across visits.

const SCOPE_FILTER_KEY = 'scope.filter';
// A choice of none matches no photo: the server is sent a value no photo has.
const SCOPE_NONE = '∅';

const ScopeState = {
    // libraries, models, lenses: null for all, else the chosen values.
    // from, until: months as "YYYY-MM", '' for an open end.
    value: { libraries: null, from: '', until: '', models: null, lenses: null },
    _listeners: new Set(),

    load() {
        try {
            const saved = JSON.parse(localStorage.getItem(SCOPE_FILTER_KEY) || 'null');
            if (saved && typeof saved === 'object') this.value = { ...this.value, ...saved };
        } catch { /* a private window or old data: start unfiltered */ }
    },

    set(patch) {
        this.value = { ...this.value, ...patch };
        try { localStorage.setItem(SCOPE_FILTER_KEY, JSON.stringify(this.value)); } catch { /* not kept */ }
        for (const fn of this._listeners) fn(this.value);
    },

    reset() {
        this.set({ libraries: null, from: '', until: '', models: null, lenses: null });
    },

    onChange(fn) {
        this._listeners.add(fn);
        return () => this._listeners.delete(fn);
    },

    // Whether anything is narrowed; time only where it counts.
    narrowed({ time = true } = {}) {
        const v = this.value;
        return v.libraries !== null || v.models !== null || v.lenses !== null || (time && !!(v.from || v.until));
    },

    // The request parameters, as [key, value] pairs (model and lens repeat).
    params({ time = true } = {}) {
        const v = this.value;
        const pairs = [];
        if (v.libraries !== null) pairs.push(['ids', v.libraries.length ? v.libraries.join(',') : SCOPE_NONE]);
        if (time && v.from) pairs.push(['month_from', v.from]);
        if (time && v.until) pairs.push(['month_until', v.until]);
        for (const [key, values] of [['model', v.models], ['lens', v.lenses]]) {
            if (values === null) continue;
            for (const value of (values.length ? values : [SCOPE_NONE])) pairs.push([key, value]);
        }
        return pairs;
    },

    query(opts) {
        return new URLSearchParams(this.params(opts)).toString();
    },
};
ScopeState.load();

// "2024-07" ⇄ months since year 0, as MapTimeRange counts them.
function monthNumber(ym) {
    const m = /^(\d{4})-(\d{2})$/.exec(ym || '');
    return m ? Number(m[1]) * 12 + Number(m[2]) - 1 : null;
}

function monthText(n) {
    return `${String(Math.floor(n / 12)).padStart(4, '0')}-${String(n % 12 + 1).padStart(2, '0')}`;
}

// The EXIF values come quoted from the index; the filter shows them plain.
function plainExif(value) {
    return value.length >= 2 && value.startsWith('"') && value.endsWith('"') ? value.slice(1, -1) : value;
}

class ScopeFilter {
    // time: whether the span of months is offered (not on the Timeline,
    // whose axis is time). The element is mounted by the place; on a phone it
    // folds behind its Filter button.
    constructor({ time = true } = {}) {
        this._time = time;
        this.el = document.createElement('div');
        this.el.className = 'scope-filter';
        this.el.innerHTML = `
            <button type="button" class="btn btn-sm scope-filter-toggle" aria-expanded="false">Filter<span class="scope-filter-count"></span></button>
            <div class="scope-filter-controls">
                <span class="scope-filter-libraries"></span>
                <span class="scope-filter-time"></span>
                <span class="scope-filter-cameras"></span>
                <span class="scope-filter-lenses"></span>
                <button type="button" class="btn btn-sm scope-filter-reset" hidden>Reset</button>
            </div>`;
        this._toggle = this.el.querySelector('.scope-filter-toggle');
        this._toggle.addEventListener('click', () => {
            const open = this._toggle.getAttribute('aria-expanded') !== 'true';
            this._toggle.setAttribute('aria-expanded', String(open));
            this.el.classList.toggle('scope-filter--open', open);
        });
        this._resetBtn = this.el.querySelector('.scope-filter-reset');
        this._resetBtn.addEventListener('click', () => {
            ScopeState.reset();
            this.sync();
        });
        this._libraries = this._select('libraries', 'Libraries', 'libraries');
        this._cameras = this._select('cameras', 'Cameras', 'models');
        this._lenses = this._select('lenses', 'Lenses', 'lenses');
        this._unsubscribe = ScopeState.onChange(() => this._showNarrowed());
        this._showNarrowed();
        this._load();
    }

    destroy() { this._unsubscribe(); }

    // Takes the shared state again: after a reset, or when its place is shown
    // after another place changed it.
    sync() {
        const v = ScopeState.value;
        this._libraries.selected = v.libraries;
        this._cameras.selected = v.models;
        this._lenses.selected = v.lenses;
        this._showNarrowed();
        this._load();
    }

    _select(slot, label, key) {
        const select = new MultiSelect({
            label,
            selected: ScopeState.value[key],
            onChange: (selected) => {
                ScopeState.set({ [key]: selected });
                // Other libraries or cameras have other cameras and lenses.
                if (key !== 'lenses') this._load();
            },
        });
        this.el.querySelector(`.scope-filter-${slot}`).appendChild(select.el);
        return select;
    }

    // What there is to choose from, in the chosen libraries; the lenses of
    // the chosen cameras.
    async _load() {
        const gen = this._gen = (this._gen || 0) + 1;
        const v = ScopeState.value;
        const q = new URLSearchParams();
        if (v.libraries?.length) q.set('ids', v.libraries.join(','));
        for (const m of v.models || []) q.append('model', m);
        let libs, values;
        try {
            [libs, values] = await Promise.all([
                LibraryAPI.list(),
                fetch(`/api/library/scope-values?${q}`).then(r => (r.ok ? r.json() : Promise.reject(new Error(r.statusText)))),
            ]);
        } catch {
            return; // the place says what it cannot read; the filter stays as it was
        }
        if (gen !== this._gen) return;
        this._libraries.setOptions(libs.map(l => ({ value: String(l.id), label: l.name, count: l.photoCount })));
        this._cameras.setOptions(values.cameras.map(c => ({ value: c.name, label: plainExif(c.name), count: c.count })));
        // A chosen lens the chosen cameras never used falls away.
        if (this._lenses.setOptions(values.lenses.map(l => ({ value: l.name, label: plainExif(l.name), count: l.count })))) {
            ScopeState.set({ lenses: this._lenses.selected });
        }
        this._drawTime(values.firstMonth, values.lastMonth);
    }

    _drawTime(firstMonth, lastMonth) {
        const slot = this.el.querySelector('.scope-filter-time');
        slot.innerHTML = '';
        const first = monthNumber(firstMonth), last = monthNumber(lastMonth);
        if (!this._time || first === null || last === null || first === last) return;
        const v = ScopeState.value;
        const range = new MapTimeRange({
            first, last,
            value: { from: monthNumber(v.from) ?? first, until: monthNumber(v.until) ?? last },
            onChange: (r) => ScopeState.set(r ? { from: monthText(r.from), until: monthText(r.until) } : { from: '', until: '' }),
        });
        slot.appendChild(range.el);
    }

    // Reset shows only when there is something to reset; the Filter button
    // of a phone counts what is narrowed.
    _showNarrowed() {
        const v = ScopeState.value;
        const parts = [v.libraries, v.models, v.lenses].filter(x => x !== null).length + (this._time && (v.from || v.until) ? 1 : 0);
        this._resetBtn.hidden = parts === 0;
        this._toggle.querySelector('.scope-filter-count').textContent = parts ? ` · ${parts}` : '';
        this.el.classList.toggle('scope-filter--narrowed', parts > 0);
    }
}
