// metadata-form.js — the fields of the metadata editor (ADR-0060): title,
// description, creator, rights and rating, and the keywords. Each shows what
// the chosen files say, with where it came from; a field whose files differ
// says "Several values" and is left as it is until it is changed. Only what
// is changed is written.

const META_TEXT_FIELDS = [
    ['title', 'Title'],
    ['description', 'Description'],
    ['creator', 'Creator'],
    ['rights', 'Copyright'],
];

const META_SOURCES = {
    file: 'the file',
    sidecar: 'the sidecar',
    nfo: 'the NFO file',
    'file name': 'the file name',
};

const META_FIELD_NAMES = {
    dateTaken: 'Date taken', title: 'Title', description: 'Description',
    creator: 'Creator', rights: 'Copyright', rating: 'Rating',
};

// What the files say about one field: the value when they agree, and where
// it came from.
function metaSummary(states, field) {
    const values = states.map(s => s.current?.fields?.[field]?.value || '');
    const sources = new Set(states.map(s => s.current?.fields?.[field]?.from).filter(Boolean));
    const same = values.every(v => v === values[0]);
    return { same, value: same ? values[0] : '', sources: [...sources] };
}

function metaSourceText(sources) {
    if (!sources.length) return '';
    if (sources.length > 1) return t('from several places');
    return t('from {source}', { source: t(META_SOURCES[sources[0]] || sources[0]) });
}

class MetadataForm {
    // onChange runs after every edit; change() gives what is to be written.
    constructor(el, states, { onChange }) {
        this.el = el;
        this.states = states;
        this.onChange = onChange;
        this.edits = {};
        this.keywordsAdd = [];
        this.keywordsRemove = new Set();
        this.render();
    }

    change() {
        const change = {};
        if (Object.keys(this.edits).length) change.set = { ...this.edits };
        if (this.keywordsAdd.length) change.keywordsAdd = [...this.keywordsAdd];
        if (this.keywordsRemove.size) change.keywordsRemove = [...this.keywordsRemove];
        return change;
    }

    changedFields() {
        return Object.keys(this.edits);
    }

    render() {
        this.el.innerHTML = META_TEXT_FIELDS.map(([f, label]) => this._textFieldHTML(f, label)).join('')
            + this._ratingHTML() + `<div class="meta-field meta-keywords"></div>`;
        for (const [f] of META_TEXT_FIELDS) this._wireText(f);
        this._wireRating();
        this._renderKeywords();
    }

    _headHTML(field, label, id) {
        const s = metaSummary(this.states, field);
        return `<div class="meta-field-head">
            <label class="form-label" for="${id}">${t(label)}</label>
            <span class="meta-source">${escapeHtml(metaSourceText(s.sources))}</span>
            <button type="button" class="btn btn-sm meta-reset" data-reset="${field}" hidden>${t('Keep as it is')}</button>
        </div>`;
    }

    _textFieldHTML(field, label) {
        const s = metaSummary(this.states, field);
        const id = `meta-${field}`;
        const attrs = `id="${id}" class="form-input meta-input" data-field="${field}" placeholder="${s.same ? '' : escapeHtml(t('Several values'))}"`;
        const input = field === 'description'
            ? `<textarea ${attrs} rows="3">${escapeHtml(s.value)}</textarea>`
            : `<input type="text" ${attrs} value="${escapeHtml(s.value)}">`;
        return `<div class="meta-field" data-field="${field}">${this._headHTML(field, label, id)}${input}</div>`;
    }

    _wireText(field) {
        const input = this.el.querySelector(`#meta-${field}`);
        const original = input.value;
        input.addEventListener('input', () => this._edit(field, input.value, input.value !== original));
        this._wireReset(field, () => { input.value = original; });
    }

    // The same five stars as the info panel and the viewer. Files that
    // differ show none and say so, until a star is chosen.
    _ratingHTML() {
        const s = metaSummary(this.states, 'rating');
        this._ratingOriginal = s.same ? Rating.of(s.value) : null;
        return `<div class="meta-field" data-field="rating">${this._headHTML('rating', 'Rating', 'meta-rating')}
            <div class="meta-rating" id="meta-rating"></div></div>`;
    }

