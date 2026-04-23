// Batch rename modal — pattern-based renaming with live preview

const BATCH_RENAME_TOKENS = [
    // Date tokens
    { token: '{YYYY}', label: 'Year', example: '2026', category: 'date' },
    { token: '{MM}', label: 'Month (zero-padded)', example: '03', category: 'date' },
    { token: '{DD}', label: 'Day (zero-padded)', example: '20', category: 'date' },
    { token: '{hh}', label: 'Hour (24h)', example: '14', category: 'date' },
    { token: '{mm}', label: 'Minute', example: '07', category: 'date' },
    { token: '{ss}', label: 'Second', example: '42', category: 'date' },
    // Camera tokens
    { token: '{make}', label: 'Camera manufacturer', example: 'FUJIFILM', category: 'camera' },
    { token: '{model}', label: 'Camera model', example: 'X-T50', category: 'camera' },
    { token: '{lens}', label: 'Lens model', example: 'XF23mmF1.4-R-LM-WR', category: 'camera' },
    { token: '{filmsim}', label: 'Film simulation (Fujifilm)', example: 'Classic-Chrome', category: 'camera' },
    // Exposure tokens
    { token: '{iso}', label: 'ISO sensitivity', example: '800', category: 'exposure' },
    { token: '{aperture}', label: 'Aperture (f-number)', example: 'f1.4', category: 'exposure' },
    { token: '{focal}', label: 'Focal length', example: '23mm', category: 'exposure' },
    { token: '{shutter}', label: 'Shutter speed', example: '1-125s', category: 'exposure' },
    // File tokens
    { token: '{original}', label: 'Original filename (no extension)', example: 'DSCF1234', category: 'file' },
    { token: '{seq}', label: 'Auto-increment counter (3 digits, or {seq:N} for N digits)', example: '001', category: 'file' },
];

// Token pattern for matching in input text — matches {word} and {seq:N}
const TOKEN_REGEX = /\{(?:YYYY|MM|DD|hh|mm|ss|make|model|lens|filmsim|iso|aperture|focal|shutter|original|seq(?::\d+)?)\}/g;

class BatchRenameModal {
    constructor() {
        this.overlay = null;
        this.files = [];
        this._onSuccess = null;
        this._debounceTimer = null;
        this._onKeyDown = (e) => {
            if (e.key === 'Escape') this.close();
        };
    }

    open(files, onSuccess = null) {
        this.files = files;
        this._onSuccess = onSuccess;
        this._buildDOM();
        document.body.appendChild(this.overlay);
        document.addEventListener('keydown', this._onKeyDown);
        const input = this.overlay.querySelector('.batch-rename-input');
        if (input) input.focus();
    }

    close() {
        document.removeEventListener('keydown', this._onKeyDown);
        if (this._debounceTimer) clearTimeout(this._debounceTimer);
        if (this.overlay) {
            this.overlay.remove();
            this.overlay = null;
        }
    }

