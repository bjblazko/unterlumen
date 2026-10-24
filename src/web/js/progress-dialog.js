// Reusable progress dialog for multi-item operations

class ProgressDialog {
    constructor() {
        this.overlay = null;
        this._cancelled = false;
        this._results = [];
    }

    // title says what runs ("Deleting files"), done how its end reads
    // ("Deleted"), both already in the interface's language.
    open(items, { title, done, action, onComplete }) {
        this._cancelled = false;
        this._results = [];
        this._title = title;
        this._doneWord = done;
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
            title: this._title,
            size: 'sm',
            dismissible: false,
            className: 'progress-dialog',
            body: '<div class="progress-dialog-activity"></div>',
            actions: [{ label: t('Cancel'), onClick: () => { this._cancelled = true; } }],
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
        const files = countText(total, 'file');
        if (this._cancelled) {
            activity.done(t('Cancelled. {done}: {n} of {files}.', { done: this._doneWord, n: formatCount(completed), files }));
        } else if (errors.length > 0) {
            activity.fail(t('{done}: {n} of {files}. {failed} did not work.', { done: this._doneWord, n: formatCount(completed - errors.length), files, failed: formatCount(errors.length) }));
            host.appendChild(Activity.errorList(errors.map(e => {
                const name = typeof e.file === 'string' ? e.file.split('/').pop() : String(e.file);
                return `${name}: ${e.error}`;
            })));
        } else {
            activity.done(t('{done}: {n} of {files}.', { done: this._doneWord, n: formatCount(completed), files }));
        }

        // The run is over, so the one thing left to do is close the report.
        this._dialog.dismissible = true;
        this._dialog.setActions([{
            label: t('Close'),
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