    _wireRating() {
        const box = this.el.querySelector('#meta-rating');
        const show = (n) => {
            const several = n === null ? `<span class="form-hint">${t('Several values')}</span>` : '';
            box.innerHTML = Rating.controlHTML(n ?? 0) + several;
            Rating.wire(box, (stars) => {
                show(stars);
                // 0 is written, not removed: "no rating" here must also hide
                // a rating the file itself carries.
                this._edit('rating', String(stars), stars !== this._ratingOriginal);
            });
        };
        show(this._ratingOriginal);
        this._wireReset('rating', () => show(this._ratingOriginal));
    }

    _wireReset(field, restore) {
        this.el.querySelector(`[data-reset="${field}"]`).addEventListener('click', () => {
            restore();
            this._edit(field, '', false);
        });
    }

    _edit(field, value, changed) {
        if (changed) this.edits[field] = value;
        else delete this.edits[field];
        const box = this.el.querySelector(`.meta-field[data-field="${field}"]`);
        box.classList.toggle('is-changed', changed);
        box.querySelector('.meta-reset').hidden = !changed;
        this.onChange();
    }

    /* --- Keywords --- */

    // Every keyword of the chosen files, and in how many of them it is.
    _keywordCounts() {
        const counts = new Map();
        for (const s of this.states) {
            for (const k of s.current?.keywords || []) counts.set(k, (counts.get(k) || 0) + 1);
        }
        return counts;
    }

    _renderKeywords() {
        const box = this.el.querySelector('.meta-keywords');
        const counts = this._keywordCounts();
        const total = this.states.filter(s => s.current).length;
        const sources = [...new Set(this.states.map(s => s.current?.keywordsFrom).filter(Boolean))];
        const chips = [...counts.entries()].map(([k, n]) => this._chipHTML(k, n, total)).join('')
            + this.keywordsAdd.map(k => this._chipHTML(k, total, total, true)).join('');
        box.innerHTML = `<div class="meta-field-head">
                <label class="form-label" for="meta-keyword-input">${t('Keywords')}</label>
                <span class="meta-source">${escapeHtml(metaSourceText(sources))}</span>
            </div>
            <div class="meta-chips">${chips || `<span class="form-hint">${t('No keywords yet.')}</span>`}</div>
            <div class="meta-keyword-add">
                <input type="text" id="meta-keyword-input" class="form-input meta-input">
                <button type="button" class="btn btn-sm" id="meta-keyword-btn">${t('Add')}</button>
            </div>
            <span class="form-hint">${t('A keyword added goes to every chosen file; one removed goes from each that has it. The others stay.')}</span>`;
        this._wireKeywords(box);
    }

    _chipHTML(k, n, total, added = false) {
        const removed = this.keywordsRemove.has(k);
        const partial = n < total ? `<span class="meta-chip-count">${t('{n} of {total}', { n: formatCount(n), total: formatCount(total) })}</span>` : '';
        const cls = ['meta-chip', removed ? 'is-removed' : '', added ? 'is-added' : '', n < total ? 'is-partial' : ''].join(' ');
        const action = removed ? t('Keep {keyword}', { keyword: k }) : t('Remove {keyword}', { keyword: k });
        return `<span class="${cls}" data-keyword="${escapeHtml(k)}"><span class="meta-chip-label">${escapeHtml(k)}</span>${partial}
            <button type="button" class="meta-chip-remove" aria-label="${escapeHtml(action)}" title="${escapeHtml(action)}">${removed ? '↺' : '×'}</button></span>`;
    }

    _wireKeywords(box) {
        const input = box.querySelector('#meta-keyword-input');
        const add = () => {
            const k = input.value.trim();
            if (!k) return;
            this.keywordsRemove.delete(k);
            if (!this.keywordsAdd.includes(k) && this._keywordCounts().get(k) !== this.states.length) this.keywordsAdd.push(k);
            this._keywordsChanged();
            this.el.querySelector('#meta-keyword-input').focus();
        };
        box.querySelector('#meta-keyword-btn').addEventListener('click', add);
        input.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); add(); } });
        box.querySelector('.meta-chips').addEventListener('click', (e) => {
            const chip = e.target.closest('.meta-chip-remove')?.closest('[data-keyword]');
            if (!chip) return;
            const k = chip.dataset.keyword;
            if (this.keywordsAdd.includes(k)) this.keywordsAdd = this.keywordsAdd.filter(x => x !== k);
            else if (this.keywordsRemove.has(k)) this.keywordsRemove.delete(k);
            else this.keywordsRemove.add(k);
            this._keywordsChanged();
        });
    }

    _keywordsChanged() {
        this._renderKeywords();
        this.onChange();
    }
}
