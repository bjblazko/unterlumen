// FolderPicker — one dialog for choosing a folder, wherever a folder is asked
// for (Organize's targets, a destination's output folder, an export folder).
//
// It looks in two places, because that is how the photos are organised: the
// filesystem, and the libraries you have indexed. The libraries are one click
// away instead of six levels of /Volumes/nas/Timo/Bilder down.
//
// Usage: const path = await new FolderPicker().open(startPath, { title });
//        Resolves with the chosen path relative to the browse root, or null.
//
// The setup chooses the shared folder anywhere, so it walks the whole disk
// instead: { disk: true } lists folders through /api/setup/dirs, whose paths
// are absolute without the leading slash, and has no Libraries source.
//
// In the installed app on its own computer the system's folder dialog opens
// first (/api/folder-dialog), because it knows the NAS shares and external
// disks. The installed app's boundary is the whole disk (ADR-0047), so a
// folder outside it does not happen there; were it to, the dialog here opens
// instead and says why. Anywhere else — a phone, the container — or
// when the system dialog fails, this dialog is the one.

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

    async open(startPath = '', options = {}) {
        if (!App.config?.folderDialog) return this._openHere(startPath, options);
        let abs;
        try {
            abs = await API.folderDialog(options.title || 'Choose folder');
        } catch {
            return this._openHere(startPath, options);
        }
        if (abs === null) return null;
        const path = abs.replace(/\\/g, '/');
        if (options.disk) return path.replace(/^\//, '');
        const rel = absPathRelativeToBoundary(path, App.config.boundary);
        if (rel !== null) return rel;
        return this._openHere(startPath, {
            ...options,
            notice: `${path} is outside ${App.config.boundary}, the folder this installation serves. Choose a folder in it here.`,
        });
    }

    _openHere(startPath, { title = 'Choose folder', disk = false, home = null, notice = '' } = {}) {
        return new Promise((resolve) => {
            this._notice = notice;
            this._resolve = resolve;
            this._currentPath = startPath;
            this._disk = disk;
            this._home = home;
            if (disk) this._source = 'fs';
            this._build(title);
            document.addEventListener('keydown', this._onKeyDown);
            this._showSource(this._source, startPath);
        });
    }

    _close(result) {
        this._dialog?.close(result);
    }

    // Escape and the scrim are the dialog's job; this settles the promise
    // whichever way the dialog was closed.
    _settle(result) {
        this._overlay = null;
        document.removeEventListener('keydown', this._onKeyDown);
        if (this._resolve) {
            this._resolve(result ?? null);
            this._resolve = null;
        }
    }

    // Enter takes the folder that the footer names, the way a file dialog
    // does — but not while the focus is on a row, where Enter opens it.
    _onKeyDown(e) {
        if (e.key === 'Enter' && this._overlay && !e.target.closest('.fp-body')) {
            e.preventDefault();
            this._close(this._currentPath);
        }
    }

    _build(title) {
        this._dialog = new Dialog({
            title,
            size: 'md',
            className: 'fp-dialog',
            body: `
                <div class="seg fp-sources" role="group" aria-label="Where to look"${this._disk ? ' hidden' : ''}>
                    <button data-source="fs">Filesystem</button>
                    <button data-source="libs">Libraries</button>
                </div>
                ${this._notice ? `<p class="fp-msg fp-msg-error fp-notice">${escapeHtml(this._notice)}</p>` : ''}
                <div class="fp-crumbs" aria-label="Path"></div>
                <div class="fp-body"></div>`,
            actions: [
                { label: 'Cancel', id: 'fp-cancel', onClick: () => this._close(null) },
                { label: 'Select', kind: 'primary', id: 'fp-select', onClick: () => this._close(this._currentPath) },
            ],
            onClose: (result) => this._settle(result),
        });
        this._overlay = this._dialog.open();

        this._overlay.querySelector('.fp-sources').addEventListener('click', (e) => {
            const btn = e.target.closest('[data-source]');
            if (btn) this._showSource(btn.dataset.source);
        });
    }

    _showSource(source, startPath = null) {
        this._source = source;
        if (!this._disk) writeSource(source);
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
        Activity.in(body, 'Reading the folder…');

        let data;
        try {
            const params = new URLSearchParams({ path: relPath || '' });
            const resp = await fetch(`${this._disk ? '/api/setup/dirs' : '/api/browse/dirs'}?${params}`);
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
        const home = this._home ?? App.config?.homePath ?? App.config?.startPath ?? '';
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
        Activity.in(body, 'Reading the libraries…');

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
        this._dialog.setNote(path ? shortenPath(path, 3) : '/ (the browse root)');
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
