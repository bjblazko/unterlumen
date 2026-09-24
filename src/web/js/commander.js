// Commander mode — dual pane file browser

const CMD_ICONS = {
    copy: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><rect x="4.5" y="4.5" width="8" height="8" rx="1"/><path d="M1.5 9.5v-7a1 1 0 0 1 1-1h7"/></svg>',
    move: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><rect x="1.5" y="3.5" width="7" height="8" rx="1"/><path d="M10.5 5l2.5 2-2.5 2"/><path d="M8.5 7h4.5"/></svg>',
    delete: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><path d="M2.5 4.5h9"/><path d="M5 4.5V3a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1.5"/><path d="M3.5 4.5l.5 7.5a1 1 0 0 0 1 1h4a1 1 0 0 0 1-1l.5-7.5"/></svg>',
    mkdir: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><path d="M1.5 4.5a1 1 0 0 1 1-1h3l1.5 1.5h4a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1z"/><path d="M7 7.5v3"/><path d="M5.5 9h3"/></svg>',
    rename: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 2.5l2 2-6 6H3.5v-2z"/><path d="M8 4l2 2"/></svg>',
};

class Commander {
    constructor(container, initialPath, options = {}) {
        this.container = container;
        this.initialPath = initialPath || '';
        this._initialPreselect = options.preselectFiles || null;
        this.leftPane = null;
        this.rightPane = null;
        this.activePane = 'left';
        this.onImageClick = null;
        this.onToolInvoke = null;
    }

    init() {
        this.container.innerHTML = `
            <div class="commander">
                <div class="commander-pane left-pane" id="left-pane"></div>
                <div class="commander-resizer" id="cmd-resizer"></div>
                <div class="commander-actions">
                    <div class="cmd-top-actions">
                        <button class="btn btn-sm" id="cmd-delete" title="Mark the selection for deletion (Delete)" disabled>Mark for deletion</button>
                        <button class="btn btn-sm" id="cmd-mkdir" title="Create a folder in the active pane">New folder</button>
                        <button class="btn btn-sm" id="cmd-rename" title="Rename the selection" disabled>Rename…</button>
                    </div>
                    <div class="cmd-dir-actions">
                        <button class="btn" id="cmd-copy" disabled>Copy <span class="key">F5</span></button>
                        <button class="btn" id="cmd-move" disabled>Move <span class="key">F6</span></button>
                    </div>
                    <div class="cmd-status" id="cmd-status" hidden></div>
                    <div class="cmd-lib-actions">
                        <select class="btn btn-sm" id="cmd-lib-select" aria-label="Jump to library" title="Navigate the active pane to a library folder">
                            <option value="">Jump to library…</option>
                        </select>
                    </div>
                </div>
                <div class="commander-pane right-pane" id="right-pane"></div>
            </div>
        `;

        const leftEl = document.getElementById('left-pane');
        const rightEl = document.getElementById('right-pane');

        this.leftPane = new BrowsePane(leftEl, {
            onImageClick: (path) => {
                if (this.onImageClick) this.onImageClick(path, this.leftPane);
            },
            onSelectionChange: () => this.updateActions(),
            onFocusChange: () => this.updateActions(),
            onToolInvoke: (params) => { if (this.onToolInvoke) this.onToolInvoke(params); },
        });

        this.rightPane = new BrowsePane(rightEl, {
            onImageClick: (path) => {
                if (this.onImageClick) this.onImageClick(path, this.rightPane);
            },
            onSelectionChange: () => this.updateActions(),
            onFocusChange: () => this.updateActions(),
            onToolInvoke: (params) => { if (this.onToolInvoke) this.onToolInvoke(params); },
        });

        // Track active pane
        leftEl.addEventListener('click', () => {
            this.activePane = 'left';
            leftEl.classList.add('active');
            rightEl.classList.remove('active');
            this.updateActions();
        });

        rightEl.addEventListener('click', () => {
            this.activePane = 'right';
            rightEl.classList.add('active');
            leftEl.classList.remove('active');
            this.updateActions();
        });

        // Buttons
        document.getElementById('cmd-copy').addEventListener('click', () => this.doCopy());
        document.getElementById('cmd-move').addEventListener('click', () => this.doMove());
        document.getElementById('cmd-delete').addEventListener('click', () => this.doDelete());
        document.getElementById('cmd-mkdir').addEventListener('click', () => this.doMkdir());
        // One rename for one or many files: the batch dialog previews what it
        // would do, where the old "Single" option was a browser prompt().
        document.getElementById('cmd-rename').addEventListener('click', () => this.doRename('batch-rename'));

        // Set default views: left=grid, right=list
        this.leftPane.view = 'grid';
        this.rightPane.view = 'list';

        this._wireLibrarySelect();

        // Load both panes (prime preselect before loading left pane)
        leftEl.classList.add('active');
        if (this._initialPreselect) this.leftPane.primePreselect(this._initialPreselect);
        this.leftPane.load(this.initialPath);
        this.rightPane.load(this.initialPath);

        // Set initial pane labels
        this._updatePaneLabels();

        this._initResizer();
    }

