// stats-people-compare.js — Compare in the People topic of Statistics: the
// people its charts show side by side. Nobody chosen is the people with
// the most photos, as before; chosen, the charts show those, in that order
// and in the first colours of the ramp (peopleCompared). Not the filter's
// People, which keeps only the photos with all of them together. The choice
// is this browser's.

const COMPARE_KEY = 'stats.people.compare';
const COMPARE_MOST = 8; // as many as the ramp has colours

class PeopleCompare {
    // onChange is called with the names after every change.
    constructor({ onChange }) {
        this._onChange = onChange;
        this._names = [];
        this._chosen = PeopleCompare._stored();
        this.el = document.createElement('div');
        this.el.className = 'stats-compare';
        this._draw();
    }

    get names() { return this._chosen; }

    // setNames gives the names that can be chosen: the people of the scope.
    setNames(names) {
        this._names = names;
        this._draw();
    }

    static _stored() {
        try {
            const v = JSON.parse(localStorage.getItem(COMPARE_KEY) || '[]');
            return Array.isArray(v) ? v.filter(n => typeof n === 'string') : [];
        } catch { return []; }
    }

    _set(chosen) {
        this._chosen = chosen;
        try { localStorage.setItem(COMPARE_KEY, JSON.stringify(chosen)); } catch { /* kept for this page only */ }
        this._draw();
        this._onChange(chosen);
    }

    _draw() {
        const chosen = new Set(this._chosen.map(n => n.toLowerCase()));
        const free = this._names.filter(n => !chosen.has(n.toLowerCase()));
        const chips = this._chosen.map(n => `<button type="button" class="lib-filter-chip stats-compare-chip" data-name="${escapeHtml(n)}" title="${t('Leave {name} out', { name: escapeHtml(n) })}">` +
            `<span class="chip-label">${escapeHtml(n)}</span><span class="chip-remove" aria-hidden="true">×</span></button>`).join('');
        const add = this._chosen.length < COMPARE_MOST && free.length
            ? `<select class="btn btn-sm select-btn stats-compare-add" aria-label="${t('Add a person to compare')}">
                <option value="">${this._chosen.length ? t('Add someone…') : t('The most photos')}</option>
                ${free.map(n => `<option value="${escapeHtml(n)}">${escapeHtml(n)}</option>`).join('')}</select>` : '';
        this.el.innerHTML = `<span class="stats-period-label">${t('Compare')}</span>${chips}${add}`;
        this.el.querySelector('.stats-compare-add')?.addEventListener('change', (e) => {
            if (e.target.value) this._set([...this._chosen, e.target.value]);
        });
        for (const chip of this.el.querySelectorAll('.stats-compare-chip')) {
            chip.addEventListener('click', () => this._set(this._chosen.filter(n => n !== chip.dataset.name)));
        }
    }
}
