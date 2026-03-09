// Main application — routing and state

const App = {
    mode: 'browse', // 'browse', 'commander', or 'wastebin'
    browsePane: null,
    infoPanel: null,
    commander: null,
    viewer: null,
    currentBrowsePath: '',
    _browseEl: null,
    _commanderEl: null,
    _wastebinEl: null,
    wasteBin: new Map(), // key: full relative path, value: {name, type, date, size, dir}
    wasteBinSelected: new Set(),
    wasteBinLastClickedIndex: -1,
    isMac: /Mac|iPhone|iPad|iPod/.test(navigator.platform),
    uiHidden: false,

    init() {
        this.viewer = new Viewer(document.getElementById('app'));

        // Mode switcher
        document.getElementById('mode-browse').addEventListener('click', () => this.setMode('browse'));
        document.getElementById('mode-commander').addEventListener('click', () => this.setMode('commander'));
        document.getElementById('mode-wastebin').addEventListener('click', () => this.setMode('wastebin'));

        // Set mode button tooltips with platform-appropriate shortcut keys
        document.getElementById('mode-browse').title = `Select (1)`;
        document.getElementById('mode-wastebin').title = `Review (2)`;
        document.getElementById('mode-commander').title = `Organize (3)`;

        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => this.handleGlobalKey(e));

        this.initTheme();
        this.initThumbnailQuality();
        this._initUIVisibility();
        this.initSettingsMenu();

        API.config().then(cfg => {
            this.currentBrowsePath = cfg.startPath || '';
            this.setMode('browse');
        }).catch(() => {
            this.setMode('browse');
        });
    },

    _initUIVisibility() {
        if (localStorage.getItem('ui-hidden') === '1') {
            this.uiHidden = true;
            document.body.classList.add('ui-hidden');
            this._showUIHint();
        }
    },

    toggleUIVisibility() {
        this.uiHidden = !this.uiHidden;
        document.body.classList.toggle('ui-hidden', this.uiHidden);
        localStorage.setItem('ui-hidden', this.uiHidden ? '1' : '0');
        if (this.uiHidden) {
            this._showUIHint();
        }
    },

    _showUIHint() {
        const hint = document.getElementById('ui-hint');
        if (!hint) return;
        hint.textContent = 'Press H to show the interface again';
        hint.classList.add('visible');
        clearTimeout(this._uiHintTimer);
        this._uiHintTimer = setTimeout(() => hint.classList.remove('visible'), 3000);
    },

    initTheme() {
        const saved = localStorage.getItem('theme') || 'auto';
        this._applyTheme(saved);
        this._updateThemeButtons(saved);

        window.matchMedia('(prefers-color-scheme: dark)')
            .addEventListener('change', () => {
                if ((localStorage.getItem('theme') || 'auto') === 'auto') {
                    this._applyTheme('auto');
                }
            });
    },

    _applyTheme(preference) {
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        const resolved = preference === 'auto' ? (prefersDark ? 'dark' : 'light') : preference;
        document.documentElement.dataset.theme = resolved;
    },

    _updateThemeButtons(preference) {
        document.querySelectorAll('[data-theme-set]').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.themeSet === preference);
        });
    },

    initThumbnailQuality() {
        const saved = localStorage.getItem('thumbnail-quality') || 'standard';
        this._updateThumbQualityButtons(saved);
    },

    _updateThumbQualityButtons(quality) {
        document.querySelectorAll('[data-thumb-quality]').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.thumbQuality === quality);
        });
    },

    _setThumbnailQuality(quality) {
        localStorage.setItem('thumbnail-quality', quality);
        this._updateThumbQualityButtons(quality);
        if (this.browsePane) this.browsePane.reloadThumbnails();
        if (this.commander) {
            if (this.commander.leftPane) this.commander.leftPane.reloadThumbnails();
            if (this.commander.rightPane) this.commander.rightPane.reloadThumbnails();
        }
    },

    initSettingsMenu() {
        const btn = document.getElementById('settings-btn');
        const menu = document.getElementById('settings-menu');

        const closeMenu = () => {
            menu.style.display = 'none';
            document.removeEventListener('click', closeMenu);
        };

        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            if (menu.style.display !== 'none') {
                closeMenu();
            } else {
                menu.style.display = '';
                document.addEventListener('click', closeMenu);
            }
        });

        menu.addEventListener('click', (e) => e.stopPropagation());

        menu.addEventListener('click', (e) => {
            const themeBtn = e.target.closest('[data-theme-set]');
            if (themeBtn) {
                const preference = themeBtn.dataset.themeSet;
                localStorage.setItem('theme', preference);
                this._applyTheme(preference);
                this._updateThemeButtons(preference);
            }
            const thumbBtn = e.target.closest('[data-thumb-quality]');
            if (thumbBtn) {
                this._setThumbnailQuality(thumbBtn.dataset.thumbQuality);
            }
        });

        document.getElementById('settings-hide-ui').addEventListener('click', () => {
            this.toggleUIVisibility();
            closeMenu();
        });
    },

    setMode(mode) {
        // Close any open viewer before switching modes
        if (this.viewer) {
            this.viewer.close();
            this.viewer = null;
        }

        const prevMode = this.mode;

        // Preserve path from previous mode
        if (prevMode === 'commander' && this.commander) {
            this.currentBrowsePath = this.commander.getActivePane().path;
        }

        this.mode = mode;

        // Workflow step order: browse=0, wastebin=1, commander=2
        const stepOrder = { browse: 0, wastebin: 1, commander: 2 };
        const prevIdx = stepOrder[prevMode] ?? 0;
        const currIdx = stepOrder[mode];

        // Update workflow step states
        const steps = [
            { el: document.getElementById('mode-browse'), idx: 0 },
            { el: document.getElementById('mode-wastebin'), idx: 1 },
            { el: document.getElementById('mode-commander'), idx: 2 },
        ];
        for (const step of steps) {
            step.el.classList.remove('active', 'completed');
            if (step.idx === currIdx) {
                step.el.classList.add('active');
            } else if (step.idx < currIdx) {
                step.el.classList.add('completed');
            }
        }

        const appEl = document.getElementById('app');

        // Create browse DOM once, then hide/show
        if (mode === 'browse') {
            if (!this._browseEl) {
                this._browseEl = document.createElement('div');
                this._browseEl.className = 'browse-layout';
                this._browseEl.innerHTML =
                    '<div id="browse-container" class="browse-container"></div>' +
                    '<div id="info-panel-container"></div>';
                appEl.appendChild(this._browseEl);
                this.browsePane = new BrowsePane(this._browseEl.querySelector('#browse-container'), {
                    onNavigate: (path) => { this.currentBrowsePath = path; },
                    onImageClick: (path) => this.openViewer(path, this.browsePane),
                    onSelectionChange: (selected) => this.handleSelectionChange(selected),
                    onFocusChange: (path) => this.handleFocusChange(path),
                });
                this.infoPanel = new InfoPanel(this._browseEl.querySelector('#info-panel-container'));
                this.infoPanel.onToggle = () => {
                    if (this.browsePane && this.browsePane.view === 'justified') {
                        this.browsePane._scheduleJustifiedRelayout();
                    }
                };
                this.browsePane.load(this.currentBrowsePath);
            }
        }

        // Create commander DOM once, then hide/show
        if (mode === 'commander') {
            if (!this._commanderEl) {
                this._commanderEl = document.createElement('div');
                this._commanderEl.style.height = '100%';
                appEl.appendChild(this._commanderEl);
                this.commander = new Commander(this._commanderEl, this.currentBrowsePath);
                this.commander.onImageClick = (path, pane) => this.openViewer(path, pane);
                this.commander.init();
            }
        }

        // Wastebin is always re-rendered (reflects mutable state)
        if (mode === 'wastebin') {
            if (!this._wastebinEl) {
                this._wastebinEl = document.createElement('div');
                this._wastebinEl.style.height = '100%';
                appEl.appendChild(this._wastebinEl);
            }
            this.wasteBinSelected.clear();
            this.wasteBinLastClickedIndex = -1;
            this.renderWasteBin(this._wastebinEl);
        }

        // Show active, hide others
        if (this._browseEl) this._browseEl.style.display = mode === 'browse' ? '' : 'none';
        if (this._commanderEl) this._commanderEl.style.display = mode === 'commander' ? '' : 'none';
        if (this._wastebinEl) this._wastebinEl.style.display = mode === 'wastebin' ? '' : 'none';

        // Transition animation
        const activeEl = mode === 'browse' ? this._browseEl :
                         mode === 'commander' ? this._commanderEl : this._wastebinEl;
        if (activeEl && prevMode !== mode) {
            const cls = currIdx > prevIdx ? 'mode-enter-right' : 'mode-enter-left';
            activeEl.classList.remove('mode-enter-right', 'mode-enter-left');
            void activeEl.offsetWidth; // force reflow
            activeEl.classList.add(cls);
            activeEl.addEventListener('animationend', () => activeEl.classList.remove(cls), { once: true });
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

            // Reload panes so deleted entries are removed from browse/commander
            if (this.browsePane) this.browsePane.load(this.browsePane.path);
            if (this.commander) {
                if (this.commander.leftPane)  this.commander.leftPane.load(this.commander.leftPane.path);
                if (this.commander.rightPane) this.commander.rightPane.load(this.commander.rightPane.path);
            }

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
        countEl.style.display = count > 0 ? 'inline' : 'none';
    },

    renderWasteBin(appEl) {
        const items = Array.from(this.wasteBin.entries());

        if (items.length === 0) {
            appEl.innerHTML = '<div class="browse-container"><div class="wastebin-empty">No photos marked yet. Use Select to browse and mark photos.</div></div>';
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
                <img src="${API.thumbnailURL(path, 200)}" alt="${entry.name}" loading="lazy" onload="this.classList.add('img-loaded')">
                <div class="item-name">${entry.name}</div>
            </div>`;
        });
        const grid = `<div class="grid">${gridItems.join('')}</div>`;

        appEl.innerHTML = `<div class="browse-container"><div class="browse-header">${header}${actions}</div><div class="browse-content">${grid}</div></div>`;

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

        // Save display state and scroll positions before hiding
        const savedDisplay = new Map();
        existingChildren.forEach(el => savedDisplay.set(el, el.style.display));
        const scrollPositions = new Map();
        appEl.querySelectorAll('.browse-container').forEach(el => {
            scrollPositions.set(el, el.scrollTop);
        });

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
            savedDisplay.forEach((display, el) => { el.style.display = display; });
            // Restore scroll positions
            scrollPositions.forEach((top, el) => { el.scrollTop = top; });
        };
        this.viewer.onDelete = (path) => {
            const entries = pane.entries || [];
            const dir = pane.path || '';
            this.markForDeletion([path], entries, dir);
        };
        this.viewer.open(imagePath, images);
    },

    handleSelectionChange(selected) {
        // Selection changes don't drive the info panel; focus does.
    },

    handleFocusChange(path) {
        if (!this.infoPanel || !this.infoPanel.expanded) return;
        if (path) {
            this.infoPanel.loadInfo(path);
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

        // Escape to go up in browse mode
        if (e.key === 'Escape' && this.mode === 'browse' && this.browsePane) {
            if (document.querySelector('.viewer')) return; // Don't navigate while viewing
            e.preventDefault();
            const parts = this.browsePane.path.split('/').filter(Boolean);
            parts.pop();
            const parentPath = parts.join('/');
            this.browsePane.load(parentPath);
            this.currentBrowsePath = parentPath;
        }

        // Escape to go up in commander mode
        if (e.key === 'Escape' && this.mode === 'commander' && this.commander) {
            e.preventDefault();
            const pane = this.commander.getActivePane();
            const parts = pane.path.split('/').filter(Boolean);
            parts.pop();
            pane.load(parts.join('/'));
        }

        // Backspace to mark selected (or focused) files for waste bin (browse mode)
        if (e.key === 'Backspace' && this.mode === 'browse' && this.browsePane) {
            e.preventDefault(); // prevent Safari back-navigation before any early returns
            if (document.querySelector('.viewer')) return; // viewer's own handler takes over
            const targets = this.browsePane.getActionableFiles();
            if (targets.length === 0) return;
            this.markForDeletion(targets, this.browsePane.entries, this.browsePane.path);
            this.browsePane.selected.clear();
            this.browsePane.updateSelectionClasses();
            this.browsePane.updateMarkedForDeletion();
        }

        // Backspace to mark for deletion in commander mode
        if (e.key === 'Backspace' && this.mode === 'commander' && this.commander) {
            e.preventDefault(); // prevent Safari back-navigation before any early returns
            const targets = this.commander.getActivePane().getActionableFiles();
            if (targets.length === 0) return;
            this.commander.doDelete();
        }

        // I to toggle info panel in browse mode
        if ((e.key === 'i' || e.key === 'I') && this.mode === 'browse' && this.infoPanel) {
            if (document.querySelector('.viewer')) return;
            e.preventDefault();
            this.infoPanel.toggle();
            if (this.infoPanel.expanded && this.browsePane) {
                this.browsePane._notifyFocusChange();
            }
        }

        // Delete key to mark selected (or focused) files for waste bin
        if (e.key === 'Delete' && this.mode === 'browse' && this.browsePane) {
            if (document.querySelector('.viewer')) return;
            const targets = this.browsePane.getActionableFiles();
            if (targets.length === 0) return;
            e.preventDefault();
            this.markForDeletion(targets, this.browsePane.entries, this.browsePane.path);
            this.browsePane.selected.clear();
            this.browsePane.updateSelectionClasses();
            this.browsePane.updateMarkedForDeletion();
        }
        if (e.key === 'Delete' && this.mode === 'commander' && this.commander) {
            const targets = this.commander.getActivePane().getActionableFiles();
            if (targets.length === 0) return;
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
                this.renderWasteBin(this._wastebinEl);
            }
        }

        // Cmd/Ctrl+D to mark for deletion
        if (modKey && (e.key === 'd' || e.key === 'D') && !e.shiftKey && !e.altKey) {
            if (this.mode === 'browse' && this.browsePane && !document.querySelector('.viewer')) {
                e.preventDefault();
                const targets = this.browsePane.getActionableFiles();
                if (targets.length > 0) {
                    this.markForDeletion(targets, this.browsePane.entries, this.browsePane.path);
                    this.browsePane.selected.clear();
                    this.browsePane.updateSelectionClasses();
                    this.browsePane.updateMarkedForDeletion();
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
            else if (e.key === '2') { e.preventDefault(); this.setMode('wastebin'); }
            else if (e.key === '3') { e.preventDefault(); this.setMode('commander'); }
        }

        // H to toggle interface visibility (bare key, no modifier)
        if ((e.key === 'h' || e.key === 'H') && !e.metaKey && !e.ctrlKey && !e.altKey) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') return;
            if (document.querySelector('.viewer')) return; // viewer handles it
            e.preventDefault();
            this.toggleUIVisibility();
        }
    },
};

document.addEventListener('DOMContentLoaded', () => App.init());
