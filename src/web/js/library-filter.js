// LibraryFilterPanel — the Libraries photo filter: EXIF ranges, date, text and chip criteria

// Fixed chip namespaces that map to EXIF fields or special filter params.
const CHIP_NS_FIXED = [
    { ns: 'camera',  label: t('Camera'),        hint: t('Camera model'),    type: 'exif', param: 'Model' },
    { ns: 'lens',    label: t('Lens'),           hint: t('Lens model'),      type: 'exif', param: 'LensModel' },
    { ns: 'film',    label: t('Film sim'),       hint: t('Film simulation'), type: 'exif', param: 'FilmSimulation' },
    { ns: 'format',  label: t('Format'),         hint: t('File format'),     type: 'exif', param: 'ext' },
    { ns: 'flash',   label: t('Flash'),          hint: t('Flash mode'),      type: 'exif', param: 'Flash' },
    { ns: 'wb',      label: t('White balance'),  hint: t('White balance'),   type: 'exif', param: 'WhiteBalance' },
    { ns: 'channel', label: t('Destination'),    hint: t('Published to this destination'), type: 'channel', param: 'channel' },
    { ns: 'album',   label: t('Gallery title'),  hint: t('Any gallery with this title'), type: 'album', param: 'album_title' },
    { ns: 'person',  label: t('Person'),         hint: t('Named in People'), type: 'person', param: 'person' },
    { ns: 'thing',   label: t('Thing'),          hint: t('Found in the photo'), type: 'thing', param: 'thing' },
    { ns: 'shows',   label: t('Shows'),          hint: t('Words for what the photo shows'), type: 'words', param: 'text' },
];

// EXIF fields handled by numeric sliders — exclude from chip namespace list to avoid duplication.
const SLIDER_FIELDS = new Set([
    'ExposureTime', 'FNumber', 'FocalLength', 'FocalLength35',
    'FocalLengthIn35mmFilm', 'ISOSpeedRatings',
]);

// Translates a chip {ns, nsInfo, value} to a search param {key, value}.
// nsInfo is the full namespace descriptor stored by ChipInput when a chip is created.
// One gallery by its membership (<destination>:<postID>) rather than by
// title, so a renamed gallery still finds its photos. Only a "Show photos"
// link sets it; the chip shows the title.
const GALLERY_CHIP_NS = { ns: 'gallery', label: t('Gallery'), hint: t('One gallery'), type: 'gallery', param: 'album' };

function chipToParam(chip) {
    const ns = chip.nsInfo || CHIP_NS_FIXED.find(n => n.ns === chip.ns);
    if (ns) {
        if (ns.type === 'gallery') return { key: 'album', value: chip.value };
        if (ns.type === 'exif') return { key: ns.param, value: chip.value };
        if (ns.type === 'channel') return { key: 'channel', value: chip.value };
        if (ns.type === 'album') return { key: 'album_title', value: chip.value };
        if (ns.type === 'person') return { key: 'person', value: chip.value };
        if (ns.type === 'thing') return { key: 'thing', value: chip.value };
        if (ns.type === 'words') return { key: 'text', value: chip.value };
        if (ns.type === 'meta') return { key: ns.param, value: chip.value };
    }
    return { key: 'meta_' + chip.ns, value: chip.value };
}

const EXIF_TEXT_FILTER_FIELDS = [
    { field: 'Model',     label: t('Camera')   },
    { field: 'LensModel', label: t('Lens')     },
    { field: 'FilmSimulation', label: t('Film sim') },
];

const EXIF_FILTER_FIELDS = [
    {
        field: 'ExposureTime',
        label: t('Shutter speed'),
        format: formatShutterSpeed,
        log: true,
    },
    {
        field: 'FNumber',
        label: t('Aperture'),
        format: v => `f/${v.toFixed(1)}`,
        log: true,
    },
    {
        field: 'FocalLength',
        label: t('Focal length'),
        format: v => `${Math.round(v)} mm`,
        log: false,
    },
    {
        field: 'ISOSpeedRatings',
        label: t('ISO'),
        format: v => String(Math.round(v)),
        log: true,
    },
];