    // The panes are navigation, and a library is a place to navigate to, so
    // this is a list to pick from rather than a menu of actions. Libraries
    // outside the server root cannot be reached and say so instead of being
    // offered.
    async _wireLibrarySelect() {
        const select = document.getElementById('cmd-lib-select');
        select.addEventListener('change', () => {
            const path = select.value;
            select.selectedIndex = 0;
            if (path !== '') this.getActivePane().load(path);
        });

        let libs;
        try {
            libs = await LibraryAPI.list();
        } catch {
            select.firstElementChild.textContent = 'Libraries did not load';
            select.disabled = true;
            return;
        }
        if (libs.length === 0) {
            select.firstElementChild.textContent = 'No libraries yet';
            select.disabled = true;
            return;
        }
        const boundary = App.config?.boundary;
        for (const lib of libs) {
            const sourcePath = lib.sourcePath.replace(/\/$/, '');
            const relPath = absPathRelativeToBoundary(sourcePath, boundary);
            const option = document.createElement('option');
            option.value = relPath ?? '';
            option.textContent = relPath === null ? `${lib.name} — outside the server root` : lib.name;
            option.disabled = relPath === null;
            select.appendChild(option);
        }
    }

    _initResizer() {
        const resizer = document.getElementById('cmd-resizer');
        const commanderEl = this.container.querySelector('.commander');
        const leftEl = document.getElementById('left-pane');
        const rightEl = document.getElementById('right-pane');
        const MIN_PX = 100;

        let dragging = false;
        let startX = 0;
        let startLeftWidth = 0;
        let startRightWidth = 0;
        let totalWidth = 0;

        const onMouseMove = (e) => {
            if (!dragging) return;
            const delta = e.clientX - startX;
            let newLeft = Math.max(MIN_PX, Math.min(totalWidth - MIN_PX, startLeftWidth + delta));
            leftEl.style.width = newLeft + 'px';
            rightEl.style.width = (totalWidth - newLeft) + 'px';
        };

        const onMouseUp = () => {
            if (!dragging) return;
            dragging = false;
            resizer.classList.remove('dragging');
            document.body.style.cursor = '';
            document.body.style.userSelect = '';
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
            // Persist ratio
            const ratio = parseFloat(leftEl.style.width) / totalWidth;
            if (isFinite(ratio)) localStorage.setItem('commander-split', ratio.toFixed(4));
        };

        const onMouseDown = (e) => {
            if (e.button !== 0) return;
            dragging = true;
            startX = e.clientX;
            startLeftWidth = leftEl.getBoundingClientRect().width;
            startRightWidth = rightEl.getBoundingClientRect().width;
            totalWidth = startLeftWidth + startRightWidth;

            leftEl.style.flex = 'none';
            leftEl.style.width = startLeftWidth + 'px';
            rightEl.style.flex = 'none';
            rightEl.style.width = startRightWidth + 'px';

            resizer.classList.add('dragging');
            document.body.style.cursor = 'col-resize';
            document.body.style.userSelect = 'none';
            e.preventDefault();
            document.addEventListener('mousemove', onMouseMove);
            document.addEventListener('mouseup', onMouseUp);
        };

        resizer.addEventListener('mousedown', onMouseDown);

        // Restore saved split ratio (default 0.6 = left pane wider)
        const saved = parseFloat(localStorage.getItem('commander-split'));
        const ratio = (saved > 0 && saved < 1) ? saved : 0.6;
        requestAnimationFrame(() => {
            const actionsEl = commanderEl.querySelector('.commander-actions');
            const available = commanderEl.getBoundingClientRect().width
                - resizer.getBoundingClientRect().width
                - actionsEl.getBoundingClientRect().width;
            const lw = Math.max(MIN_PX, Math.min(available - MIN_PX, ratio * available));
            leftEl.style.flex = 'none';
            leftEl.style.width = lw + 'px';
            rightEl.style.flex = 'none';
            rightEl.style.width = (available - lw) + 'px';
        });

        this._resizerCleanup = () => {
            resizer.removeEventListener('mousedown', onMouseDown);
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
        };
    }

