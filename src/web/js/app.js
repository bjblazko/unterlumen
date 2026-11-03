// App — orchestration: init, mode switching, modal wiring, viewer

// The places a phone is for: looking at photos and seeing where they went.
const PHONE_PLACES = new Set(['browse', 'library', 'map', 'timeline', 'statistics', 'people', 'guide', 'licenses', 'about', 'privacy', 'warranty']);

const App = {
    mode: 'library',
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
    _timelineEl: null,
    _timelinePane: null,
    _statsEl: null,
    _statsPane: null,
    _statsRoute: { topic: '', library: '', path: '' },
    _peopleEl: null,
    _peoplePane: null,
    _peopleRoute: { library: '' },
    _galleriesEl: null,
    _galleriesPane: null,
    _destinationsEl: null,
    _destinationsPane: null,
    _settingsEl: null,
    _settingsPane: null,
    _placeBeforeSettings: null,
    uiHidden: false,
    wastebin: null,
    theme: null,
    keyboard: null,

    init() {
        translateStatic();
        this.wastebin = new Wastebin();
        this.theme = new ThemeManager(this);
        this.keyboard = new GlobalKeyboard(this);
        this.search = new SearchBar(document.getElementById('topbar'));
        // The logo leads to About, as the About entry at the foot does.
        const aboutTrigger = document.getElementById('about-trigger');
        aboutTrigger.addEventListener('click', () => this.setMode('about'));
        aboutTrigger.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); this.setMode('about'); }
        });
        WarrantyNotice.show();

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
            ThingWords.load(), // the things' names in the interface's language, before a place shows them
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
        browse: { hash: 'folders', id: 'mode-browse' },
        wastebin: { hash: 'marked', id: 'mode-wastebin' },
        duplicates: { hash: 'duplicates', id: 'mode-duplicates' },
        organize: { hash: 'organize', id: 'mode-organize' },
        library: { hash: 'libraries', id: 'mode-library' },
        metadata: { hash: 'metadata', id: 'mode-metadata' },
        map: { hash: 'map', id: 'mode-map' },
        timeline: { hash: 'timeline', id: 'mode-timeline' },
        people: { hash: 'people', id: 'mode-people' },
        statistics: { hash: 'statistics', id: 'mode-statistics' },
        published: { hash: 'galleries', id: 'mode-published' },
        destinations: { hash: 'destinations', id: 'mode-destinations' },
        settings: { hash: 'settings', id: 'mode-settings', key: ',' },
        about: { hash: 'about', id: 'mode-about' },
        // Reached from the sentence under each place, not from the sidebar.
        guide: { hash: 'guide' },
        // About's topics, listed under it in the sidebar while one is open.
        licenses: { hash: 'licenses' },
        privacy: { hash: 'privacy' },
        warranty: { hash: 'warranty' },
        // Opened on a first start, and from Settings.
        setup: { hash: 'setup' },
    },

    initNav() {
        for (const [mode, place] of Object.entries(this.NAV)) {
            const el = document.getElementById(place.id);
            if (!el) continue;
            const name = el.querySelector('.nav-text').textContent;
            el.title = place.key ? `${name} (${place.key})` : name;
            el.addEventListener('click', (e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return; // let the browser open a new tab
                e.preventDefault();
                this.enterPlace(mode);
            });
        }

        // The phone's tab bar, and any other link that names a place, point at
        // the same places as the sidebar. A hash link on its own would not
        // fire popstate, so the app would stay where it is.
        for (const el of document.querySelectorAll('.tabbar-item, .desk-only-notice a[data-mode]')) {
            el.addEventListener('click', (e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
                e.preventDefault();
                this.enterPlace(el.dataset.mode);
            });
        }

        // Links to a place in running text (placeLink) are built after this
        // runs, so they are routed from the document.
        document.addEventListener('click', (e) => {
            const link = e.target.closest('a.place-link[data-mode], a.library-link, a.folder-link');
            if (!link || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
            e.preventDefault();
            if (link.classList.contains('library-link')) this.openLibrary(link.dataset.libraryId);
            else if (link.classList.contains('folder-link')) this.openFolder(link.dataset.path);
            else this.enterPlace(link.dataset.mode);
        });

        window.addEventListener('popstate', () => this.setMode(this._modeFromHash(), { fromHistory: true }));

        this._renderStatisticsNav();

        const collapseBtn = document.getElementById('sidebar-collapse');
        collapseBtn.addEventListener('click', () => this.toggleSidebar());
        this._applySidebarState(localStorage.getItem('sidebar-collapsed') === '1');
    },

    // enterPlace opens a place at its start, as a click on its name does:
    // Libraries at the overview, Statistics at its topics, People at the
    // people, Galleries for every destination. History keeps where one was.
    enterPlace(mode) {
        if (mode === 'published' && this._galleriesPane) this._galleriesPane.setFilterChannel(null);
        if (mode === 'statistics') this._statsRoute = { ...this._statsRoute, topic: '' };
        if (mode === 'people') this._peopleRoute = {};
        if (mode === 'library') this._libraryTab?.leaveLibrary();
        this.setMode(mode);
    },

    // A place's address is its first segment; Statistics carries a topic
    // and a scope after it, which it keeps in _statsRoute.
    _modeFromHash() {
        const hash = location.hash.replace(/^#/, '').split(/[/?]/)[0];
        const entry = Object.entries(this.NAV).find(([, place]) => place.hash === hash);
        if (!entry) return 'library'; // a bare address starts in Libraries, on the desk as on a phone
        if (entry[0] === 'statistics') this._statsRoute = statsRouteFromHash(location.hash);
        if (entry[0] === 'people') this._peopleRoute = peopleRouteFromHash(location.hash);
        return entry[0];
    },

    _placeHash(mode) {
        if (mode === 'statistics') return statsHash(this._statsRoute);
        if (mode === 'people') return peopleHash(this._peopleRoute);
        return '#' + this.NAV[mode].hash;
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
        btn.setAttribute('aria-label', collapsed ? t('Expand sidebar') : t('Collapse sidebar'));
        btn.title = collapsed ? t('Expand sidebar (\\)') : t('Collapse sidebar (\\)');
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

    // One sub-entry per topic of Statistics, shown only while Statistics is
    // open, so the sidebar stays short elsewhere. Like the libraries' entries,
    // they go when the sidebar is collapsed; the overview's cards lead to them
    // then.
    _renderStatisticsNav() {
        const wrap = document.getElementById('nav-statistics');
        if (!wrap) return;
        wrap.innerHTML = STATS_TOPICS.map(topic => `
            <a class="nav-item nav-sub" href="${statsHash({ topic: topic.id })}" data-stats-topic="${topic.id}">
                <span class="nav-text">${escapeHtml(topic.label)}</span>
            </a>`).join('');
        for (const el of wrap.querySelectorAll('[data-stats-topic]')) {
            el.addEventListener('click', (e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
                e.preventDefault();
                this.openStatistics({ ...this._statsRoute, topic: el.dataset.statsTopic });
            });
        }
    },

    // Statistics at a topic and a scope: { topic, library, path }, the folder
    // relative to the library.
    openStatistics(route) {
        this._statsRoute = { topic: '', library: '', path: '', ...route };
        this.setMode('statistics');
    },

    // People at a library, a group or a person: { library, view, id }.
    openPeople(route, { replace = false } = {}) {
        this._peopleRoute = { library: '', ...route };
        this.setMode('people', { replaceHistory: replace });
    },

    // The topic entries show while Statistics is open, point at the current
    // scope, and the one shown is marked.
    _markCurrentStatisticsNav() {
        const wrap = document.getElementById('nav-statistics');
        if (wrap) wrap.hidden = this.mode !== 'statistics';
        for (const el of document.querySelectorAll('#nav-statistics [data-stats-topic]')) {
            el.href = statsHash({ ...this._statsRoute, topic: el.dataset.statsTopic });
            const isCurrent = this.mode === 'statistics' && el.dataset.statsTopic === this._statsRoute.topic;
            if (isCurrent) el.setAttribute('aria-current', 'page');
            else el.removeAttribute('aria-current');
        }
    },

    // Libraries, filtered to the photos a gallery or destination holds.
    showPhotos(criteria) {
        this.setMode('library');
        if (this._libraryTab) this._libraryTab.showFiltered(criteria);
    },

    // Folders at a folder, given relative to the browse boundary.
    // A first visit builds Folders at currentBrowsePath, so it is already there.
    openFolder(relPath) {
        this.currentBrowsePath = relPath;
        this.setMode('browse');
        if (this.browsePane.path !== relPath) this.browsePane.load(relPath);
    },

    // showInLibrary opens a library at one of its folders with one photo
    // selected — the way from a photo seen elsewhere to where it is kept.
    showInLibrary(libraryId, relDir, name) {
        this.setMode('library');
        this._libraryTab?.showPhoto(libraryId, relDir, name);
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
        if (this.uiHidden) this._showUIHint();
    },

    _showUIHint() {
        const hint = document.getElementById('ui-hint');
        if (!hint) return;
        hint.textContent = t('Press H to show the interface again');
        hint.classList.add('visible');
        clearTimeout(this._uiHintTimer);
        this._uiHintTimer = setTimeout(() => hint.classList.remove('visible'), 3000);
    },

    showToast(msg) {
        const hint = document.getElementById('ui-hint');
        if (!hint) return;
        hint.textContent = t(msg);
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
        if (this.mode === 'organize' && this.organize) {
            this.currentBrowsePath = this.organize.pane.path;
        }
        // Settings is a place you step into and back out of: Done returns here.
        // The setup is opened from Settings, and Cancel goes back the same way.
        if (mode === 'settings' && this.mode !== 'settings' && this.mode !== 'setup') {
            this._placeBeforeSettings = this.mode || null;
        }
        // A place told it was shown again from within itself (a topic of
        // Statistics) need not read again what only changes elsewhere.
        this.previousMode = this.mode;
        this.mode = mode;
        this._markPlace(mode);

        const hash = this._placeHash(mode);
        if (!fromHistory && location.hash !== hash) {
            if (replaceHistory) history.replaceState(null, '', hash);
            else history.pushState(null, '', hash);
        }

        this._openPlace(mode);
        for (const [elKey, place] of this.PLACE_ELEMENTS) {
            if (this[elKey]) this[elKey].style.display = mode === place ? '' : 'none';
        }
        this._markCurrentLibraryNav();
        this._markCurrentStatisticsNav();
    },

    // leaveSettings goes back to the place Settings was opened from, or to
    // Libraries when it was opened directly.
    leaveSettings() {
        this.setMode(this._placeBeforeSettings || 'library');
    },

    // Each place's element, in the order they are shown or hidden.
    PLACE_ELEMENTS: [
        ['_browseEl', 'browse'], ['_organizeEl', 'organize'], ['_wastebinEl', 'wastebin'], ['_duplicatesEl', 'duplicates'],
        ['_libraryEl', 'library'], ['_metadataEl', 'metadata'], ['_mapEl', 'map'], ['_timelineEl', 'timeline'], ['_peopleEl', 'people'], ['_statsEl', 'statistics'], ['_galleriesEl', 'published'], ['_destinationsEl', 'destinations'],
        ['_settingsEl', 'settings'], ['_aboutEl', 'about'], ['_guideEl', 'guide'], ['_privacyEl', 'privacy'], ['_warrantyEl', 'warranty'], ['_creditsEl', 'licenses'], ['_setupEl', 'setup'],
    ],

    // Mark where we are. There is no "done" or "next" — these are places.
    _markPlace(mode) {
        // About's topics have no entries of their own; About stands for them.
        const current = ABOUT_PLACES.has(mode) ? 'about' : mode;
        for (const [navMode, place] of Object.entries(this.NAV)) {
            const el = document.getElementById(place.id);
            if (!el) continue;
            if (navMode === current) el.setAttribute('aria-current', 'page');
            else el.removeAttribute('aria-current');
        }
        for (const el of document.querySelectorAll('.tabbar-item')) {
            if (el.dataset.mode === mode) el.setAttribute('aria-current', 'page');
            else el.removeAttribute('aria-current');
        }
        // A phone browses; it does not cull, organise or configure. Those
        // places say so rather than showing controls that cannot work there.
        document.body.classList.toggle('desk-only-place', !PHONE_PLACES.has(mode));
    },

    // _openPlace builds a place the first time it is visited and brings it
    // up to date.
    _openPlace(mode) {
        switch (mode) {
            case 'browse': if (!this._browseEl) this._buildBrowse(); break;
            case 'organize': if (!this._organizeEl) this._buildOrganize(); break;
            case 'wastebin':
                if (!this._wastebinEl) this._wastebinEl = this._placeElement();
                this.wastebin.selected.clear();
                this.wastebin.render(this._wastebinEl, () => this._refreshPanes());
                break;
            case 'duplicates': this._openPane('_duplicatesEl', '_duplicatesPane', DuplicatesPane); break;
            case 'library': this._openPane('_libraryEl', '_libraryTab', LibraryTab); break;
            case 'metadata': this._openPane('_metadataEl', '_metadataPane', MetadataPane); break;
            case 'map': this._openPane('_mapEl', '_mapPane', MapPane); break;
            case 'timeline': this._openPane('_timelineEl', '_timelinePane', TimelinePane); break;
            case 'people':
                if (!this._peopleEl) {
                    this._peopleEl = this._placeElement();
                    this._peoplePane = new PeoplePane(this._peopleEl);
                }
                this._peoplePane.show(this._peopleRoute);
                break;
            case 'statistics':
                if (!this._statsEl) {
                    this._statsEl = this._placeElement();
                    this._statsPane = new StatsPane(this._statsEl);
                }
                this._statsPane.show(this._statsRoute);
                break;
            case 'settings': this._openPane('_settingsEl', '_settingsPane', SettingsPane); break;
            case 'destinations': this._openPane('_destinationsEl', '_destinationsPane', DestinationsPane); break;
            case 'published': this._openPane('_galleriesEl', '_galleriesPane', GalleriesPane); break;
            case 'guide': this._openPane('_guideEl', '_guidePane', GuidePane); break;
            case 'licenses': this._openPane('_creditsEl', '_creditsPane', CreditsPane); break;
            case 'about': this._openPane('_aboutEl', '_aboutPane', AboutPane); break;
            case 'privacy': this._openPane('_privacyEl', '_privacyPane', PrivacyPane); break;
            case 'warranty': this._openPane('_warrantyEl', '_warrantyPane', WarrantyPane); break;
            case 'setup': this._openPane('_setupEl', '_setupPane', SetupPane); break;
        }
    },

    // _placeElement adds an empty full-height element for a place to #app.
    _placeElement() {
        const el = document.createElement('div');
        el.style.height = '100%';
        document.getElementById('app').appendChild(el);
        return el;
    },

    // _openPane builds a place that is one pane in one element, once, and
    // renders it.
    _openPane(elKey, paneKey, PaneClass) {
        if (!this[elKey]) {
            this[elKey] = this._placeElement();
            this[paneKey] = new PaneClass(this[elKey]);
        }
        this[paneKey].render();
    },

    _buildBrowse() {
        this._browseEl = document.createElement('div');
        this._browseEl.className = 'browse-layout';
        this._browseEl.innerHTML =
            '<div id="browse-container" class="browse-container"></div>' +
            '<div id="info-panel-container"></div>';
        document.getElementById('app').appendChild(this._browseEl);
        // Info opens from the tool strip at the edge, as every panel of the
        // right-hand column does.
        this._infoBtn = panelToggleButton('Info');
        this._browseEl.appendChild(toolStrip(this._infoBtn));
        this.browsePane = new BrowsePane(this._browseEl.querySelector('#browse-container'), {
            onNavigate: (path) => { this.currentBrowsePath = path; },
            onImageClick: (path) => this.openViewer(path, this.browsePane),
            onSelectionChange: (selected) => this.handleSelectionChange(selected),
            onFocusChange: (path, type) => this.handleFocusChange(path, type),
            onToolInvoke: (params) => this.handleToolInvoke(params),
            onSlideshowInvoke: () => this.handleSlideshowInvoke(),
            lede: placeLede(t('Your folders as they are on disk. Look through it, mark what should go, and move what stays. To search, map or publish photos, catalog a folder as a {library}.', { library: placeLink('library', 'libraries', t('library')) })),
        });
        this.infoPanel = new InfoPanel(this._browseEl.querySelector('#info-panel-container'));
        this.infoPanel.attachToggle(this._infoBtn);
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
            actions: ['export', 'download', 'rename', 'location', 'metadata', 'mark'],
            onAction: (action) => this.runSelectionAction(action, this.browsePane),
        });
        this.browsePane.load(this.currentBrowsePath);
    },

    _buildOrganize() {
        this._organizeEl = this._placeElement();
        this.organize = new OrganizePane(this._organizeEl, this.currentBrowsePath, {
            preselectFiles: this._organizePreselect,
            onImageClick: (path, pane) => this.openViewer(path, pane),
            onToolInvoke: (params) => this.handleToolInvoke(params),
        });
        this._organizePreselect = null;
        this.organize.init();
    },

    _refreshPanes() {
        if (this.browsePane) this.browsePane.load(this.browsePane.path);
        if (this.organize) this.organize.reload();
    },

    // leaveRemovedFolders moves Folders and Organize out of folders that
    // were just removed, to the nearest one left, before they load again.
    leaveRemovedFolders(removed) {
        const outside = (path) => {
            for (const f of removed) {
                if (path === f || path.startsWith(f + '/')) return dirOf(f);
            }
            return path;
        };
        this.currentBrowsePath = outside(this.currentBrowsePath);
        if (this.browsePane) this.browsePane.path = outside(this.browsePane.path);
        if (this.organize) this.organize.pane.path = outside(this.organize.pane.path);
    },

    // Opens one destination's settings — used by the Galleries screen.
    showDestination(slug) {
        this.setMode('destinations');
        if (this._destinationsPane) this._destinationsPane.openBySlug(slug);
    },

    // Opens Galleries filtered to one destination — used by the channels
    // list's status link.
    // Opens one published gallery's page under Galleries.
    showGallery(channelSlug, postID) {
        this.setMode('published');
        if (this._galleriesPane) this._galleriesPane.openGallery(channelSlug, postID);
    },

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

    // markInPane marks paths of a pane for deletion, with what the pane
    // knows about deleting each (a library photo's ID, a folder's place on
    // disk).
    markInPane(pane, paths) {
        const meta = {};
        for (const p of paths) {
            const m = pane.wastebinMeta?.(p);
            if (m) meta[p] = m;
        }
        this.wastebin.mark(paths, pane.entries, pane.path, Object.keys(meta).length ? meta : null);
        pane.updateMarkedForDeletion();
    },

    // renameSelectedFolder renames the one folder selected in a pane, if
    // that is what is selected, and says whether it did.
    renameSelectedFolder(pane, onDone) {
        if (pane.getSelectedFiles().length || pane.selectedDirs?.size !== 1) return false;
        const diskPath = pane.diskPath([...pane.selectedDirs][0]);
        if (diskPath === null) { this._outsideBoundary(); return true; }
        openFolderRenameDialog(diskPath, () => {
            pane.load(pane.path);
            onDone?.();
        });
        return true;
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
        const viewer = this.showViewer(imagePath, images, {
            imageURLFn: pane.viewerImageURL ? (p) => pane.viewerImageURL(p) : undefined,
            thumbURLFn:  pane.viewerThumbURL  ? (p) => pane.viewerThumbURL(p)  : undefined,
            previewURLFn: pane.viewerPreviewURL ? (p) => pane.viewerPreviewURL(p) : undefined,
            liveURLFn: pane.viewerLiveURL ? (p) => pane.viewerLiveURL(p) : undefined,
            infoLoadFn:  pane.viewerLoadInfo  ? (p, ip) => pane.viewerLoadInfo(p, ip)  : undefined,
            libraryRef: pane.getLibraryMeta ? (p) => {
                const meta = pane.getLibraryMeta(p);
                return meta ? { lib: meta.libID, id: meta.photoID } : null;
            } : undefined,
        }, () => {
            if (pane.updateMarkedForDeletion) pane.updateMarkedForDeletion();
        });
        viewer.onDelete = (path) => {
            const libMeta = pane.getLibraryMeta?.(path);
            const photoMeta = libMeta ? { [path]: libMeta } : null;
            this.wastebin.mark([path], pane.entries || [], pane.path || '', photoMeta);
        };
    },

    // showViewer puts the viewer over the current place and brings the place
    // back, scrolled where it was, when the viewer closes.
    showViewer(imagePath, images, options, onClosed) {
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

        this.viewer = new Viewer(viewerEl, options);
        this.viewer.onClose = () => {
            viewerEl.remove();
            savedDisplay.forEach((display, el) => { el.style.display = display; });
            scrollPositions.forEach((top, el) => { el.scrollTop = top; });
            onClosed?.();
        };
        this.viewer.open(imagePath, images);
        return this.viewer;
    },

    async handleSlideshowInvoke(pane) {
        pane = pane || this.browsePane;
        if (!pane) return;
        // Resolve each path to its correct image URL (LibraryPane overrides viewerImageURL
        // to use /api/library/{id}/photo/{photoID}; BrowsePane falls back to API.imageURL).
        const toURL = p => pane.viewerImageURL ? pane.viewerImageURL(p) : API.imageURL(p);
        // A slideshow shows photos; videos are played in the viewer.
        const stills = paths => paths.filter(p => !isVideoName(p));
        let images;
        if (pane.selectedDirs?.size > 0) {
            const n = pane.selectedDirs.size;
            const allPaths = await this.whileSlow(n === 1 ? t('Collecting the photos of 1 folder…') : t('Collecting the photos of {n} folders…', { n: formatCount(n) }), async () => {
                const paths = [];
                for (const dirPath of pane.selectedDirs) paths.push(...await pane.fetchRecursivePhotoPaths(dirPath));
                return paths;
            });
            images = stills(allPaths).map(toURL);
        } else if (pane.selection.selected.size > 0) {
            images = stills(Array.from(pane.selection.selected)).map(toURL);
        } else {
            images = stills(pane.getImageEntries().map(e => pane.fullPath(e.name))).map(toURL);
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

        // Starting is the tap the browser needs to allow full screen.
        const enteredFullscreen = Fullscreen.enter();
        const player = new SlideshowPlayer(slideshowEl);
        player.onClose = () => {
            enteredFullscreen.then(entered => { if (entered) Fullscreen.exit(); });
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
        // actions live. Folders count: Download takes them.
        const dirs = this.browsePane?.selectedDirs?.size ?? 0;
        this._browseSelectionBar?.update(selected.length + dirs, {
            onlyFolders: selected.length === 0 && dirs > 0,
            onlyVideos: selected.length > 0 && selected.every(isVideoName),
        });
    },

    // Every action on a selection goes through here, from the bar and from the
    // keyboard alike, so the two can never drift apart.
    runSelectionAction(action, pane) {
        if (!pane) return;
        const files = pane.getSelectedFiles();
        // Folders count for Download: they come with everything in them.
        if (action === 'download') {
            Originals.download(files, pane, { dirs: [...(pane.selectedDirs || [])] });
            return;
        }
        if (action === 'mark') {
            this.markInPane(pane, pane.getActionableFiles());
            return;
        }
        if (action === 'rename' && this.renameSelectedFolder(pane)) return;
        if (!files.length) return;
        const tool = action === 'export' ? 'export'
            : action === 'location' ? 'set-location'
            : action === 'metadata' ? 'edit-metadata'
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

    // rateSelection gives the selected photos, or the focused one, n stars
    // (0 takes them away): the keys 1–5 and 0 in Folders and a library.
    async rateSelection(n) {
        const pane = this.mode === 'organize' ? null : this.getActiveBrowsePane();
        if (!pane) return;
        const images = new Set(pane.getImageEntries().map(e => pane.fullPath(e.name)));
        const files = pane.getActionableFiles().filter(f => images.has(f));
        const sourcePath = this.mode === 'library' && pane === this._libraryTab?._pane ? this._libraryTab.currentLibrary?.sourcePath : null;
        const resolved = this._boundaryPaths(files, sourcePath);
        if (!resolved) { this._outsideBoundary(); return; }
        const done = new Set(await Rating.rate(resolved, n));
        this.showRating(pane, files.filter((f, i) => done.has(resolved[i])), n);
    },

    // showRating puts new stars on a pane's tiles and rows, and in the info
    // panel when it shows one of the photos; files are the pane's paths.
    showRating(pane, files, n) {
        if (!pane || !files.length) return;
        pane._entryMeta ||= {};
        for (const e of pane.getImageEntries()) {
            if (files.includes(pane.fullPath(e.name))) pane._entryMeta[e.name] = { ...pane._entryMeta[e.name], rating: n };
        }
        pane._updateOverlays?.();
        const panel = this.mode === 'library' ? this._libraryTab?._infoPanel : this.infoPanel;
        if (files.some(f => panel?.currentPath === this._infoKey(pane, f))) panel.ratingChanged(n);
    },

    // ratedInPanel puts the stars set in an info panel on the tile of the
    // photo it shows (key: its path, or lib:<library>:<photo>).
    ratedInPanel(key, n) {
        const pane = this.getActiveBrowsePane();
        if (!pane) return;
        const files = pane.getImageEntries().map(e => pane.fullPath(e.name)).filter(f => this._infoKey(pane, f) === key);
        this.showRating(pane, files, n);
    },

    // _infoKey is how an info panel names a pane's photo.
    _infoKey(pane, path) {
        const meta = pane.getLibraryMeta?.(path);
        return meta ? `lib:${meta.libID}:${meta.photoID}` : path;
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
        if (LIBRARY_TOOLS[tool]) {
            this._runLibraryTool(tool, path, onDone);
            return;
        }
        // Videos are listed and played, never changed or exported (ADR-0058).
        if (['set-location', 'batch-rename', 'export'].includes(tool)) {
            files = (files || []).filter(f => !isVideoName(f));
            if (!files.length) return;
        }
        switch (tool) {
            case 'make-library': this._makeLibraryAt(path); break;
            case 'set-location': this._setLocation(files, sourcePath, onDone, pane); break;
            case 'batch-rename': this._batchRename(files, sourcePath, onDone, pane); break;
            case 'export': this._openExport(files, sourcePath, pane); break;
            case 'edit-metadata': this._editMetadata(files, sourcePath, onDone); break;
        }
    },

    // _editMetadata brings the files to Metadata (ADR-0060); videos come
    // too, their metadata goes beside them.
    _editMetadata(files, sourcePath, onDone) {
        const resolved = this._boundaryPaths(files || [], sourcePath);
        if (!resolved) { this._outsideBoundary(onDone); return; }
        if (!resolved.length) return;
        if (!this._metadataEl) {
            this._metadataEl = this._placeElement();
            this._metadataPane = new MetadataPane(this._metadataEl);
        }
        this._metadataPane.setFiles(resolved);
        if (onDone) onDone();
        this.setMode('metadata');
    },

    // _filesChanged refreshes a pane, and the info panel when it shows one of
    // the changed files.
    _filesChanged(pane, changedFiles) {
        if (pane) pane.notifyFilesChanged(changedFiles);
        if (this.infoPanel && this.infoPanel.expanded && pane) {
            const focused = pane.getFocusedFile();
            if (focused && changedFiles.includes(focused)) {
                this.infoPanel.loadInfo(focused);
            }
        }
    },

    _makeLibraryAt(path) {
        // Ensure LibraryTab exists so it can open the dialog.
        if (!this._libraryEl) {
            this._libraryEl = this._placeElement();
            this._libraryTab = new LibraryTab(this._libraryEl);
            this._libraryEl.style.display = 'none';
        }
        const absPath = path && !path.startsWith('/') ? '/' + path : (path || '');
        this._libraryTab.openCreateDialogForPath(absPath);
    },

    // _boundaryPaths turns the files of a tool into what every path-taking
    // API expects: paths relative to the server's browse boundary. Files from
    // a library are relative to its folder (sourcePath); files from Folders
    // already are; filter results are absolute pathHints. null means a file
    // lies outside the boundary, which the caller must say.
    _boundaryPaths(files, sourcePath) {
        const boundary = this.config?.boundary;
        const out = [];
        for (const f of files) {
            const abs = f.startsWith('/') ? f
                : sourcePath ? `${sourcePath.replace(/\/$/, '')}/${f}` : null;
            const rel = abs === null ? f : absPathRelativeToBoundary(abs, boundary);
            if (rel === null) return null;
            out.push(rel);
        }
        return out;
    },

    _outsideBoundary(onDone) {
        this.showToast(t('This is outside the folder this installation serves, so it cannot be changed from here.'));
        if (onDone) onDone();
    },

    _setLocation(files, sourcePath, onDone, pane) {
        const resolved = this._boundaryPaths(files, sourcePath);
        if (!resolved) { this._outsideBoundary(onDone); return; }
        this.locationModal.open(resolved, (changed) => {
            this._filesChanged(pane, files.filter((f, i) => changed.includes(resolved[i])));
            if (onDone) onDone();
        });
    },

    _batchRename(files, sourcePath, onDone, pane) {
        const resolved = this._boundaryPaths(files, sourcePath);
        if (!resolved) { this._outsideBoundary(onDone); return; }
        this.batchRenameModal.open(resolved, (libraryUpdated) => {
            if (pane) pane.load(pane.path);
            if (libraryUpdated) this.reloadLibraryPane();
            if (onDone) onDone();
        });
    },

    _openExport(files, sourcePath, pane) {
        if (!this.exportModal) this.exportModal = new ExportModal();
        // Library photos go by library and ID for the ZIP: filter results
        // carry absolute paths, which a server refuses.
        const photoRefs = new Map();
        for (const f of files) {
            const meta = pane?.getLibraryMeta?.(f);
            if (meta) photoRefs.set(f, { library: meta.libID, id: meta.photoID });
        }
        this.exportModal.open(files, {
            serverRole: this.config?.serverRole ?? false,
            exiftoolAvailable: this.toolsStatus?.exiftool ?? false,
            webpSupport: this.toolsStatus?.webpAvailable ?? false,
            sourcePath: sourcePath || null,
            photoRefs,
        });
    },

    // _runLibraryTool starts a scan-like job on a folder of the open library.
    _runLibraryTool(tool, path, onDone) {
        const lib = this._libraryTab?.currentLibrary;
        if (!lib) { if (onDone) onDone(); return; }
        const { label, run } = LIBRARY_TOOLS[tool];
        // The run can take minutes, so its progress lives in the status
        // line at the foot of the sidebar, which stays while you move on.
        App.showToast(`${label} this folder. Progress shows at the foot of the sidebar.`);
        const onProgress = (p) => {
            if (p.finished && onDone) onDone();
        };
        LibraryAPI[run](lib.id, onProgress, path || '').catch(e => {
            App.showToast(t('It did not start: {e}', { e: e.message }));
            if (onDone) onDone();
        });
    },
};

// The library jobs a folder's menu can start: what the toast calls them and
// the LibraryAPI method that runs them.
const LIBRARY_TOOLS = {
    'lib-match':         { label: t('Matching the index with'), run: 'matchFolder' },
    'lib-reindex':       { label: t('Indexing'), run: 'reindex' },
    'lib-cleanup':       { label: t('Checking'), run: 'cleanup' },
    'lib-regen-missing': { label: t('Generating missing previews for'), run: 'regenMissingPreviews' },
    'lib-rebuild-all':   { label: t('Rebuilding the previews of'), run: 'rebuildAllPreviews' },
};

document.addEventListener('DOMContentLoaded', () => App.init());
