// metadata-form.js — the fields of the metadata editor (ADR-0060, ADR-0061),
// in groups that fold: the date taken; title, description, keywords,
// creator, copyright and rating; the IPTC fields of where, the event, origin
// and rights, and who and what is shown; and own fields. Each shows what the
// chosen files say, with where it came from; a field whose files differ says
// "Several values" and is left as it is until it is changed. Only what is
// changed is written. A group that is empty in every file starts folded.

// tag is the name a field's value has among a photo's tags (the index, the
// info panel), as the server's field table names it.
const META_GROUPS = [
    { id: 'date', title: 'Date taken', date: true },
    { id: 'description', title: 'Description', fields: [
        { field: 'title', label: 'Title' },
        { field: 'description', label: 'Description', kind: 'textarea' },
        { field: 'keywords', label: 'Keywords', kind: 'keywords' },
        { field: 'creator', label: 'Creator' },
        { field: 'rights', label: 'Copyright' },
        { field: 'rating', label: 'Rating', kind: 'rating' },
    ] },
    { id: 'location', title: 'Location', fields: [
        { field: 'sublocation', label: 'Sublocation', tag: 'Sublocation' },
        { field: 'city', label: 'City', tag: 'City' },
        { field: 'state', label: 'Province or state', tag: 'State' },
        { field: 'country', label: 'Country', tag: 'Country' },
        { field: 'countryCode', label: 'Country code', kind: 'mono', tag: 'CountryCode' },
    ] },
    { id: 'event', title: 'Event and headline', fields: [
        { field: 'headline', label: 'Headline', tag: 'Headline' },
        { field: 'event', label: 'Event', tag: 'Event' },
    ] },
    { id: 'origin', title: 'Origin and rights', fields: [
        { field: 'credit', label: 'Credit line', tag: 'Credit' },
        { field: 'source', label: 'Source', tag: 'Source' },
        { field: 'usageTerms', label: 'Usage terms', kind: 'textarea', tag: 'UsageTerms' },
        { field: 'webStatement', label: 'Web statement of rights', kind: 'mono', tag: 'WebStatement' },
        { field: 'instructions', label: 'Instructions', kind: 'textarea', tag: 'Instructions' },
        { field: 'jobId', label: 'Job ID', kind: 'mono', tag: 'TransmissionReference' },
    ] },
    { id: 'shown', title: 'Shown', fields: [
        { field: 'personShown', label: 'People shown', kind: 'list', tag: 'PersonInImage' },
        { field: 'organisationShown', label: 'Organisations shown', kind: 'list', tag: 'OrganisationInImageName' },
    ] },
    { id: 'own', title: 'Own fields', own: true },
];

const META_SOURCES = {
    file: 'the file',
    sidecar: 'the sidecar',
    nfo: 'the NFO file',
    'file name': 'the file name',
};

// The name of each field, for the preview.
const META_FIELD_NAMES = Object.fromEntries([['dateTaken', 'Date taken'], ['own', 'Own fields'],
    ...META_GROUPS.flatMap(g => (g.fields || []).map(f => [f.field, f.label]))]);

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

// splitList splits "Ann; Bob", a list field's value, into its entries.
function splitList(v) {
    return (v || '').split(';').map(e => e.trim()).filter(Boolean);
}

class MetadataForm {
    // onChange runs after every edit; change() gives what is to be written.
    constructor(el, states, { onChange }) {
        this.el = el;
        this.states = states;
        this.onChange = () => { this._updateGroups(); onChange(); };
        this.edits = {};
        this.lists = {};
        this.render();
    }

    change() {
        const change = {};
        if (Object.keys(this.edits).length) change.set = { ...this.edits };
        const kw = this.lists.keywords;
        if (kw.added.length) change.keywordsAdd = [...kw.added];
        if (kw.removed.size) change.keywordsRemove = [...kw.removed];
        for (const [field, chips] of Object.entries(this.lists)) {
            if (field === 'keywords') continue;
            if (chips.added.length) (change.add ||= {})[field] = [...chips.added];
            if (chips.removed.size) (change.remove ||= {})[field] = [...chips.removed];
        }
        const own = this.own.change();
        if (own) change.own = own;
        if (this.countryFromGps) {
            change.countryFromGps = { lang: I18n.lang };
            if (change.set) { delete change.set.country; delete change.set.countryCode; }
            if (change.set && !Object.keys(change.set).length) delete change.set;
        }
        return change;
    }

    changedFields() {
        const fields = Object.keys(this.edits);
        for (const [field, chips] of Object.entries(this.lists)) if (field !== 'keywords' && chips.isChanged()) fields.push(field);
        if (this.own.isChanged()) fields.push('own');
        if (this.countryFromGps) fields.push('country');
        return fields;
    }

    keywordsChanged() {
        return this.lists.keywords.isChanged();
    }

    // dateSlot is where the place puts the correction of the date taken.
    get dateSlot() {
        return this.el.querySelector('.meta-date');
    }

    render() {
        this.el.innerHTML = META_GROUPS.map(g => this._groupHTML(g)).join('');
        for (const g of META_GROUPS) for (const f of g.fields || []) this._wireField(f);
        this._wireCountryFromGPS();
        this.own = new MetaOwnFields(this.el.querySelector('.meta-own'), this.states, { onChange: this.onChange });
        this._updateGroups();
    }

    /* --- Groups --- */

