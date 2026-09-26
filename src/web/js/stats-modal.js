// StatsModal — photo library statistics with D3 visualisations

class StatsModal {
    // libs: array of Library objects (from /api/library/)
    // opts.pathPrefix: absolute path string — scopes stats to a folder
    // opts.fixedScope: if true, library selector is disabled
    // opts.scopeLabel: subtitle shown below title when scope is fixed
    open(libs, opts = {}) {
        this._libs = libs;
        this._selectedId = opts.libraryId ?? null;
        this._pathPrefix = opts.pathPrefix ?? '';
        this._fixedScope = opts.fixedScope ?? false;
        this._activeTab = 'snapshot';
        this._granularity = '';
        this._snapData = null;
        this._tlData = null;
        this._tlGeneration = 0;

        this._dialog = new Dialog({
            title: 'Statistics',
            subtitle: opts.scopeLabel || '',
            size: 'lg',
            className: 'stats-dialog',
            body: `
                <div class="stats-lib-filter" id="stats-lib-filter"></div>
                <div class="stats-body" id="stats-body"></div>`,
            actions: [{ label: 'Close', onClick: () => this.close() }],
        });
        const overlay = this._dialog.open();
        this._overlay = overlay;

        this._buildLibFilter(overlay.querySelector('#stats-lib-filter'));
        this._load(overlay.querySelector('#stats-body'));
    }

    close() {
        this._dialog?.close(null);
    }

    _buildLibFilter(el) {
        if (!this._libs?.length) return;
        const sel = document.createElement('select');
        sel.className = 'stats-lib-select';
        if (this._fixedScope) sel.disabled = true;

        const allOpt = document.createElement('option');
        allOpt.value = '';
        allOpt.textContent = 'All libraries';
        sel.appendChild(allOpt);
        for (const lib of this._libs) {
            const opt = document.createElement('option');
            opt.value = lib.id;
            opt.textContent = lib.name;
            sel.appendChild(opt);
        }
        if (this._selectedId) sel.value = this._selectedId;
        sel.addEventListener('change', () => {
            this._selectedId = sel.value || null;
            this._load(this._overlay.querySelector('#stats-body'));
        });
        el.appendChild(sel);
    }

    async _load(body) {
        this._body = body;
        this._tlData = null;
        Activity.in(body, 'Counting the photos…', { area: true });
        try {
            const qs = this._buildQS();
            const r = await fetch(`/api/library/statistics${qs}`);
            if (!r.ok) throw new Error(await r.text());
            this._snapData = await r.json();
            this._renderWithTabs();
        } catch (err) {
            body.innerHTML = `<div class="stats-error">Failed to load statistics: ${escapeHtml(err.message)}</div>`;
        }
    }

    _buildQS() {
        const params = new URLSearchParams();
        if (this._selectedId) params.set('ids', this._selectedId);
        if (this._pathPrefix) params.set('pathPrefix', this._pathPrefix);
        return params.toString() ? '?' + params.toString() : '';
    }

    _renderWithTabs() {
        const body = this._body;
        body.innerHTML = '';

        const tabs = document.createElement('div');
        tabs.className = 'stats-tabs';
        const makeTab = (id, label) => {
            const btn = document.createElement('button');
            btn.className = 'stats-tab-btn' + (this._activeTab === id ? ' stats-tab-active' : '');
            btn.textContent = label;
            btn.addEventListener('click', () => {
                this._activeTab = id;
                this._renderWithTabs();
            });
            return btn;
        };
        tabs.appendChild(makeTab('snapshot', 'Snapshot'));
        tabs.appendChild(makeTab('timeline', 'Timeline'));
        body.appendChild(tabs);

        if (this._activeTab === 'snapshot') {
            this._renderSnapshot(body, this._snapData);
        } else {
            this._renderTimeline().catch(err => {
                body.appendChild(Object.assign(document.createElement('div'), {
                    className: 'stats-error',
                    textContent: 'Failed to load timeline: ' + err.message,
                }));
            });
        }
    }

