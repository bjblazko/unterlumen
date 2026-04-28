// LibrarySearchPanel — cross-library EXIF numeric range search with slider UI

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
    //   onResults:        if set, called with results instead of rendering a pane inside the panel
    //   onClose:          called when the panel is closed
    //   resultsContainer: DOM element where SearchResultPane is mounted (list-page path)
    //   onFocusChange:    forwarded to SearchResultPane for info panel integration
    constructor(container, toggleBtn, initialLibID = null, options = {}) {
        this._container = container;
        this._toggleBtn = toggleBtn;
        this._initialLibID = initialLibID;
        this._options = options;
        this._ranges = {};
        this._active = {};
        this._debounceTimer = null;
        this._libraries = [];
        this._searchPane = null;

        toggleBtn.addEventListener('click', () => this._toggle());
    }

    async _toggle() {
        const opening = !this._container.classList.contains('visible');
        if (!opening) {
            this._container.classList.remove('visible');
            this._toggleBtn.classList.remove('active');
            if (this._options.onClose) this._options.onClose();
            return;
        }
        this._container.classList.add('visible');
        this._toggleBtn.classList.add('active');

        if (!this._built) {
            await this._build();
        }
    }

    async _build() {
        this._built = true;
        this._container.innerHTML = '';
        this._container.className = 'lib-search-panel visible';

        // Load libraries, ranges, and text field values in parallel.
        const [libraries, ranges, ...textValues] = await Promise.all([
            LibraryAPI.list().catch(() => []),
            this._fetchRanges(this._initialLibID),
            ...EXIF_TEXT_FILTER_FIELDS.map(f =>
                LibraryAPI.exifValues(f.field, this._initialLibID || undefined).catch(() => [])
            ),
        ]);
        this._libraries = libraries;
        this._ranges = ranges;
        this._textValues = {};
        this._textActive = {};
        EXIF_TEXT_FILTER_FIELDS.forEach((f, i) => {
            this._textValues[f.field] = textValues[i];
        });

        this._buildControls();
        this._buildSliders();
        this._buildTextFilters();
        this._buildResults();
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
            const r = this._ranges[f.field];
            return r && r.min < r.max;
        });
        for (const spec of fields) {
            const r = this._ranges[spec.field];
            this._active[spec.field] = { min: r.min, max: r.max };
            this._slidersWrap.appendChild(this._buildGroup(spec, r));
        }
        if (fields.length === 0) {
            const msg = document.createElement('div');
            msg.style.cssText = 'font-size:11px;color:var(--text-sec);padding:4px 0';
            msg.textContent = 'No numeric EXIF data — re-index the library to populate.';
            this._slidersWrap.appendChild(msg);
        }
    }

    _buildGroup(spec, range) {
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

        // From row
        const [fromRow, fromSlider, fromVal] = this._buildSliderRow('From', 0, spec, range);
        // To row
        const [toRow, toSlider, toVal] = this._buildSliderRow('To', 1000, spec, range);

        fromSlider.addEventListener('input', () => {
            if (+fromSlider.value > +toSlider.value) fromSlider.value = toSlider.value;
            const v = sliderToValue(+fromSlider.value / 1000, range.min, range.max, spec.log);
            fromVal.textContent = spec.format(v);
            this._active[spec.field] = {
                min: v,
                max: sliderToValue(+toSlider.value / 1000, range.min, range.max, spec.log),
            };
            displaySpan.textContent = spec.format(this._active[spec.field].min) + ' – ' + spec.format(this._active[spec.field].max);
            this._scheduleQuery();
        });

        toSlider.addEventListener('input', () => {
            if (+toSlider.value < +fromSlider.value) toSlider.value = fromSlider.value;
            const v = sliderToValue(+toSlider.value / 1000, range.min, range.max, spec.log);
            toVal.textContent = spec.format(v);
            this._active[spec.field] = {
                min: sliderToValue(+fromSlider.value / 1000, range.min, range.max, spec.log),
                max: v,
            };
            displaySpan.textContent = spec.format(this._active[spec.field].min) + ' – ' + spec.format(this._active[spec.field].max);
            this._scheduleQuery();
        });

        group.appendChild(fromRow);
        group.appendChild(toRow);
        return group;
    }

    _buildSliderRow(tag, initialValue, spec, range) {
        const row = document.createElement('div');
        row.className = 'lib-slider-row';

        const tagEl = document.createElement('span');
        tagEl.className = 'lib-slider-tag';
        tagEl.textContent = tag;

        const slider = document.createElement('input');
        slider.type = 'range';
        slider.className = 'lib-filter-range';
        slider.min = 0;
        slider.max = 1000;
        slider.value = initialValue;

        const valEl = document.createElement('span');
        valEl.className = 'lib-slider-val';
        valEl.textContent = spec.format(sliderToValue(initialValue / 1000, range.min, range.max, spec.log));

        row.appendChild(tagEl);
        row.appendChild(slider);
        row.appendChild(valEl);
        return [row, slider, valEl];
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

    _buildResults() {
        const status = document.createElement('div');
        status.className = 'lib-search-status';
        status.style.display = 'none';
        this._statusEl = status;
        this._container.appendChild(status);

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
        });
    }

    _reset() {
        this._textActive = {};
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
            if (active.min > r.min + 1e-12) params[`${field}_min`] = active.min;
            if (active.max < r.max - 1e-12) params[`${field}_max`] = active.max;
        }
        for (const [field, val] of Object.entries(this._textActive || {})) {
            if (val) params[field] = val;
        }
        return params;
    }

    async _runQuery() {
        const params = this._buildParams();
        try {
            const result = await LibraryAPI.search({ limit: 100, ...params });
            this._renderResults(result);
        } catch { /* ignore transient errors */ }
    }

    _renderResults(result) {
        const { results, total } = result;
        const multiLib = !this._initialLibID && this._libraries.length > 1;

        if (this._statusEl) {
            this._statusEl.style.display = '';
            const suffix = total > results.length ? ` · showing ${results.length}` : '';
            this._statusEl.innerHTML = `<strong>${total}</strong> photo${total !== 1 ? 's' : ''} match${suffix}`;
        }

        if (this._options.onResults) {
            this._options.onResults(results, multiLib);
            return;
        }

        if (this._searchPane) {
            this._searchPane.loadResults(results, multiLib);
        }
    }
}
