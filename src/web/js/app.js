// App — orchestration: init, mode switching, modal wiring, viewer

const App = {
    mode: 'browse',
    locationModal: null,
    batchRenameModal: null,
    exportModal: null,
    slideshowModal: null,
    browsePane: null,
    infoPanel: null,
    commander: null,
    viewer: null,
    currentBrowsePath: '',
    _commanderPreselect: null,
    config: null,
    toolsStatus: null,
    _browseEl: null,
    _commanderEl: null,
    _wastebinEl: null,
    _libraryEl: null,
    _libraryTab: null,
    uiHidden: false,
    wastebin: null,
    theme: null,
    keyboard: null,
    aboutModal: null,

    init() {
        this.wastebin = new Wastebin();
        this.theme = new ThemeManager(this);
        this.keyboard = new GlobalKeyboard(this);
        this.aboutModal = new AboutModal();

        const aboutTrigger = document.getElementById('about-trigger');
        aboutTrigger.addEventListener('click', () => this.aboutModal.open(this.config?.version));
        aboutTrigger.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); this.aboutModal.open(this.config?.version); }
        });

        this.viewer = new Viewer(document.getElementById('app'));

        document.getElementById('mode-browse').addEventListener('click', () => this.setMode('browse'));
        document.getElementById('mode-commander').addEventListener('click', () => this.setMode('commander'));
        document.getElementById('mode-wastebin').addEventListener('click', () => this.setMode('wastebin'));
        document.getElementById('mode-library').addEventListener('click', () => this.setMode('library'));
        document.getElementById('mode-library-create').addEventListener('click', (e) => {
            e.stopPropagation();
            this.setMode('library');
            if (this._libraryTab) this._libraryTab._showCreateDialog();
        });

        document.getElementById('mode-browse').title = `Select (1)`;
        document.getElementById('mode-wastebin').title = `Review (2)`;
        document.getElementById('mode-commander').title = `Organize (3)`;
        document.getElementById('mode-library').title = `Libraries (4)`;

        this.keyboard.attach();
        this.theme.init();
        this._initUIVisibility();
        this.initSettingsMenu();

        this._updateLibraryButton();

        Promise.all([
            API.config(),
            API.toolsCheck().catch(() => ({ exiftool: false })),
        ]).then(([cfg, tools]) => {
            this.config = cfg;
            this.toolsStatus = tools;
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
        if (this._hideUiToggle) this._hideUiToggle.setState(!this.uiHidden);
        if (this.uiHidden) this._showUIHint();
    },

    _showUIHint() {
        const hint = document.getElementById('ui-hint');
        if (!hint) return;
        hint.textContent = 'Press H to show the interface again';
        hint.classList.add('visible');
        clearTimeout(this._uiHintTimer);
        this._uiHintTimer = setTimeout(() => hint.classList.remove('visible'), 3000);
    },

    showToast(msg) {
        const hint = document.getElementById('ui-hint');
        if (!hint) return;
        hint.textContent = msg;
        hint.classList.add('visible');
        clearTimeout(this._toastTimer);
        this._toastTimer = setTimeout(() => hint.classList.remove('visible'), 3000);
    },

    initSettingsMenu() {
        const btn = document.getElementById('settings-btn');
        const menu = document.getElementById('settings-menu');

        const loadCacheInfo = () => {
            API.cacheInfo().then(info => {
                const mb = (info.bytes / 1048576).toFixed(1);
                document.getElementById('settings-cache-size').textContent = mb + ' MB';
                document.getElementById('settings-cache-path').textContent = info.path;
            }).catch(() => {
                document.getElementById('settings-cache-size').textContent = 'unavailable';
            });
        };

        const { close: closeMenu } = Dropdown.init(btn, menu, { onOpen: loadCacheInfo });

        menu.addEventListener('click', (e) => {
            const themeBtn = e.target.closest('[data-theme-set]');
            if (themeBtn) {
                const preference = themeBtn.dataset.themeSet;
                localStorage.setItem('theme', preference);
                this.theme._apply(preference);
                this.theme._updateButtons(preference);
            }
        });

        const savedQ = localStorage.getItem('thumbnail-quality') || 'standard';
        this._qualityToggle = Toggle.create(document.getElementById('settings-thumb-quality-wrap'), {
            initial: savedQ === 'high',
            labelOn: 'High',
            labelOff: 'Standard',
            onChange: (on) => this.theme.setQuality(on ? 'high' : 'standard')
        });

        this._hideUiToggle = Toggle.create(document.getElementById('settings-hide-ui-wrap'), {
            initial: !this.uiHidden,
            onChange: () => {
                this.toggleUIVisibility();
                closeMenu();
            }
        });

        document.getElementById('settings-clear-cache').addEventListener('click', () => {
            const btn = document.getElementById('settings-clear-cache');
            btn.disabled = true;
            API.cacheClear().then(loadCacheInfo).finally(() => { btn.disabled = false; });
        });

        document.getElementById('settings-check-deps').addEventListener('click', () => {
            closeMenu();
            new DepsModal().open(this.toolsStatus);
        });
    },

    setMode(mode) {
        if (this.viewer) {
            this.viewer.close();
            this.viewer = null;
        }

        const prevMode = this.mode;

        if (prevMode === 'commander' && this.commander) {
            this.currentBrowsePath = this.commander.getActivePane().path;
        }

        this.mode = mode;

        const stepOrder = { browse: 0, wastebin: 1, commander: 2, library: 3 };
        const prevIdx = stepOrder[prevMode] ?? 0;
        const currIdx = stepOrder[mode];

        const steps = [
            { el: document.getElementById('mode-browse'), idx: 0 },
            { el: document.getElementById('mode-wastebin'), idx: 1 },
            { el: document.getElementById('mode-commander'), idx: 2 },
            { el: document.getElementById('mode-library'), idx: 3 },
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
                    onFocusChange: (path, type) => this.handleFocusChange(path, type),
                    onToolInvoke: (params) => this.handleToolInvoke(params),
                    onSlideshowInvoke: () => this.handleSlideshowInvoke(),
                });
                this.infoPanel = new InfoPanel(this._browseEl.querySelector('#info-panel-container'));
                this.infoPanel.onToggle = () => {
                    if (this.browsePane && this.browsePane.view === 'justified') {
                        this.browsePane._justifiedRenderer.scheduleRelayout();
                    }
                    if (this.infoPanel.expanded) {
                        const filePath = this.browsePane ? this.browsePane.getFocusedFile() : null;
                        const dirPath = this.browsePane ? this.browsePane.getFocusedDir() : null;
                        if (filePath) this.infoPanel.loadInfo(filePath);
                        else if (dirPath) this.infoPanel.loadFolderInfo(dirPath);
                        else this.infoPanel.clear();
                    }
                };
                this.infoPanel.onDirNavigate = (path) => {
                    if (this.browsePane) {
                        this.browsePane.load(path);
                        this.currentBrowsePath = path;
                    }
                };
                this.browsePane.load(this.currentBrowsePath);
            }
        }

        if (mode === 'commander') {
            if (!this._commanderEl) {
                this._commanderEl = document.createElement('div');
                this._commanderEl.style.height = '100%';
                appEl.appendChild(this._commanderEl);
                this.commander = new Commander(this._commanderEl, this.currentBrowsePath, {
                    preselectFiles: this._commanderPreselect,
                });
                this._commanderPreselect = null;
                this.commander.onImageClick = (path, pane) => this.openViewer(path, pane);
                this.commander.onToolInvoke = (params) => this.handleToolInvoke(params);
                this.commander.init();
            }
        }

        if (mode === 'wastebin') {
            if (!this._wastebinEl) {
                this._wastebinEl = document.createElement('div');
                this._wastebinEl.style.height = '100%';
                appEl.appendChild(this._wastebinEl);
            }
            this.wastebin.selected.clear();
            this.wastebin.render(this._wastebinEl, () => this._refreshPanes());
        }

        if (mode === 'library') {
            if (!this._libraryEl) {
                this._libraryEl = document.createElement('div');
                this._libraryEl.style.height = '100%';
                appEl.appendChild(this._libraryEl);
                this._libraryTab = new LibraryTab(this._libraryEl);
            }
            this._libraryTab.render();
        }

        if (this._browseEl) this._browseEl.style.display = mode === 'browse' ? '' : 'none';
        if (this._commanderEl) this._commanderEl.style.display = mode === 'commander' ? '' : 'none';
        if (this._wastebinEl) this._wastebinEl.style.display = mode === 'wastebin' ? '' : 'none';
        if (this._libraryEl) this._libraryEl.style.display = mode === 'library' ? '' : 'none';

        const activeEl = mode === 'browse' ? this._browseEl :
                         mode === 'commander' ? this._commanderEl :
                         mode === 'library' ? this._libraryEl : this._wastebinEl;
        if (activeEl && prevMode !== mode) {
            const cls = currIdx > prevIdx ? 'mode-enter-right' : 'mode-enter-left';
            activeEl.classList.remove('mode-enter-right', 'mode-enter-left');
            void activeEl.offsetWidth;
            activeEl.classList.add(cls);
            activeEl.addEventListener('animationend', () => activeEl.classList.remove(cls), { once: true });
        }
    },

    _refreshPanes() {
        if (this.browsePane) this.browsePane.load(this.browsePane.path);
        if (this.commander) {
            if (this.commander.leftPane) this.commander.leftPane.load(this.commander.leftPane.path);
            if (this.commander.rightPane) this.commander.rightPane.load(this.commander.rightPane.path);
        }
    },

    // Public delegation methods — referenced by renderers and commander

    markForDeletion(selectedPaths, entries, currentDir) {
        this.wastebin.mark(selectedPaths, entries, currentDir);
    },

    restoreFromWasteBin(paths) {
        this.wastebin.restore(paths);
    },

    async permanentlyDelete(paths) {
        await this.wastebin.permanentlyDelete(paths, () => this._refreshPanes());
    },

    isMarkedForDeletion(path) {
        return this.wastebin.isMarked(path);
    },

    openViewer(imagePath, pane) {
        let images = pane.getImageEntries().map(e => pane.fullPath(e.name));
        if (pane.selection.selected.size >= 2) {
            images = images.filter(path => pane.selection.selected.has(path));
        }
        const appEl = document.getElementById('app');
        const existingChildren = Array.from(appEl.children);
        const savedDisplay = new Map();
        existingChildren.forEach(el => savedDisplay.set(el, el.style.display));
        const scrollPositions = new Map();
        appEl.querySelectorAll('.browse-container').forEach(el => {
            scrollPositions.set(el, el.scrollTop);
        });
        existingChildren.forEach(el => el.style.display = 'none');

        const viewerEl = document.createElement('div');
        viewerEl.id = 'viewer-container';
        viewerEl.style.height = '100%';
        appEl.appendChild(viewerEl);

        this.viewer = new Viewer(viewerEl, {
            imageURLFn: pane.viewerImageURL ? (p) => pane.viewerImageURL(p) : undefined,
            thumbURLFn:  pane.viewerThumbURL  ? (p) => pane.viewerThumbURL(p)  : undefined,
            infoLoadFn:  pane.viewerLoadInfo  ? (p, ip) => pane.viewerLoadInfo(p, ip)  : undefined,
        });
        this.viewer.onClose = () => {
            viewerEl.remove();
            savedDisplay.forEach((display, el) => { el.style.display = display; });
            scrollPositions.forEach((top, el) => { el.scrollTop = top; });
        };
        this.viewer.onDelete = (path) => {
            this.wastebin.mark([path], pane.entries || [], pane.path || '');
        };
        this.viewer.open(imagePath, images);
    },

    async handleSlideshowInvoke(pane) {
        pane = pane || this.browsePane;
        if (!pane) return;
        // Resolve each path to its correct image URL (LibraryPane overrides viewerImageURL
        // to use /api/library/{id}/photo/{photoID}; BrowsePane falls back to API.imageURL).
        const toURL = p => pane.viewerImageURL ? pane.viewerImageURL(p) : API.imageURL(p);
        let images;
        if (pane.selectedDirs?.size > 0) {
            const allPaths = [];
            for (const dirPath of pane.selectedDirs) {
                const paths = await pane.fetchRecursivePhotoPaths(dirPath);
                allPaths.push(...paths);
            }
            images = allPaths.map(toURL);
        } else if (pane.selection.selected.size > 0) {
            images = Array.from(pane.selection.selected).map(toURL);
        } else {
            images = pane.getImageEntries().map(e => toURL(pane.fullPath(e.name)));
        }
        if (images.length === 0) return;
        if (!this.slideshowModal) this.slideshowModal = new SlideshowModal();
        this.slideshowModal.onStart = (imgs, opts) => this.openSlideshow(imgs, opts);
        this.slideshowModal.open(images);
    },

    openSlideshow(images, options) {
        const appEl = document.getElementById('app');
        const existingChildren = Array.from(appEl.children);
        const savedDisplay = new Map();
        existingChildren.forEach(el => savedDisplay.set(el, el.style.display));
        const scrollPositions = new Map();
        appEl.querySelectorAll('.browse-container').forEach(el => scrollPositions.set(el, el.scrollTop));
        existingChildren.forEach(el => el.style.display = 'none');

        document.body.classList.add('slideshow-active');

        const slideshowEl = document.createElement('div');
        slideshowEl.id = 'slideshow-container';
        slideshowEl.style.height = '100%';
        appEl.appendChild(slideshowEl);

        const player = new SlideshowPlayer(slideshowEl);
        player.onClose = () => {
            slideshowEl.remove();
            document.body.classList.remove('slideshow-active');
            savedDisplay.forEach((display, el) => { el.style.display = display; });
            scrollPositions.forEach((top, el) => { el.scrollTop = top; });
        };
        player.open(images, options);
    },

    handleSelectionChange(selected) {
        // Selection changes don't drive the info panel; focus does.
    },

    handleFocusChange(path, type) {
        if (!this.infoPanel || !this.infoPanel.expanded) return;
        if (path && type === 'image') {
            this.infoPanel.loadInfo(path);
        } else if (path && type === 'dir') {
            this.infoPanel.loadFolderInfo(path);
        } else {
            this.infoPanel.clear();
        }
    },

    async _updateLibraryButton() {
        const btn = document.getElementById('mode-library');
        if (!btn) return;
        try {
            const libs = await LibraryAPI.list();
            btn.style.display = libs.length === 0 ? 'none' : '';
        } catch {
            btn.style.display = 'none';
        }
    },

    refreshLibraryVisibility() {
        this._updateLibraryButton();
    },

    openCommanderAt(physicalDir, preselectNames) {
        const boundary = (this.config && this.config.boundary)
            ? this.config.boundary.replace(/\/$/, '') : '';
        let relPath;
        const sp = physicalDir.replace(/\/$/, '');
        if (sp === boundary) {
            relPath = '';
        } else if (!boundary) {
            relPath = sp.replace(/^\//, '');
        } else if (sp.startsWith(boundary + '/')) {
            relPath = sp.substring(boundary.length + 1);
        } else {
            return;
        }

        const wasCreated = !!this._commanderEl;
        this.currentBrowsePath = relPath;

        if (!wasCreated) {
            this._commanderPreselect = preselectNames;
        }

        this.setMode('commander');

        if (wasCreated) {
            const pane = this.commander.getActivePane();
            if (preselectNames && preselectNames.length) pane.primePreselect(preselectNames);
            pane.load(relPath);
        }
    },

    reloadLibraryPane() {
        const pane = this._libraryTab?._pane;
        if (pane) pane.load(pane.path);
    },

    getActiveBrowsePane() {
        if (this.mode === 'browse') return this.browsePane;
        if (this.mode === 'commander' && this.commander) return this.commander.getActivePane();
        if (this.mode === 'library' && this._libraryTab) return this._libraryTab.getActivePaneForKeyboard();
        return null;
    },

    handleToolInvoke({ tool, files, path, sourcePath, onDone }) {
        if (!this.locationModal) this.locationModal = new LocationModal();
        if (!this.batchRenameModal) this.batchRenameModal = new BatchRenameModal();
        const pane = this.getActiveBrowsePane();
        const onSuccess = (changedFiles) => {
            if (pane) pane.notifyFilesChanged(changedFiles);
            if (this.infoPanel && this.infoPanel.expanded && pane) {
                const focused = pane.getFocusedFile();
                if (focused && changedFiles.includes(focused)) {
                    this.infoPanel.loadInfo(focused);
                }
            }
        };
        if (tool === 'make-library') {
            // Ensure LibraryTab exists so it can open the dialog.
            if (!this._libraryEl) {
                this._libraryEl = document.createElement('div');
                this._libraryEl.style.height = '100%';
                document.getElementById('app').appendChild(this._libraryEl);
                this._libraryTab = new LibraryTab(this._libraryEl);
                this._libraryEl.style.display = 'none';
            }
            const absPath = path && !path.startsWith('/') ? '/' + path : (path || '');
            this._libraryTab.openCreateDialogForPath(absPath);
            return;
        } else if (tool === 'set-location') {
            this.locationModal.open(files, onSuccess);
        } else if (tool === 'remove-location') {
            this.locationModal.openRemove(files, onSuccess);
        } else if (tool === 'rename') {
            if (files.length !== 1) return;
            const filePath = files[0];
            const oldName = filePath.split('/').pop();
            const newName = prompt('New name:', oldName);
            if (!newName || !newName.trim() || newName.trim() === oldName) return;
            API.rename(filePath, newName.trim()).then(() => {
                if (pane) pane.load(pane.path);
                if (this.mode === 'commander' && this.commander) {
                    const other = this.commander.getOtherPane();
                    if (other) other.load(other.path);
                }
            }).catch(err => alert('Rename failed: ' + err.message));
        } else if (tool === 'batch-rename') {
            this.batchRenameModal.open(files, () => {
                if (pane) pane.load(pane.path);
                if (this.mode === 'commander' && this.commander) {
                    const other = this.commander.getOtherPane();
                    if (other) other.load(other.path);
                }
            });
        } else if (tool === 'export') {
            if (!this.exportModal) this.exportModal = new ExportModal();
            this.exportModal.open(files, {
                serverRole: this.config?.serverRole ?? false,
                exiftoolAvailable: this.toolsStatus?.exiftool ?? false,
                webpSupport: this.toolsStatus?.webpAvailable ?? false,
                sourcePath: sourcePath || null,
            });
        } else if (tool === 'clear-cache') {
            const prefix = sourcePath || '';
            const toPath = f => prefix ? `${prefix}/${f}` : f;
            const evictPaths = files.length > 0
                ? files.map(toPath)
                : path ? [toPath(path)] : [];
            if (evictPaths.length === 0) { if (onDone) onDone(); return; }
            App.showToast('Clearing cache…');
            API.cacheEvict(evictPaths)
                .then(r => App.showToast(`Cache cleared for ${r.evicted} file${r.evicted !== 1 ? 's' : ''}`))
                .catch(e => App.showToast(`Cache clear failed: ${e.message}`))
                .finally(() => { if (onDone) onDone(); });
        } else if (['lib-scan-new', 'lib-reindex', 'lib-cleanup', 'lib-regen-missing', 'lib-rebuild-all'].includes(tool)) {
            const lib = this._libraryTab?.currentLibrary;
            if (!lib) { if (onDone) onDone(); return; }
            const labelMap = {
                'lib-scan-new': 'Scanning',
                'lib-reindex': 'Indexing',
                'lib-cleanup': 'Checking',
                'lib-regen-missing': 'Generating',
                'lib-rebuild-all': 'Rebuilding',
            };
            App.showToast(`${labelMap[tool]}…`);
            const onProgress = (p) => {
                if (p.finished) {
                    App.showToast(`Done — ${p.done} photo${p.done !== 1 ? 's' : ''}`);
                    if (onDone) onDone();
                } else {
                    const cur = p.current
                        ? (p.current.split('/').pop() || p.current) + (p.parent ? ` in "${p.parent}"` : '') + ' · '
                        : '';
                    App.showToast(`${cur}${p.done}${p.total ? '/' + p.total : ''}`);
                }
            };
            const subfolder = path || '';
            const apiMap = {
                'lib-scan-new':      () => LibraryAPI.scanNew(lib.id, onProgress, subfolder),
                'lib-reindex':       () => LibraryAPI.reindex(lib.id, onProgress, subfolder),
                'lib-cleanup':       () => LibraryAPI.cleanup(lib.id, onProgress, subfolder),
                'lib-regen-missing': () => LibraryAPI.regenMissingPreviews(lib.id, onProgress, subfolder),
                'lib-rebuild-all':   () => LibraryAPI.rebuildAllPreviews(lib.id, onProgress, subfolder),
            };
            apiMap[tool]().catch(e => {
                App.showToast(`Error: ${e.message}`);
                if (onDone) onDone();
            });
        }
    },
};

document.addEventListener('DOMContentLoaded', () => App.init());