function formatShutterSpeed(seconds) {
    if (seconds >= 1) {
        const s = seconds % 1 === 0 ? seconds : seconds.toFixed(1);
        return `${s} s`;
    }
    const denom = Math.round(1 / seconds);
    return `1/${denom}`;
}

// The ends of the track are the range's own bounds, exactly: exp(log(51200))
// is 51199.99999999997, and a slider nobody touched would otherwise count as
// narrowed — an ISO chip that can't be dropped, and the top ISO silently
// filtered out of every result.
function sliderToValue(pos, min, max, log) {
    if (pos <= 0) return min;
    if (pos >= 1) return max;
    if (log) return Math.exp(Math.log(min) + pos * (Math.log(max) - Math.log(min)));
    return min + pos * (max - min);
}

function isNarrowed(active, range) {
    return active.min > range.min || active.max < range.max;
}

function valueToSlider(val, min, max, log) {
    if (log) return (Math.log(val) - Math.log(min)) / (Math.log(max) - Math.log(min));
    return (val - min) / (max - min);
}

// "Show photos" on a gallery or a destination is a link to Libraries; a plain
// click opens it with the filter set to what that gallery or destination
// holds, a modified click leaves the browser to open the place itself.
// searchWords says what a link to photos looks for, while it is looked for.
function searchWords(criteria) {
    const people = [criteria.person ?? []].flat();
    if (people.length) return t('Finding the photos of {people}…', { people: sentenceList(people) });
    const found = [criteria.thing ?? []].flat();
    if (found.length) return t('Finding the photos with {found}…', { found: sentenceList(found) });
    if (criteria.text) return t('Finding photos that show {text}…', { text: criteria.text });
    if (criteria.gallery) return t('Finding the photos of {title}…', { title: criteria.gallery.title });
    return t('Finding the photos…');
}

function showPhotosLink(el, criteria) {
    if (!el) return;
    el.addEventListener('click', (e) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
        e.preventDefault();
        App.showPhotos(criteria);
    });
}

// The one photo filter of the Libraries place. The overview and a library's
// detail both open it the same way; they differ only in the scope it starts
// with — every library, or the one that is open — and the scope select in
// the panel changes that either way.
class LibraryFilterPanel {
    // container    — the element that holds the full panel
    // toggleBtn    — the button that opens/closes the panel
    // initialLibID — pre-select this library (null = all)
    // options:
    //   onResults(photos, multiLib, {total, fetchPage}) — the host shows them
    //   onLoading(isLoading), onError(), onActiveCount(n), onOpen(), onClose()
    constructor(container, toggleBtn, initialLibID = null, options = {}) {
        this._container = container;
        this._toggleBtn = toggleBtn;
        this._initialLibID = initialLibID;
        this._startLibID = initialLibID;
        this._options = options;
        this._ranges = {};
        this._active = {};
        this._use35mm = false;
        this._debounceTimer = null;
        this._libraries = [];
        this._queryGen = 0;
        this._kind = ''; // 'photo', 'video' or '' for both (ADR-0058)
        this._dateMin = '';
        this._dateMax = '';
        this._dateMinInput = null;
        this._dateMaxInput = null;
        this._chipInput = null;

        // Opening the filter must not throw away the folder you are looking
        // at: the panel appears, the photos behind it stay as they are until
        // a criterion is actually set.
        toggleBtn.addEventListener('click', () => {
            if (this._container.classList.contains('visible')) this.close();
            else this.open();
        });
    }

    close() {
        if (!this._container.classList.contains('visible')) return;
        this._container.classList.remove('visible');
        this._toggleBtn.dataset.state = 'off';
        this._toggleBtn.setAttribute('aria-pressed', 'false');
        this._toggleBtn.setAttribute('aria-expanded', 'false');
        if (this._options.onClose) this._options.onClose();
        // With nothing set, a hidden filter has no results: the place shows
        // what they replaced again.
        if (this._narrowsNothing()) {
            clearTimeout(this._debounceTimer);
            this._queryGen++;
            this._options.onCleared?.();
        }
    }

