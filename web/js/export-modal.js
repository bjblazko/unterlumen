// ExportModal — convert and export selected images to JPEG, PNG, or WebP.
// Follows the same class pattern as LocationModal and BatchRenameModal.

class ExportModal {
    constructor() {
        this.overlay = null;
        this._onKeyDown = this._onKeyDown.bind(this);
        this._estimateTimer = null;
        this._estimateAbort = null; // AbortController for exact estimation
    }

    open(files, { serverRole = false, exiftoolAvailable = false } = {}) {
        if (this.overlay) this.close();
        this._files = files;
        this._serverRole = serverRole;
        this._exiftoolAvailable = exiftoolAvailable;

        this._buildDOM();
        document.body.appendChild(this.overlay);
        document.addEventListener('keydown', this._onKeyDown);

        this._refreshEstimates();
    }

    close() {
        if (this.overlay) {
            this.overlay.remove();
            this.overlay = null;
        }
        document.removeEventListener('keydown', this._onKeyDown);
        if (this._estimateTimer) {
            clearTimeout(this._estimateTimer);
            this._estimateTimer = null;
        }
    }

    _onKeyDown(e) {
        if (e.key === 'Escape') this.close();
    }

    _buildDOM() {
        const files = this._files;
        const serverRole = this._serverRole;
        const exiftoolAvailable = this._exiftoolAvailable;

        const gpsNote = exiftoolAvailable ? '' : ' <span class="export-note">(requires exiftool)</span>';
        const gpsDisabled = exiftoolAvailable ? '' : ' disabled';

        const outputSection = serverRole ? '' : `
            <div class="export-section">
                <div class="export-section-title">Output</div>
                <label class="export-radio-row">
                    <input type="radio" name="output-mode" value="folder" checked> Save to folder
                </label>
                <div class="export-destination-wrap">
                    <input type="text" class="export-destination-input" placeholder="/path/to/output/folder">
                </div>
                <label class="export-radio-row">
                    <input type="radio" name="output-mode" value="zip"> Download as ZIP
                </label>
            </div>`;

        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay';
        this.overlay.innerHTML = `
            <div class="modal export-modal">
                <div class="modal-header">
                    <span class="modal-title">Export ${files.length === 1 ? '1 image' : files.length + ' images'}</span>
                    <button class="info-collapse-btn modal-close-btn" title="Close">&times;</button>
                </div>
                <div class="modal-body">

                    <div class="export-section">
                        <div class="export-row">
                            <span class="export-label">Format</span>
                            <div class="export-format-tabs">
                                <button class="btn btn-sm active" data-format="jpeg">JPEG</button>
                                <button class="btn btn-sm" data-format="png">PNG</button>
                                <button class="btn btn-sm" data-format="webp">WebP</button>
                            </div>
                        </div>
                        <div class="export-row export-quality-row">
                            <span class="export-label">Quality</span>
                            <input type="range" class="export-quality-slider" min="1" max="100" value="85">
                            <span class="export-quality-value">85</span>
                        </div>
                    </div>

                    <div class="export-section">
                        <div class="export-section-title">Scale</div>
                        <label class="export-radio-row">
                            <input type="radio" name="scale-mode" value="none" checked> Original size
                        </label>
                        <label class="export-radio-row">
                            <input type="radio" name="scale-mode" value="percent"> Percentage
                            <span class="export-sub-input export-percent-input" style="display:none">
                                <input type="number" class="export-input export-percent-val" min="1" max="400" value="50"> %
                            </span>
                        </label>
                        <label class="export-radio-row">
                            <input type="radio" name="scale-mode" value="max_dim"> Maximum dimension
                            <span class="export-sub-input export-maxdim-input" style="display:none">
                                <label class="export-ar-label"><input type="radio" name="max-dim-axis" value="width" checked> Width</label>
                                <label class="export-ar-label"><input type="radio" name="max-dim-axis" value="height"> Height</label>
                                <input type="number" class="export-input export-maxdim-val" placeholder="px" min="1" style="width:70px">
                            </span>
                        </label>
                    </div>

                    <div class="export-section">
                        <div class="export-section-title">Metadata</div>
                        <label class="export-radio-row">
                            <input type="radio" name="exif-mode" value="strip" checked> Strip all EXIF
                        </label>
                        <label class="export-radio-row">
                            <input type="radio" name="exif-mode" value="keep"> Keep all EXIF
                        </label>
                        <label class="export-radio-row">
                            <input type="radio" name="exif-mode" value="keep_no_gps"${gpsDisabled}> Keep EXIF, remove GPS${gpsNote}
                        </label>
                    </div>

                    <div class="export-section">
                        <div class="export-section-header">
                            <span class="export-section-title">Files</span>
                            <div class="export-estimate-controls">
                                <div class="export-estimate-toggle">
                                    <button class="btn btn-sm active" data-estimate="heuristic" title="Fast estimate">~</button>
                                    <button class="btn btn-sm" data-estimate="encode" title="Exact encode">&#9702;</button>
                                </div>
                                <button type="button" class="btn btn-sm export-estimate-abort" style="display:none">Abort</button>
                            </div>
                        </div>
                        <div class="export-progress-row" style="display:none">
                            <span class="export-progress-text">Calculating exact sizes…</span>
                            <div class="export-progress-bar"><div class="export-progress-fill"></div></div>
                            <span class="export-progress-label"></span>
                        </div>
                        <div class="export-file-list"></div>
                        <div class="export-total-row">
                            <span class="export-total-label">Total</span>
                            <span class="export-total-in">—</span>
                            <span class="export-file-arrow">→</span>
                            <span class="export-total-out">—</span>
                        </div>
                    </div>

                    ${outputSection}

                </div>
                <div class="modal-footer">
                    <div class="export-status"></div>
                    <button class="btn" id="export-cancel-btn">Cancel</button>
                    <button class="btn btn-accent" id="export-confirm-btn">Export</button>
                </div>
            </div>`;

        // Close on overlay click
        this.overlay.addEventListener('click', (e) => {
            if (e.target === this.overlay) this.close();
        });

        // Close and cancel buttons
        this.overlay.querySelector('.modal-close-btn').addEventListener('click', () => this.close());
        this.overlay.querySelector('#export-cancel-btn').addEventListener('click', () => this.close());

        // Format tabs
        this.overlay.querySelectorAll('[data-format]').forEach(btn => {
            btn.addEventListener('click', () => {
                this.overlay.querySelectorAll('[data-format]').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                const isPNG = btn.dataset.format === 'png';
                this.overlay.querySelector('.export-quality-row').style.display = isPNG ? 'none' : '';
                this._scheduleEstimate();
            });
        });

        // Quality slider
        const slider = this.overlay.querySelector('.export-quality-slider');
        const qval = this.overlay.querySelector('.export-quality-value');
        slider.addEventListener('input', () => {
            qval.textContent = slider.value;
            this._scheduleEstimate();
        });

        // Scale mode radios
        this.overlay.querySelectorAll('[name="scale-mode"]').forEach(radio => {
            radio.addEventListener('change', () => {
                this._updateScaleSubInputs();
                this._scheduleEstimate();
            });
        });

        // Scale value inputs
        this.overlay.querySelector('.export-percent-val').addEventListener('input', () => this._scheduleEstimate());
        this.overlay.querySelector('.export-maxdim-val').addEventListener('input', () => this._scheduleEstimate());
        this.overlay.querySelectorAll('[name="max-dim-axis"]').forEach(r => r.addEventListener('change', () => this._scheduleEstimate()));

        // Estimate method toggle
        this.overlay.querySelectorAll('[data-estimate]').forEach(btn => {
            btn.addEventListener('click', () => {
                this.overlay.querySelectorAll('[data-estimate]').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                this._refreshEstimates();
            });
        });

        // Abort exact estimation
        this.overlay.querySelector('.export-estimate-abort').addEventListener('click', () => {
            if (this._estimateAbort) {
                this._estimateAbort.abort();
                this._estimateAbort = null;
            }
        });

        // Export button
        this.overlay.querySelector('#export-confirm-btn').addEventListener('click', () => this._doExport());

        // Build initial file list
        this._buildFileList();
    }