    _buildDOM() {
        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay';
        this.overlay.addEventListener('click', (e) => {
            if (e.target === this.overlay) this.close();
        });

        const tokenButtons = BATCH_RENAME_TOKENS.map(t =>
            `<button class="btn btn-sm batch-rename-token batch-rename-cat-${t.category}" data-token="${t.token}" title="${t.label} — e.g. ${t.example}" draggable="true">${t.token}</button>`
        ).join('');

        this.overlay.innerHTML = `
            <div class="modal" style="max-width:560px">
                <div class="modal-header">
                    <span class="modal-title">Batch Rename</span>
                    <button class="info-collapse-btn modal-close-btn">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="batch-rename-pattern-section">
                        <label class="dropdown-label">Pattern</label>
                        <div class="batch-rename-input-wrap">
                            <div class="batch-rename-highlight" aria-hidden="true"></div>
                            <input type="text" class="batch-rename-input" value="{YYYY}-{MM}-{DD}_{original}" placeholder="{YYYY}_{original}">
                        </div>
                    </div>
                    <div class="batch-rename-tokens">${tokenButtons}</div>
                    <div class="batch-rename-preview">
                        <div class="batch-rename-preview-header">Preview (${this.files.length} file${this.files.length !== 1 ? 's' : ''})</div>
                        <div class="batch-rename-preview-list">
                            <div class="batch-rename-preview-empty">Enter a pattern to see preview</div>
                        </div>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn" id="batch-rename-cancel">Cancel</button>
                    <button class="btn btn-accent" id="batch-rename-apply" disabled>Rename</button>
                </div>
            </div>`;

        this.overlay.querySelector('.modal-close-btn').addEventListener('click', () => this.close());
        this.overlay.querySelector('#batch-rename-cancel').addEventListener('click', () => this.close());
        this.overlay.querySelector('#batch-rename-apply').addEventListener('click', () => this._execute());

        const input = this.overlay.querySelector('.batch-rename-input');
        input.addEventListener('input', () => {
            this._updateHighlight();
            this._schedulePreview();
        });
        input.addEventListener('scroll', () => this._syncScroll());

        // Token buttons: click to insert at cursor
        this.overlay.querySelectorAll('.batch-rename-token').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                this._insertToken(btn.dataset.token);
            });

            // Drag start — offset the ghost image above the cursor so the drop zone stays visible
            btn.addEventListener('dragstart', (e) => {
                e.dataTransfer.setData('text/plain', btn.dataset.token);
                e.dataTransfer.effectAllowed = 'copy';
                const ghost = btn.cloneNode(true);
                ghost.classList.add('batch-rename-drag-ghost');
                document.body.appendChild(ghost);
                e.dataTransfer.setDragImage(ghost, ghost.offsetWidth / 2, ghost.offsetHeight + 64);
                requestAnimationFrame(() => ghost.remove());
            });
        });

        // Track drag insertion position (-1 = not dragging)
        this._dragInsertPos = -1;

        // Drop on input
        input.addEventListener('dragover', (e) => {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'copy';
            input.classList.add('batch-rename-input-dragover');
            const pos = this._caretPosFromX(input, e.clientX);
            if (pos !== this._dragInsertPos) {
                this._dragInsertPos = pos;
                this._updateHighlight();
            }
        });
        input.addEventListener('dragleave', () => {
            input.classList.remove('batch-rename-input-dragover');
            this._dragInsertPos = -1;
            this._updateHighlight();
        });
        input.addEventListener('drop', (e) => {
            e.preventDefault();
            input.classList.remove('batch-rename-input-dragover');
            const insertPos = this._dragInsertPos;
            this._dragInsertPos = -1;
            const token = e.dataTransfer.getData('text/plain');
            if (!token) return;

            const pos = insertPos >= 0 ? insertPos : this._caretPosFromX(input, e.clientX);
            input.value = input.value.slice(0, pos) + token + input.value.slice(pos);
            input.focus();
            const newPos = pos + token.length;
            input.setSelectionRange(newPos, newPos);
            this._updateHighlight();
            this._schedulePreview();
        });

        // Initial highlight + preview
        this._updateHighlight();
        this._schedulePreview();
    }

    _insertToken(token) {
        const input = this.overlay.querySelector('.batch-rename-input');
        const pos = input.selectionStart ?? input.value.length;
        input.value = input.value.slice(0, pos) + token + input.value.slice(pos);
        input.focus();
        input.setSelectionRange(pos + token.length, pos + token.length);
        this._updateHighlight();
        this._schedulePreview();
    }

    _caretPosFromX(input, clientX) {
        // Use a hidden measuring span to find the character position closest to clientX
        const rect = input.getBoundingClientRect();
        const style = getComputedStyle(input);
        const paddingLeft = parseFloat(style.paddingLeft) || 0;
        const relX = clientX - rect.left - paddingLeft + input.scrollLeft;

        const span = document.createElement('span');
        span.style.cssText = `position:absolute;visibility:hidden;white-space:pre;font:${style.font};letter-spacing:${style.letterSpacing};`;
        document.body.appendChild(span);

        const text = input.value;
        let bestPos = text.length;
        for (let i = 0; i <= text.length; i++) {
            span.textContent = text.slice(0, i);
            if (span.offsetWidth >= relX) {
                // Check if previous position is closer
                if (i > 0) {
                    span.textContent = text.slice(0, i - 1);
                    const prevW = span.offsetWidth;
                    span.textContent = text.slice(0, i);
                    const curW = span.offsetWidth;
                    bestPos = (relX - prevW < curW - relX) ? i - 1 : i;
                } else {
                    bestPos = 0;
                }
                break;
            }
        }
        span.remove();
        return Math.max(0, Math.min(text.length, bestPos));
    }

    _updateHighlight() {
        if (!this.overlay) return;
        const input = this.overlay.querySelector('.batch-rename-input');
        const highlight = this.overlay.querySelector('.batch-rename-highlight');
        if (!input || !highlight) return;

        const text = input.value;
        const insertPos = this._dragInsertPos;

        // Build category lookup
        const catMap = {};
        for (const t of BATCH_RENAME_TOKENS) catMap[t.token] = t.category;

        // Build an array of segments: { start, end, category | null }
        const segments = [];
        let lastIndex = 0;
        let match;
        TOKEN_REGEX.lastIndex = 0;
        while ((match = TOKEN_REGEX.exec(text)) !== null) {
            if (match.index > lastIndex) {
                segments.push({ start: lastIndex, end: match.index, cat: null });
            }
            const tok = match[0];
            const baseTok = tok.startsWith('{seq') ? '{seq}' : tok;
            const cat = catMap[baseTok] || 'file';
            segments.push({ start: match.index, end: match.index + tok.length, cat });
            lastIndex = match.index + tok.length;
        }
        if (lastIndex < text.length) {
            segments.push({ start: lastIndex, end: text.length, cat: null });
        }

        const CARET = '<span class="batch-rename-drop-marker"></span>';

        let html = '';
        let caretInserted = false;
        for (const seg of segments) {
            // Insert caret marker if it falls before this segment
            if (insertPos >= 0 && !caretInserted && insertPos <= seg.start) {
                html += CARET;
                caretInserted = true;
            }

            const segText = text.slice(seg.start, seg.end);

            // Check if caret falls within this segment
            if (insertPos >= 0 && !caretInserted && insertPos > seg.start && insertPos < seg.end) {
                const before = text.slice(seg.start, insertPos);
                const after = text.slice(insertPos, seg.end);
                if (seg.cat) {
                    html += `<span class="batch-rename-hl-${seg.cat}">${this._esc(before)}</span>`;
                    html += CARET;
                    html += `<span class="batch-rename-hl-${seg.cat}">${this._esc(after)}</span>`;
                } else {
                    html += this._esc(before) + CARET + this._esc(after);
                }
                caretInserted = true;
                continue;
            }

            if (seg.cat) {
                html += `<span class="batch-rename-hl-${seg.cat}">${this._esc(segText)}</span>`;
            } else {
                html += this._esc(segText);
            }
        }

        // Caret at end of text
        if (insertPos >= 0 && !caretInserted) {
            html += CARET;
        }

        // Trailing space to keep height consistent
        highlight.innerHTML = html + '\u00a0';
        this._syncScroll();
    }

    _syncScroll() {
        if (!this.overlay) return;
        const input = this.overlay.querySelector('.batch-rename-input');
        const highlight = this.overlay.querySelector('.batch-rename-highlight');
        if (input && highlight) {
            highlight.scrollLeft = input.scrollLeft;
        }
    }

    _schedulePreview() {
        if (this._debounceTimer) clearTimeout(this._debounceTimer);
        this._debounceTimer = setTimeout(() => this._loadPreview(), 300);
    }

    async _loadPreview() {
        if (!this.overlay) return;
        const input = this.overlay.querySelector('.batch-rename-input');
        const pattern = input.value.trim();
        const list = this.overlay.querySelector('.batch-rename-preview-list');
        const applyBtn = this.overlay.querySelector('#batch-rename-apply');

        if (!pattern) {
            list.innerHTML = '<div class="batch-rename-preview-empty">Enter a pattern to see preview</div>';
            applyBtn.disabled = true;
            return;
        }

        list.innerHTML = '<div class="batch-rename-preview-loading"><div class="browse-spinner"></div></div>';
        applyBtn.disabled = true;

        try {
            const result = await API.batchRenamePreview(this.files, pattern);
            if (!this.overlay) return;

            const header = this.overlay.querySelector('.batch-rename-preview-header');
            const conflictNote = result.conflicts > 0 ? ` — ${result.conflicts} conflict${result.conflicts !== 1 ? 's' : ''} resolved` : '';
            header.textContent = `Preview (${this.files.length} file${this.files.length !== 1 ? 's' : ''})${conflictNote}`;

            let hasErrors = false;
            const rows = result.mappings.map(m => {
                const oldName = m.file.split('/').pop();
                if (m.error) {
                    hasErrors = true;
                    return `<div class="batch-rename-row batch-rename-row-error">
                        <span class="batch-rename-old">${this._esc(oldName)}</span>
                        <span class="batch-rename-error">${this._esc(m.error)}</span>
                    </div>`;
                }
                const conflictClass = result.conflicts > 0 && m.newName.match(/_\d{3}\.[^.]+$/) ? ' batch-rename-row-conflict' : '';
                return `<div class="batch-rename-row${conflictClass}">
                    <span class="batch-rename-old">${this._esc(oldName)}</span>
                    <span class="batch-rename-arrow">&rarr;</span>
                    <span class="batch-rename-new">${this._esc(m.newName)}</span>
                </div>`;
            });

            list.innerHTML = rows.join('');
            applyBtn.disabled = hasErrors;
        } catch (err) {
            if (!this.overlay) return;
            list.innerHTML = `<div class="batch-rename-preview-empty" style="color:#c0392b">${this._esc(err.message)}</div>`;
            applyBtn.disabled = true;
        }
    }

    async _execute() {
        if (!this.overlay) return;
        const input = this.overlay.querySelector('.batch-rename-input');
        const pattern = input.value.trim();
        const total = this.files.length;

        // Replace modal body with progress UI
        const body = this.overlay.querySelector('.modal-body');
        body.innerHTML = `
            <div class="progress-status">Renaming 0 of ${total} files...</div>
            <div class="progress-bar-track"><div class="progress-bar-fill" style="width:0"></div></div>
            <div class="progress-detail"></div>`;
        const footer = this.overlay.querySelector('.modal-footer');
        footer.innerHTML = '';

        const statusEl = body.querySelector('.progress-status');
        const fillEl = body.querySelector('.progress-bar-fill');
        const detailEl = body.querySelector('.progress-detail');

        // Animate indeterminate progress while waiting
        let progress = 0;
        const tick = setInterval(() => {
            progress = Math.min(progress + (90 - progress) * 0.08, 90);
            fillEl.style.width = progress + '%';
            statusEl.textContent = `Renaming ${Math.round(progress / 100 * total)} of ${total} files...`;
        }, 200);

        try {
            const result = await API.batchRenameExecute(this.files, pattern);
            clearInterval(tick);
            fillEl.style.width = '100%';

            const successes = result.results.filter(r => r.success).length;
            const failures = result.results.filter(r => !r.success);
            statusEl.textContent = `Renamed ${successes} of ${total} file${total !== 1 ? 's' : ''}.`;
            detailEl.textContent = '';

            if (failures.length > 0) {
                statusEl.textContent += ` ${failures.length} failed.`;
                const maxShow = 5;
                const shown = failures.slice(0, maxShow);
                const lines = shown.map(f => `${f.file.split('/').pop()}: ${f.error}`);
                if (failures.length > maxShow) lines.push(`...and ${failures.length - maxShow} more`);
                detailEl.innerHTML = '<div class="progress-errors">' +
                    lines.map(l => '<div class="progress-error-line">' + this._esc(l) + '</div>').join('') +
                    '</div>';
            }

            if (successes > 0 && this._onSuccess) this._onSuccess();

            footer.innerHTML = '<button class="btn btn-accent" id="batch-rename-done">OK</button>';
            footer.querySelector('#batch-rename-done').addEventListener('click', () => this.close());

            if (failures.length === 0) {
                setTimeout(() => this.close(), 1200);
            }
        } catch (err) {
            clearInterval(tick);
            fillEl.style.width = '100%';
            fillEl.style.background = '#c0392b';
            statusEl.textContent = 'Rename failed.';
            detailEl.innerHTML = `<div class="progress-errors"><div class="progress-error-line">${this._esc(err.message)}</div></div>`;
            footer.innerHTML = '<button class="btn" id="batch-rename-done">OK</button>';
            footer.querySelector('#batch-rename-done').addEventListener('click', () => this.close());
        }
    }

    _esc(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
}
