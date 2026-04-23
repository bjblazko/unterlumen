// Reusable progress dialog for multi-item operations

class ProgressDialog {
    constructor() {
        this.overlay = null;
        this._cancelled = false;
        this._results = [];
    }

    open(items, { verb, action, onComplete }) {
        this._cancelled = false;
        this._results = [];
        this._verb = verb;
        this._items = items;
        this._action = action;
        this._onComplete = onComplete;
        this._buildDOM();
        document.body.appendChild(this.overlay);
        this._run();
    }

    _buildDOM() {
        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay progress-overlay';

        this.overlay.innerHTML = `
            <div class="modal" style="max-width: 400px">
                <div class="modal-header">
                    <span class="modal-title">${this._verb} files</span>
                </div>
                <div class="modal-body">
                    <div class="progress-status">${this._verb} 0 of ${this._items.length} files...</div>
                    <div class="progress-bar-track"><div class="progress-bar-fill"></div></div>
                    <div class="progress-detail"></div>
                </div>
                <div class="modal-footer">
                    <button class="btn" id="progress-cancel">Cancel</button>
                </div>
            </div>`;

        this.overlay.querySelector('#progress-cancel').addEventListener('click', () => {
            this._cancelled = true;
        });

        // Block escape and overlay clicks during operation
        this.overlay.addEventListener('click', (e) => {
            e.stopPropagation();
        });
        this._onKeyDown = (e) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                e.stopPropagation();
            }
        };
        document.addEventListener('keydown', this._onKeyDown, true);
    }

    async _run() {
        const total = this._items.length;
        const statusEl = this.overlay.querySelector('.progress-status');
        const fillEl = this.overlay.querySelector('.progress-bar-fill');
        const detailEl = this.overlay.querySelector('.progress-detail');
        let completed = 0;
        let errors = [];

        for (let i = 0; i < total; i++) {
            if (this._cancelled) break;

            const item = this._items[i];
            const displayName = typeof item === 'string' ? item.split('/').pop() : String(item);
            statusEl.textContent = `${this._verb} ${i + 1} of ${total} files...`;
            detailEl.textContent = displayName;
            fillEl.style.width = ((i / total) * 100) + '%';

            try {
                const result = await this._action(item);
                this._results.push(result);
                if (result && !result.success && result.error) {
                    errors.push({ file: item, error: result.error });
                }
            } catch (err) {
                this._results.push({ success: false, error: err.message });
                errors.push({ file: item, error: err.message });
            }
            completed++;
        }

        fillEl.style.width = this._cancelled ? ((completed / total) * 100) + '%' : '100%';
        detailEl.textContent = '';

        const pastTense = this._verb.endsWith('ing') ? this._verb.slice(0, -3) + 'ed' : this._verb + 'd';

        if (this._cancelled) {
            statusEl.textContent = `Cancelled. ${completed} of ${total} files ${pastTense.toLowerCase()}.`;
        } else if (errors.length > 0) {
            statusEl.textContent = `${pastTense} ${completed - errors.length} of ${total} files. ${errors.length} error${errors.length !== 1 ? 's' : ''}.`;
            this._showErrors(errors);
        } else {
            statusEl.textContent = `${pastTense} ${completed} of ${total} files.`;
        }

        // Replace Cancel with OK
        const footer = this.overlay.querySelector('.modal-footer');
        footer.innerHTML = '<button class="btn btn-accent" id="progress-ok">OK</button>';
        footer.querySelector('#progress-ok').addEventListener('click', () => {
            this._close();
            if (this._onComplete) this._onComplete(this._results);
        });
    }

    _showErrors(errors) {
        const detailEl = this.overlay.querySelector('.progress-detail');
        const maxShow = 5;
        const shown = errors.slice(0, maxShow);
        const lines = shown.map(e => {
            const name = typeof e.file === 'string' ? e.file.split('/').pop() : String(e.file);
            return `${name}: ${e.error}`;
        });
        if (errors.length > maxShow) {
            lines.push(`...and ${errors.length - maxShow} more`);
        }
        detailEl.innerHTML = '<div class="progress-errors">' + lines.map(l =>
            '<div class="progress-error-line">' + l.replace(/</g, '&lt;') + '</div>'
        ).join('') + '</div>';
    }

    _close() {
        document.removeEventListener('keydown', this._onKeyDown, true);
        if (this.overlay) {
            this.overlay.remove();
            this.overlay = null;
        }
    }
}