    _buildFileList() {
        const list = this.overlay.querySelector('.export-file-list');
        list.innerHTML = this._files.map(f => {
            const name = f.split('/').pop();
            return `<div class="export-file-row" data-file="${f}">
                <span class="export-file-name">${name}</span>
                <span class="export-file-dims">—</span>
                <span class="export-file-size"><span class="export-file-orig">—</span> <span class="export-file-arrow">→</span> <span class="export-file-out">—</span></span>
            </div>`;
        }).join('');
    }

    _updateScaleSubInputs() {
        const mode = this._getScaleMode();
        const show = (cls, visible) => {
            const el = this.overlay.querySelector(cls);
            if (el) el.style.display = visible ? '' : 'none';
        };
        show('.export-percent-input', mode === 'percent');
        show('.export-maxdim-input', mode === 'max_dim');
    }

    _getFormat() {
        const active = this.overlay.querySelector('[data-format].active');
        return active ? active.dataset.format : 'jpeg';
    }

    _getQuality() {
        return parseInt(this.overlay.querySelector('.export-quality-slider').value) || 85;
    }

    _getScaleMode() {
        const checked = this.overlay.querySelector('[name="scale-mode"]:checked');
        return checked ? checked.value : 'none';
    }

    _getScaleOptions() {
        const mode = this._getScaleMode();
        const opts = { mode };
        if (mode === 'percent') {
            opts.percent = parseFloat(this.overlay.querySelector('.export-percent-val').value) || 50;
        } else if (mode === 'max_dim') {
            const axis = this.overlay.querySelector('[name="max-dim-axis"]:checked');
            opts.maxDimension = axis ? axis.value : 'width';
            opts.maxValue = parseInt(this.overlay.querySelector('.export-maxdim-val').value) || 0;
        }
        return opts;
    }