    isOpen() { return this._container.classList.contains('visible'); }

    // Nothing is set when no criterion is on and the libraries are the ones
    // the filter started with — another library chosen is a choice.
    _narrowsNothing() {
        return this.activeCount() === 0 && String(this._initialLibID ?? '') === String(this._startLibID ?? '');
    }

    // Opens the filter already set, one chip per criterion — { channel: slug },
    // { gallery: { id: "<slug>:<postID>", title } } or { person: name } — then one query. This is how a gallery or a destination
    // shows its photos; the host opens it in the overview, so the scope is
    // every library.
    async openWith(criteria) {
        await this.open();
        this._suppressQuery = true;
        this._reset();
        if (criteria.channel) {
            this._chipInput.add(CHIP_NS_FIXED.find(n => n.ns === 'channel'), criteria.channel);
        }
        if (criteria.gallery) {
            this._chipInput.add(GALLERY_CHIP_NS, criteria.gallery.id, criteria.gallery.title);
        }
        // One person, or several who must all be in the photo.
        for (const name of [criteria.person ?? []].flat()) {
            this._chipInput.add(CHIP_NS_FIXED.find(n => n.ns === 'person'), name);
        }
        for (const thing of [criteria.thing ?? []].flat()) {
            this._chipInput.add(CHIP_NS_FIXED.find(n => n.ns === 'thing'), thing, ThingWords.name(thing));
        }
        if (criteria.text) this._chipInput.add(CHIP_NS_FIXED.find(n => n.ns === 'shows'), criteria.text);
        if (criteria.hues) this._setHues(criteria.hues);
        this._suppressQuery = false;
        this._runQuery();
    }

    // Nothing is filtered yet when the panel appears, so the libraries or
    // the library's folders stay in view until a criterion is actually set.
    async open() {
        if (this._container.classList.contains('visible')) return;
        this._container.classList.add('visible');
        this._toggleBtn.dataset.state = 'on';
        this._toggleBtn.setAttribute('aria-pressed', 'true');
        this._toggleBtn.setAttribute('aria-expanded', 'true');
        if (this._options.onOpen) this._options.onOpen();

        if (!this._built) {
            Activity.in(this._container, t('Reading the filters…'));
            await this._build();
        } else {
            // Clear chip input caches so re-opening the panel picks up newly built albums etc.
            this._chipInput?.clearCache();
        }
    }

    async _build() {
        this._built = true;
        this._container.innerHTML = '';
        this._container.className = 'lib-filter-panel visible';
        this._buildHead();
        await this._loadChoices();
        this._buildStatus();
        this._buildControls();
        this._buildKindFilter();
        this._buildDateFilter();
        this._buildColourFilter();
        this._buildSliders();
        this._buildTextFilters();
        this._buildChipFilters();
    }

    // Everything the filter offers to choose from, read at once.
    async _loadChoices() {
        const ids = this._initialLibID || undefined;
        const [libraries, ranges, channels, metaKeys, albumTitles, personNames, thingTotals, exifFields, ...textValues] = await Promise.all([
            LibraryAPI.list().catch(() => []),
            this._fetchRanges(this._initialLibID),
            ChannelAPI.list().catch(() => []),
            LibraryAPI.metaKeys(ids).catch(() => []),
            LibraryAPI.albumTitles(ids).catch(() => []),
            LibraryAPI.personNames(ids).catch(() => []),
            LibraryAPI.thingTotals(ids).catch(() => []),
            LibraryAPI.exifFields(ids).catch(() => []),
            ...EXIF_TEXT_FILTER_FIELDS.map(f => LibraryAPI.exifValues(f.field, ids).catch(() => [])),
        ]);
        Object.assign(this, {
            _libraries: libraries, _ranges: ranges, _channels: channels, _metaKeys: metaKeys,
            _albumTitles: albumTitles, _personNames: personNames, _exifFields: exifFields,
            _thingNames: thingTotals.map(th => th.thing), _textValues: {}, _textActive: {},
        });
        EXIF_TEXT_FILTER_FIELDS.forEach((f, i) => { this._textValues[f.field] = textValues[i]; });
    }