    _groupHTML(g) {
        const open = g.date || !this._groupEmpty(g) ? ' open' : '';
        let body;
        if (g.date) body = '<div class="meta-field meta-date"></div>';
        else if (g.own) body = '<div class="meta-own"></div>';
        else body = g.fields.map(f => this._fieldHTML(f)).join('');
        if (g.id === 'location') body += '<div class="meta-field meta-country-gps"></div>';
        return `<details class="meta-group" data-group="${g.id}"${open}>
            <summary class="meta-group-head"><span class="meta-group-title">${t(g.title)}</span><span class="meta-group-state"></span></summary>
            <div class="meta-group-body">${body}</div></details>`;
    }

    _groupEmpty(g) {
        return this.states.every(s => {
            const c = s.current;
            if (!c) return true;
            if (g.own) return !Object.keys(c.own || {}).length;
            return g.fields.every(f => f.kind === 'keywords' ? !(c.keywords || []).length : !c.fields?.[f.field]?.value);
        });
    }

    // A folded group says whether it is empty or changed, so nothing that
    // is written hides in it.
    _updateGroups() {
        for (const g of META_GROUPS) {
            if (g.date) continue;
            const box = this.el.querySelector(`[data-group="${g.id}"]`);
            // Only a field counts, never the group's own state line.
            const changed = Boolean(box.querySelector('.meta-field.is-changed') || (g.own && this.own?.isChanged()));
            const state = box.querySelector('.meta-group-state');
            state.textContent = changed ? t('Changed') : (this._groupEmpty(g) ? t('Empty') : '');
            state.classList.toggle('is-changed', changed);
        }
    }

    /* --- Country from the GPS position --- */

    // The country of each file's own position, worked out offline from the
    // borders built into the app (ADR-0061); the preview shows it per file.
    _wireCountryFromGPS() {
        const box = this.el.querySelector('.meta-country-gps');
        const withGPS = this.states.filter(s => s.current?.gps).length;
        const total = this.states.filter(s => s.current).length;
        box.innerHTML = `<div class="meta-field-head">
                <span class="form-label">${t('Country from the GPS position')}</span>
                <span class="meta-own-actions meta-country-switch"></span>
            </div>
            <span class="form-hint">${escapeHtml(this._gpsHint(withGPS, total))}</span>`;
        if (!withGPS) return;
        const toggle = Toggle.create(box.querySelector('.meta-country-switch'), {
            initial: false,
            onChange: (on) => this._countryFromGPS(on, box),
        });
        toggle.el.setAttribute('aria-label', t('Country from the GPS position'));
    }

    _gpsHint(withGPS, total) {
        if (!withGPS) return t('None of the chosen files has a GPS position.');
        const which = withGPS === total ? t('Every chosen file has one.') : t('{n} of {total} have one; the others are left as they are.', { n: formatCount(withGPS), total: formatCount(total) });
        return t('Fills Country and Country code from where each file was taken, from the borders built into the app; nothing is sent anywhere.') + ' ' + which;
    }

    _countryFromGPS(on, box) {
        this.countryFromGps = on;
        box.classList.toggle('is-changed', on);
        for (const f of ['country', 'countryCode']) {
            const input = this.el.querySelector(`#meta-${f}`);
            input.disabled = on;
            if (on) {
                this.el.querySelector(`[data-reset="${f}"]`).click();
            }
        }
        this.onChange();
    }

    /* --- Fields --- */

    _fieldHTML(f) {
        if (f.kind === 'rating') return this._ratingHTML();
        if (f.kind === 'keywords' || f.kind === 'list') return `<div class="meta-field meta-list" data-field="${f.field}"></div>`;
        return this._textFieldHTML(f);
    }

    _wireField(f) {
        if (f.kind === 'rating') this._wireRating();
        else if (f.kind === 'keywords') this._keywordChips();
        else if (f.kind === 'list') this._listChips(f);
        else this._wireText(f.field);
    }

    _headHTML(field, label, id) {
        const s = metaSummary(this.states, field);
        return `<div class="meta-field-head">
            <label class="form-label" for="${id}">${t(label)}</label>
            <span class="meta-source">${escapeHtml(metaSourceText(s.sources))}</span>
            <button type="button" class="btn btn-sm meta-reset" data-reset="${field}" hidden>${t('Keep as it is')}</button>
        </div>`;
    }

    _textFieldHTML({ field, label, kind }) {
        const s = metaSummary(this.states, field);
        const id = `meta-${field}`;
        const cls = `form-input meta-input${kind === 'mono' ? ' meta-input--mono' : ''}`;
        const attrs = `id="${id}" class="${cls}" data-field="${field}" placeholder="${s.same ? '' : escapeHtml(t('Several values'))}"`;
        const input = kind === 'textarea'
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

    _keywordChips() {
        this.lists.keywords = new MetaChips(this.el.querySelector('.meta-list[data-field="keywords"]'), {
            states: this.states, id: 'meta-keyword-input', label: 'Keywords',
            entriesOf: s => s.current?.keywords || [],
            sources: states => [...new Set(states.map(s => s.current?.keywordsFrom).filter(Boolean))],
            hint: 'A keyword added goes to every chosen file; one removed goes from each that has it. The others stay.',
            empty: 'No keywords yet.', onChange: this.onChange,
        });
    }

    _listChips({ field, label }) {
        this.lists[field] = new MetaChips(this.el.querySelector(`.meta-list[data-field="${field}"]`), {
            states: this.states, id: `meta-${field}`, label,
            entriesOf: s => splitList(s.current?.fields?.[field]?.value),
            sources: states => metaSummary(states, field).sources,
            empty: 'Nobody named yet.', onChange: this.onChange,
        });
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
}
