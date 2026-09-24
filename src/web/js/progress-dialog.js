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
        this._run();
    }

    // Not dismissible: files are being moved behind it, and a stray Escape
    // in the middle of that would leave the person guessing what happened.
    _buildDOM() {
        this._dialog = new Dialog({
            title: `${this._verb} files`,
            size: 'sm',
            dismissible: false,
            className: 'progress-dialog',
            body: `
                <div class="progress-status">${this._verb} 0 of ${this._items.length} files…</div>
                <div class="progress-bar-track"><div class="progress-bar-fill"></div></div>
                <div class="progress-detail"></div>`,
            actions: [{ label: 'Cancel', onClick: () => { this._cancelled = true; } }],
        });
        this.overlay = this._dialog.open();
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

        // The run is over, so the one thing left to do is close the report.
        this._dialog.dismissible = true;
        this._dialog.setActions([{
            label: 'Close',
            kind: 'primary',
            onClick: () => {
                this._close();
                if (this._onComplete) this._onComplete(this._results);
            },
        }]);
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
        this._dialog?.close(null);
        this.overlay = null;
    }
}