    _getExifMode() {
        const checked = this.overlay.querySelector('[name="exif-mode"]:checked');
        return checked ? checked.value : 'strip';
    }

    _getEstimateMethod() {
        const active = this.overlay.querySelector('[data-estimate].active');
        return active ? active.dataset.estimate : 'heuristic';
    }

    _getOutputMode() {
        if (this._serverRole) return 'zip';
        const checked = this.overlay.querySelector('[name="output-mode"]:checked');
        return checked ? checked.value : 'folder';
    }

    _getDestination() {
        const inp = this.overlay.querySelector('.export-destination-input');
        return inp ? inp.value.trim() : '';
    }

    _scheduleEstimate() {
        if (this._estimateTimer) clearTimeout(this._estimateTimer);
        this._estimateTimer = setTimeout(() => this._refreshEstimates(), 400);
    }

    async _refreshEstimates() {
        if (!this.overlay) return;

        // Cancel any running exact estimation
        if (this._estimateAbort) {
            this._estimateAbort.abort();
            this._estimateAbort = null;
        }

        const basePayload = {
            format: this._getFormat(),
            quality: this._getQuality(),
            scale: this._getScaleOptions(),
        };
        const method = this._getEstimateMethod();
        const totalInEl = this.overlay.querySelector('.export-total-in');
        const totalOutEl = this.overlay.querySelector('.export-total-out');

        totalInEl.textContent = '…';
        totalOutEl.textContent = '…';

        if (method === 'heuristic') {
            this._setProgress(false, 0, 0, '', false);
            try {
                const resp = await API.exportEstimate({ ...basePayload, files: this._files, method: 'heuristic' });
                if (!this.overlay) return;
                this._applyEstimates(resp.estimates, totalInEl, totalOutEl);
            } catch {
                if (this.overlay) { totalInEl.textContent = '—'; totalOutEl.textContent = '—'; }
            }
            return;
        }

        // Exact mode: encode per file with progress
        const abortCtrl = new AbortController();
        this._estimateAbort = abortCtrl;
        this._setProgress(true, 0, this._files.length, 'Calculating exact sizes…', true);

        let totalIn = 0, totalOut = 0, done = 0;

        for (const file of this._files) {
            if (abortCtrl.signal.aborted || !this.overlay) break;

            try {
                const resp = await API.exportEstimate(
                    { ...basePayload, files: [file], method: 'encode' },
                    abortCtrl.signal,
                );
                if (!this.overlay) break;

                const est = resp.estimates[0];
                if (est) {
                    const row = this.overlay.querySelector(`[data-file="${CSS.escape(file)}"]`);
                    if (row) {
                        if (est.error) {
                            _applyRowError(row, est);
                        } else {
                            row.querySelector('.export-file-orig').textContent = est.inputBytes ? _fmtBytes(est.inputBytes) : '—';
                            row.querySelector('.export-file-out').textContent = est.outputBytes ? _fmtBytes(est.outputBytes) : '—';
                            _applyDims(row, est);
                        }
                    }
                    if (est.inputBytes) totalIn += est.inputBytes;
                    if (est.outputBytes) totalOut += est.outputBytes;
                }
            } catch (err) {
                if (err.name === 'AbortError') break;
                // Non-fatal: leave this file's row as-is
            }

            done++;
            if (this.overlay) {
                this._setProgress(true, done, this._files.length, 'Calculating exact sizes…', true);
                totalInEl.textContent = totalIn > 0 ? _fmtBytes(totalIn) : '—';
                totalOutEl.textContent = totalOut > 0 ? _fmtBytes(totalOut) : '—';
            }
        }

        if (this.overlay) {
            const aborted = abortCtrl.signal.aborted;
            this._setProgress(false);
            if (aborted) {
                // Keep partial results already shown; reset totals if nothing completed
                if (done === 0) { totalInEl.textContent = '—'; totalOutEl.textContent = '—'; }
            }
        }

        if (this._estimateAbort === abortCtrl) this._estimateAbort = null;
    }

