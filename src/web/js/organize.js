// organize.js — Organize: one source folder, many targets (ADR-0032).
//
// The dual pane treated source and destination as equals. Sorting an import
// is not symmetric: you look at one folder and aim at several. So the source
// is a full browse pane at Folders size, and the targets are a narrow list
// with a number key each. A move runs immediately, as it always did, and the
// line under it takes the last one back.

const ORGANIZE_TARGETS_KEY = 'organize.targets';

class OrganizePane {
    constructor(container, initialPath, options = {}) {
        this.container = container;
        this.initialPath = initialPath || '';
        this._preselect = options.preselectFiles || null;
        this.onImageClick = options.onImageClick || null;
        this.onToolInvoke = options.onToolInvoke || null;
        this.pane = null;
        this.targets = loadTargets();
        this.current = 0;
        this._sorted = 0;
        this._undo = null;
        this._result = null;
        this._resultKind = '';
    }

    init() {
        this.container.innerHTML = `
            <div class="organize">
                ${placeHeaderHTML({
                    title: t('Organize'),
                    meta: '<span class="organize-path mono" id="org-path"></span> <span class="organize-session num" id="org-session"></span>',
                    actions: '<button class="btn btn-sm" id="org-change">' + t('Change folder…') + '</button>',
                    lede: placeLede(t('Sort one folder into others: select photos and press a target’s number to move them there. The files move on disk.')),
                })}
                <div class="organize-body">
                    <div class="organize-source" id="org-source"></div>
                    <aside class="organize-targets" id="org-targets" aria-label="${t('Targets')}"></aside>
                </div>
                <div class="organize-result" id="org-result" hidden></div>
            </div>`;

        this.pane = new BrowsePane(this.container.querySelector('#org-source'), {
            onImageClick: (path) => { if (this.onImageClick) this.onImageClick(path, this.pane); },
            onSelectionChange: () => this._updateSelectionBar(),
            onFocusChange: () => this._updateSelectionBar(),
            onNavigate: () => this._renderPath(),
            placeHeader: false,
        });
        this.pane.view = 'justified';

        // Actions on a selection live in the bar, as they do everywhere else;
        // "Mark for deletion" is a target here, so it is not repeated in it.
        this._selectionBar = new SelectionBar(this.container.querySelector('.organize'), {
            actions: ['export', 'download', 'rename', 'location'],
            onAction: (action) => this._runSelectionAction(action),
        });

        this.container.querySelector('#org-change').addEventListener('click', () => this._changeFolder());
        this.container.querySelector('#org-targets').addEventListener('click', (e) => this._onTargetClick(e));

        if (this._preselect) this.pane.primePreselect(this._preselect);
        this._preselect = null;
        this.pane.load(this.initialPath);
        this._renderPath();
        this._renderTargets();
    }

    load(path, preselectNames = null) {
        if (preselectNames && preselectNames.length) this.pane.primePreselect(preselectNames);
        this._sorted = 0;
        this._undo = null;
        this._say('');
        this.pane.load(path);
        this._renderPath();
    }

    reload() {
        this.pane.load(this.pane.path);
    }

    /* --- Head --- */

    _renderPath() {
        const path = this.pane.path || '/';
        this.container.querySelector('#org-path').textContent = path;
        const session = this.container.querySelector('#org-session');
        session.textContent = this._sorted ? `${this._sorted} sorted in this session` : '';
    }

    async _changeFolder() {
        const picked = await new FolderPicker().open(this.pane.path, { title: t('Folder to sort') });
        if (picked === null) return;
        this.load(picked);
    }

    /* --- Targets --- */

    _renderTargets() {
        const el = this.container.querySelector('#org-targets');
        const rows = this.targets.map((target, i) => {
            const key = i < 9 ? String(i + 1) : '';
            const name = targetName(target.path);
            const where = shortenPath(target.path);
            return `
            <button class="org-target${target.missing ? ' org-target--missing' : ''}" data-target="${i}" aria-current="${i === this.current}">
                <span class="org-target-name">${escapeHtml(name)}</span>
                <span class="org-target-path mono" title="${escapeHtml(target.path)}">${escapeHtml(where)}</span>
                ${target.missing ? `<span class="org-target-note">${t('This folder is not there any more.')}</span>` : ''}
                ${target.moved ? `<span class="org-target-note num">${t('{n} in this session', { n: formatCount(target.moved) })}</span>` : ''}
                ${key ? `<span class="key">${key}</span>` : ''}
            </button>`;
        }).join('');

        const markKey = this.targets.length < 9 ? String(this.targets.length + 1) : '';
        el.innerHTML = `
            <p class="section-title">${t('Targets')}</p>
            ${rows}
            <button class="org-target org-target--mark" data-target="mark" aria-current="${this.current === 'mark'}">
                <span class="org-target-name">${t('Mark for deletion')}</span>
                <span class="org-target-path">${t('To the waste bin, not off the disk')}</span>
                ${markKey ? `<span class="key">${markKey}</span>` : ''}
            </button>
            <div class="org-target-actions">
                <button class="btn btn-sm" data-org="add">${t('Add target…')}</button>
            </div>
            <ul class="org-keys">
                <li>${t('{key} selects the photo in focus', { key: `<span class="key">${t('Space')}</span>` })}</li>
                <li>${t('{keys} move the selection to that target; hold {option} to copy', { keys: '<span class="key">1</span>–<span class="key">9</span>', option: '<span class="key">⌥</span>' })}</li>
                <li>${t('{key} undoes the last move', { key: '<span class="key">U</span>' })}</li>
            </ul>
            ${this.targets.some(x => x.missing) ? `<button class="btn btn-sm" data-org="forget">${t('Forget the missing folders')}</button>` : ''}`;
    }

