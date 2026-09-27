// LibraryFilterPanel — the Libraries photo filter: EXIF ranges, date, text and chip criteria

// Fixed chip namespaces that map to EXIF fields or special filter params.
const CHIP_NS_FIXED = [
    { ns: 'camera',  label: 'Camera',        hint: 'Camera model',    type: 'exif', param: 'Model' },
    { ns: 'lens',    label: 'Lens',           hint: 'Lens model',      type: 'exif', param: 'LensModel' },
    { ns: 'film',    label: 'Film sim',       hint: 'Film simulation', type: 'exif', param: 'FilmSimulation' },
    { ns: 'format',  label: 'Format',         hint: 'File format',     type: 'exif', param: 'ext' },
    { ns: 'flash',   label: 'Flash',          hint: 'Flash mode',      type: 'exif', param: 'Flash' },
    { ns: 'wb',      label: 'White balance',  hint: 'White balance',   type: 'exif', param: 'WhiteBalance' },
    { ns: 'channel', label: 'Destination',    hint: 'Published to this destination', type: 'channel', param: 'channel' },
    { ns: 'album',   label: 'Gallery title',  hint: 'Any gallery with this title', type: 'album', param: 'album_title' },
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
const GALLERY_CHIP_NS = { ns: 'gallery', label: 'Gallery', hint: 'One gallery', type: 'gallery', param: 'album' };

function chipToParam(chip) {
    const ns = chip.nsInfo || CHIP_NS_FIXED.find(n => n.ns === chip.ns);
    if (ns) {
        if (ns.type === 'gallery') return { key: 'album', value: chip.value };
        if (ns.type === 'exif') return { key: ns.param, value: chip.value };
        if (ns.type === 'channel') return { key: 'channel', value: chip.value };
        if (ns.type === 'album') return { key: 'album_title', value: chip.value };
        if (ns.type === 'meta') return { key: ns.param, value: chip.value };
    }
    return { key: 'meta_' + chip.ns, value: chip.value };
}

const EXIF_TEXT_FILTER_FIELDS = [
    { field: 'Model',     label: 'Camera'   },
    { field: 'LensModel', label: 'Lens'     },
    { field: 'FilmSimulation', label: 'Film sim' },
];

const EXIF_FILTER_FIELDS = [
    {
        field: 'ExposureTime',
        label: 'Shutter speed',
        format: formatShutterSpeed,
        log: true,
    },
    {
        field: 'FNumber',
        label: 'Aperture',
        format: v => `f/${v.toFixed(1)}`,
        log: true,
    },
    {
        field: 'FocalLength',
        label: 'Focal length',
        format: v => `${Math.round(v)} mm`,
        log: false,
    },
    {
        field: 'ISOSpeedRatings',
        label: 'ISO',
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
        this._options = options;
        this._ranges = {};
        this._active = {};
        this._use35mm = false;
        this._debounceTimer = null;
        this._libraries = [];
        this._queryGen = 0;
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
    }

    // The × on the results goes back to the library itself: the criteria are
    // dropped and the panel goes with them, without running one last query
    // for a filter nobody set any more — not even a debounced one, which
    // would bring the results back 300 ms after they were closed.
    clearAndHide() {
        this._suppressQuery = true;
        if (this._built) this._reset();
        this._suppressQuery = false;
        this._renderActiveFilters();
        this.close();
    }

    // Opens the filter already set, one chip per criterion — { channel: slug }
    // or { gallery: { id: "<slug>:<postID>", title } } — then one query. This is how a gallery or a destination
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
            Activity.in(this._container, 'Reading the filters…');
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

        const ids = this._initialLibID || undefined;
        // Load libraries, ranges, text field values, channels, meta keys, album titles, and all EXIF fields in parallel.
        const [libraries, ranges, channels, metaKeys, albumTitles, exifFields, ...textValues] = await Promise.all([
            LibraryAPI.list().catch(() => []),
            this._fetchRanges(this._initialLibID),
            ChannelAPI.list().catch(() => []),
            LibraryAPI.metaKeys(ids).catch(() => []),
            LibraryAPI.albumTitles(ids).catch(() => []),
            LibraryAPI.exifFields(ids).catch(() => []),
            ...EXIF_TEXT_FILTER_FIELDS.map(f =>
                LibraryAPI.exifValues(f.field, ids).catch(() => [])
            ),
        ]);
        this._libraries = libraries;
        this._ranges = ranges;
        this._channels = channels;
        this._metaKeys = metaKeys;
        this._albumTitles = albumTitles;
        this._exifFields = exifFields;
        this._textValues = {};
        this._textActive = {};
        EXIF_TEXT_FILTER_FIELDS.forEach((f, i) => {
            this._textValues[f.field] = textValues[i];
        });

        this._buildStatus();
        this._buildControls();
        this._buildDateFilter();
        this._buildSliders();
        this._buildTextFilters();
        this._buildChipFilters();
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
            <span class="lib-filter-title">Filter</span>
            <span class="lib-filter-count" hidden></span>
            <button class="btn btn-sm lib-filter-close">Done</button>`;
        head.querySelector('.lib-filter-close').addEventListener('click', () => this.close());
        this._container.appendChild(head);
        this._countEl = head.querySelector('.lib-filter-count');

        const active = document.createElement('div');
        active.className = 'lib-filter-active';
        this._activeEl = active;
        this._container.appendChild(active);
    }

    // What is filtering right now, as one chip each — the reason the count
    // under the photos is 412 and not 13 371. Every chip drops its own
    // criterion; "Reset filters" drops them all.
    activeFilters() {
        const out = [];

        for (const f of EXIF_FILTER_FIELDS) {
            const range = this._ranges[f.field];
            const active = this._active[f.field];
            if (!range || !active) continue;
            if (!isNarrowed(active, range)) continue;
            out.push({
                label: `${f.label} ${f.format(active.min)} – ${f.format(active.max)}`,
                clear: () => {
                    this._active[f.field] = { min: range.min, max: range.max };
                    this._rebuildSliders();
                    this._runQuery();
                },
            });
        }

        if (this._dateMin || this._dateMax) {
            const from = this._dateMin || 'the beginning';
            const to = this._dateMax || 'today';
            out.push({
                label: `Taken ${from} – ${to}`,
                clear: () => {
                    this._dateMin = '';
                    this._dateMax = '';
                    if (this._dateMinInput) this._dateMinInput.value = '';
                    if (this._dateMaxInput) this._dateMaxInput.value = '';
                    this._runQuery();
                },
            });
        }

        for (const f of EXIF_TEXT_FILTER_FIELDS) {
            const value = (this._textActive || {})[f.field];
            if (!value) continue;
            out.push({
                label: `${f.label}: ${value}`,
                clear: () => {
                    delete this._textActive[f.field];
                    this._rebuildTextFilters();
                    this._runQuery();
                },
            });
        }

        for (const chip of (this._chipInput?.getChips() || [])) {
            out.push({
                label: `${chip.label}: ${chip.displayValue || chip.value}`,
                clear: () => {
                    this._chipInput.removeChip(chip);
                    this._runQuery();
                },
            });
        }

        return out;
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
            chip.title = `Drop "${filter.label}"`;
            chip.addEventListener('click', () => filter.clear());
            this._activeEl.appendChild(chip);
        }
        if (this._countEl) {
            this._countEl.textContent = active.length ? `${active.length} on` : '';
            this._countEl.hidden = active.length === 0;
        }
        if (this._options.onActiveCount) this._options.onActiveCount(active.length);
    }

    _buildControls() {
        const bar = document.createElement('div');
        bar.className = 'lib-filter-controls';

        // Library selector
        const sel = document.createElement('select');
        sel.className = 'lib-filter-select';
        sel.innerHTML = `<option value="">All libraries</option>` +
            this._libraries.map(l =>
                `<option value="${escapeHtml(l.id)}"${l.id === this._initialLibID ? ' selected' : ''}>${escapeHtml(l.name)}</option>`
            ).join('');
        sel.addEventListener('change', async () => {
            this._setLoading(true);
            this._initialLibID = sel.value || null;
            const ids = this._initialLibID || undefined;
            let ranges;
            try {
                [ranges] = await Promise.all([
                    this._fetchRanges(this._initialLibID),
                    ...EXIF_TEXT_FILTER_FIELDS.map(f =>
                        LibraryAPI.exifValues(f.field, ids).catch(() => []).then(vals => {
                            this._textValues[f.field] = vals;
                        })
                    ),
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
        });
        this._libSelect = sel;

        // Reset button
        const reset = document.createElement('button');
        reset.className = 'btn btn-sm lib-filter-reset';
        reset.textContent = 'Reset filters';
        reset.addEventListener('click', () => this._reset());

        bar.appendChild(sel);
        bar.appendChild(reset);
        this._container.appendChild(bar);
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
            msg.textContent = 'No numeric EXIF data — re-index the library to populate.';
            this._slidersWrap.appendChild(msg);
        }
    }

    _buildGroup(spec, activeField, range) {
        const group = document.createElement('div');
        group.className = 'lib-filter-group';

        // Header row: field label + current range display
        const header = document.createElement('div');
        header.className = 'lib-filter-label';
        const nameSpan = document.createElement('span');
        nameSpan.textContent = spec.label;
        const displaySpan = document.createElement('span');
        displaySpan.className = 'lib-filter-range-display';
        displaySpan.textContent = spec.format(range.min) + ' – ' + spec.format(range.max);
        header.appendChild(nameSpan);
        header.appendChild(displaySpan);
        group.appendChild(header);

        group.appendChild(this._buildRangeSlider(spec, activeField, range, displaySpan));

        if (spec.field === 'FocalLength') {
            const wrap = document.createElement('div');
            wrap.className = 'lib-filter-35mm';
            Toggle.create(wrap, {
                initial: this._use35mm,
                labelOn: '35mm',
                labelOff: 'Native',
                onChange: (on) => {
                    this._use35mm = on;
                    this._rebuildSliders();
                    this._runQuery();
                }
            });
            group.appendChild(wrap);
        }

        return group;
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
        sel.innerHTML = `<option value="">All</option>` +
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

    _buildDateFilter() {
        const wrap = document.createElement('div');
        wrap.className = 'lib-filter-groups';

        const group = document.createElement('div');
        group.className = 'lib-filter-group lib-filter-group--date';

        const label = document.createElement('div');
        label.className = 'lib-filter-label';
        label.textContent = 'Date taken';
        group.appendChild(label);

        const makeRow = (labelText, initVal, onChange) => {
            const row = document.createElement('div');
            row.className = 'lib-date-row';
            const lbl = document.createElement('span');
            lbl.className = 'lib-date-row-label';
            lbl.textContent = labelText;
            const input = document.createElement('input');
            input.type = 'date';
            input.className = 'lib-date-input';
            input.value = initVal;
            input.addEventListener('change', () => onChange(input.value));
            // Escape blurs without changing value, restoring keyboard navigation.
            input.addEventListener('keydown', (e) => { if (e.key === 'Escape') input.blur(); });
            row.appendChild(lbl);
            row.appendChild(input);
            return { row, input };
        };

        const { row: fromRow, input: fromInput } = makeRow('From', this._dateMin, v => {
            this._dateMin = v;
            this._scheduleQuery();
        });
        const { row: toRow, input: toInput } = makeRow('To', this._dateMax, v => {
            this._dateMax = v;
            this._scheduleQuery();
        });
        this._dateMinInput = fromInput;
        this._dateMaxInput = toInput;

        group.appendChild(fromRow);
        group.appendChild(toRow);
        wrap.appendChild(group);
        this._container.appendChild(wrap);
    }

    _buildChipFilters() {
        const section = document.createElement('div');
        section.className = 'lib-filter-groups lib-chip-filter-section';

        const label = document.createElement('div');
        label.className = 'lib-filter-group-header';
        label.textContent = 'More filters';
        section.appendChild(label);

        const chipContainer = document.createElement('div');
        chipContainer.className = 'lib-chip-filter-wrap';
        section.appendChild(chipContainer);
        this._container.appendChild(section);

        const ids = this._initialLibID || undefined;
        const channels = this._channels || [];
        const metaKeys = this._metaKeys || [];
        const albumTitles = this._albumTitles || [];
        const exifFields = this._exifFields || [];

        // Params already covered by fixed namespaces — don't add them again as dynamic EXIF entries.
        const fixedParams = new Set(CHIP_NS_FIXED.filter(n => n.type === 'exif').map(n => n.param));

        const buildNamespaces = () => {
            const nss = [...CHIP_NS_FIXED];
            // Add all EXIF fields from the index (excluding slider fields and already-fixed ones).
            for (const field of exifFields) {
                if (field === 'DateTaken' || field === 'DateTimeOriginal') continue;
                if (SLIDER_FIELDS.has(field)) continue;
                if (fixedParams.has(field)) continue;
                if (!nss.find(n => n.ns === field)) {
                    nss.push({ ns: field, label: field, hint: 'EXIF field', type: 'exif', param: field });
                }
            }
            // Add dynamic user-defined meta keys (exclude built:* system keys).
            for (const key of metaKeys) {
                if (key.startsWith('built:')) continue;
                if (!nss.find(n => n.ns === key)) {
                    nss.push({ ns: key, label: key, hint: 'Custom tag', type: 'meta', param: 'meta_' + key });
                }
            }
            return nss;
        };

        this._chipInput = new ChipInput(chipContainer, {
            onFetchNamespaces: async () => buildNamespaces(),
            onFetchValues: async (ns) => {
                // Look up the full namespace descriptor built by buildNamespaces().
                const nsDesc = buildNamespaces().find(n => n.ns === ns);
                if (nsDesc) {
                    if (nsDesc.ns === 'channel') return channels.map(c => c.slug);
                    if (nsDesc.ns === 'album') return albumTitles;
                    if (nsDesc.type === 'exif') {
                        const raw = await LibraryAPI.exifValues(nsDesc.param, ids).catch(() => []);
                        return labeledExifValues(nsDesc.param, raw);
                    }
                    if (nsDesc.type === 'meta') return LibraryAPI.metaValues(ns, ids).catch(() => []);
                }
                return LibraryAPI.metaValues(ns, ids).catch(() => []);
            },
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
            if (!r) continue;
            if (isNarrowed(active, r)) {
                params[`${field}_min`] = active.min;
                params[`${field}_max`] = active.max;
            }
        }
        for (const [field, val] of Object.entries(this._textActive || {})) {
            if (val) params[field] = val;
        }
        if (this._dateMin) params.date_taken_min = this._dateMin;
        if (this._dateMax) params.date_taken_max = this._dateMax;
        for (const chip of (this._chipInput?.getChips() || [])) {
            const p = chipToParam(chip);
            if (p) params[p.key] = p.value;
        }
        return params;
    }

    // The chips and the count say what the filter is at once; only the
    // photos wait for the server. A newer query supersedes an older one, so a
    // slow answer never overwrites a newer filter.
    async _runQuery() {
        if (this._suppressQuery) return;
        clearTimeout(this._debounceTimer);
        this._renderActiveFilters();
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
                if (this._statusEl && this._statusEl.style.display !== 'none') {
                    this._statusEl.textContent = 'Searching…';
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
        this._statusEl.innerHTML = `<span class="lib-filter-status-error">The search did not answer: ${escapeHtml(err.message)}. Change a filter to try again.</span>`;
    }

    _renderResults(result, params = this._lastParams || {}) {
        const { results, total } = result;
        const multiLib = !this._initialLibID && this._libraries.length > 1;

        if (this._statusEl) {
            this._statusEl.style.display = '';
            this._statusEl.innerHTML = `<strong>${total}</strong> photo${total !== 1 ? 's' : ''} match`;
        }

        const fetchPage = async (offset, limit) => {
            const r = await LibraryAPI.search({ limit, offset, ...params });
            return r.results;
        };

        this._options.onResults(results, multiLib, { total, fetchPage });
    }
}