    async _fetchRanges(libID) {
        try {
            if (libID) return await LibraryAPI.exifRanges(libID);
            return await LibraryAPI.globalExifRanges();
        } catch { return {}; }
    }

    // The panel says what it is and offers the way out. It floats over the
    // photos instead of pushing them aside, so the thumbnails behind it show
    // what the filter does while it is being set (ADR-0008, principle 4).
    _buildHead() {
        const head = document.createElement('div');
        head.className = 'lib-filter-head';
        head.innerHTML = `
            <span class="lib-filter-title">${t('Filter')}</span>
            ${panelHideButtonHTML('lib-filter-close', t('Hide filter'))}`;
        head.querySelector('.lib-filter-close').addEventListener('click', () => this.close());
        this._container.appendChild(head);

        const active = document.createElement('div');
        active.className = 'lib-filter-active';
        this._activeEl = active;
        this._container.appendChild(active);
        this._container.appendChild(this._statisticsHint());
    }

    // Statistics find photos by what no filter here offers: colour, warmth,
    // an hour of the day. Said on the desk only; a phone has no room for it.
    _statisticsHint() {
        const route = { topic: 'colour', library: this._initialLibID ? String(this._initialLibID) : '', path: '' };
        const hint = document.createElement('p');
        hint.className = 'lib-filter-hint desk-only';
        hint.innerHTML = t('More ways to find photos, such as by colour, are in {statistics}: click a value in a chart to see its photos.', {
            statistics: `<a href="${statsHash(route)}">${t('Statistics')}</a>`,
        });
        hint.querySelector('a').addEventListener('click', (e) => {
            if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
            e.preventDefault();
            App.openStatistics(route);
        });
        return hint;
    }

    // What is filtering right now, as one chip each — the reason the count
    // under the photos is 412 and not 13 371. Every chip drops its own
    // criterion; "Reset filters" drops them all.
    activeFilters() {
        return [
            ...this._rangeFilters(),
            ...(this._kind ? [{ label: t('Kind: {kind}', { kind: this._kind === 'video' ? t('Videos') : t('Photos') }), clear: () => { this._setKind(''); this._runQuery(); } }] : []),
            ...this._dateFilters(),
            ...(this._hues?.length ? [{ label: t('Colours: {hues}', { hues: comboName(this._hues) }), clear: () => this._setHues([]) }] : []),
            ...this._textFilters(),
            ...(this._chipInput?.getChips() || []).map(chip => ({
                label: `${chip.label}: ${chip.displayValue || chip.value}`,
                clear: () => { this._chipInput.removeChip(chip); this._runQuery(); },
            })),
        ];
    }

    _rangeFilters() {
        return EXIF_FILTER_FIELDS.filter(f => {
            const range = this._ranges[f.field];
            const active = this._active[f.field];
            return range && active && isNarrowed(active, range);
        }).map(f => ({
            label: `${f.label} ${f.format(this._active[f.field].min)} – ${f.format(this._active[f.field].max)}`,
            clear: () => {
                const range = this._ranges[f.field];
                this._active[f.field] = { min: range.min, max: range.max };
                this._rebuildSliders();
                this._runQuery();
            },
        }));
    }

    _dateFilters() {
        if (!this._dateMin && !this._dateMax) return [];
        return [{
            label: t('Taken {from} – {to}', { from: this._dateMin || t('the beginning'), to: this._dateMax || t('today') }),
            clear: () => {
                this._dateMin = '';
                this._dateMax = '';
                if (this._dateMinInput) this._dateMinInput.value = '';
                if (this._dateMaxInput) this._dateMaxInput.value = '';
                this._runQuery();
            },
        }];
    }

    _textFilters() {
        return EXIF_TEXT_FILTER_FIELDS.filter(f => (this._textActive || {})[f.field]).map(f => ({
            label: `${f.label}: ${this._textActive[f.field]}`,
            clear: () => {
                delete this._textActive[f.field];
                this._rebuildTextFilters();
                this._runQuery();
            },
        }));
    }