    _applyEstimates(estimates, totalInEl, totalOutEl) {
        let totalIn = 0, totalOut = 0;
        estimates.forEach(est => {
            const row = this.overlay.querySelector(`[data-file="${CSS.escape(est.file)}"]`);
            if (!row) return;
            if (est.error) {
                _applyRowError(row, est);
            } else {
                row.querySelector('.export-file-orig').textContent = est.inputBytes ? _fmtBytes(est.inputBytes) : '—';
                row.querySelector('.export-file-out').textContent = est.outputBytes ? '~' + _fmtBytes(est.outputBytes) : '—';
                _applyDims(row, est);
                if (est.inputBytes) totalIn += est.inputBytes;
                if (est.outputBytes) totalOut += est.outputBytes;
            }
        });
        totalInEl.textContent = totalIn > 0 ? _fmtBytes(totalIn) : '—';
        totalOutEl.textContent = totalOut > 0 ? '~' + _fmtBytes(totalOut) : '—';
    }

    _setProgress(visible, done = 0, total = 0, label = '', showAbort = false) {
        const row = this.overlay.querySelector('.export-progress-row');
        const fill = this.overlay.querySelector('.export-progress-fill');
        const abortBtn = this.overlay.querySelector('.export-estimate-abort');
        if (!visible) {
            row.style.display = 'none';
            abortBtn.style.display = 'none';
            fill.classList.remove('export-progress-indeterminate');
            return;
        }
        row.style.display = '';
        abortBtn.style.display = showAbort ? '' : 'none';
        this.overlay.querySelector('.export-progress-text').textContent = label;
        if (total === 0) {
            fill.classList.add('export-progress-indeterminate');
            fill.style.width = '';
            this.overlay.querySelector('.export-progress-label').textContent = '';
        } else {
            fill.classList.remove('export-progress-indeterminate');
            fill.style.width = Math.round(done / total * 100) + '%';
            this.overlay.querySelector('.export-progress-label').textContent = `${done} of ${total}`;
        }
    }

    async _doExport() {
        const confirmBtn = this.overlay.querySelector('#export-confirm-btn');
        const cancelBtn = this.overlay.querySelector('#export-cancel-btn');
        const statusEl = this.overlay.querySelector('.export-status');

        confirmBtn.disabled = true;
        cancelBtn.disabled = true;
        statusEl.textContent = '';

        const basePayload = {
            format: this._getFormat(),
            quality: this._getQuality(),
            scale: this._getScaleOptions(),
            exifMode: this._getExifMode(),
        };

        const outputMode = this._getOutputMode();

        try {
            if (outputMode === 'zip' || this._serverRole) {
                // Stream SSE progress while the server builds the ZIP, then download.
                this._setProgress(true, 0, this._files.length, 'Exporting…', false);

                const resp = await fetch('/api/export/zip-stream', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ ...basePayload, files: this._files }),
                });
                if (!resp.ok) throw new Error(await resp.text());

                const reader = resp.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';
                let token = null;

                while (true) {
                    const { done, value } = await reader.read();
                    if (done) break;
                    buffer += decoder.decode(value, { stream: true });

                    // Parse SSE blocks (separated by blank lines)
                    const blocks = buffer.split('\n\n');
                    buffer = blocks.pop() ?? '';

                    for (const block of blocks) {
                        const dataLine = block.split('\n').find(l => l.startsWith('data: '));
                        if (!dataLine) continue;
                        try {
                            const evt = JSON.parse(dataLine.slice(6));
                            if (evt.complete) {
                                token = evt.token;
                            } else if (this.overlay) {
                                this._setProgress(true, evt.done, evt.total, evt.file || 'Exporting…', false);
                            }
                        } catch { /* malformed event, skip */ }
                    }
                }

                if (!token) throw new Error('Export stream ended without a download token');

