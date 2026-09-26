// App — orchestration: init, mode switching, modal wiring, viewer

// The places a phone is for: looking at photos and seeing where they went.
const PHONE_PLACES = new Set(['browse', 'library', 'published']);

const App = {
    mode: 'browse',
    locationModal: null,
    batchRenameModal: null,
    exportModal: null,
    slideshowModal: null,
    browsePane: null,
    infoPanel: null,
    organize: null,
    viewer: null,
    currentBrowsePath: '',
    _organizePreselect: null,
    config: null,
    toolsStatus: null,
    _browseEl: null,
    _organizeEl: null,
    _wastebinEl: null,
    _libraryEl: null,
    _libraryTab: null,
    _galleriesEl: null,
    _galleriesPane: null,
    _destinationsEl: null,
    _destinationsPane: null,
    _settingsEl: null,
    _settingsPane: null,
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

        this.initNav();
        this.statusLine = new StatusLine(document.getElementById('status-line'));

        this.keyboard.attach();
        this.theme.init();
        this._initUIVisibility();

        this._updateLibraryButton();

        Promise.all([
            API.config(),
            API.toolsCheck().catch(() => ({ exiftool: false })),
        ]).then(([cfg, tools]) => {
            this.config = cfg;
            this.toolsStatus = tools;
            this.currentBrowsePath = cfg.startPath || '';
            this.setMode(this._modeFromHash(), { replaceHistory: true });
        }).catch(() => {
            this.setMode(this._modeFromHash(), { replaceHistory: true });
        });
    },

    // —— Navigation: places, not steps (ADR-0028) ——
    //
    // Every place has an address, so the back button, deep links and
    // "open in a new tab" all work. The nav entries are real links; the
    // click handler only exists to avoid a reload.

    NAV: {
        browse: { hash: 'folders', id: 'mode-browse', key: '1' },
        wastebin: { hash: 'marked', id: 'mode-wastebin', key: '2' },
        organize: { hash: 'organize', id: 'mode-organize', key: '3' },
        library: { hash: 'libraries', id: 'mode-library', key: '4' },
        published: { hash: 'galleries', id: 'mode-published', key: '5' },
        destinations: { hash: 'destinations', id: 'mode-destinations', key: '6' },
        settings: { hash: 'settings', id: 'mode-settings', key: ',' },
    },

    initNav() {
        for (const [mode, place] of Object.entries(this.NAV)) {
            const el = document.getElementById(place.id);
            if (!el) continue;
            el.title = `${el.querySelector('.nav-text').textContent} (${place.key})`;
            el.addEventListener('click', (e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return; // let the browser open a new tab
                e.preventDefault();
                if (mode === 'published' && this._galleriesPane) this._galleriesPane.setFilterChannel(null);
                this.setMode(mode);
            });
        }

        // The phone's tab bar, and any other link that names a place, point at
        // the same places as the sidebar. A hash link on its own would not
        // fire popstate, so the app would stay where it is.
        for (const el of document.querySelectorAll('.tabbar-item, .desk-only-notice a[data-mode]')) {
            el.addEventListener('click', (e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
                e.preventDefault();
                this.setMode(el.dataset.mode);
            });
        }

        window.addEventListener('popstate', () => this.setMode(this._modeFromHash(), { fromHistory: true }));

        const collapseBtn = document.getElementById('sidebar-collapse');
        collapseBtn.addEventListener('click', () => this.toggleSidebar());
        this._applySidebarState(localStorage.getItem('sidebar-collapsed') === '1');
    },

    _modeFromHash() {
        const hash = location.hash.replace(/^#/, '');
        const entry = Object.entries(this.NAV).find(([, place]) => place.hash === hash);
        return entry ? entry[0] : 'browse';
    },

    toggleSidebar() {
        const collapsed = !document.querySelector('.shell').classList.contains('sidebar-collapsed');
        this._applySidebarState(collapsed);
        localStorage.setItem('sidebar-collapsed', collapsed ? '1' : '0');
        // The justified layout measures its container, so it has to re-pack.
        if (this.browsePane && this.browsePane.view === 'justified') {
            this.browsePane._justifiedRenderer.scheduleRelayout();
        }
    },

    _applySidebarState(collapsed) {
        document.querySelector('.shell').classList.toggle('sidebar-collapsed', collapsed);
        const btn = document.getElementById('sidebar-collapse');
        btn.setAttribute('aria-expanded', collapsed ? 'false' : 'true');
        btn.setAttribute('aria-label', collapsed ? 'Expand sidebar' : 'Collapse sidebar');
        btn.title = `${collapsed ? 'Expand' : 'Collapse'} sidebar (\\)`;
        btn.querySelector('.collapse-chevron').setAttribute('d', collapsed ? 'M8.5 6l2 2-2 2' : 'M11 6l-2 2 2 2');
    },

    // One sub-entry per library, so a library is one click away from anywhere.
    async _renderLibraryNav() {
        const wrap = document.getElementById('nav-libraries');
        if (!wrap) return;
        let libs = [];
        try {
            libs = await LibraryAPI.list();
        } catch {
            wrap.innerHTML = '';
            return;
        }
        wrap.innerHTML = libs.map(lib => `
            <a class="nav-item nav-sub" href="#libraries" data-library-id="${lib.id}" title="${escapeHtml(lib.name)}">
                <span class="nav-text">${escapeHtml(lib.name)}</span>
            </a>`).join('');
        for (const el of wrap.querySelectorAll('[data-library-id]')) {
            el.addEventListener('click', (e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
                e.preventDefault();
                this.openLibrary(el.dataset.libraryId);
            });
        }
        this._markCurrentLibraryNav();
    },

    // Libraries, filtered to the photos a gallery or destination holds.
    showPhotos(criteria) {
        this.setMode('library');
        if (this._libraryTab) this._libraryTab.showFiltered(criteria);
    },

    openLibrary(libraryId) {
        this.setMode('library');
        if (this._libraryTab) this._libraryTab.openLibraryById(libraryId);
    },

    _markCurrentLibraryNav() {
        const currentId = this._libraryTab?.currentLibrary?.id;
        for (const el of document.querySelectorAll('#nav-libraries [data-library-id]')) {
            const isCurrent = this.mode === 'library' && el.dataset.libraryId === String(currentId ?? '');
            el.toggleAttribute('aria-current', isCurrent);
            if (isCurrent) el.setAttribute('aria-current', 'page');
        }
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

    // Work that is usually quick but can take a while (the photos of a folder
    // of folders) says so in the hint line, only when it is slow, and for as
    // long as it runs.
    async whileSlow(text, work) {
        let shown = false;
        const timer = setTimeout(() => {
            shown = true;
            this.showToast(text);
            clearTimeout(this._toastTimer);
        }, ACTIVITY_DELAY_MS);
        try {
            return await work();
        } finally {
            clearTimeout(timer);
            if (shown) document.getElementById('ui-hint')?.classList.remove('visible');
        }
    },


    setMode(mode, { fromHistory = false, replaceHistory = false } = {}) {
        if (this.viewer) {
            this.viewer.close();
            this.viewer = null;
        }

        const prevMode = this.mode;

        if (prevMode === 'organize' && this.organize) {
            this.currentBrowsePath = this.organize.pane.path;
        }

        this.mode = mode;

        // Mark where we are. There is no "done" or "next" — these are places.
        for (const [navMode, place] of Object.entries(this.NAV)) {
            const el = document.getElementById(place.id);
            if (!el) continue;
            if (navMode === mode) el.setAttribute('aria-current', 'page');
            else el.removeAttribute('aria-current');
        }
        for (const el of document.querySelectorAll('.tabbar-item')) {
            if (el.dataset.mode === mode) el.setAttribute('aria-current', 'page');
            else el.removeAttribute('aria-current');
        }
        // A phone browses; it does not cull, organise or configure. Those
        // places say so rather than showing controls that cannot work there.
        document.body.classList.toggle('desk-only-place', !PHONE_PLACES.has(mode));

        const hash = '#' + this.NAV[mode].hash;
        if (!fromHistory && location.hash !== hash) {
            if (replaceHistory) history.replaceState(null, '', hash);
            else history.pushState(null, '', hash);
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
                // One bar for every action on a selection. "Add to gallery"
                // is missing on purpose: collecting needs library photos, and
                // a folder is not a library.
                this._browseSelectionBar = new SelectionBar(this._browseEl, {
                    actions: ['export', 'rename', 'location', 'mark'],
                    onAction: (action) => this.runSelectionAction(action, this.browsePane),
                });
                this.browsePane.load(this.currentBrowsePath);
            }
        }

        if (mode === 'organize') {
            if (!this._organizeEl) {
                this._organizeEl = document.createElement('div');
                this._organizeEl.style.height = '100%';
                appEl.appendChild(this._organizeEl);
                this.organize = new OrganizePane(this._organizeEl, this.currentBrowsePath, {
                    preselectFiles: this._organizePreselect,
                    onImageClick: (path, pane) => this.openViewer(path, pane),
                    onToolInvoke: (params) => this.handleToolInvoke(params),
                });
                this._organizePreselect = null;
                this.organize.init();
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

        if (mode === 'settings') {
            if (!this._settingsEl) {
                this._settingsEl = document.createElement('div');
                this._settingsEl.style.height = '100%';
                appEl.appendChild(this._settingsEl);
                this._settingsPane = new SettingsPane(this._settingsEl);
            }
            this._settingsPane.render();
        }

        if (mode === 'destinations') {
            if (!this._destinationsEl) {
                this._destinationsEl = document.createElement('div');
                this._destinationsEl.style.height = '100%';
                appEl.appendChild(this._destinationsEl);
                this._destinationsPane = new DestinationsPane(this._destinationsEl);
            }
            this._destinationsPane.render();
        }

        if (mode === 'published') {
            if (!this._galleriesEl) {
                this._galleriesEl = document.createElement('div');
                this._galleriesEl.style.height = '100%';
                appEl.appendChild(this._galleriesEl);
                this._galleriesPane = new GalleriesPane(this._galleriesEl);
            }
            this._galleriesPane.render();
        }

        if (this._browseEl) this._browseEl.style.display = mode === 'browse' ? '' : 'none';
        if (this._organizeEl) this._organizeEl.style.display = mode === 'organize' ? '' : 'none';
        if (this._wastebinEl) this._wastebinEl.style.display = mode === 'wastebin' ? '' : 'none';
        if (this._libraryEl) this._libraryEl.style.display = mode === 'library' ? '' : 'none';
        if (this._galleriesEl) this._galleriesEl.style.display = mode === 'published' ? '' : 'none';
        if (this._destinationsEl) this._destinationsEl.style.display = mode === 'destinations' ? '' : 'none';
        if (this._settingsEl) this._settingsEl.style.display = mode === 'settings' ? '' : 'none';

        this._markCurrentLibraryNav();
    },

    _refreshPanes() {
        if (this.browsePane) this.browsePane.load(this.browsePane.path);
        if (this.organize) this.organize.reload();
    },

    // Opens one destination's settings — used by the Galleries screen.
    showDestination(slug) {
        this.setMode('destinations');
        if (this._destinationsPane) this._destinationsPane.openBySlug(slug);
    },

    // Opens Galleries filtered to one destination — used by the channels
    // list's status link.
    showGalleriesForChannel(channelSlug) {
        this.setMode('published');
        if (this._galleriesPane) {
            this._galleriesPane.setFilterChannel(channelSlug);
            this._galleriesPane._load();
        }
    },

    // Public delegation methods — referenced by renderers and Organize

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
            if (pane.updateMarkedForDeletion) pane.updateMarkedForDeletion();
        };
        this.viewer.onDelete = (path) => {
            const libMeta = pane.getLibraryMeta?.(path);
            const photoMeta = libMeta ? { [path]: libMeta } : null;
            this.wastebin.mark([path], pane.entries || [], pane.path || '', photoMeta);
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
            const n = pane.selectedDirs.size;
            const allPaths = await this.whileSlow(`Collecting the photos of ${n} folder${n !== 1 ? 's' : ''}…`, async () => {
                const paths = [];
                for (const dirPath of pane.selectedDirs) paths.push(...await pane.fetchRecursivePhotoPaths(dirPath));
                return paths;
            });
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
        // Selection changes don't drive the info panel; focus does. They do
        // drive the selection bar, which is the only place a selection's
        // actions live.
        this._browseSelectionBar?.update(selected.length, {
            // Renaming one file at a time is a different dialog than renaming
            // by metadata, but both are reachable — nothing is disabled here
            // without a reason the tooltip can state.
        });
    },

    // Every action on a selection goes through here, from the bar and from the
    // keyboard alike, so the two can never drift apart.
    runSelectionAction(action, pane) {
        if (!pane) return;
        const files = pane.getSelectedFiles();
        if (action === 'clear') {
            pane.selection.clear();
            pane.updateSelectionClasses();
            pane.onSelectionChange?.([]);
            return;
        }
        if (!files.length) return;
        if (action === 'mark') {
            this.markForDeletion(files, pane.entries, pane.path);
            pane.updateMarkedForDeletion();
            return;
        }
        const tool = action === 'export' ? 'export'
            : action === 'location' ? 'set-location'
            // Always the batch dialog, even for one photo: it previews the
            // result, and the single-file path was a browser prompt().
            : action === 'rename' ? 'batch-rename'
            : null;
        if (!tool) return;
        this.handleToolInvoke({ tool, files, path: pane.path });
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

    // Libraries stays visible even with none configured: it is the only place
    // where the first one gets created. Only the per-library entries come and go.
    async _updateLibraryButton() {
        this._renderLibraryNav();
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

        const wasCreated = !!this._organizeEl;
        this.currentBrowsePath = relPath;

        if (!wasCreated) {
            this._organizePreselect = preselectNames;
        }

        this.setMode('organize');

        if (wasCreated) this.organize.load(relPath, preselectNames);
    },

    reloadLibraryPane() {
        const pane = this._libraryTab?._pane;
        if (pane) pane.load(pane.path);
    },

    getActiveBrowsePane() {
        if (this.mode === 'browse') return this.browsePane;
        if (this.mode === 'organize' && this.organize) return this.organize.pane;
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
                } else if (tool === 'batch-rename') {
            // Files from SearchResultPane are absolute pathHints; files from
            // LibraryPane are relative to the library source dir. The batch rename
            // API validates against the server's browse boundary, so an absolute
            // sourcePath must be made relative to that boundary specifically —
            // not to filesystem root "/" (only coincidentally the same when the
            // server has no navigation restriction, e.g. desktop installs).
            let srcPrefix = '';
            if (sourcePath) {
                srcPrefix = absPathRelativeToBoundary(sourcePath, this.config?.boundary);
                if (srcPrefix === null) {
                    this.showToast('This library\'s folder is outside the server\'s browse root, so batch rename cannot reach it.');
                    if (onDone) onDone();
                    return;
                }
            }
            const resolvedFiles = files.map(f =>
                f.startsWith('/') ? f.slice(1) : (srcPrefix ? `${srcPrefix}/${f}` : f)
            );
            this.batchRenameModal.open(resolvedFiles, (libraryUpdated) => {
                if (pane) pane.load(pane.path);
                if (libraryUpdated) this.reloadLibraryPane();
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
                'lib-regen-missing': 'Generating missing previews for',
                'lib-rebuild-all': 'Rebuilding the previews of',
            };
            // The run can take minutes, so its progress lives in the status
            // line at the foot of the sidebar, which stays while you move on.
            App.showToast(`${labelMap[tool]} this folder. Progress shows at the foot of the sidebar.`);
            const onProgress = (p) => {
                if (p.finished && onDone) onDone();
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
                App.showToast(`It did not start: ${e.message}`);
                if (onDone) onDone();
            });
        }
    },
};

document.addEventListener('DOMContentLoaded', () => App.init());
