// LibrarySearchPanel — cross-library EXIF numeric range search with slider UI

// Fixed chip namespaces that map to EXIF fields or special filter params.
const CHIP_NS_FIXED = [
    { ns: 'camera',  label: 'Camera',        hint: 'Camera model',    type: 'exif', param: 'Model' },
    { ns: 'lens',    label: 'Lens',           hint: 'Lens model',      type: 'exif', param: 'LensModel' },
    { ns: 'film',    label: 'Film sim',       hint: 'Film simulation', type: 'exif', param: 'FilmSimulation' },
    { ns: 'format',  label: 'Format',         hint: 'File format',     type: 'exif', param: 'ext' },
    { ns: 'flash',   label: 'Flash',          hint: 'Flash mode',      type: 'exif', param: 'Flash' },
    { ns: 'wb',      label: 'White balance',  hint: 'White balance',   type: 'exif', param: 'WhiteBalance' },
    { ns: 'channel', label: 'Channel',        hint: 'Published to channel', type: 'channel', param: 'channel' },
    { ns: 'album',   label: 'Album',          hint: 'Gallery / album title', type: 'album', param: 'album_title' },
];

// EXIF fields handled by numeric sliders — exclude from chip namespace list to avoid duplication.
const SLIDER_FIELDS = new Set([
    'ExposureTime', 'FNumber', 'FocalLength', 'FocalLength35',
    'FocalLengthIn35mmFilm', 'ISOSpeedRatings',
]);