    activeCount() { return this.activeFilters().length; }

    _renderActiveFilters() {
        if (!this._activeEl) return;
        const active = this.activeFilters();
        this._activeEl.innerHTML = '';
        for (const filter of active) {
            const chip = document.createElement('button');
            chip.className = 'lib-filter-chip';
            chip.innerHTML = `${escapeHtml(filter.label)}<span aria-hidden="true">×</span>`;
            chip.title = t('Drop "{label}"', { label: filter.label });
            chip.addEventListener('click', () => filter.clear());
            this._activeEl.appendChild(chip);
        }
        if (this._options.onActiveCount) this._options.onActiveCount(active.length);
    }

    _buildControls() {
        const bar = document.createElement('div');
        bar.className = 'lib-filter-controls';
        const sel = document.createElement('select');
        sel.className = 'lib-filter-select';
        sel.innerHTML = `<option value="">${t('All libraries')}</option>` +
            this._libraries.map(l =>
                `<option value="${escapeHtml(l.id)}"${l.id === this._initialLibID ? ' selected' : ''}>${escapeHtml(l.name)}</option>`
            ).join('');
        sel.addEventListener('change', () => this._chooseLibrary(sel.value || null));
        this._libSelect = sel;
        const reset = document.createElement('button');
        reset.className = 'btn btn-sm lib-filter-reset';
        reset.textContent = t('Reset filters');
        reset.addEventListener('click', () => this._reset());
        bar.appendChild(sel);
        bar.appendChild(reset);
        this._container.appendChild(bar);
    }

    // Another library has other ranges and values: they are read again, and
    // what was chosen of the old ones goes.
    async _chooseLibrary(libID) {
        this._setLoading(true);
        this._initialLibID = libID;
        const ids = libID || undefined;
        let ranges;
        try {
            [ranges] = await Promise.all([
                this._fetchRanges(libID),
                ...EXIF_TEXT_FILTER_FIELDS.map(f => LibraryAPI.exifValues(f.field, ids).catch(() => []).then(vals => {
                    this._textValues[f.field] = vals;
                })),
            ]);
        } catch (err) {
            this._setLoading(false);
            this._showQueryError(err);
            return;
        }
        this._ranges = ranges;
        this._textActive = {};
        this._rebuildSliders();
        this._rebuildTextFilters();
        this._runQuery();
    }

    _buildSliders() {
        const wrap = document.createElement('div');
        wrap.className = 'lib-filter-groups';
        this._slidersWrap = wrap;
        this._container.appendChild(wrap);
        this._rebuildSliders();
    }

    _rebuildSliders() {
        this._slidersWrap.innerHTML = '';
        this._active = {};
        const fields = EXIF_FILTER_FIELDS.filter(f => {
            const activeField = (f.field === 'FocalLength' && this._use35mm) ? 'FocalLength35' : f.field;
            const r = this._ranges[activeField];
            return r && r.min < r.max;
        });
        for (const spec of fields) {
            const activeField = (spec.field === 'FocalLength' && this._use35mm) ? 'FocalLength35' : spec.field;
            const r = this._ranges[activeField];
            this._active[activeField] = { min: r.min, max: r.max };
            this._slidersWrap.appendChild(this._buildGroup(spec, activeField, r));
        }
        if (fields.length === 0) {
            const msg = document.createElement('div');
            msg.style.cssText = 'font-size:11px;color:var(--fg-2);padding:4px 0';
            msg.textContent = t('No numeric EXIF data — re-index the library to populate.');
            this._slidersWrap.appendChild(msg);
        }
    }