    async _renderTimeline() {
        const body = this._body;
        const generation = ++this._tlGeneration;

        const controls = document.createElement('div');
        controls.className = 'stats-tl-controls';
        const toggle = document.createElement('div');
        toggle.className = 'stats-focal-toggle';
        const granOptions = [
            { value: '', label: 'Auto' },
            { value: 'month', label: 'Month' },
            { value: 'year', label: 'Year' },
        ];
        for (const opt of granOptions) {
            const btn = document.createElement('button');
            btn.className = 'stats-toggle-btn' + (this._granularity === opt.value ? ' stats-toggle-active' : '');
            btn.textContent = opt.label;
            btn.addEventListener('click', () => {
                if (this._granularity === opt.value) return;
                this._granularity = opt.value;
                this._tlData = null;
                this._renderWithTabs();
            });
            toggle.appendChild(btn);
        }
        controls.appendChild(toggle);
        body.appendChild(controls);

        if (!this._tlData) {
            const loading = document.createElement('div');
            body.appendChild(loading);
            Activity.in(loading, 'Reading the timeline…', { area: true });

            const params = new URLSearchParams();
            if (this._selectedId) params.set('ids', this._selectedId);
            if (this._pathPrefix) params.set('pathPrefix', this._pathPrefix);
            if (this._granularity) params.set('granularity', this._granularity);
            const qs = params.toString() ? '?' + params.toString() : '';
            const r = await fetch(`/api/library/timeline${qs}`);
            if (generation !== this._tlGeneration) return;
            if (!r.ok) throw new Error(await r.text());
            this._tlData = await r.json();
            if (generation !== this._tlGeneration) return;
            loading.remove();
        }

        const tlData = this._tlData;
        toggle.querySelector('button').textContent = `Auto (${tlData.granularity})`;

        const panel = document.createElement('div');
        panel.className = 'stats-timeline';
        body.appendChild(panel);

        this._addTlChart(panel, 'Camera usage', 'Photos per camera per period',
            el => renderCameraStream(el, tlData));
        this._addTlChart(panel, 'Focal length drift', 'Median 35mm equiv. with IQR band',
            el => renderFocalDrift(el, tlData));
        this._addTlChart(panel, 'ISO evolution', 'Median ISO per period',
            el => renderISOEvolution(el, tlData));
        this._addTlChart(panel, 'Aperture usage', 'Normalised share of shots per f-stop',
            el => renderApertureHeat(el, tlData));
        this._addTlChart(panel, 'Aspect ratio mix', 'Proportion of frame shapes per period',
            el => renderAspectRiver(el, tlData));
        this._addTlChart(panel, 'Megapixel timeline', 'Max and average sensor resolution per period',
            el => renderMegapixelTimeline(el, tlData));
    }

    _addTlChart(panel, title, subtitle, renderFn) {
        const card = document.createElement('div');
        card.className = 'stats-tl-chart';
        const titleEl = document.createElement('div');
        titleEl.className = 'stats-tl-title';
        titleEl.textContent = title;
        const subtitleEl = document.createElement('div');
        subtitleEl.className = 'stats-tl-subtitle';
        subtitleEl.textContent = subtitle;
        card.appendChild(titleEl);
        card.appendChild(subtitleEl);
        const content = document.createElement('div');
        card.appendChild(content);
        panel.appendChild(card);
        try { renderFn(content); } catch (_) {
            content.innerHTML = '<div class="stats-tl-nodata">No data</div>';
        }
    }

    _renderSnapshot(body, data) {
        const totalEl = document.createElement('div');
        totalEl.className = 'stats-total';
        totalEl.textContent = `${data.totalPhotos.toLocaleString()} photos`;
        body.appendChild(totalEl);

        const warnings = [];
        if (data.indexingPhotos > 0)
            warnings.push(`${data.indexingPhotos.toLocaleString()} photos are still being indexed — statistics are incomplete.`);
        for (const w of (data.warnings ?? []))
            warnings.push(w);
        if (warnings.length) {
            const banner = document.createElement('div');
            banner.className = 'stats-warning';
            banner.textContent = warnings.join(' ');
            body.appendChild(banner);
        }

        const grid = document.createElement('div');
        grid.className = 'stats-grid';
        body.appendChild(grid);

        const total = data.totalPhotos;
        const sumCounts = arr => (arr ?? []).reduce((s, v) => s + v.count, 0);
        const coverageSub = count => (count < total && total > 0)
            ? `${count.toLocaleString()} of ${total.toLocaleString()} photos`
            : null;

        const focalCount  = sumCounts(data.focalLengths);
        const aperCount   = sumCounts(data.apertures);
        const isoCount    = sumCounts(data.isos);

        this._addChart(grid, 'Format', false, el => renderFormatDonut(el, data.formats));
        this._addChart(grid, 'Time of day', false, el => renderShootingClock(el, data.shootingHours));
        if (data.filmSims?.some(s => s.name !== 'None')) {
            this._addChart(grid, 'Film simulation', true, el => renderFilmSimBar(el, data.filmSims));
        }
        this._addChart(grid, 'Focal length', true, el => renderFocalHistogram(el, expandValues(data.focalLengths), expandValues(data.focalLengths35)), coverageSub(focalCount));
        this._addChart(grid, 'Aperture', false, el => renderApertureHistogram(el, expandValues(data.apertures)), coverageSub(aperCount));
        this._addChart(grid, 'ISO', false, el => renderISOHistogram(el, expandValues(data.isos)), coverageSub(isoCount));
        this._addChart(grid, 'Camera × Lens', true, el => renderCameraLensTreemap(el, data.cameraLens, data.totalPhotos));
        this._addChart(grid, 'Shooting calendar', true, el => renderCalendarHeatmap(el, data.shootingDays));
    }

    _addChart(grid, title, fullWidth, renderFn, subtitle) {
        const card = document.createElement('div');
        card.className = 'stats-chart' + (fullWidth ? ' stats-chart--full' : '');
        const h = document.createElement('div');
        h.className = 'stats-chart-title';
        h.textContent = title;
        card.appendChild(h);
        if (subtitle) {
            const sub = document.createElement('div');
            sub.className = 'stats-chart-subtitle';
            sub.textContent = subtitle;
            card.appendChild(sub);
        }
        const content = document.createElement('div');
        content.className = 'stats-chart-content';
        card.appendChild(content);
        grid.appendChild(card);
        try { renderFn(content); } catch (_) { content.textContent = 'No data'; }
    }
}