    destroy() {
        if (this._resizerCleanup) {
            this._resizerCleanup();
            this._resizerCleanup = null;
        }
    }

    getActivePane() {
        return this.activePane === 'left' ? this.leftPane : this.rightPane;
    }

    getOtherPane() {
        return this.activePane === 'left' ? this.rightPane : this.leftPane;
    }

    updateActions() {
        this._updatePaneLabels();
        const active = this.getActivePane();
        const actionable = active.getActionableFiles();
        const hasTargets = actionable.length > 0;
        const focused = active.focusedIndex >= 0 && active.focusedIndex < active.entries.length
            ? active.entries[active.focusedIndex] : null;

        document.getElementById('cmd-copy').disabled = !hasTargets;
        document.getElementById('cmd-move').disabled = !hasTargets;
        document.getElementById('cmd-delete').disabled = !hasTargets;
        document.getElementById('cmd-rename').disabled = !hasTargets && !focused;

        // The button itself says where the files would go: the arrow points at
        // the receiving pane, and the title spells it out in words.
        const targetSide = this.activePane === 'left' ? 'right' : 'left';
        const toRight = targetSide === 'right';
        const label = (verb) => toRight ? `${verb} →` : `← ${verb}`;
        document.getElementById('cmd-copy').innerHTML = `${label('Copy')} <span class="key">F5</span>`;
        document.getElementById('cmd-move').innerHTML = `${label('Move')} <span class="key">F6</span>`;
        document.getElementById('cmd-copy').title = `Copy the selection to the ${targetSide} pane (F5)`;
        document.getElementById('cmd-move').title = `Move the selection to the ${targetSide} pane (F6)`;
        document.getElementById('cmd-delete').innerHTML = 'Mark for deletion';
        document.getElementById('cmd-mkdir').innerHTML = 'New folder';
        document.getElementById('cmd-rename').innerHTML = 'Rename…';

        this._updatePaneLabels();
    }

    // Each pane's header says where it is and how much of it is selected —
    // the two things you need before pressing Copy or Move.
    _updatePaneLabels() {
        const describe = (pane, isActive) => {
            if (!pane) return isActive ? 'From' : 'To';
            const path = pane.path ? '…/' + pane.path.split('/').filter(Boolean).slice(-2).join('/') : '/';
            const selected = pane.selection.selected.size + (pane.selectedDirs?.size ?? 0);
            const total = pane.entries.length;
            const count = selected > 0 ? `${selected} of ${total} selected` : `${total} item${total !== 1 ? 's' : ''}`;
            return `${path}  ·  ${count}`;
        };
        const leftEl = document.getElementById('left-pane');
        const rightEl = document.getElementById('right-pane');
        if (leftEl) leftEl.dataset.paneLabel = describe(this.leftPane, this.activePane === 'left');
        if (rightEl) rightEl.dataset.paneLabel = describe(this.rightPane, this.activePane === 'right');
    }

    // Failures and questions belong next to the buttons that caused them.
    _say(message, kind = 'info') {
        const el = document.getElementById('cmd-status');
        if (!el) return;
        el.className = 'cmd-status' + (kind === 'error' ? ' cmd-status--error' : '');
        el.textContent = message;
        el.hidden = !message;
    }

    _ask(question, confirmLabel, onConfirm) {
        const el = document.getElementById('cmd-status');
        if (!el) return;
        el.hidden = false;
        el.className = 'cmd-status';
        el.innerHTML = `<p>${escapeHtml(question)}</p>
            <div class="cmd-status-actions">
                <button class="btn btn-sm" data-ask="cancel">Cancel</button>
                <button class="btn btn-sm btn-danger" data-ask="confirm">${escapeHtml(confirmLabel)}</button>
            </div>`;
        el.querySelector('[data-ask="cancel"]').addEventListener('click', () => this._say(''));
        el.querySelector('[data-ask="confirm"]').addEventListener('click', () => { this._say(''); onConfirm(); });
    }

