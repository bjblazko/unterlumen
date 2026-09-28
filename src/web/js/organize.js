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
                <div class="organize-head">
                    <h1 class="organize-title">Organize</h1>
                    <span class="organize-path mono" id="org-path"></span>
                    <span class="organize-session num" id="org-session"></span>
                    <span class="organize-spacer"></span>
                    <button class="btn btn-sm" id="org-change">Change folder…</button>
                    <span class="organize-keys"><span class="key">Space</span> select · <span class="key">1</span>–<span class="key">9</span> send to target · <span class="key">Enter</span> current target · <span class="key">U</span> undo</span>
                    ${placeLede('Sort one folder into others: select photos and press a target’s number to move them there. The files move on disk.')}
                </div>
                <div class="organize-body">
                    <div class="organize-source" id="org-source"></div>
                    <aside class="organize-targets" id="org-targets" aria-label="Targets"></aside>
                </div>
                <div class="organize-result" id="org-result" hidden></div>
            </div>`;

        this.pane = new BrowsePane(this.container.querySelector('#org-source'), {
            onImageClick: (path) => { if (this.onImageClick) this.onImageClick(path, this.pane); },
            onSelectionChange: () => this._updateSelectionBar(),
            onFocusChange: () => this._updateSelectionBar(),
            onNavigate: () => this._renderPath(),
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
        const picked = await new FolderPicker().open(this.pane.path, { title: 'Folder to sort' });
        if (picked === null) return;
        this.load(picked);
    }

    /* --- Targets --- */

    _renderTargets() {
        const el = this.container.querySelector('#org-targets');
        const rows = this.targets.map((t, i) => {
            const key = i < 9 ? String(i + 1) : '';
            const name = targetName(t.path);
            const where = shortenPath(t.path);
            return `
            <button class="org-target${t.missing ? ' org-target--missing' : ''}" data-target="${i}" aria-current="${i === this.current}">
                <span class="org-target-name">${escapeHtml(name)}</span>
                <span class="org-target-path mono" title="${escapeHtml(t.path)}">${escapeHtml(where)}</span>
                ${t.missing ? '<span class="org-target-note">This folder is not there any more.</span>' : ''}
                ${t.moved ? `<span class="org-target-note num">${t.moved} in this session</span>` : ''}
                ${key ? `<span class="key">${key}</span>` : ''}
            </button>`;
        }).join('');

        const markKey = this.targets.length < 9 ? String(this.targets.length + 1) : '';
        el.innerHTML = `
            <p class="section-title">Targets</p>
            ${rows}
            <button class="org-target org-target--mark" data-target="mark" aria-current="${this.current === 'mark'}">
                <span class="org-target-name">Mark for deletion</span>
                <span class="org-target-path">To the waste bin, not off the disk</span>
                ${markKey ? `<span class="key">${markKey}</span>` : ''}
            </button>
            <div class="org-target-actions">
                <button class="btn btn-sm" data-org="add">Add target…</button>
                <button class="btn btn-sm" data-org="new">New target…</button>
            </div>
            <p class="form-hint">Moving is what a target does. Hold ⌥ to copy instead. The orange one is where <span class="key">Enter</span> sends the selection.</p>
            ${this.targets.some(t => t.missing) ? '<button class="btn btn-sm" data-org="forget">Forget the missing folders</button>' : ''}`;
    }

    _onTargetClick(e) {
        const action = e.target.closest('[data-org]');
        if (action) {
            if (action.dataset.org === 'add') this._addTarget();
            else if (action.dataset.org === 'new') this._newTarget();
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
        const picked = await new FolderPicker().open(this.pane.path, { title: 'Folder to add as a target' });
        if (picked === null) return;
        if (this.targets.some(t => t.path === picked)) {
            this._say(`${targetName(picked)} is already a target.`);
            return;
        }
        this.targets.push({ path: picked });
        saveTargets(this.targets);
        this._renderTargets();
    }

    async _newTarget() {
        const parent = await new FolderPicker().open(this.pane.path, { title: 'Where the new folder goes' });
        if (parent === null) return;
        const name = await promptForName(this.container, 'Name of the new folder');
        if (!name) return;
        const path = parent ? `${parent}/${name}` : name;
        this._say(`Creating ${path}…`);
        try {
            await API.mkdir(path);
        } catch (err) {
            this._say(`Could not create ${path}: ${err.message}`, 'error');
            return;
        }
        this.targets.push({ path });
        saveTargets(this.targets);
        this._renderTargets();
        this._say(`Created ${path}.`);
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
            this._say('Folders can only go to "Mark for deletion"; photos go to any target.', 'error');
            return;
        }
        await this._moveFiles(index, files, copy);
    }

    _mark(files, dirs) {
        const targets = [...files, ...dirs];
        App.markForDeletion(targets, this.pane.entries, this.pane.path);
        this._sorted += targets.length;
        this.pane.selection.clear();
        this.pane.selectedDirs.clear();
        this.pane.render();
        this._undo = { kind: 'mark', paths: targets };
        this._say(`${countLabel(targets.length)} marked for deletion.`);
        this._renderTargets();
        this._renderPath();
        this._updateSelectionBar();
    }

    async _moveFiles(index, files, copy) {
        const target = this.targets[index];
        const name = targetName(target.path);
        this._say(`${copy ? 'Copying' : 'Moving'} ${countLabel(files.length)} to ${name}…`);
        let result;
        try {
            result = copy ? await API.copy(files, target.path) : await API.move(files, target.path);
        } catch (err) {
            this._say(`Could not reach ${name}: ${err.message}`, 'error');
            return;
        }

        const results = result.results || [];
        const failures = results.filter(r => !r.success);
        const moved = results.filter(r => r.success).map(r => r.file);
        if (!moved.length) {
            target.missing = failures.length > 0;
            this._renderTargets();
            this._say(`Nothing went to ${name}: ${failures.map(f => f.error).join(', ')}`, 'error');
            return;
        }

        target.missing = false;
        target.moved = (target.moved || 0) + moved.length;
        this._sorted += moved.length;
        this._undo = copy ? null : {
            kind: 'move',
            paths: moved.map(f => `${target.path}/${f.split('/').pop()}`),
            back: this.pane.path,
            name,
        };
        this._say(`${countLabel(moved.length)} ${copy ? 'copied' : 'moved'} to ${name}.`
            + (failures.length ? ` ${failures.length} did not: ${failures.map(f => f.error).join(', ')}` : ''),
            failures.length ? 'error' : '');
        this.pane.selection.clear();
        this.pane.load(this.pane.path);
        this._renderTargets();
        this._renderPath();
        if (result.libraryUpdated) App.reloadLibraryPane();
    }

    async undo() {
        const u = this._undo;
        if (!u) return;
        this._undo = null;
        if (u.kind === 'mark') {
            App.wastebin.restore(new Set(u.paths));
            this._sorted = Math.max(0, this._sorted - u.paths.length);
            this.pane.render();
            this._say(`${countLabel(u.paths.length)} back in the folder.`);
            this._renderPath();
            return;
        }
        this._say(`Moving ${countLabel(u.paths.length)} back from ${u.name}…`);
        try {
            await API.move(u.paths, u.back);
            this._sorted = Math.max(0, this._sorted - u.paths.length);
            const target = this.targets.find(t => targetName(t.path) === u.name);
            if (target) target.moved = Math.max(0, (target.moved || 0) - u.paths.length);
            this.pane.load(this.pane.path);
            this._renderTargets();
            this._renderPath();
            this._say(`${countLabel(u.paths.length)} moved back from ${u.name}.`);
        } catch (err) {
            this._say(`Could not move them back: ${err.message}`, 'error');
        }
    }

    /* --- Result line --- */

    _say(message, kind = '') {
        this._result = message;
        this._resultKind = kind;
        const el = this.container.querySelector('#org-result');
        if (!el) return;
        el.hidden = !message;
        el.className = `organize-result${kind === 'error' ? ' organize-result--problem' : ''}`;
        el.innerHTML = message
            ? `<span>${escapeHtml(message)}</span>${this._undo ? '<button class="btn btn-sm" id="org-undo">Undo <span class="key">U</span></button>' : ''}`
            : '';
        el.querySelector('#org-undo')?.addEventListener('click', () => this.undo());
    }

    /* --- Selection --- */

    _updateSelectionBar() {
        const files = this.pane.selection.selected.size;
        const dirs = this.pane.selectedDirs.size;
        this._selectionBar.update(files + dirs, { onlyFolders: files === 0 && dirs > 0 });
    }

    _runSelectionAction(action) {
        if (action === 'clear') {
            this.pane.selection.clear();
            this.pane.selectedDirs.clear();
            this.pane.render();
            this._updateSelectionBar();
            return;
        }
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
    // Enter keeps opening a folder when nothing is selected, as in Folders.
    handleKey(e) {
        const hasSelection = this.pane.selection.selected.size + this.pane.selectedDirs.size > 0;
        if (hasSelection && /^[1-9]$/.test(e.key)) {
            const n = Number(e.key) - 1;
            const isMark = n === this.targets.length;
            if (n >= this.targets.length && !isMark) return false;
            e.preventDefault();
            this.sendTo(isMark ? 'mark' : n, { copy: e.altKey });
            return true;
        }
        if (hasSelection && e.key === 'Enter') {
            e.preventDefault();
            this.sendTo(this.current, { copy: e.altKey });
            return true;
        }
        if ((e.key === 'u' || e.key === 'U') && this._undo) {
            e.preventDefault();
            this.undo();
            return true;
        }
        return false;
    }
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
    return `${n} ${n === 1 ? 'item' : 'items'}`;
}

// Asks for a name on the screen that needs it, not in a browser prompt.
function promptForName(container, question) {
    return new Promise((resolve) => {
        const el = document.createElement('div');
        el.className = 'organize-ask';
        el.innerHTML = `
            <label class="form-label" for="org-ask-name">${escapeHtml(question)}</label>
            <input id="org-ask-name" class="organize-ask-input" type="text" autocomplete="off">
            <button class="btn btn-sm" id="org-ask-ok">Create</button>
            <button class="btn btn-sm" id="org-ask-cancel">Cancel</button>`;
        container.querySelector('.organize').appendChild(el);
        const input = el.querySelector('#org-ask-name');
        input.focus();
        const done = (value) => { el.remove(); resolve(value); };
        el.querySelector('#org-ask-ok').addEventListener('click', () => done(input.value.trim()));
        el.querySelector('#org-ask-cancel').addEventListener('click', () => done(null));
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') done(input.value.trim());
            if (e.key === 'Escape') done(null);
        });
    });
}