    _onTargetClick(e) {
        const action = e.target.closest('[data-org]');
        if (action) {
            if (action.dataset.org === 'add') this._addTarget();
            else if (action.dataset.org === 'forget') this._forgetMissing();
            return;
        }
        const row = e.target.closest('[data-target]');
        if (!row) return;
        const raw = row.dataset.target;
        const index = raw === 'mark' ? 'mark' : Number(raw);
        this.sendTo(index, { copy: e.altKey });
    }

    async _addTarget() {
        const picked = await new FolderPicker().open(this.pane.path, { title: t('Folder to add as a target'), create: true });
        if (picked === null) return;
        if (this.targets.some(t => t.path === picked)) {
            this._say(t('{name} is already a target.', { name: targetName(picked) }));
            return;
        }
        this.targets.push({ path: picked });
        saveTargets(this.targets);
        this._renderTargets();
    }

    _forgetMissing() {
        this.targets = this.targets.filter(t => !t.missing);
        saveTargets(this.targets);
        if (typeof this.current === 'number' && this.current >= this.targets.length) this.current = 0;
        this._renderTargets();
    }

    /* --- Sending the selection --- */

    // index is a number (a folder target) or 'mark' (the waste bin).
    async sendTo(index, { copy = false } = {}) {
        if (index !== 'mark' && !this.targets[index]) return;
        // One move at a time: a second press while the first runs would send
        // the same photos again, and find them gone.
        if (this._moving) return;
        const files = this.pane.getActionableFiles();
        const dirs = [...this.pane.selectedDirs];
        if (!files.length && !dirs.length) {
            this.current = index;
            this._renderTargets();
            return;
        }
        this.current = index;

        if (index === 'mark') {
            this._mark(files, dirs);
            return;
        }
        if (dirs.length) {
            this._say(t('Folders can only go to "Mark for deletion"; photos go to any target.'), 'error');
            return;
        }
        this._moving = true;
        try {
            await this._moveFiles(index, files, copy);
        } finally {
            this._moving = false;
        }
    }

    _mark(files, dirs) {
        const targets = [...files, ...dirs];
        App.markForDeletion(targets, this.pane.entries, this.pane.path);
        this._sorted += targets.length;
        this.pane.selection.clear();
        this.pane.selectedDirs.clear();
        this.pane.render();
        this._undo = { kind: 'mark', paths: targets };
        this._say(t('{photos} marked for deletion.', { photos: countLabel(targets.length) }));
        this._renderTargets();
        this._renderPath();
        this._updateSelectionBar();
    }

    async _moveFiles(index, files, copy) {
        const target = this.targets[index];
        const name = targetName(target.path);
        this._working(copy ? t('Copying {photos} to {name}…', { photos: countLabel(files.length), name })
            : t('Moving {photos} to {name}…', { photos: countLabel(files.length), name }));
        let result;
        try {
            result = copy ? await API.copy(files, target.path) : await API.move(files, target.path);
        } catch (err) {
            this._say(t('Could not reach {name}: {err}', { name, err: err.message }), 'error');
            return;
        }
        const results = result.results || [];
        const failures = results.filter(r => !r.success).map(failureLine);
        const moved = results.filter(r => r.success).map(r => r.file);
        if (!moved.length) {
            target.missing = failures.length > 0;
            this._renderTargets();
            this._say(t('Nothing went to {name}.', { name }), 'error', failures);
            return;
        }
        this._recordMoved(target, moved, copy, name);
        const done = copy ? t('{photos} copied to {name}.', { photos: countLabel(moved.length), name })
            : t('{photos} moved to {name}.', { photos: countLabel(moved.length), name });
        this._say(failures.length ? `${done} ${t('{n} did not:', { n: formatCount(failures.length) })}` : done,
            failures.length ? 'error' : '', failures);
        this.pane.selection.clear();
        this.pane.load(this.pane.path);
        this._renderTargets();
        this._renderPath();
        if (result.libraryUpdated) App.reloadLibraryPane();
    }

    _recordMoved(target, moved, copy, name) {
        target.missing = false;
        target.moved = (target.moved || 0) + moved.length;
        this._sorted += moved.length;
        this._undo = copy ? null : {
            kind: 'move',
            paths: moved.map(f => `${target.path}/${f.split('/').pop()}`),
            back: this.pane.path,
            name,
        };
    }