// Translates a chip {ns, nsInfo, value} to a search param {key, value}.
// nsInfo is the full namespace descriptor stored by ChipInput when a chip is created.
function chipToParam(chip) {
    const ns = chip.nsInfo || CHIP_NS_FIXED.find(n => n.ns === chip.ns);
    if (ns) {
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

function sliderToValue(pos, min, max, log) {
    if (log) return Math.exp(Math.log(min) + pos * (Math.log(max) - Math.log(min)));
    return min + pos * (max - min);
}

function valueToSlider(val, min, max, log) {
    if (log) return (Math.log(val) - Math.log(min)) / (Math.log(max) - Math.log(min));
    return (val - min) / (max - min);
}

class LibrarySearchPanel {
    // container    — the element that holds the full panel
    // toggleBtn    — the button that opens/closes the panel
    // initialLibID — pre-select this library (null = all)
    // options      — { onResults(photos, multiLib), onClose(), resultsContainer, onFocusChange }
    //   onResults:          if set, called with results instead of rendering a pane inside the panel
    //   onClose:            called when the panel is closed
    //   resultsContainer:   DOM element where SearchResultPane is mounted (list-page path)
    //   onFocusChange:      forwarded to SearchResultPane for info panel integration
    //   onSelectionChange:  forwarded to SearchResultPane; called when selection changes
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
        this._searchPane = null;
        this._dateMin = '';
        this._dateMax = '';
        this._dateMinInput = null;
        this._dateMaxInput = null;
        this._chipInput = null;

        toggleBtn.addEventListener('click', () => this._toggle());
    }

    close() {
        if (!this._container.classList.contains('visible')) return;
        this._container.classList.remove('visible');
        this._toggleBtn.dataset.state = 'off';
        this._toggleBtn.setAttribute('aria-pressed', 'false');
        if (this._options.onClose) this._options.onClose();
    }

    async _toggle() {
        const opening = !this._container.classList.contains('visible');
        if (!opening) {
            this.close();
            return;
        }
        this._container.classList.add('visible');
        this._toggleBtn.dataset.state = 'on';
        this._toggleBtn.setAttribute('aria-pressed', 'true');

        if (!this._built) {
            this._container.innerHTML = '<div class="lib-search-loading">Loading filters…</div>';
            await this._build();
        } else {
            // Clear chip input caches so re-opening the panel picks up newly published albums etc.
            this._chipInput?.clearCache();
        }
    }

    async _build() {
        this._built = true;
        this._container.innerHTML = '';
        this._container.className = 'lib-search-panel visible';

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
        this._buildResultsPane();
        this._runQuery();
    }

    async _fetchRanges(libID) {
        try {
            if (libID) return await LibraryAPI.exifRanges(libID);
            return await LibraryAPI.globalExifRanges();
        } catch { return {}; }
    }

    _buildControls() {
        const bar = document.createElement('div');
        bar.className = 'lib-search-controls';

        // Library selector
        const sel = document.createElement('select');
        sel.className = 'lib-search-select';
        sel.innerHTML = `<option value="">All libraries</option>` +
            this._libraries.map(l =>
                `<option value="${escapeHtml(l.id)}"${l.id === this._initialLibID ? ' selected' : ''}>${escapeHtml(l.name)}</option>`
            ).join('');
        sel.addEventListener('change', async () => {
            this._initialLibID = sel.value || null;
            const ids = this._initialLibID || undefined;
            const [ranges] = await Promise.all([
                this._fetchRanges(this._initialLibID),
                ...EXIF_TEXT_FILTER_FIELDS.map(f =>
                    LibraryAPI.exifValues(f.field, ids).catch(() => []).then(vals => {
                        this._textValues[f.field] = vals;
                    })
                ),
            ]);
            this._ranges = ranges;
            this._textActive = {};
            this._rebuildSliders();
            this._rebuildTextFilters();
            this._runQuery();
        });
        this._libSelect = sel;

        // Reset button
        const reset = document.createElement('button');
        reset.className = 'lib-search-reset';
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
            msg.style.cssText = 'font-size:11px;color:var(--text-sec);padding:4px 0';
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
        const wrap = document.createElement('div');
        wrap.className = 'lib-range-slider';

        const track = document.createElement('div');
        track.className = 'lib-range-track';
        const fill = document.createElement('div');
        fill.className = 'lib-range-fill';
        track.appendChild(fill);
        wrap.appendChild(track);

        const minHandle = document.createElement('div');
        minHandle.className = 'lib-range-handle lib-range-handle--min';
        const maxHandle = document.createElement('div');
        maxHandle.className = 'lib-range-handle lib-range-handle--max';
        wrap.appendChild(minHandle);
        wrap.appendChild(maxHandle);

        let minPos = 0;
        let maxPos = 1;

        const updateUI = () => {
            minHandle.style.left = `${minPos * 100}%`;
            maxHandle.style.left = `${maxPos * 100}%`;
            fill.style.left = `${minPos * 100}%`;
            fill.style.width = `${(maxPos - minPos) * 100}%`;
            const minVal = sliderToValue(minPos, range.min, range.max, spec.log);
            const maxVal = sliderToValue(maxPos, range.min, range.max, spec.log);
            this._active[activeField] = { min: minVal, max: maxVal };
            displaySpan.textContent = spec.format(minVal) + ' – ' + spec.format(maxVal);
        };

        const attachDrag = (handle, isMin) => {
            handle.addEventListener('mousedown', (e) => {
                if (e.button !== 0) return;
                e.preventDefault();
                document.body.style.userSelect = 'none';

                const onMove = (e) => {
                    const rect = wrap.getBoundingClientRect();
                    const pos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
                    if (isMin) minPos = Math.min(pos, maxPos);
                    else maxPos = Math.max(pos, minPos);
                    updateUI();
                    this._scheduleQuery();
                };

                const onUp = () => {
                    document.body.style.userSelect = '';
                    document.removeEventListener('mousemove', onMove);
                    document.removeEventListener('mouseup', onUp);
                };

                document.addEventListener('mousemove', onMove);
                document.addEventListener('mouseup', onUp);
            });
        };

        attachDrag(minHandle, true);
        attachDrag(maxHandle, false);

        updateUI();
        return wrap;
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
        sel.className = 'lib-search-select lib-text-filter-select';
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
            // Add dynamic user-defined meta keys (exclude published:* system keys).
            for (const key of metaKeys) {
                if (key.startsWith('published:')) continue;
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
        status.className = 'lib-search-status';
        status.style.display = 'none';
        this._statusEl = status;
        this._container.appendChild(status);
    }

    _buildResultsPane() {
        // When onResults is set, the caller owns the results pane (detail-page path).
        if (this._options.onResults) return;

        // Otherwise mount a SearchResultPane in the provided container or inside the panel.
        const paneEl = this._options.resultsContainer || (() => {
            const div = document.createElement('div');
            div.className = 'lib-search-results';
            this._container.appendChild(div);
            return div;
        })();

        this._searchPane = new SearchResultPane(paneEl, {
            onImageClick: (path) => App.openViewer(path, this._searchPane),
            onSlideshowInvoke: () => App.handleSlideshowInvoke(this._searchPane),
            onFocusChange: this._options.onFocusChange || null,
            onSelectionChange: this._options.onSelectionChange || null,
            onToolInvoke: this._options.onToolInvoke || null,
            onClose: () => this.close(),
        });
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
        this._debounceTimer = setTimeout(() => this._runQuery(), 300);
    }

    _buildParams() {
        const params = {};
        if (this._initialLibID) params.ids = this._initialLibID;
        for (const [field, active] of Object.entries(this._active)) {
            const r = this._ranges[field];
            if (!r) continue;
            const minMoved = active.min > r.min + 1e-12;
            const maxMoved = active.max < r.max - 1e-12;
            if (minMoved || maxMoved) {
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

    async _runQuery() {
        const params = this._buildParams();
        this._lastParams = params;
        this._setLoading(true);
        try {
            const result = await LibraryAPI.search({ limit: 100, ...params });
            this._renderResults(result, params);
        } catch { /* ignore transient errors */ }
        finally { this._setLoading(false); }
    }

    _setLoading(on) {
        if (this._options.onLoading) {
            this._options.onLoading(on);
            return;
        }
        // List-view path: overlay a spinner on the results container.
        const container = this._searchPane?.container;
        if (!container) return;
        if (on) {
            if (!container.querySelector('.lib-results-spinner-wrap')) {
                container.insertAdjacentHTML('afterbegin',
                    '<div class="lib-results-spinner-wrap lib-results-spinner-overlay">' +
                    '<div class="lib-results-spinner"></div></div>');
            }
        } else {
            container.querySelector('.lib-results-spinner-wrap')?.remove();
        }
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

        if (this._options.onResults) {
            this._options.onResults(results, multiLib, { total, fetchPage });
            return;
        }

        if (this._searchPane) {
            this._searchPane.loadResults(results, multiLib, { total, fetchPage });
        }
    }
}