    _buildGroup(spec, activeField, range) {
        const group = document.createElement('div');
        group.className = 'lib-filter-group';
        // Header row: field label + current range display
        group.innerHTML = `<div class="lib-filter-label"><span>${escapeHtml(spec.label)}</span><span class="lib-filter-range-display"></span></div>`;
        const displaySpan = group.querySelector('.lib-filter-range-display');
        displaySpan.textContent = spec.format(range.min) + ' – ' + spec.format(range.max);
        group.appendChild(this._buildRangeSlider(spec, activeField, range, displaySpan));
        if (spec.field === 'FocalLength') group.appendChild(this._focal35Switch());
        return group;
    }

    // Focal lengths as they are, or as on 35 mm film.
    _focal35Switch() {
        const wrap = document.createElement('div');
        wrap.className = 'lib-filter-35mm';
        Toggle.create(wrap, {
            initial: this._use35mm,
            labelOn: '35mm',
            labelOff: t('Native'),
            onChange: (on) => {
                this._use35mm = on;
                this._rebuildSliders();
                this._runQuery();
            },
        });
        return wrap;
    }

    _buildRangeSlider(spec, activeField, range, displaySpan) {
        const valueAt = (pos) => sliderToValue(pos, range.min, range.max, spec.log);
        const show = (minVal, maxVal) => {
            this._active[activeField] = { min: minVal, max: maxVal };
            displaySpan.textContent = spec.format(minVal) + ' – ' + spec.format(maxVal);
        };
        const slider = new RangeSlider({
            label: spec.label,
            valueText: (pos) => spec.format(valueAt(pos)),
            onInput: (minPos, maxPos) => {
                show(valueAt(minPos), valueAt(maxPos));
                this._scheduleQuery();
            },
        });
        show(range.min, range.max);
        return slider.el;
    }

    _buildTextFilters() {
        const wrap = document.createElement('div');
        wrap.className = 'lib-filter-groups lib-text-filter-groups';
        this._textFiltersWrap = wrap;
        this._container.appendChild(wrap);
        this._rebuildTextFilters();
    }

    _rebuildTextFilters() {
        this._textFiltersWrap.innerHTML = '';
        const fields = EXIF_TEXT_FILTER_FIELDS.filter(f =>
            this._textValues[f.field] && this._textValues[f.field].length > 1
        );
        for (const spec of fields) {
            this._textFiltersWrap.appendChild(this._buildDropdown(spec));
        }
        this._textFiltersWrap.style.display = fields.length ? '' : 'none';
    }

    _buildDropdown(spec) {
        const group = document.createElement('div');
        group.className = 'lib-filter-group';

        const label = document.createElement('div');
        label.className = 'lib-filter-label';
        label.textContent = spec.label;
        group.appendChild(label);

        const sel = document.createElement('select');
        sel.className = 'lib-filter-select lib-text-filter-select';
        sel.innerHTML = `<option value="">${t('All')}</option>` +
            this._textValues[spec.field].map(v =>
                `<option value="${escapeHtml(v)}"${this._textActive[spec.field] === v ? ' selected' : ''}>${escapeHtml(v)}</option>`
            ).join('');

        sel.addEventListener('change', () => {
            if (sel.value) {
                this._textActive[spec.field] = sel.value;
            } else {
                delete this._textActive[spec.field];
            }
            this._scheduleQuery();
        });

        group.appendChild(sel);
        return group;
    }

    // Colours: up to three of the twelve hues, each covering at least a tenth
    // of the photo — orange and teal, say (ADR-0049). Named ranges, not exact
    // colours, as the Colour statistics count them.
    _buildColourFilter() {
        this._hues ??= [];
        const wrap = document.createElement('div');
        wrap.className = 'lib-filter-groups';
        const group = document.createElement('div');
        group.className = 'lib-filter-group';
        group.innerHTML = `<div class="lib-filter-label" id="lib-colour-label">${t('Colours')}</div>
            <div class="lib-colour-chips" role="group" aria-labelledby="lib-colour-label">${HUE_NAMES.map((name, bin) =>
                `<button type="button" class="lib-colour-chip" data-bin="${bin}" aria-pressed="false"><span class="lib-colour-swatch" style="background: oklch(0.66 0.12 ${bin * 30 + 15})"></span>${escapeHtml(t(name))}</button>`).join('')}</div>
            <span class="form-hint">${t('Up to three; each covers at least {pct} % of the photo.', { pct: COMBO_SHARE_MIN_PERCENT })}</span>`;
        group.querySelectorAll('.lib-colour-chip').forEach(chip => chip.addEventListener('click', () => {
            const bin = Number(chip.dataset.bin);
            this._setHues(this._hues.includes(bin) ? this._hues.filter(b => b !== bin) : [...this._hues, bin].sort((a, b) => a - b));
        }));
        this._colourChips = group;
        wrap.appendChild(group);
        this._container.appendChild(wrap);
        this._syncColourChips();
    }

