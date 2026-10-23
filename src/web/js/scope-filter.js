// scope-filter.js — the one filter of the Map, the Statistics and the
// Timeline (ADR-0050): libraries, a span of months, cameras, lenses and
// people, each optional and all of everything at first. Its state is shared, so a place
// shows what the others show; it is kept in the browser across visits.

const SCOPE_FILTER_KEY = 'scope.filter';
// A choice of none matches no photo: the server is sent a value no photo has.
const SCOPE_NONE = '∅';

const ScopeState = {
    // libraries, models, lenses: null for all, else the chosen values.
    // from, until: months as "YYYY-MM", '' for an open end.
    // people: names who must all be in the photo, as in a library's filter.
    value: { libraries: null, from: '', until: '', models: null, lenses: null, people: [] },
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
        this.set({ libraries: null, from: '', until: '', models: null, lenses: null, people: [] });
    },

    onChange(fn) {
        this._listeners.add(fn);
        return () => this._listeners.delete(fn);
    },

    // Whether anything is narrowed; time only where it counts.
    narrowed({ time = true } = {}) {
        const v = this.value;
        return v.libraries !== null || v.models !== null || v.lenses !== null || v.people.length > 0 || (time && !!(v.from || v.until));
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
        for (const name of v.people) pairs.push(['person', name]);
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

// ScopeFilter is the same kind of filter as a library's: the conditions as
// chips in the place's head, each dropping its own, a Filter toggle with
// their number in the place's tool strip, and a column on the right to set
// them (ADR-0050 shares the filter itself between the three places).
class ScopeFilter {
    // time: whether the span of months is offered (not on the Timeline,
    // whose axis is time). onOpen/onClose: the column came or went, so the
    // place can make room or take its own column back.
    constructor({ time = true, onOpen = () => {}, onClose = () => {} } = {}) {
        this._time = time;
        this._onOpen = onOpen;
        this._onClose = onClose;
        this.el = document.createElement('span');
        this.el.className = 'scope-filter';
        this.el.innerHTML = '<span class="scope-filter-chips"></span>';
        // The toggle stands in the place's tool strip; the chips in its head.
        this.toggle = panelToggleButton('Filter', 'scope-filter-toggle');
        this.panel = document.createElement('div');
        this.panel.className = 'lib-filter-panel scope-filter-panel';
        this.panel.innerHTML = `
            <div class="lib-filter-head">
                <span class="lib-filter-title">Filter</span>
                <button type="button" class="btn btn-sm scope-filter-reset" hidden>Reset filters</button>
                ${panelHideButtonHTML('lib-filter-close', 'Hide filter')}
            </div>
            <div class="lib-filter-groups">
                <div class="lib-filter-group"><div class="lib-filter-label">Libraries</div><span class="scope-filter-libraries"></span></div>
                <div class="lib-filter-group scope-filter-time-group"${time ? '' : ' hidden'}><div class="lib-filter-label">Taken</div><span class="scope-filter-time"></span></div>
                <div class="lib-filter-group"><div class="lib-filter-label">Cameras</div><span class="scope-filter-cameras"></span></div>
                <div class="lib-filter-group"><div class="lib-filter-label">Lenses</div><span class="scope-filter-lenses"></span></div>
                <div class="lib-filter-group scope-filter-people-group" hidden>
                    <div class="lib-filter-label">People</div>
                    <select class="btn btn-sm select-btn scope-filter-person" aria-label="Add a person to the filter"></select>
                    <span class="scope-filter-people"></span>
                </div>
            </div>`;
        this._toggle = this.toggle;
        this._toggle.addEventListener('click', () => (this.isOpen() ? this.close() : this.open()));
        this.panel.querySelector('.lib-filter-close').addEventListener('click', () => this.close());
        this._resetBtn = this.panel.querySelector('.scope-filter-reset');
        this._resetBtn.addEventListener('click', () => {
            ScopeState.reset();
            this.sync();
        });
        this._libraries = this._select('libraries', 'Libraries', 'libraries');
        this._cameras = this._select('cameras', 'Cameras', 'models');
        this._lenses = this._select('lenses', 'Lenses', 'lenses');
        this._person = this.panel.querySelector('.scope-filter-person');
        this._person.addEventListener('change', () => {
            if (this._person.value) this._setPeople([...ScopeState.value.people, this._person.value]);
        });
        this._unsubscribe = ScopeState.onChange(() => this._showNarrowed());
        this._showNarrowed();
        this._load();
    }

    destroy() { this._unsubscribe(); }

    isOpen() { return this.panel.classList.contains('visible'); }

    open() {
        if (this.isOpen()) return;
        this.panel.classList.add('visible');
        this._setPressed(true);
        this._onOpen();
    }

    close() {
        if (!this.isOpen()) return;
        this.panel.classList.remove('visible');
        this._setPressed(false);
        this._onClose();
    }

    _setPressed(on) { setPanelToggle(this._toggle, on); }

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

    _setPeople(people) {
        ScopeState.set({ people });
        this._drawPeople();
    }

    // People: a choice adds one, and each chosen one stands below it with
    // what drops it. Several must all be in the photo. Without names given
    // in People there is nothing to choose, and the group stays away.
    _drawPeople() {
        const chosen = ScopeState.value.people;
        const names = this._personNames || [];
        this.panel.querySelector('.scope-filter-people-group').hidden = !names.length && !chosen.length;
        const left = names.filter(n => !chosen.some(c => c.toLowerCase() === n.toLowerCase()));
        this._person.innerHTML = `<option value="">${chosen.length ? 'Add someone else…' : 'Add a person…'}</option>`
            + left.map(n => `<option value="${escapeHtml(n)}">${escapeHtml(n)}</option>`).join('');
        this._person.disabled = !left.length;
        const slot = this.panel.querySelector('.scope-filter-people');
        slot.innerHTML = '';
        for (const name of chosen) {
            slot.appendChild(this._chip(name, `Drop ${name}`, () => this._setPeople(ScopeState.value.people.filter(n => n !== name))));
        }
    }

    _chip(label, title, drop) {
        const chip = document.createElement('button');
        chip.type = 'button';
        chip.className = 'lib-filter-chip';
        chip.innerHTML = `${escapeHtml(label)}<span aria-hidden="true">×</span>`;
        chip.title = title;
        chip.addEventListener('click', drop);
        return chip;
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
        this.panel.querySelector(`.scope-filter-${slot}`).appendChild(select.el);
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
        let libs, values, names;
        try {
            [libs, values, names] = await Promise.all([
                LibraryAPI.list(),
                fetch(`/api/library/scope-values?${q}`).then(r => (r.ok ? r.json() : Promise.reject(new Error(r.statusText)))),
                LibraryAPI.personNames(v.libraries?.length ? v.libraries.join(',') : undefined),
            ]);
        } catch {
            return; // the place says what it cannot read; the filter stays as it was
        }
        if (gen !== this._gen) return;
        this._libNames = new Map(libs.map(l => [String(l.id), l.name]));
        this._libraries.setOptions(libs.map(l => ({ value: String(l.id), label: l.name, count: l.photoCount })));
        this._cameras.setOptions(values.cameras.map(c => ({ value: c.name, label: plainExif(c.name), count: c.count })));
        // A chosen lens the chosen cameras never used falls away.
        if (this._lenses.setOptions(values.lenses.map(l => ({ value: l.name, label: plainExif(l.name), count: l.count })))) {
            ScopeState.set({ lenses: this._lenses.selected });
        }
        this._drawTime(values.firstMonth, values.lastMonth);
        this._personNames = names;
        this._drawPeople();
        this._showNarrowed();
    }

    _drawTime(firstMonth, lastMonth) {
        const slot = this.panel.querySelector('.scope-filter-time');
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

    // conditions lists what narrows the photos, each with what drops it.
    _conditions() {
        const v = ScopeState.value;
        const some = (label, values, name) => `${label}: ${values.length === 1 ? name(values[0]) : values.length ? `${values.length}` : 'none'}`;
        const out = [];
        if (v.libraries !== null) out.push({ label: some('Libraries', v.libraries, id => this._libNames?.get(id) || id), drop: { libraries: null } });
        if (this._time && (v.from || v.until)) out.push({ label: `Taken: ${v.from || '…'} – ${v.until || '…'}`, drop: { from: '', until: '' } });
        if (v.models !== null) out.push({ label: some('Camera', v.models, plainExif), drop: { models: null } });
        if (v.lenses !== null) out.push({ label: some('Lens', v.lenses, plainExif), drop: { lenses: null } });
        for (const name of v.people) out.push({ label: `Person: ${name}`, drop: { people: v.people.filter(n => n !== name) } });
        return out;
    }

    // The head shows each condition as a chip that drops it, and the Filter
    // button counts them, so a hidden column never hides why fewer photos show.
    _showNarrowed() {
        const conditions = this._conditions();
        this._resetBtn.hidden = conditions.length === 0;
        const badge = this._toggle.querySelector('.lib-filter-btn-count');
        badge.textContent = conditions.length ? String(conditions.length) : '';
        const chips = this.el.querySelector('.scope-filter-chips');
        chips.innerHTML = '';
        for (const c of conditions) {
            chips.appendChild(this._chip(c.label, `Drop "${c.label}"`, () => {
                ScopeState.set(c.drop);
                this.sync();
            }));
        }
    }
}
