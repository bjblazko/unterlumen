// Commander mode — dual pane file browser

const CMD_ICONS = {
    copy: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><rect x="4.5" y="4.5" width="8" height="8" rx="1"/><path d="M1.5 9.5v-7a1 1 0 0 1 1-1h7"/></svg>',
    move: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><rect x="1.5" y="3.5" width="7" height="8" rx="1"/><path d="M10.5 5l2.5 2-2.5 2"/><path d="M8.5 7h4.5"/></svg>',
    delete: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><path d="M2.5 4.5h9"/><path d="M5 4.5V3a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1.5"/><path d="M3.5 4.5l.5 7.5a1 1 0 0 0 1 1h4a1 1 0 0 0 1-1l.5-7.5"/></svg>',
    mkdir: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><path d="M1.5 4.5a1 1 0 0 1 1-1h3l1.5 1.5h4a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1z"/><path d="M7 7.5v3"/><path d="M5.5 9h3"/></svg>',
    rename: '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 2.5l2 2-6 6H3.5v-2z"/><path d="M8 4l2 2"/></svg>',
};

class Commander {
    constructor(container, initialPath) {
        this.container = container;
        this.initialPath = initialPath || '';
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
                        <button class="btn btn-action" id="cmd-delete" title="Mark for Deletion (Del)" disabled>${CMD_ICONS.delete} Delete</button>
                        <button class="btn btn-action" id="cmd-mkdir" title="New Folder">${CMD_ICONS.mkdir} Folder</button>
                        <button class="btn btn-action" id="cmd-rename" title="Rename" disabled>${CMD_ICONS.rename} Rename</button>
                    </div>
                    <div class="cmd-dir-actions">
                        <svg id="cmd-direction-arrow" class="cmd-arrow" viewBox="0 0 100 160" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
                            <path d="M 0,56 L 20,56 L 20,0 L 100,80 L 20,160 L 20,104 L 0,104 Z"/>
                        </svg>
                        <button class="btn btn-action" id="cmd-copy" title="Copy (F5)" disabled>${CMD_ICONS.copy} Copy</button>
                        <button class="btn btn-action" id="cmd-move" title="Move (F6)" disabled>${CMD_ICONS.move} Move</button>
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
        document.getElementById('cmd-rename').addEventListener('click', () => this.doRename());

        // Set default views: left=grid, right=list
        this.leftPane.view = 'grid';
        this.rightPane.view = 'list';

        // Load both panes
        leftEl.classList.add('active');
        this.leftPane.load(this.initialPath);
        this.rightPane.load(this.initialPath);

        // Set initial pane labels
        leftEl.dataset.paneLabel = 'From';
        rightEl.dataset.paneLabel = 'To';

        this._initResizer();
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
        const active = this.getActivePane();
        const actionable = active.getActionableFiles();
        const hasTargets = actionable.length > 0;
        const focused = active.focusedIndex >= 0 && active.focusedIndex < active.entries.length
            ? active.entries[active.focusedIndex] : null;

        document.getElementById('cmd-copy').disabled = !hasTargets;
        document.getElementById('cmd-move').disabled = !hasTargets;
        document.getElementById('cmd-delete').disabled = !hasTargets;
        document.getElementById('cmd-rename').disabled = !focused;

        document.getElementById('cmd-copy').innerHTML = `${CMD_ICONS.copy} Copy`;
        document.getElementById('cmd-move').innerHTML = `${CMD_ICONS.move} Move`;
        document.getElementById('cmd-delete').innerHTML = `${CMD_ICONS.delete} Delete`;
        document.getElementById('cmd-mkdir').innerHTML = `${CMD_ICONS.mkdir} Folder`;
        document.getElementById('cmd-rename').innerHTML = `${CMD_ICONS.rename} Rename`;

        // Flip arrow: left-active → points right (default); right-active → points left
        const arrow = document.getElementById('cmd-direction-arrow');
        if (arrow) {
            arrow.style.transform = this.activePane === 'right' ? 'scaleX(-1)' : '';
        }

        // Update pane labels
        document.getElementById('left-pane').dataset.paneLabel = this.activePane === 'left' ? 'From' : 'To';
        document.getElementById('right-pane').dataset.paneLabel = this.activePane === 'right' ? 'From' : 'To';
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
            alert('Copy failed: ' + err.message);
            return;
        }

        // For single items with no directory expansion, use direct API
        if (allFiles.length <= 1 && allDirs.length === 0) {
            try {
                const result = await API.copy(actionable, dest);
                this.showResults('Copy', result.results);
                otherPane.load(otherPane.path);
            } catch (err) {
                alert('Copy failed: ' + err.message);
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
                    return result.results[0] || { success: true };
                } catch (err) {
                    return { success: false, error: err.message };
                }
            },
            onComplete: () => {
                otherPane.load(otherPane.path);
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
                return;
            } catch (err) {
                // If direct move fails for dirs, fall through would be complex;
                // for now just report the error
                alert('Move failed: ' + err.message);
                return;
            }
        }

        // Multiple files without directories — use progress dialog
        const dialog = new ProgressDialog();
        dialog.open(actionable, {
            verb: 'Moving',
            action: async (file) => {
                try {
                    const result = await API.move([file], dest);
                    return result.results[0] || { success: true };
                } catch (err) {
                    return { success: false, error: err.message };
                }
            },
            onComplete: () => {
                this.leftPane.load(this.leftPane.path);
                this.rightPane.load(this.rightPane.path);
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
            if (!confirm(msg)) return;

            // Delete directories directly via API
            API.delete(dirItems).then(result => {
                const failures = result.results.filter(r => !r.success);
                if (failures.length > 0) {
                    const msgs = failures.map(f => `${f.file}: ${f.error}`).join('\n');
                    alert(`Delete: ${failures.length} error(s):\n${msgs}`);
                }
                // Mark remaining file items for waste bin
                if (fileItems.length > 0) {
                    App.markForDeletion(fileItems, active.entries, active.path);
                }
                active.selected.clear();
                active.load(active.path);
                this.updateActions();
            }).catch(err => {
                alert('Delete failed: ' + err.message);
            });
            return;
        }

        // Files only: use existing wastebin flow
        App.markForDeletion(fileItems, active.entries, active.path);
        active.selected.clear();
        active.render();
        this.updateActions();
    }

    async doMkdir() {
        const pane = this.getActivePane();
        const name = prompt('New folder name:');
        if (!name || !name.trim()) return;
        const path = pane.path ? pane.path + '/' + name.trim() : name.trim();
        try {
            await API.mkdir(path);
            pane.load(pane.path);
        } catch (err) {
            alert('Create folder failed: ' + err.message);
        }
    }

    async doRename() {
        const pane = this.getActivePane();
        const entry = pane.focusedIndex >= 0 ? pane.entries[pane.focusedIndex] : null;
        if (!entry) return;
        const newName = prompt('New name:', entry.name);
        if (!newName || !newName.trim() || newName.trim() === entry.name) return;
        const oldPath = pane.fullPath(entry.name);
        try {
            await API.rename(oldPath, newName.trim());
            pane.load(pane.path);
        } catch (err) {
            alert('Rename failed: ' + err.message);
        }
    }

    showResults(op, results) {
        const failures = results.filter(r => !r.success);
        if (failures.length > 0) {
            const msgs = failures.map(f => `${f.file}: ${f.error}`).join('\n');
            alert(`${op}: ${failures.length} error(s):\n${msgs}`);
        }
    }
}