    _setHues(hues) {
        this._hues = hues.slice(0, 3);
        this._syncColourChips();
        this._scheduleQuery();
    }

    // A chosen chip is pressed; with three chosen the others wait.
    _syncColourChips() {
        if (!this._colourChips) return;
        for (const chip of this._colourChips.querySelectorAll('.lib-colour-chip')) {
            const on = this._hues.includes(Number(chip.dataset.bin));
            chip.setAttribute('aria-pressed', String(on));
            chip.disabled = !on && this._hues.length >= 3;
        }
    }

    // Photos or videos only, one press away: the formats of More filters
    // would have to be chosen one by one.
    _buildKindFilter() {
        const el = document.createElement('div');
        el.className = 'lib-filter-groups';
        el.innerHTML = `<div class="lib-filter-group lib-filter-group--kind">
            <div class="lib-filter-label">${t('Kind')}</div>
            <div class="seg lib-filter-kind" role="group" aria-label="${t('Kind')}">
                ${[['', t('Photos and videos')], ['photo', t('Photos')], ['video', t('Videos')]].map(([v, text]) =>
                    `<button type="button" data-kind="${v}" aria-pressed="${v === this._kind}">${text}</button>`).join('')}
            </div></div>`;
        el.querySelector('.lib-filter-kind').addEventListener('click', (e) => {
            const btn = e.target.closest('[data-kind]');
            if (!btn || btn.dataset.kind === this._kind) return;
            this._setKind(btn.dataset.kind);
            this._scheduleQuery();
        });
        this._kindEl = el;
        this._container.appendChild(el);
    }

    _setKind(kind) {
        this._kind = kind;
        for (const b of this._kindEl?.querySelectorAll('[data-kind]') || []) {
            b.setAttribute('aria-pressed', String(b.dataset.kind === kind));
        }
    }

    _buildDateFilter() {
        const { el, from, to } = dateTakenFilter(this._dateMin, this._dateMax, {
            from: v => { this._dateMin = v; this._scheduleQuery(); },
            to: v => { this._dateMax = v; this._scheduleQuery(); },
        });
        this._dateMinInput = from;
        this._dateMaxInput = to;
        this._container.appendChild(el);
    }

    _buildChipFilters() {
        const section = document.createElement('div');
        section.className = 'lib-filter-groups lib-chip-filter-section';
        section.innerHTML = `<div class="lib-filter-group-header">${t('More filters')}</div><div class="lib-chip-filter-wrap"></div>`;
        this._container.appendChild(section);
        const namespaces = () => chipNamespaces(this._exifFields || [], this._metaKeys || []);
        const sources = {
            channel: () => (this._channels || []).map(c => c.slug),
            album: () => this._albumTitles || [],
            person: () => this._personNames || [],
            thing: () => (this._thingNames || []).map(id => ({ value: id, label: ThingWords.name(id) })),
            shows: () => [],
        };
        this._chipInput = new ChipInput(section.querySelector('.lib-chip-filter-wrap'), {
            onFetchNamespaces: async () => namespaces(),
            onFetchValues: (ns) => chipValues(namespaces().find(n => n.ns === ns), ns, sources, this._initialLibID || undefined),
            onChange: () => this._scheduleQuery(),
        });
    }

    _buildStatus() {
        const status = document.createElement('div');
        status.className = 'lib-filter-status';
        status.style.display = 'none';
        this._statusEl = status;
        this._container.appendChild(status);
    }