    async undo() {
        const u = this._undo;
        if (!u || this._moving) return;
        this._undo = null;
        if (u.kind === 'mark') {
            App.wastebin.restore(new Set(u.paths));
            this._sorted = Math.max(0, this._sorted - u.paths.length);
            this.pane.render();
            this._say(t('{photos} back in the folder.', { photos: countLabel(u.paths.length) }));
            this._renderPath();
            return;
        }
        this._working(t('Moving {paths} back from {name}…', { paths: countLabel(u.paths.length), name: u.name }));
        this._moving = true;
        try {
            await API.move(u.paths, u.back);
            this._sorted = Math.max(0, this._sorted - u.paths.length);
            const target = this.targets.find(t => targetName(t.path) === u.name);
            if (target) target.moved = Math.max(0, (target.moved || 0) - u.paths.length);
            this.pane.load(this.pane.path);
            this._renderTargets();
            this._renderPath();
            this._say(t('{photos} moved back from {name}.', { photos: countLabel(u.paths.length), name: u.name }));
        } catch (err) {
            this._say(t('Could not move them back: {err}', { err: err.message }), 'error');
        } finally {
            this._moving = false;
        }
    }

    /* --- Result line --- */

    // details are lines under the message: what did not work, and why.
    _say(message, kind = '', details = []) {
        this._result = message;
        this._resultKind = kind;
        const el = this.container.querySelector('#org-result');
        if (!el) return;
        el.hidden = !message;
        el.className = `organize-result${kind === 'error' ? ' organize-result--problem' : ''}`;
        el.innerHTML = message
            ? `<span>${escapeHtml(message)}</span>${this._undo ? '<button class="btn btn-sm" id="org-undo">' + t('Undo ') + '<span class="key">U</span></button>' : ''}`
            : '';
        if (details.length) el.appendChild(Activity.errorList(details));
        el.querySelector('#org-undo')?.addEventListener('click', () => this.undo());
    }

    // While files move, the result line says so — and after a few seconds
    // for how long (ADR-0036).
    _working(text) {
        const el = this.container.querySelector('#org-result');
        if (!el) return;
        el.hidden = false;
        el.className = 'organize-result';
        el.innerHTML = '';
        Activity.in(el, text);
    }

    /* --- Selection --- */

    _updateSelectionBar() {
        const files = this.pane.selection.selected.size;
        const dirs = this.pane.selectedDirs.size;
        this._selectionBar.update(files + dirs, { onlyFolders: files === 0 && dirs > 0 });
    }

    _runSelectionAction(action) {
        if (action === 'download') {
            Originals.download(this.pane.getSelectedFiles(), this.pane, { dirs: [...this.pane.selectedDirs] });
            return;
        }
        const files = this.pane.getActionableFiles();
        if (!files.length) return;
        const tool = { export: 'export', rename: 'batch-rename', location: 'set-location' }[action];
        if (tool && this.onToolInvoke) this.onToolInvoke({ tool, files, path: this.pane.path });
    }

    /* --- Keyboard (called from app-keyboard.js) --- */

    // The number keys are the place shortcuts everywhere else, so they only
    // aim at a target while something is selected: you select, then you send.
    // Enter opens, as everywhere else; it never moves a file. A held key
    // does not send again.
    handleKey(e) {
        const hasSelection = this.pane.selection.selected.size + this.pane.selectedDirs.size > 0;
        if (hasSelection && /^[1-9]$/.test(e.key)) {
            const n = Number(e.key) - 1;
            const isMark = n === this.targets.length;
            if (n >= this.targets.length && !isMark) return false;
            e.preventDefault();
            if (!e.repeat) this.sendTo(isMark ? 'mark' : n, { copy: e.altKey });
            return true;
        }
        if ((e.key === 'u' || e.key === 'U') && this._undo && !e.repeat) {
            e.preventDefault();
            this.undo();
            return true;
        }
        return false;
    }
}

// failureLine is one file that did not move: its name and why, without the
// path the server's message repeats.
function failureLine(r) {
    const name = String(r.file || '').split('/').pop();
    const reason = String(r.error || '').split(': ').pop();
    return name ? `${name}: ${reason}` : reason;
}

/* --- Targets are remembered client-side (ADR-0012) --- */

function loadTargets() {
    try {
        const raw = JSON.parse(localStorage.getItem(ORGANIZE_TARGETS_KEY) || '[]');
        return Array.isArray(raw) ? raw.filter(t => t && typeof t.path === 'string') : [];
    } catch {
        return [];
    }
}

function saveTargets(targets) {
    try {
        localStorage.setItem(ORGANIZE_TARGETS_KEY, JSON.stringify(targets.map(t => ({ path: t.path }))));
    } catch { /* a browser that refuses storage still sorts photos */ }
}

function targetName(path) {
    return path.split('/').filter(Boolean).pop() || path;
}

function countLabel(n) {
    return countText(n, 'item');
}
