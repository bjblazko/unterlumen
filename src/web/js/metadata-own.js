// metadata-own.js — own fields in the metadata editor (ADR-0061): key and
// value for what no standard names, written only to the sidecar of a photo.
// A field can also be a keyword, a machine tag "key=value" ("ul:key=value"
// for a key without a namespace of its own), so other programs find it;
// whether it is one is read from the keywords, as the server does.

function machineTag(key, value) {
    return `${key.includes(':') ? key : 'ul:' + key}=${value}`;
}

class MetaOwnFields {
    constructor(el, states, { onChange }) {
        this.el = el;
        this.states = states.filter(s => s.current && !s.current.video);
        this.videos = states.filter(s => s.current?.video).length;
        this.onChange = onChange;
        this.edits = {};   // key → { value?, mirror? }
        this.added = [];   // keys new to every file
        this.render();
    }

    // change is what to send as change.own; null for nothing.
    change() {
        const own = {};
        for (const [k, e] of Object.entries(this.edits)) if (Object.keys(e).length) own[k] = { ...e };
        return Object.keys(own).length ? own : null;
    }

    isChanged() {
        return this.change() !== null;
    }

    // Each key of the chosen photos: its value when they agree, in how many
    // it is, and whether it is a keyword in all of them.
    _summaries() {
        const keys = new Map();
        for (const s of this.states) {
            for (const [k, v] of Object.entries(s.current.own || {})) {
                const e = keys.get(k) || { values: [], mirrored: 0 };
                e.values.push(v);
                if ((s.current.keywords || []).includes(machineTag(k, v))) e.mirrored++;
                keys.set(k, e);
            }
        }
        for (const k of this.added) if (!keys.has(k)) keys.set(k, { values: [], mirrored: 0 });
        return [...keys.entries()].map(([key, e]) => ({
            key,
            n: e.values.length,
            same: e.values.every(v => v === e.values[0]),
            value: e.values.every(v => v === e.values[0]) ? (e.values[0] || '') : '',
            mirrored: e.values.length > 0 && e.mirrored === e.values.length,
        }));
    }

    render() {
        const rows = this._summaries().map(s => this._rowHTML(s)).join('');
        const videos = this.videos ? ' ' + t('{videos} have none and are left out.', { videos: countText(this.videos, 'video') }) : '';
        this.el.innerHTML = `<div class="meta-own-list">${rows || `<span class="form-hint">${t('No own fields yet.')}</span>`}</div>
            <div class="meta-own-add">
                <input type="text" id="meta-own-key" class="form-input meta-input meta-input--mono" placeholder="${escapeHtml(t('Key'))}" aria-label="${escapeHtml(t('Key'))}">
                <input type="text" id="meta-own-value" class="form-input meta-input" placeholder="${escapeHtml(t('Value'))}" aria-label="${escapeHtml(t('Value'))}">
                <button type="button" class="btn btn-sm" id="meta-own-btn">${t('Add')}</button>
            </div>
            <span class="form-hint">${escapeHtml(t('Own fields are always written to the sidecar, wherever the rest goes.') + videos)}</span>`;
        for (const s of this._summaries()) this._wireRow(s);
        this._wireAdd();
    }

    _rowHTML(s) {
        const id = `meta-own-${cssId(s.key)}`;
        const total = this.states.length;
        const count = s.n && s.n < total ? t('in {n} of {total}', { n: formatCount(s.n), total: formatCount(total) }) : '';
        const edit = this.edits[s.key] || {};
        const value = edit.value ?? s.value;
        const removed = edit.value === '';
        return `<div class="meta-field meta-own-row${removed ? ' is-removed' : ''}" data-own="${escapeHtml(s.key)}">
            <div class="meta-field-head">
                <label class="form-label meta-own-key" for="${id}">${escapeHtml(s.key)}</label>
                <span class="meta-source">${escapeHtml(count)}</span>
                <span class="meta-own-actions">
                    <span class="meta-own-mirror-label">${t('As keyword')}</span><span class="meta-own-mirror"></span>
                    <button type="button" class="btn btn-sm meta-own-remove">${removed ? t('Keep') : t('Remove')}</button>
                </span>
            </div>
            <input type="text" id="${id}" class="form-input meta-input" value="${escapeHtml(removed ? '' : value)}"
                placeholder="${s.same ? '' : escapeHtml(t('Several values'))}"${removed ? ' disabled' : ''}>
        </div>`;
    }

    _wireRow(s) {
        const row = this.el.querySelector(`[data-own="${CSS.escape(s.key)}"]`);
        const edit = () => (this.edits[s.key] ||= {});
        const toggle = Toggle.create(row.querySelector('.meta-own-mirror'), {
            initial: this.edits[s.key]?.mirror ?? s.mirrored,
            onChange: (on) => {
                if (on === s.mirrored && !this.added.includes(s.key)) delete edit().mirror;
                else edit().mirror = on;
                this._changed(row, s.key);
            },
        });
        toggle.el.setAttribute('aria-label', t('{key} as keyword', { key: s.key }));
        row.querySelector('input').addEventListener('input', (e) => {
            const v = e.target.value;
            if (v === s.value && s.same && s.n) delete edit().value;
            else edit().value = v;
            this._changed(row, s.key);
        });
        row.querySelector('.meta-own-remove').addEventListener('click', () => {
            if (edit().value === '') delete edit().value;
            else edit().value = '';
            this._changed(null, s.key);
            this.render();
        });
    }

    _wireAdd() {
        const key = this.el.querySelector('#meta-own-key');
        const value = this.el.querySelector('#meta-own-value');
        const add = () => {
            const k = key.value.trim();
            if (!k || /\s|=/.test(k)) {
                key.setCustomValidity(t('A key has no spaces and no =.'));
                key.reportValidity();
                return;
            }
            key.setCustomValidity('');
            if (!this.added.includes(k)) this.added.push(k);
            this.edits[k] = { ...(this.edits[k] || {}), value: value.value.trim() || '' };
            if (!this.edits[k].value) delete this.edits[k].value;
            this.render();
            this.onChange();
            this.el.querySelector(`#meta-own-${cssId(k)}`)?.focus();
        };
        this.el.querySelector('#meta-own-btn').addEventListener('click', add);
        for (const input of [key, value]) {
            input.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); add(); } });
        }
    }

    _changed(row, key) {
        row?.classList.toggle('is-changed', Boolean(this.edits[key] && Object.keys(this.edits[key]).length));
        this.onChange();
    }
}

// cssId turns a key into the rest of an element ID.
function cssId(key) {
    return Array.from(key, c => /[A-Za-z0-9_-]/.test(c) ? c : '_' + c.codePointAt(0).toString(16)).join('');
}
