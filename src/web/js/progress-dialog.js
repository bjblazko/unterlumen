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
            body: '<div class="progress-dialog-activity"></div>',
            actions: [{ label: 'Cancel', onClick: () => { this._cancelled = true; } }],
        });
        this.overlay = this._dialog.open();
    }

    async _run() {
        const total = this._items.length;
        const host = this.overlay.querySelector('.progress-dialog-activity');
        const activity = Activity.in(host);
        let completed = 0;
        const errors = [];

        for (let i = 0; i < total; i++) {
            if (this._cancelled) break;

            const item = this._items[i];
            const displayName = typeof item === 'string' ? item.split('/').pop() : String(item);
            activity.count(i, total, { current: displayName });

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

        activity.count(completed, total);
        const pastTense = this._verb.endsWith('ing') ? this._verb.slice(0, -3) + 'ed' : this._verb + 'd';

        if (this._cancelled) {
            activity.done(`Cancelled. ${completed} of ${total} files ${pastTense.toLowerCase()}.`);
        } else if (errors.length > 0) {
            activity.fail(`${pastTense} ${completed - errors.length} of ${total} files. ${errors.length} could not be ${pastTense.toLowerCase()}.`);
            host.appendChild(Activity.errorList(errors.map(e => {
                const name = typeof e.file === 'string' ? e.file.split('/').pop() : String(e.file);
                return `${name}: ${e.error}`;
            })));
        } else {
            activity.done(`${pastTense} ${completed} of ${total} files.`);
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

    _close() {
        this._dialog?.close(null);
        this.overlay = null;
    }
}