    _reset() {
        clearTimeout(this._debounceTimer);
        this._textActive = {};
        this._use35mm = false;
        this._dateMin = '';
        this._dateMax = '';
        if (this._dateMinInput) this._dateMinInput.value = '';
        if (this._dateMaxInput) this._dateMaxInput.value = '';
        this._hues = [];
        this._setKind('');
        this._syncColourChips();
        this._chipInput?.reset();
        this._rebuildSliders();
        this._rebuildTextFilters();
        this._runQuery();
    }

    _scheduleQuery() {
        clearTimeout(this._debounceTimer);
        if (this._suppressQuery) return;
        this._renderActiveFilters();
        this._debounceTimer = setTimeout(() => this._runQuery(), 300);
    }

    _buildParams() {
        const params = {};
        if (this._initialLibID) params.ids = this._initialLibID;
        for (const [field, active] of Object.entries(this._active)) {
            const r = this._ranges[field];
            if (r && isNarrowed(active, r)) {
                params[`${field}_min`] = active.min;
                params[`${field}_max`] = active.max;
            }
        }
        for (const [field, val] of Object.entries(this._textActive || {})) {
            if (val) params[field] = val;
        }
        if (this._dateMin) params.date_taken_min = this._dateMin;
        if (this._dateMax) params.date_taken_max = this._dateMax;
        if (this._hues?.length) params.hues = this._hues.join(',');
        addChipParams(params, this._chipInput?.getChips() || []);
        if (this._kind) params.kind = this._kind;
        // Videos hidden among the photos are hidden from the results too,
        // unless the filter asks for a kind itself.
        else if (!VideoPrefs.shown()) params.kind = 'photo';
        return params;
    }

    // The chips and the count say what the filter is at once; only the
    // photos wait for the server. A newer query supersedes an older one, so a
    // slow answer never overwrites a newer filter.
    async _runQuery() {
        if (this._suppressQuery) return;
        clearTimeout(this._debounceTimer);
        this._renderActiveFilters();
        // The last condition taken away while the panel is hidden ends the
        // results.
        if (!this.isOpen() && this._narrowsNothing()) {
            this._queryGen++;
            this._options.onCleared?.();
            return;
        }
        const params = this._buildParams();
        this._lastParams = params;
        const gen = ++this._queryGen;
        this._setLoading(true);
        let failure = null;
        try {
            const result = await LibraryAPI.search({ limit: 100, ...params });
            if (gen === this._queryGen) this._renderResults(result, params);
        } catch (err) {
            failure = err;
        } finally { if (gen === this._queryGen) this._setLoading(false); }
        if (failure && gen === this._queryGen) this._showQueryError(failure);
    }

    // A quick answer replaces the count in place; only a slow one says it is
    // still searching.
    _setLoading(on) {
        clearTimeout(this._searchingTimer);
        if (on) {
            this._searchingTimer = setTimeout(() => {
                if (this._statusEl) {
                    this._statusEl.style.display = '';
                    this._statusEl.textContent = t('Searching…');
                }
            }, ACTIVITY_DELAY_MS);
        }
        this._options.onLoading?.(on);
    }

    // The results on screen belong to an earlier filter now, so they stay
    // faded until a search answers.
    _showQueryError(err) {
        this._options.onError?.();
        if (!this._statusEl) return;
        this._statusEl.style.display = '';
        this._statusEl.innerHTML = `<span class="lib-filter-status-error">${t('The search did not answer: {error}. Change a filter to try again.', { error: escapeHtml(err.message) })}</span>`;
    }

    _renderResults(result, params = this._lastParams || {}) {
        const { results, total } = result;
        const multiLib = !this._initialLibID && this._libraries.length > 1;

        // The results' head says how many; the panel says only what is slow
        // or wrong.
        if (this._statusEl) this._statusEl.style.display = 'none';

        const fetchPage = async (offset, limit) => {
            const r = await LibraryAPI.search({ limit, offset, ...params });
            return r.results;
        };

        this._options.onResults(results, multiLib, { total, fetchPage });
    }
}
