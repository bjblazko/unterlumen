// Main application — routing and state

const App = {
    mode: 'browse', // 'browse', 'commander', or 'wastebin'
    browsePane: null,
    infoPanel: null,
    commander: null,
    viewer: null,
    currentBrowsePath: '',
    wasteBin: new Map(), // key: full relative path, value: {name, type, date, size, dir}
    wasteBinSelected: new Set(),
    wasteBinLastClickedIndex: -1,
    isMac: /Mac|iPhone|iPad|iPod/.test(navigator.platform),

    init() {
        this.viewer = new Viewer(document.getElementById('app'));

        // Mode switcher
        document.getElementById('mode-browse').addEventListener('click', () => this.setMode('browse'));
        document.getElementById('mode-commander').addEventListener('click', () => this.setMode('commander'));
        document.getElementById('mode-wastebin').addEventListener('click', () => this.setMode('wastebin'));

        // Set mode button tooltips with platform-appropriate shortcut keys
        document.getElementById('mode-browse').title = `Browse & Cull (1)`;
        document.getElementById('mode-commander').title = `File Manager (2)`;
        document.getElementById('mode-wastebin').title = `Waste Bin (3)`;

        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => this.handleGlobalKey(e));

        this.setMode('browse');
    },

    setMode(mode) {
        this.mode = mode;
        document.getElementById('mode-browse').classList.toggle('active', mode === 'browse');
        document.getElementById('mode-commander').classList.toggle('active', mode === 'commander');
        document.getElementById('mode-wastebin').classList.toggle('active', mode === 'wastebin');

        const appEl = document.getElementById('app');

        if (mode === 'browse') {
            appEl.innerHTML = '<div class="browse-layout">' +
                '<div id="browse-container" class="browse-container"></div>' +
                '<div id="info-panel-container"></div>' +
                '</div>';
            this.browsePane = new BrowsePane(document.getElementById('browse-container'), {
                onNavigate: (path) => { this.currentBrowsePath = path; },
                onImageClick: (path) => this.openViewer(path, this.browsePane),
                onSelectionChange: (selected) => this.handleSelectionChange(selected),
            });
            this.infoPanel = new InfoPanel(document.getElementById('info-panel-container'));
            this.browsePane.load(this.currentBrowsePath);
        } else if (mode === 'commander') {
            this.commander = new Commander(appEl);
            this.commander.onImageClick = (path, pane) => this.openViewer(path, pane);
            this.commander.init();
        } else if (mode === 'wastebin') {
            this.wasteBinSelected.clear();
            this.wasteBinLastClickedIndex = -1;
            this.renderWasteBin(appEl);
        }
    },

    markForDeletion(selectedPaths, entries, currentDir) {
        for (const path of selectedPaths) {
            if (this.wasteBin.has(path)) continue;
            const entry = entries.find(e => {
                const fp = currentDir ? `${currentDir}/${e.name}` : e.name;
                return fp === path;
            });
            if (entry) {
                this.wasteBin.set(path, {
                    name: entry.name,
                    type: entry.type,
                    date: entry.date,
                    size: entry.size,
                    dir: currentDir,
                });
            }
        }
        this.updateWasteBinBadge();
    },

    restoreFromWasteBin(paths) {
        for (const path of paths) {
            this.wasteBin.delete(path);
        }
        this.updateWasteBinBadge();
    },

    async permanentlyDelete(paths) {
        const filePaths = Array.from(paths);
        try {
            const result = await API.delete(filePaths);
            for (const r of result.results) {
                if (r.success || r.error.includes('no such file')) {
                    this.wasteBin.delete(r.file);
                }
            }
            this.updateWasteBinBadge();

            const failures = result.results.filter(r => !r.success && !r.error.includes('no such file'));
            if (failures.length > 0) {
                const msgs = failures.map(f => `${f.file}: ${f.error}`).join('\n');
                alert(`Delete: ${failures.length} error(s):\n${msgs}`);
            }
        } catch (err) {
            alert('Delete failed: ' + err.message);
        }
    },

    updateWasteBinBadge() {
        const countEl = document.getElementById('wastebin-count');
        if (!countEl) return;
        const count = this.wasteBin.size;
        countEl.textContent = count > 0 ? count : '';
        countEl.style.display = count > 0 ? '' : 'none';
    },

    renderWasteBin(appEl) {
        const items = Array.from(this.wasteBin.entries());

        if (items.length === 0) {
            appEl.innerHTML = '<div class="browse-container"><div class="wastebin-empty">Waste bin is empty</div></div>';
            return;
        }

        const header = `<div class="wastebin-header">${items.length} file${items.length !== 1 ? 's' : ''} marked for deletion</div>`;
        const selectedCount = this.wasteBinSelected.size;

        const actions = `<div class="wastebin-actions">
            <button class="btn btn-action" id="wb-restore" ${selectedCount === 0 ? 'disabled' : ''}>Restore${selectedCount > 0 ? ` (${selectedCount})` : ''}</button>
            <button class="btn btn-action btn-danger" id="wb-delete" ${selectedCount === 0 ? 'disabled' : ''}>Delete permanently${selectedCount > 0 ? ` (${selectedCount})` : ''}</button>
        </div>`;

        const gridItems = items.map(([path, entry], idx) => {
            const selectedClass = this.wasteBinSelected.has(path) ? ' selected' : '';
            return `<div class="grid-item image-item${selectedClass}" data-index="${idx}" data-path="${path}" data-type="image">
                <img src="${API.thumbnailURL(path)}" alt="${entry.name}" loading="lazy">
                <div class="item-name">${entry.name}</div>
            </div>`;
        });
        const grid = `<div class="grid">${gridItems.join('')}</div>`;

        appEl.innerHTML = `<div class="browse-container">${header}${actions}${grid}</div>`;

        // Attach events
        document.getElementById('wb-restore').addEventListener('click', () => {
            this.restoreFromWasteBin(this.wasteBinSelected);
            this.wasteBinSelected.clear();
            this.renderWasteBin(appEl);
        });

        document.getElementById('wb-delete').addEventListener('click', async () => {
            const count = this.wasteBinSelected.size;
            if (!confirm(`Permanently delete ${count} file${count !== 1 ? 's' : ''}? This cannot be undone.`)) return;
            await this.permanentlyDelete(this.wasteBinSelected);
            this.wasteBinSelected.clear();
            this.renderWasteBin(appEl);
        });

        // Grid selection
        appEl.querySelectorAll('[data-type="image"]').forEach(el => {
            el.addEventListener('click', (e) => {
                const path = el.dataset.path;
                const idx = parseInt(el.dataset.index);

                if (e.ctrlKey || e.metaKey) {
                    if (this.wasteBinSelected.has(path)) {
                        this.wasteBinSelected.delete(path);
                    } else {
                        this.wasteBinSelected.add(path);
                    }
                    this.wasteBinLastClickedIndex = idx;
                } else if (e.shiftKey && this.wasteBinLastClickedIndex >= 0) {
                    const start = Math.min(this.wasteBinLastClickedIndex, idx);
                    const end = Math.max(this.wasteBinLastClickedIndex, idx);
                    for (let i = start; i <= end; i++) {
                        this.wasteBinSelected.add(items[i][0]);
                    }
                } else {
                    this.wasteBinSelected.clear();
                    this.wasteBinSelected.add(path);
                    this.wasteBinLastClickedIndex = idx;
                }

                this.renderWasteBin(appEl);
            });
        });
    },

    isMarkedForDeletion(path) {
        return this.wasteBin.has(path);
    },

    openViewer(imagePath, pane) {
        const images = pane.getImageEntries().map(e => pane.fullPath(e.name));
        const appEl = document.getElementById('app');

        // Hide existing content instead of destroying it
        const existingChildren = Array.from(appEl.children);
        existingChildren.forEach(el => el.style.display = 'none');

        // Create a separate container for the viewer
        const viewerEl = document.createElement('div');
        viewerEl.id = 'viewer-container';
        viewerEl.style.height = '100%';
        appEl.appendChild(viewerEl);

        this.viewer = new Viewer(viewerEl);
        this.viewer.onClose = () => {
            // Remove viewer and restore previous content
            viewerEl.remove();
            existingChildren.forEach(el => el.style.display = '');
        };
        this.viewer.onDelete = (path) => {
            const entries = pane.entries || [];
            const dir = pane.path || '';
            this.markForDeletion([path], entries, dir);
        };
        this.viewer.open(imagePath, images);
    },

    handleSelectionChange(selected) {
        if (!this.infoPanel) return;
        if (selected.length === 1) {
            this.infoPanel.loadInfo(selected[0]);
        } else {
            this.infoPanel.clear();
        }
    },

    getActiveBrowsePane() {
        if (this.mode === 'browse') return this.browsePane;
        if (this.mode === 'commander' && this.commander) return this.commander.getActivePane();
        return null;
    },

    handleGlobalKey(e) {
        // Tab to switch panes in commander mode
        if (e.key === 'Tab' && this.mode === 'commander' && this.commander) {
            e.preventDefault();
            const leftEl = document.getElementById('left-pane');
            const rightEl = document.getElementById('right-pane');
            if (this.commander.activePane === 'left') {
                this.commander.activePane = 'right';
                leftEl.classList.remove('active');
                rightEl.classList.add('active');
            } else {
                this.commander.activePane = 'left';
                rightEl.classList.remove('active');
                leftEl.classList.add('active');
            }
            this.commander.updateActions();
        }

        // Backspace to go up in browse mode
        if (e.key === 'Backspace' && this.mode === 'browse' && this.browsePane) {
            if (document.querySelector('.viewer')) return; // Don't navigate while viewing
            e.preventDefault();
            const parts = this.browsePane.path.split('/').filter(Boolean);
            parts.pop();
            const parentPath = parts.join('/');
            this.browsePane.load(parentPath);
            this.currentBrowsePath = parentPath;
        }

        // I to toggle info panel in browse mode
        if ((e.key === 'i' || e.key === 'I') && this.mode === 'browse' && this.infoPanel) {
            if (document.querySelector('.viewer')) return;
            e.preventDefault();
            this.infoPanel.toggle();
        }

        // Delete key to mark selected files for waste bin
        if (e.key === 'Delete' && this.mode === 'browse' && this.browsePane) {
            if (document.querySelector('.viewer')) return;
            const selected = this.browsePane.getSelectedFiles();
            if (selected.length === 0) return;
            e.preventDefault();
            this.markForDeletion(selected, this.browsePane.entries, this.browsePane.path);
            this.browsePane.selected.clear();
            this.browsePane.render();
        }
        if (e.key === 'Delete' && this.mode === 'commander' && this.commander) {
            const selected = this.commander.getActivePane().getSelectedFiles();
            if (selected.length === 0) return;
            e.preventDefault();
            this.commander.doDelete();
        }

        // F5/F6 to copy/move in commander mode
        if (this.mode === 'commander' && this.commander) {
            if (e.key === 'F5') { e.preventDefault(); this.commander.doCopy(); }
            else if (e.key === 'F6') { e.preventDefault(); this.commander.doMove(); }
        }

        const modKey = this.isMac ? e.metaKey : e.ctrlKey;

        // Cmd/Ctrl+A to select all
        if (modKey && (e.key === 'a' || e.key === 'A') && !e.shiftKey && !e.altKey) {
            if (this.mode === 'browse' && this.browsePane && !document.querySelector('.viewer')) {
                e.preventDefault();
                this.browsePane.selectAll();
            } else if (this.mode === 'commander' && this.commander) {
                e.preventDefault();
                this.commander.getActivePane().selectAll();
            } else if (this.mode === 'wastebin') {
                e.preventDefault();
                this.wasteBin.forEach((_, p) => this.wasteBinSelected.add(p));
                this.renderWasteBin(document.getElementById('app'));
            }
        }

        // Cmd/Ctrl+D to mark for deletion
        if (modKey && (e.key === 'd' || e.key === 'D') && !e.shiftKey && !e.altKey) {
            if (this.mode === 'browse' && this.browsePane && !document.querySelector('.viewer')) {
                e.preventDefault();
                const selected = this.browsePane.getSelectedFiles();
                if (selected.length > 0) {
                    this.markForDeletion(selected, this.browsePane.entries, this.browsePane.path);
                    this.browsePane.selected.clear();
                    this.browsePane.render();
                }
            } else if (this.mode === 'commander' && this.commander) {
                e.preventDefault();
                this.commander.doDelete();
            }
        }

        // Arrow keys / Enter / Space for grid/list keyboard navigation
        if (!e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') return;
            if (!document.querySelector('.viewer')) {
                const pane = this.getActiveBrowsePane();
                if (pane) {
                    if (e.key === 'ArrowLeft') {
                        e.preventDefault(); pane.moveFocus(-1);
                    } else if (e.key === 'ArrowRight') {
                        e.preventDefault(); pane.moveFocus(1);
                    } else if (e.key === 'ArrowUp') {
                        e.preventDefault(); pane.moveFocus(-pane.getColumnCount());
                    } else if (e.key === 'ArrowDown') {
                        e.preventDefault(); pane.moveFocus(pane.getColumnCount());
                    } else if (e.key === 'Enter') {
                        e.preventDefault(); pane.activateFocused();
                    } else if (e.key === ' ') {
                        e.preventDefault(); pane.toggleFocusedSelection();
                    }
                }
            }
        }

        // 1/2/3 to switch views (bare keys, no modifier)
        if (!e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') return;
            if (e.key === '1') { e.preventDefault(); this.setMode('browse'); }
            else if (e.key === '2') { e.preventDefault(); this.setMode('commander'); }
            else if (e.key === '3') { e.preventDefault(); this.setMode('wastebin'); }
        }
    },
};

document.addEventListener('DOMContentLoaded', () => App.init());
