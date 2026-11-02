// metadata-chips.js — a list of entries in the metadata editor that are
// added and removed, never replaced (ADR-0060, ADR-0061): the keywords, the
// people and the organisations shown. Each chip says in how many of the
// chosen files it is; one added goes to every file, one removed goes from
// each that has it, the others stay.

class MetaChips {
    // entriesOf(state) is a file's entries; sources(states) where they come
    // from. onChange runs after every edit.
    constructor(el, { states, id, label, entriesOf, sources, hint, empty, onChange }) {
        Object.assign(this, { el, states, id, label, entriesOf, sources, hint, empty, onChange });
        this.added = [];
        this.removed = new Set();
        this.render();
    }

    isChanged() {
        return this.added.length > 0 || this.removed.size > 0;
    }

    // Every entry of the chosen files, and in how many of them it is.
    _counts() {
        const counts = new Map();
        for (const s of this.states) {
            for (const k of this.entriesOf(s)) counts.set(k, (counts.get(k) || 0) + 1);
        }
        return counts;
    }

    _total() {
        return this.states.filter(s => s.current).length;
    }

    render() {
        const counts = this._counts();
        const total = this._total();
        const chips = [...counts.entries()].map(([k, n]) => this._chipHTML(k, n, total)).join('')
            + this.added.map(k => this._chipHTML(k, total, total, true)).join('');
        this.el.classList.toggle('is-changed', this.isChanged());
        this.el.innerHTML = `<div class="meta-field-head">
                <label class="form-label" for="${this.id}">${escapeHtml(t(this.label))}</label>
                <span class="meta-source">${escapeHtml(metaSourceText(this.sources(this.states)))}</span>
            </div>
            <div class="meta-chips">${chips || `<span class="form-hint">${escapeHtml(t(this.empty))}</span>`}</div>
            <div class="meta-keyword-add">
                <input type="text" id="${this.id}" class="form-input meta-input">
                <button type="button" class="btn btn-sm meta-chip-add">${t('Add')}</button>
            </div>
            ${this.hint ? `<span class="form-hint">${escapeHtml(t(this.hint))}</span>` : ''}`;
        this._wire();
    }

    _chipHTML(k, n, total, added = false) {
        const removed = this.removed.has(k);
        const partial = n < total ? `<span class="meta-chip-count">${t('{n} of {total}', { n: formatCount(n), total: formatCount(total) })}</span>` : '';
        const cls = ['meta-chip', removed ? 'is-removed' : '', added ? 'is-added' : '', n < total ? 'is-partial' : ''].join(' ');
        const action = removed ? t('Keep {keyword}', { keyword: k }) : t('Remove {keyword}', { keyword: k });
        return `<span class="${cls}" data-entry="${escapeHtml(k)}"><span class="meta-chip-label">${escapeHtml(k)}</span>${partial}
            <button type="button" class="meta-chip-remove" aria-label="${escapeHtml(action)}" title="${escapeHtml(action)}">${removed ? '↺' : '×'}</button></span>`;
    }

    _wire() {
        const input = this.el.querySelector(`#${this.id}`);
        const add = () => {
            const k = input.value.trim();
            if (!k) return;
            this.removed.delete(k);
            if (!this.added.includes(k) && this._counts().get(k) !== this._total()) this.added.push(k);
            this._changed();
            this.el.querySelector(`#${this.id}`).focus();
        };
        this.el.querySelector('.meta-chip-add').addEventListener('click', add);
        input.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); add(); } });
        this.el.querySelector('.meta-chips').addEventListener('click', (e) => {
            const chip = e.target.closest('.meta-chip-remove')?.closest('[data-entry]');
            if (!chip) return;
            const k = chip.dataset.entry;
            if (this.added.includes(k)) this.added = this.added.filter(x => x !== k);
            else if (this.removed.has(k)) this.removed.delete(k);
            else this.removed.add(k);
            this._changed();
        });
    }

    _changed() {
        this.render();
        this.onChange();
    }
}