                if (this.overlay) this._setProgress(true, this._files.length, this._files.length, 'Downloading…', false);
                const blob = await API.exportZipDownload(token);
                this._setProgress(false);
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'unterlumen-export.zip';
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                URL.revokeObjectURL(url);
                this.close();
            } else {
                const destination = this._getDestination();
                if (!destination) {
                    statusEl.textContent = 'Please enter a destination folder.';
                    confirmBtn.disabled = false;
                    cancelBtn.disabled = false;
                    return;
                }

                // Per-file loop so we can show progress
                let done = 0, failed = 0;
                this._setProgress(true, 0, this._files.length, 'Exporting…', false);

                for (const file of this._files) {
                    if (!this.overlay) break;
                    const filename = file.split('/').pop();
                    this._setProgress(true, done, this._files.length, filename, false);
                    const row = this.overlay.querySelector(`[data-file="${CSS.escape(file)}"]`);
                    try {
                        const result = await API.exportSave({ ...basePayload, files: [file], destination });
                        const r = result.results?.[0];
                        if (!r?.success) {
                            failed++;
                            if (row && r?.error) _applyRowError(row, { error: r.error });
                        }
                    } catch (err) {
                        failed++;
                        if (row) _applyRowError(row, { error: err.message });
                    }
                    done++;
                }

                if (!this.overlay) return;
                this._setProgress(false);
                if (failed === 0) {
                    this.close();
                } else {
                    statusEl.textContent = `${failed} file(s) failed.`;
                    confirmBtn.disabled = false;
                    cancelBtn.disabled = false;
                }
            }
        } catch (err) {
            if (this.overlay) {
                this._setProgress(false);
                statusEl.textContent = 'Export failed: ' + err.message;
                confirmBtn.disabled = false;
                cancelBtn.disabled = false;
            }
        }
    }
}

// Escape HTML special characters for use in attribute values.
function _escHtml(s) {
    return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// Format bytes to human-readable string.
function _fmtBytes(n) {
    if (n >= 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
    if (n >= 1024) return (n / 1024).toFixed(0) + ' KB';
    return n + ' B';
}

// Apply output dimensions and upscale warning to a file row.
function _applyDims(row, est) {
    const dimsEl = row.querySelector('.export-file-dims');
    if (!dimsEl) return;

    if (!est.width || !est.height) {
        dimsEl.textContent = '—';
        dimsEl.innerHTML = '—';
        return;
    }

    const upscaleW = est.origWidth  > 0 && est.width  > est.origWidth;
    const upscaleH = est.origHeight > 0 && est.height > est.origHeight;
    const upscale  = upscaleW || upscaleH;

    let warn = '';
    if (upscale) {
        const parts = [];
        if (upscaleW) parts.push(`width ${est.origWidth} → ${est.width} px`);
        if (upscaleH) parts.push(`height ${est.origHeight} → ${est.height} px`);
        const tooltip = `Upscaling detected: ${parts.join(', ')}. `
            + `Enlarging beyond the original resolution cannot recover detail that was never captured — `
            + `the result will appear softer or show artefacts.`;
        warn = ` <span class="export-upscale-warn" title="${tooltip}">!</span>`;
    }

    dimsEl.innerHTML = `${est.width}&thinsp;&times;&thinsp;${est.height}${warn}`;
}

// Apply error state to a file row: show error text in the dims column spanning
// cols 2–5, hide the orig/arrow/out cells so the message has room.
function _applyRowError(row, est) {
    const dimsEl  = row.querySelector('.export-file-dims');
    const origEl  = row.querySelector('.export-file-orig');
    const arrowEl = row.querySelector('.export-file-arrow');
    const outEl   = row.querySelector('.export-file-out');

    if (origEl)  origEl.style.display  = 'none';
    if (arrowEl) arrowEl.style.display = 'none';
    if (outEl)   outEl.style.display   = 'none';

    if (dimsEl) {
        dimsEl.style.gridColumn = '2 / 6';
        dimsEl.style.textAlign  = 'left';
        dimsEl.style.color      = '';
        const msg   = _shortError(est.error || 'unknown error');
        const title = _escHtml(est.error || '');
        dimsEl.innerHTML = `<span class="export-file-error" title="${title}">${_escHtml(msg)}</span>`;
    }
}

// Extract the last meaningful (non-empty, non-whitespace) line from an error
// string. ffmpeg errors are multiline; the last line is usually the most specific.
function _shortError(s) {
    const lines = (s || '').split('\n').map(l => l.trim()).filter(Boolean);
    return lines[lines.length - 1] || s;
}