    async doCopy() {
        const active = this.getActivePane();
        const otherPane = this.getOtherPane();
        const dest = otherPane.getFocusedDir() || otherPane.path;
        const actionable = active.getActionableFiles();
        if (actionable.length === 0) return;

        // Partition into files and directories
        const dirs = [];
        const files = [];
        for (const path of actionable) {
            const entry = active.entries.find(e => active.fullPath(e.name) === path);
            if (entry && entry.type === 'dir') {
                dirs.push(path);
            } else {
                files.push(path);
            }
        }

        // If directories are involved, expand them for per-file progress
        let allFiles = [...files];
        let allDirs = [];
        try {
            for (const dir of dirs) {
                const listing = await API.listRecursive(dir);
                allDirs.push(...(listing.dirs || []));
                allFiles.push(...(listing.files || []));
            }
        } catch (err) {
            this._say('Copy failed: ' + err.message, 'error');
            return;
        }

        // For single items with no directory expansion, use direct API
        if (allFiles.length <= 1 && allDirs.length === 0) {
            try {
                const result = await API.copy(actionable, dest);
                this.showResults('Copy', result.results);
                otherPane.load(otherPane.path);
                if (result.libraryUpdated) App.reloadLibraryPane();
            } catch (err) {
                this._say('Copy failed: ' + err.message, 'error');
            }
            return;
        }

        // Create destination directories: first the top-level folder for each source dir, then subdirs
        try {
            for (const dir of dirs) {
                const srcBaseName = dir.split('/').pop();
                const topDir = dest ? dest + '/' + srcBaseName : srcBaseName;
                await API.mkdir(topDir).catch(() => {}); // ignore if exists
            }
            for (const dir of allDirs) {
                const srcBase = dirs.find(d => dir === d || dir.startsWith(d + '/'));
                if (srcBase) {
                    const srcBaseName = srcBase.split('/').pop();
                    const rel = srcBaseName + '/' + dir.substring(srcBase.length + 1);
                    const mkdirPath = dest ? dest + '/' + rel : rel;
                    await API.mkdir(mkdirPath).catch(() => {}); // ignore if exists
                }
            }
        } catch (err) {
            // Continue with file copies even if some mkdirs fail
        }

        // Use progress dialog for file copies
        let copyLandedInLibrary = false;
        const dialog = new ProgressDialog();
        dialog.open(allFiles, {
            verb: 'Copying',
            action: async (file) => {
                try {
                    // Find which source dir this file belongs to, to compute correct destination
                    const srcDir = dirs.find(d => file.startsWith(d + '/'));
                    let fileDest = dest;
                    if (srcDir) {
                        const srcBaseName = srcDir.split('/').pop();
                        const relPath = file.substring(srcDir.length + 1);
                        const dirPart = relPath.includes('/') ? relPath.substring(0, relPath.lastIndexOf('/')) : '';
                        fileDest = dirPart
                            ? (dest ? dest + '/' + srcBaseName + '/' + dirPart : srcBaseName + '/' + dirPart)
                            : (dest ? dest + '/' + srcBaseName : srcBaseName);
                    }
                    const result = await API.copy([file], fileDest);
                    if (result.libraryUpdated) copyLandedInLibrary = true;
                    return result.results[0] || { success: true };
                } catch (err) {
                    return { success: false, error: err.message };
                }
            },
            onComplete: () => {
                otherPane.load(otherPane.path);
                if (copyLandedInLibrary) App.reloadLibraryPane();
            },
        });
    }

    async doMove() {
        const active = this.getActivePane();
        const otherPane = this.getOtherPane();
        const dest = otherPane.getFocusedDir() || otherPane.path;
        const actionable = active.getActionableFiles();
        if (actionable.length === 0) return;

        // Check if any items are directories
        const hasDirs = actionable.some(path => {
            const entry = active.entries.find(e => active.fullPath(e.name) === path);
            return entry && entry.type === 'dir';
        });

        // For directories, try direct move first (os.Rename is fast for same-filesystem)
        if (hasDirs || actionable.length <= 1) {
            try {
                const result = await API.move(actionable, dest);
                this.showResults('Move', result.results);
                this.leftPane.load(this.leftPane.path);
                this.rightPane.load(this.rightPane.path);
                if (result.libraryUpdated) App.reloadLibraryPane();
                return;
            } catch (err) {
                // If direct move fails for dirs, fall through would be complex;
                // for now just report the error
                this._say('Move failed: ' + err.message, 'error');
                return;
            }
        }

        // Multiple files without directories — use progress dialog
        let moveLandedInLibrary = false;
        const dialog = new ProgressDialog();
        dialog.open(actionable, {
            verb: 'Moving',
            action: async (file) => {
                try {
                    const result = await API.move([file], dest);
                    if (result.libraryUpdated) moveLandedInLibrary = true;
                    return result.results[0] || { success: true };
                } catch (err) {
                    return { success: false, error: err.message };
                }
            },
            onComplete: () => {
                this.leftPane.load(this.leftPane.path);
                this.rightPane.load(this.rightPane.path);
                if (moveLandedInLibrary) App.reloadLibraryPane();
            },
        });
    }

