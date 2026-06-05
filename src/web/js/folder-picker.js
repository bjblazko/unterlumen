// FolderPicker — in-browser directory browser modal.
// Opens a modal-overlay, navigates via /api/browse/dirs, and resolves with the
// selected relative path (or null if cancelled).
//
// Usage: const path = await new FolderPicker().open(startPath);

class FolderPicker {
    constructor() {
        this._overlay = null;
        this._currentPath = '';
        this._resolve = null;
        this._onKeyDown = this._onKeyDown.bind(this);
    }

    open(startPath = '') {
        return new Promise((resolve) => {
            this._resolve = resolve;
            this._currentPath = startPath;
            this._build();
            document.body.appendChild(this._overlay);
            document.addEventListener('keydown', this._onKeyDown);
            this._loadDir(startPath);
        });
    }

    _close(result) {
        this._overlay?.remove();
        this._overlay = null;
        document.removeEventListener('keydown', this._onKeyDown);
        if (this._resolve) {
            this._resolve(result);
            this._resolve = null;
        }
    }

    _onKeyDown(e) {
        if (e.key === 'Escape') this._close(null);
    }

    _build() {
        this._overlay = document.createElement('div');
        this._overlay.className = 'modal-overlay';
        this._overlay.innerHTML = `
            <div class="modal fp-modal">
                <div class="modal-header">
                    <span class="modal-title">Choose Folder</span>
                    <button class="info-collapse-btn modal-close-btn" title="Close">&times;</button>
                </div>
                <div class="fp-breadcrumb"></div>
                <div class="modal-body fp-body"></div>
                <div class="modal-footer">
                    <span class="fp-selected"></span>
                    <button class="btn" id="fp-cancel">Cancel</button>
                    <button class="btn btn-accent" id="fp-select">Select</button>
                </div>
            </div>`;

        this._overlay.addEventListener('click', e => { if (e.target === this._overlay) this._close(null); });
        this._overlay.querySelector('.modal-close-btn').addEventListener('click', () => this._close(null));
        this._overlay.querySelector('#fp-cancel').addEventListener('click', () => this._close(null));
        this._overlay.querySelector('#fp-select').addEventListener('click', () => this._close(this._currentPath));
    }

    async _loadDir(relPath) {
        const body = this._overlay.querySelector('.fp-body');
        const breadcrumb = this._overlay.querySelector('.fp-breadcrumb');
        const selected = this._overlay.querySelector('.fp-selected');

        body.innerHTML = '<div class="fp-msg">Loading…</div>';

        try {
            const params = new URLSearchParams({ path: relPath });
            const resp = await fetch(`/api/browse/dirs?${params}`);
            if (!resp.ok) throw new Error(await resp.text());
            const { path, parent, dirs } = await resp.json();

            this._currentPath = path;
            selected.textContent = path || '/ (root)';
            breadcrumb.textContent = path ? '/ ' + path.split('/').join(' / ') : '/';

            const rows = [];
            if (parent !== null && parent !== undefined) {
                rows.push(`<button class="fp-dir fp-up" data-path="${escapeHtml(String(parent))}">↑  ..</button>`);
            }
            const dirList = dirs || [];
            if (dirList.length === 0) {
                rows.push('<div class="fp-msg">No subdirectories here.</div>');
            } else {
                for (const d of dirList) {
                    const childPath = path ? path + '/' + d.name : d.name;
                    rows.push(`<button class="fp-dir" data-path="${escapeHtml(childPath)}">${escapeHtml(d.name)}</button>`);
                }
            }

            body.innerHTML = rows.join('');
            body.querySelectorAll('[data-path]').forEach(btn => {
                btn.addEventListener('click', () => this._loadDir(btn.dataset.path));
            });
        } catch (err) {
            body.innerHTML = `<div class="fp-msg fp-msg-error">Failed to load: ${escapeHtml(err.message)}</div>`;
        }
    }
}
