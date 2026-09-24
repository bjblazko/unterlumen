// FolderPicker — one dialog for choosing a folder, wherever a folder is asked
// for (Organize's targets, a destination's output folder, an export folder).
//
// It looks in two places, because that is how the photos are organised: the
// filesystem, and the libraries you have indexed. The libraries are one click
// away instead of six levels of /Volumes/nas/Timo/Bilder down.
//
// Usage: const path = await new FolderPicker().open(startPath, { title });
//        Resolves with the chosen path relative to the browse root, or null.

const FOLDER_PICKER_SOURCE_KEY = 'folderPicker.source';

class FolderPicker {
    constructor() {
        this._overlay = null;
        this._currentPath = '';
        this._resolve = null;
        this._source = readSource();
        this._libs = null;
        this._onKeyDown = this._onKeyDown.bind(this);
    }

    open(startPath = '', { title = 'Choose folder' } = {}) {
        return new Promise((resolve) => {
            this._resolve = resolve;
            this._currentPath = startPath;
            this._build(title);
            document.body.appendChild(this._overlay);
            document.addEventListener('keydown', this._onKeyDown);
            this._showSource(this._source, startPath);
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
        if (e.key === 'Escape') { e.preventDefault(); this._close(null); }
        // Enter takes the folder that the footer names, the way a file dialog
        // does — but not while the focus is on a row, where Enter opens it.
        if (e.key === 'Enter' && !e.target.closest('.fp-body')) {
            e.preventDefault();
            this._close(this._currentPath);
        }
    }

    _build(title) {
        this._overlay = document.createElement('div');
        this._overlay.className = 'modal-overlay';
        this._overlay.innerHTML = `
            <div class="modal fp-modal" role="dialog" aria-label="${escapeHtml(title)}">
                <div class="modal-header">
                    <span class="modal-title">${escapeHtml(title)}</span>
                    <div class="seg fp-sources" role="group" aria-label="Where to look">
                        <button data-source="fs">Filesystem</button>
                        <button data-source="libs">Libraries</button>
                    </div>
                </div>
                <div class="fp-crumbs" aria-label="Path"></div>
                <div class="modal-body fp-body"></div>
                <div class="modal-footer">
                    <span class="fp-selected mono"></span>
                    <button class="btn" id="fp-cancel">Cancel</button>
                    <button class="btn btn-accent" id="fp-select">Select</button>
                </div>
            </div>`;

        // Clicking the scrim is the same as Cancel; there is no second close
        // button doing what Cancel already does.
        this._overlay.addEventListener('click', e => { if (e.target === this._overlay) this._close(null); });
        this._overlay.querySelector('#fp-cancel').addEventListener('click', () => this._close(null));
        this._overlay.querySelector('#fp-select').addEventListener('click', () => this._close(this._currentPath));
        this._overlay.querySelector('.fp-sources').addEventListener('click', (e) => {
            const btn = e.target.closest('[data-source]');
            if (btn) this._showSource(btn.dataset.source);
        });
    }

    _showSource(source, startPath = null) {
        this._source = source;
        writeSource(source);
        for (const btn of this._overlay.querySelectorAll('[data-source]')) {
            btn.setAttribute('aria-pressed', String(btn.dataset.source === source));
        }
        this._overlay.querySelector('.fp-crumbs').hidden = source !== 'fs';
        if (source === 'fs') this._loadDir(startPath ?? this._currentPath);
        else this._loadLibraries();
    }

    /* --- Filesystem --- */

    async _loadDir(relPath) {
        const body = this._overlay.querySelector('.fp-body');
        body.innerHTML = '<div class="fp-msg">Loading…</div>';

        let data;
        try {
            const params = new URLSearchParams({ path: relPath || '' });
            const resp = await fetch(`/api/browse/dirs?${params}`);
            if (!resp.ok) throw new Error(await resp.text());
            data = await resp.json();
        } catch (err) {
            body.innerHTML = `<div class="fp-msg fp-msg-error">Could not read this folder: ${escapeHtml(err.message)}</div>`;
            return;
        }

        const { path, parent, dirs } = data;
        this._currentPath = path;
        this._renderCrumbs(path, parent);
        this._renderSelected(path);

        const list = dirs || [];
        body.innerHTML = list.length
            ? list.map(d => {
                const childPath = path ? `${path}/${d.name}` : d.name;
                return `<button class="fp-row" data-path="${escapeHtml(childPath)}">
                    ${FP_FOLDER_ICON}<span class="fp-row-name">${escapeHtml(d.name)}</span>
                </button>`;
            }).join('')
            : '<div class="fp-msg">No folders in here. "Select" takes this one.</div>';

        body.querySelectorAll('[data-path]').forEach(btn => {
            btn.addEventListener('click', () => this._loadDir(btn.dataset.path));
        });
    }

    // The path is the way back up: every part of it is a button, so you do not
    // climb out of a deep folder one ".." at a time.
    _renderCrumbs(path, parent) {
        const el = this._overlay.querySelector('.fp-crumbs');
        const parts = (path || '').split('/').filter(Boolean);
        const crumbs = ['<button class="fp-crumb" data-crumb="">Root</button>'];
        parts.forEach((part, i) => {
            const upto = parts.slice(0, i + 1).join('/');
            crumbs.push('<span class="fp-crumb-sep">/</span>');
            crumbs.push(i === parts.length - 1
                ? `<span class="fp-crumb-here">${escapeHtml(part)}</span>`
                : `<button class="fp-crumb" data-crumb="${escapeHtml(upto)}">${escapeHtml(part)}</button>`);
        });
        // Home is the folder the server was started with (or the OS home), the
        // same place the Home button goes to while browsing.
        const home = App.config?.homePath ?? App.config?.startPath ?? '';
        const homeBtn = `<button class="btn btn-sm fp-home" data-crumb="${escapeHtml(home)}"${path === home ? ' disabled' : ''}>Home</button>`;
        const up = (parent !== null && parent !== undefined)
            ? `<button class="btn btn-sm fp-up" data-crumb="${escapeHtml(String(parent))}">Up</button>`
            : '';
        el.innerHTML = `${homeBtn}${up}<nav class="fp-crumb-list">${crumbs.join('')}</nav>`;
        el.querySelectorAll('[data-crumb]').forEach(btn => {
            btn.addEventListener('click', () => this._loadDir(btn.dataset.crumb));
        });
    }

    /* --- Libraries --- */

    async _loadLibraries() {
        const body = this._overlay.querySelector('.fp-body');
        body.innerHTML = '<div class="fp-msg">Loading…</div>';

        if (!this._libs) {
            try {
                this._libs = await LibraryAPI.list();
            } catch (err) {
                body.innerHTML = `<div class="fp-msg fp-msg-error">Could not load the libraries: ${escapeHtml(err.message)}</div>`;
                return;
            }
        }
        if (!this._libs.length) {
            body.innerHTML = '<div class="fp-msg">No libraries yet. Make one from a folder in Libraries.</div>';
            return;
        }

        const boundary = App.config?.boundary;
        body.innerHTML = this._libs.map(lib => {
            const source = lib.sourcePath.replace(/\/$/, '');
            const rel = absPathRelativeToBoundary(source, boundary);
            // A library outside the browse root cannot be opened from here, and
            // says so instead of failing when it is clicked.
            if (rel === null) {
                return `<span class="fp-row fp-row--out">
                    ${FP_FOLDER_ICON}<span class="fp-row-name">${escapeHtml(lib.name)}</span>
                    <span class="fp-row-note">outside the server's browse root</span>
                </span>`;
            }
            return `<button class="fp-row" data-lib="${escapeHtml(rel)}">
                ${FP_FOLDER_ICON}<span class="fp-row-name">${escapeHtml(lib.name)}</span>
                <span class="fp-row-note mono">${escapeHtml(source)}</span>
            </button>`;
        }).join('');

        body.querySelectorAll('[data-lib]').forEach(btn => {
            // Opening a library lands in its folder, where you can go deeper or
            // simply press Select.
            btn.addEventListener('click', () => this._showSource('fs', btn.dataset.lib));
        });
        this._renderSelected(this._currentPath);
    }

    // The footer names what "Select" would return — shortened from the front,
    // because the folder's own name is the part that matters.
    _renderSelected(path) {
        const el = this._overlay.querySelector('.fp-selected');
        el.textContent = path ? shortenPath(path, 3) : '/ (the browse root)';
        el.title = path || '/';
    }
}

const FP_FOLDER_ICON = '<svg class="fp-row-icon" width="16" height="13" viewBox="0 0 18 14" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round" aria-hidden="true"><path d="M1 2.5v10h16v-8.5H8L6.5 2.5H1z"/></svg>';

function readSource() {
    try {
        return localStorage.getItem(FOLDER_PICKER_SOURCE_KEY) === 'libs' ? 'libs' : 'fs';
    } catch {
        return 'fs';
    }
}

function writeSource(source) {
    try {
        localStorage.setItem(FOLDER_PICKER_SOURCE_KEY, source);
    } catch { /* a browser that refuses storage still picks folders */ }
}