    doDelete() {
        const active = this.getActivePane();
        const actionable = active.getActionableFiles();
        if (actionable.length === 0) return;

        // Check if any selected items are directories
        const dirItems = actionable.filter(path => {
            const entry = active.entries.find(e => active.fullPath(e.name) === path);
            return entry && entry.type === 'dir';
        });
        const fileItems = actionable.filter(path => !dirItems.includes(path));

        if (dirItems.length > 0) {
            const dirNames = dirItems.map(p => p.split('/').pop()).join(', ');
            const msg = dirItems.length === 1
                ? `Delete folder '${dirNames}' and all its contents? This cannot be undone.`
                : `Delete ${dirItems.length} folders (${dirNames}) and all their contents? This cannot be undone.`;
            this._ask(msg, dirItems.length === 1 ? 'Delete folder' : `Delete ${dirItems.length} folders`, () => this._deleteDirs(dirItems, fileItems, active));
            return;
        }

        // Files only: use existing wastebin flow
        App.markForDeletion(fileItems, active.entries, active.path);
        active.selected.clear();
        active.render();
        this.updateActions();
    }

    _deleteDirs(dirItems, fileItems, active) {
        API.delete(dirItems).then(result => {
            const failures = result.results.filter(r => !r.success);
            if (failures.length > 0) {
                this._say(`${failures.length} could not be deleted: ${failures.map(f => f.file + ' (' + f.error + ')').join(', ')}`, 'error');
            }
            // Files go to the waste bin, as they do everywhere else.
            if (fileItems.length > 0) {
                App.markForDeletion(fileItems, active.entries, active.path);
            }
            active.selected.clear();
            active.load(active.path);
            this.updateActions();
        }).catch(err => this._say('Delete failed: ' + err.message, 'error'));
    }

    // Asks for the name in the status line, where every other question in
    // Organize is asked, instead of a browser prompt().
    doMkdir() {
        const pane = this.getActivePane();
        const el = document.getElementById('cmd-status');
        if (!el) return;
        el.hidden = false;
        el.className = 'cmd-status';
        el.innerHTML = `
            <label class="form-label" for="cmd-mkdir-name">New folder in ${escapeHtml(pane.path || '/')}</label>
            <input class="form-input" id="cmd-mkdir-name" autocomplete="off" placeholder="Folder name">
            <div class="cmd-status-actions">
                <button class="btn btn-sm" id="cmd-mkdir-cancel">Cancel</button>
                <button class="btn btn-sm btn-accent" id="cmd-mkdir-create">Create folder</button>
            </div>`;
        const input = el.querySelector('#cmd-mkdir-name');
        input.focus();
        const create = async () => {
            const name = input.value.trim();
            if (!name) { input.focus(); return; }
            const path = pane.path ? pane.path + '/' + name : name;
            try {
                await API.mkdir(path);
                this._say('');
                pane.load(pane.path);
            } catch (err) {
                this._say('Could not create the folder: ' + err.message, 'error');
            }
        };
        el.querySelector('#cmd-mkdir-cancel').addEventListener('click', () => this._say(''));
        el.querySelector('#cmd-mkdir-create').addEventListener('click', create);
        input.addEventListener('keydown', (e) => { if (e.key === 'Enter') create(); });
    }

    doRename(tool = 'rename') {
        const pane = this.getActivePane();
        const files = pane.getActionableFiles();
        if (files.length === 0) {
            const entry = pane.focusedIndex >= 0 ? pane.entries[pane.focusedIndex] : null;
            if (!entry) return;
            if (this.onToolInvoke) this.onToolInvoke({ tool, files: [pane.fullPath(entry.name)] });
            return;
        }
        if (this.onToolInvoke) this.onToolInvoke({ tool, files });
    }

    showResults(op, results) {
        const failures = results.filter(r => !r.success);
        if (failures.length > 0) {
            const msgs = failures.map(f => `${f.file}: ${f.error}`).join('\n');
            this._say(`${op}: ${failures.length} failed — ${msgs.replace(/\n/g, ', ')}`, 'error');
        }
    }
}
