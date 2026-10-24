// Browse mode — single pane directory browser

const CHUNK_SIZE = 50;

class BrowsePane {
    constructor(container, options = {}) {
        this.container = container;
        this.path = '';
        this.entries = [];
        this.view = 'justified'; // 'grid', 'list', or 'justified'
        this.sort = 'name';
        this.order = 'asc';
        this.showNames = false;
        this.showOverlays = true;
        this._takeOptions(options);
        this._resetFolderState();
        this._toolsChecked = null;
        this._loading = false;
        this._loadController = null;
        this._renderedCount = 0;
        this._observer = null;
        this._justifiedTargetHeight = 200;
        this._resizeHandler = null;
        this._contentEl = null;
        this.selectedDirs = new Set();
        this._previewLibID = null;
        this._pendingPreselect = null;
        this._libraries = [];
        this._buildParts();
        this._attachDelegatedEvents();
    }

    // What the host gives: what happens on navigating, opening and selecting,
    // and how the place's head looks.
    _takeOptions(options) {
        this.onNavigate = options.onNavigate || null;
        this.onImageClick = options.onImageClick || null;
        this.onSelectionChange = options.onSelectionChange || null;
        this.onFocusChange = options.onFocusChange || null;
        this.onLoad = options.onLoad || null;
        // What the place is, shown at the top folder only (placeLede HTML).
        this.lede = options.lede || '';
        this.onToolInvoke = options.onToolInvoke || null;
        this.onSlideshowInvoke = options.onSlideshowInvoke || null;
        // The pane draws the place's head unless its host has one (Organize).
        this.placeHeader = options.placeHeader ?? true;
        this.rootLabel = options.rootLabel || t('Folders');
        // onHeader(slot): the host puts buttons of its own into the head.
        this.onHeader = options.onHeader || null;
    }

    _buildParts() {
        this.selection = new SelectionManager((files) => {
            if (this.onSelectionChange) this.onSelectionChange(files);
        }, () => this._syncSelectAll());
        // Select all takes the photos, as ⌘A does; Select none also the folders.
        this._selectAll = new SelectAll({
            onAll: () => this.selectAll(),
            onNone: () => this._clearAllSelection(),
        });
        this.keyboard = new BrowseKeyboard(this);
        this._gridRenderer = new GridRenderer(this);
        this._listRenderer = new ListRenderer(this);
        this._justifiedRenderer = new JustifiedRenderer(this);
    }

    // What belongs to the folder shown, forgotten when another is loaded.
    _resetFolderState() {
        this.warnings = [];
        this._exifPollPath = null;
        this._metaPollPath = null;
        this._entryMeta = {};
        this._aspectRatios = {};
        this._folderPreviews = new Map(); // subfolder name → what a library knows of it
    }

    // _clearAllSelection deselects every photo and folder, if any is selected.
    _clearAllSelection() {
        if (this.selection.selected.size === 0 && this.selectedDirs.size === 0) return;
        this.selection.clear();
        this.selection.updateClasses(this.container);
        this.selectedDirs.clear();
        this._updateDirSelectionClasses();
        if (this.onSelectionChange) this.onSelectionChange([]);
    }

    _onDirClick(e, item, idx) {
        this.keyboard.focusedIndex = idx;
        this.keyboard.updateFocusClass();
        const fp = this.fullPath(item.dataset.name);
        // On a touch screen a tap on a folder goes there: there is no double
        // tap to wait for and nothing to read beside it. With a mouse a
        // click selects the folder, so the info panel can show what it
        // holds, and a double click (or Enter) goes in.
        if (window.matchMedia('(hover: none)').matches && !(e.ctrlKey || e.metaKey)) {
            this.load(fp);
            if (this.onNavigate) this.onNavigate(fp);
            return;
        }
        this._notifyFocusChange();
        if (e.ctrlKey || e.metaKey) {
            if (this.selectedDirs.has(fp)) this.selectedDirs.delete(fp);
            else this.selectedDirs.add(fp);
        } else {
            this.selectedDirs.clear();
            this.selectedDirs.add(fp);
        }
        // Dir and photo selection are mutually exclusive
        this.selection.clear();
        this.selection.updateClasses(this.container);
        this._updateDirSelectionClasses();
        if (this.onSelectionChange) this.onSelectionChange([]);
    }

    _onImageClick(e, item, idx) {
        const fp = item.dataset.path;
        this.keyboard.focusedIndex = idx;
        this.keyboard.updateFocusClass();
        this._notifyFocusChange();
        // Clear dir selection when switching to photo selection
        if (this.selectedDirs.size > 0) {
            this.selectedDirs.clear();
            this._updateDirSelectionClasses();
        }
        this.selection.handleImageClick(e, idx, fp, this.entries, n => this.fullPath(n));
        this.selection.updateClasses(this.container);
    }

    _attachDelegatedEvents() {
        this.container.addEventListener('click', (e) => {
            // Background click on the grid/list/justified container deselects all
            if (e.target.classList.contains('grid') ||
                e.target.classList.contains('justified') ||
                e.target.classList.contains('list-view')) {
                this._clearAllSelection();
                return;
            }
            const item = e.target.closest('[data-index]');
            if (!item) return;
            const idx = parseInt(item.dataset.index);
            if (item.dataset.type === 'dir') this._onDirClick(e, item, idx);
            else if (item.dataset.type === 'image') this._onImageClick(e, item, idx);
        });

        this.container.addEventListener('dblclick', (e) => {
            const item = e.target.closest('[data-index]');
            if (!item) return;
            if (item.dataset.type === 'dir') {
                const path = this.fullPath(item.dataset.name);
                this.load(path);
                if (this.onNavigate) this.onNavigate(path);
            } else if (item.dataset.type === 'image') {
                if (this.onImageClick) this.onImageClick(item.dataset.path);
            }
        });
    }

    // Getters so external code (organize.js, renderers) can access sub-object state via the pane directly
    get focusedIndex() { return this.keyboard.focusedIndex; }
    set focusedIndex(v) { this.keyboard.focusedIndex = v; }
    get selected() { return this.selection.selected; }

    // --- Public API ---

    async load(path) {
        // Cancel any in-flight load so navigation is always immediately responsive.
        if (this._loadController) this._loadController.abort();
        const controller = new AbortController();
        this._loadController = controller;
        this._loading = true;
        const isReload = (path || '') === this.path;
        const savedScroll = isReload ? (this._contentEl || this.container).scrollTop : 0;
        this.path = path || '';
        this._leaveFolder();
        this.render();

        const data = await this._fetchFolder(controller);
        if (!data) return;
        this.entries = data.entries || [];
        this.warnings = data.warnings || [];
        this.render();
        this.keyboard.updateFocusClass();
        if (savedScroll > 0) this._restoreScroll(savedScroll);
        this._afterLoad();
    }

    // Nothing of the folder before stays selected, focused or known.
    _leaveFolder() {
        const hadSelection = this.selection.selected.size + this.selectedDirs.size > 0;
        this.selection.clear();
        this.selectedDirs.clear();
        // The bar counts folders too: going into the one selected by the
        // first click of a double click must take the bar away with it.
        if (hadSelection && this.onSelectionChange) this.onSelectionChange([]);
        this.keyboard.focusedIndex = 0;
        this.entries = [];
        this._resetFolderState();
    }

    // The folder's entries, or null when this load was replaced or failed
    // (and says so).
    async _fetchFolder(controller) {
        try {
            const data = await API.browse(this.path, this.sort, this.order, controller.signal);
            if (this._loadController !== controller) return null;
            this._loading = false;
            this._loadController = null;
            return data;
        } catch (err) {
            if (err.name === 'AbortError' || this._loadController !== controller) return null;
            this._loading = false;
            this._loadController = null;
            this.container.innerHTML = `<div class="error">${t('Failed to load: {error}', { error: escapeHtml(err.message) })}</div>`;
            return null;
        }
    }

    // A reload keeps the place it was scrolled to, drawing as many chunks as
    // that needs.
    _restoreScroll(savedScroll) {
        const el = this._contentEl || this.container;
        while (this._renderedCount < this.entries.length && el.scrollHeight <= savedScroll + el.clientHeight) {
            this._renderNextChunk();
        }
        el.scrollTop = savedScroll;
    }

    _afterLoad() {
        this._notifyFocusChange();
        if (this.entries.some(e => e.type === 'image')) {
            this._pollExifDates();
            this._pollOverlayMeta();
        }
        this._applyPendingPreselect();
        if (this.onLoad) this.onLoad();
        this._detectLibrary(this.path);
    }

    setView(view) {
        this.view = view;
        this.render();
    }

    setSort(sort, order) {
        this.sort = sort;
        this.order = order;
        this.load(this.path);
    }

    reloadThumbnails() {
        this.render();
    }

    getImageEntries() {
        return this.entries.filter(e => e.type === 'image');
    }

    getSelectedFiles() {
        return this.selection.getSelectedFiles();
    }

    getActionableFiles() {
        const selected = this.getSelectedFiles();
        if (selected.length > 0) return selected;
        const focused = this.keyboard.getFocusedEntry();
        return focused ? [focused] : [];
    }

    // Home is the top of wherever you are: the folder the server was started
    // with (or the OS home) while browsing the filesystem, and the library's
    // own root inside a library — see LibraryPane.
    homeTarget() {
        return App.config?.homePath ?? App.config?.startPath ?? '';
    }

    homeLabel() { return t('Home folder'); }

    getFocusedDir()    { return this.keyboard.getFocusedDir(); }
    getFocusedFile()   { return this.keyboard.getFocusedFile(); }
    getFocusedEntry()  { return this.keyboard.getFocusedEntry(); }
    moveFocus(delta)   { this.keyboard.moveFocus(delta); }
    activateFocused()  { this.keyboard.activateFocused(); }
    getColumnCount()   { return this.keyboard.getColumnCount(); }

    toggleFocusedSelection() { this.keyboard.toggleFocusedSelection(); }

    selectAll() {
        this.selectedDirs.clear();
        this.selection.selectAll(this.entries, n => this.fullPath(n));
        this.render();
    }

    fullPath(name) {
        return this.path ? `${this.path}/${name}` : name;
    }

    primePreselect(names) {
        this._pendingPreselect = names && names.length ? new Set(names) : null;
    }

    _applyPendingPreselect() {
        if (!this._pendingPreselect) return;
        const names = this._pendingPreselect;
        this._pendingPreselect = null;
        for (const entry of this.entries) {
            if (entry.type === 'image' && names.has(entry.name)) {
                this.selection.selected.add(this.fullPath(entry.name));
            }
        }
        if (this.selection.selected.size > 0) {
            this.selection.updateClasses(this.container);
            if (this.onSelectionChange) this.onSelectionChange(this.getSelectedFiles());
            // The first of them gets the focus and is scrolled into view.
            const first = this.entries.findIndex(e => e.type === 'image' && this.selection.selected.has(this.fullPath(e.name)));
            this.keyboard.focusedIndex = first;
            this.keyboard.updateFocusClass();
            this.keyboard.scrollFocusedIntoView();
            this._notifyFocusChange();
        }
    }

    _updateDirSelectionClasses() {
        this.container.querySelectorAll('[data-type="dir"]').forEach(el => {
            el.classList.toggle('selected', this.selectedDirs.has(this.fullPath(el.dataset.name)));
        });
    }

    async fetchRecursivePhotoPaths(dirPath) {
        const data = await API.browseRecursive(dirPath);
        return data.paths || [];
    }

    thumbURL(entry, size) {
        return API.thumbnailURL(this.fullPath(entry.name), size);
    }

    isMarkedForDeletion(fp) {
        return App.isMarkedForDeletion(fp);
    }

    updateMarkedForDeletion() {
        this.container.querySelectorAll('[data-path]').forEach(el => {
            const path = el.getAttribute('data-path');
            el.classList.toggle('marked-for-deletion', App.isMarkedForDeletion(path));
        });
    }

    updateSelectionClasses() {
        this.selection.updateClasses(this.container);
    }

    async notifyFilesChanged() {
        try {
            await API.browse(this.path, this.sort, this.order);
        } catch { /* ignore */ }
        this._pollOverlayMeta();
    }

    async _detectLibrary(path) {
        this._libraries = [];
        this._updateLibraryBadge();
        try {
            const data = await API.detectLibrary(path);
            if (this.path === path && data?.libraries?.length) {
                this._libraries = data.libraries;
                this._updateLibraryBadge();
                const covering = data.libraries[0];
                if (this.entries.some(e => e.type === 'dir')) {
                    this._previewLibID = covering.id;
                    this._loadFolderPreviews(path, covering.path || '');
                }
            }
        } catch { /* ignore */ }
    }

    _updateLibraryBadge() {
        const badge = this.container.querySelector('.browse-library-badge');
        if (!badge) return;
        // Says which libraries catalog this folder, and goes there.
        const libs = this._libraries;
        if (libs.length) {
            badge.innerHTML = (libs.length === 1 ? t('In library {links}', { links: libs.map(libraryLink).join(', ') }) : t('In libraries {links}', { links: libs.map(libraryLink).join(', ') }));
            badge.style.display = '';
        } else {
            badge.style.display = 'none';
        }
    }

    // --- Rendering ---

    render() {
        this._destroyObserver();
        this._renderedCount = 0;
        if (this._resizeHandler) {
            window.removeEventListener('resize', this._resizeHandler);
            this._resizeHandler = null;
        }
        this.container.innerHTML =
            `<div class="browse-header">${this._headerHTML()}</div>` +
            `<div class="browse-content">${this._contentHTML()}</div>`;
        this._contentEl = this.container.querySelector('.browse-content');
        this.container.querySelector('.browse-select-all')?.appendChild(this._selectAll.el);
        this.onHeader?.(this.container.querySelector('.browse-header-extra'));
        this._syncSelectAll();
        // A new layout draws the breadcrumb anew; the libraries found stay.
        this._updateLibraryBadge();
        const loadingEl = this._contentEl.querySelector('.browse-loading');
        if (loadingEl) Activity.in(loadingEl, t('Reading the folder…'), { area: true });
        this.attachEvents();
        this._setupObserver();
        if (this.view === 'justified') {
            this._justifiedRenderer.layout();
            this._resizeHandler = () => this._justifiedRenderer.scheduleRelayout();
            window.addEventListener('resize', this._resizeHandler);
        }
    }

    _headerHTML() {
        const header = [];
        if (this.placeHeader) header.push(this._renderPlaceHeader());
        if (this.warnings && this.warnings.length > 0) header.push(this._renderWarnings());
        header.push(`<div class="browse-bar">${this._renderBreadcrumb()}${this._renderControls()}</div>`);
        return header.join('');
    }

    // The first chunk of the folder; the rest follows as it scrolls into view.
    _contentHTML() {
        if (this._loading) return '<div class="browse-loading"></div>';
        if (this.entries.length === 0) return '<div class="empty">' + t('No images or folders found') + '</div>';
        const end = Math.min(CHUNK_SIZE, this.entries.length);
        this._renderedCount = end;
        return this._renderChunk(0, end) + (end < this.entries.length ? '<div class="scroll-sentinel"></div>' : '');
    }

    // The place's head: the folder's name and what is in it, the slideshow
    // and ⋯, and at the top folder what the place is. The counts are what
    // the pane already knows; nothing is fetched for this.
    _renderPlaceHeader() {
        return placeHeaderHTML({
            title: escapeHtml(this.title()),
            meta: this._loading ? '' : this._folderMeta(),
            actions: `<span class="browse-header-extra"></span>${this._slideshowButtonHTML()}<span class="browse-more"></span>`,
            lede: this.path ? '' : this.lede,
        });
    }

    // title is what the head calls the folder: its name, or the place's own
    // name at the top.
    title() {
        return this.path ? this.path.split('/').filter(Boolean).pop() : this.rootLabel;
    }

    _folderMeta() {
        const images = this.getImageEntries().length;
        const dirs = this.entries.filter(e => e.type === 'dir').length;
        const bytes = this.entries.reduce((sum, e) => sum + (e.size || 0), 0);
        return [
            countHTML(images, 'photo'),
            dirs > 0 ? countHTML(dirs, 'folder') : null,
            bytes > 0 ? formatSize(bytes) : null,
        ].filter(Boolean).join(' · ');
    }

    _renderChunk(start, end) {
        if (this.view === 'grid')      return this._gridRenderer.renderChunk(start, end);
        if (this.view === 'justified') return this._justifiedRenderer.renderChunk(start, end);
        return this._listRenderer.renderChunk(start, end);
    }

    _renderWarnings() {
        return this.warnings.map((w, i) =>
            `<div class="warning-banner" data-warning-index="${i}">
                <span class="warning-message">${escapeHtml(w.message)}</span>
                <button class="btn btn-sm warning-dismiss" data-warning-index="${i}" title="${t('Dismiss')}">&times;</button>
            </div>`
        ).join('');
    }

    _renderBreadcrumb() {
        const parts = this.path ? this.path.split('/') : [];
        const isAtRoot = parts.length === 0;
        const isAtHome = this.path === this.homeTarget();
        const upBtn = `<button class="btn btn-sm up-dir-btn" title="${t('Go up')}"${isAtRoot ? ' disabled' : ''}><svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><line x1="6.5" y1="10.5" x2="6.5" y2="2.5"/><polyline points="3 6 6.5 2.5 10 6"/></svg></button>`;
        const homeBtn = `<button class="btn btn-sm home-btn" title="${escapeHtml(this.homeLabel())}" aria-label="${escapeHtml(this.homeLabel())}"${isAtHome ? ' disabled' : ''}><svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="1.5 6.5 6.5 1.5 11.5 6.5"/><path d="M3 6V11H10V6"/></svg></button>`;
        let crumbs = `${this.crumbsBefore()}<a href="#" class="crumb${isAtRoot ? ' crumb-current' : ''}" data-path="">${escapeHtml(this.rootLabel)}</a>`;
        let accumulated = '';
        for (let i = 0; i < parts.length; i++) {
            const part = parts[i];
            accumulated = accumulated ? `${accumulated}/${part}` : part;
            const isCurrent = i === parts.length - 1;
            crumbs += `<span class="crumb-sep"> / </span><a href="#" class="crumb${isCurrent ? ' crumb-current' : ''}" data-path="${escapeHtml(accumulated)}">${escapeHtml(part)}</a>`;
        }
        // Without a head of its own (Organize), the pane keeps its slideshow
        // and ⋯ at the end of this row.
        const end = this.placeHeader ? '' : `<span class="browse-row-end">${this._slideshowButtonHTML()}<span class="browse-more"></span></span>`;
        return `<div class="breadcrumb-row">${homeBtn}${upBtn}<nav class="breadcrumb">${crumbs}</nav><span class="browse-library-badge" style="display:none"></span>${end}</div>`;
    }

    // crumbsBefore is what leads to the top folder from outside it: nothing
    // in Folders, the list of libraries in a library.
    crumbsBefore() {
        return '';
    }

    // The slideshow is always on screen where a pane offers one, beside the
    // ⋯ menu: a frame with a play mark, and its name.
    _slideshowButtonHTML() {
        if (!this.onSlideshowInvoke) return '';
        const noPhotos = this.getImageEntries().length === 0 && this.selectedDirs.size === 0;
        return `<button class="btn btn-sm browse-slideshow-btn" aria-label="${t('Slideshow')}"${noPhotos ? ` disabled title="${t('This folder holds no photos')}"` : ''}><svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><rect x="1" y="2" width="11" height="9" rx="1"/><path d="M5.25 4.75v3.5l3-1.75z" fill="currentColor"/></svg><span class="browse-slideshow-label">${t('Slideshow')}</span></button>`;
    }

    _syncSelectAll() {
        this._selectAll.update(this.selection.selected.size, this._loading ? 0 : this.getImageEntries().length);
    }

    _renderControls() {
        return `<div class="controls">
            <div class="controls-left">
            <!-- The layout is a view you switch while looking at photos, so
                 it is a visible control rather than an entry in a menu. -->
            <div class="seg" role="group" aria-label="${t('Layout')}">
                ${[['justified', t('Justified')], ['grid', t('Grid')], ['list', t('List')]].map(([v, label]) => `
                    <button aria-pressed="${this.view === v}" data-view="${v}">${label}</button>`).join('')}
            </div>
            <select class="btn btn-sm select-btn sort-field" aria-label="${t('Sort photos')}">
                ${[['taken', t('Photo taken')], ['name', t('Name')], ['date', t('File modified')], ['size', t('Size')]].map(([v, label]) => `
                    <option value="${v}" ${this.sort === v ? 'selected' : ''}>${label}</option>`).join('')}
            </select>
            <button class="btn btn-sm sort-order" title="${this.order === 'asc' ? t('Sorting oldest first') : t('Sorting newest first')}" aria-label="${t('Reverse the order')}">${this.order === 'asc' ? '↑' : '↓'}</button>
            </div>
            <span class="browse-select-all"></span>
        </div>`;
    }

    // --- Incremental rendering ---

    _setupObserver() {
        if (!this._contentEl) return;
        const sentinel = this._contentEl.querySelector('.scroll-sentinel');
        if (!sentinel) return;
        this._observer = new IntersectionObserver((entries) => {
            if (entries[0].isIntersecting) this._renderNextChunk();
        }, { root: this._contentEl, rootMargin: '200px' });
        this._observer.observe(sentinel);
    }

    _destroyObserver() {
        if (this._observer) {
            this._observer.disconnect();
            this._observer = null;
        }
    }

    _renderNextChunk() {
        const start = this._renderedCount;
        const end = Math.min(start + CHUNK_SIZE, this.entries.length);
        if (start >= end) return;
        const ct = this._contentEl || this.container;

        if (this.view === 'grid') {
            const grid = ct.querySelector('.grid');
            if (grid) grid.insertAdjacentHTML('beforeend', this._gridRenderer.renderChunk(start, end));
        } else if (this.view === 'justified') {
            const justified = ct.querySelector('.justified');
            if (justified) {
                justified.insertAdjacentHTML('beforeend', this._justifiedRenderer.renderChunk(start, end));
                this._justifiedRenderer.scheduleRelayout();
            }
        } else {
            const tbody = ct.querySelector('tbody');
            if (tbody) tbody.insertAdjacentHTML('beforeend', this._listRenderer.renderChunk(start, end));
        }

        this._renderedCount = end;
        if (this.view === 'justified') {
            this._attachJustifiedImgLoad(start, end);
        }

        if (end >= this.entries.length) {
            const sentinel = ct.querySelector('.scroll-sentinel');
            if (sentinel) sentinel.remove();
            this._destroyObserver();
        }
    }

    _ensureRenderedUpTo(index) {
        while (this._renderedCount <= index && this._renderedCount < this.entries.length) {
            this._renderNextChunk();
        }
    }

    _getThumbnailSize() {
        const quality = localStorage.getItem('thumbnail-quality') || 'standard';
        const dpr = window.devicePixelRatio || 1;
        const maxSize = quality === 'high' ? 1024 : 300;
        const clampSize = (size) => Math.max(50, Math.min(maxSize, Math.round(size)));
        if (this.view === 'grid') {
            const grid = this.container.querySelector('.grid');
            if (grid) {
                const item = grid.querySelector('.grid-item');
                if (item) return clampSize(item.offsetWidth * dpr);
            }
            return clampSize(200 * dpr);
        }
        if (this.view === 'justified') return clampSize(this._justifiedTargetHeight * dpr);
        return clampSize(32 * dpr);
    }

    // --- Events ---

    attachEvents() {
        this.container.querySelectorAll('.warning-dismiss').forEach(el => {
            el.addEventListener('click', () => {
                this.warnings.splice(parseInt(el.dataset.warningIndex), 1);
                this.render();
            });
        });
        this._wireNavigation();
        this._wireControls();
        if (this.view === 'justified') this._attachJustifiedImgLoad(0, this._renderedCount);
    }

    // The breadcrumb, Up and Home each load a folder and say where it went.
    _wireNavigation() {
        const go = (path) => {
            this.load(path);
            if (this.onNavigate) this.onNavigate(path);
        };
        this.container.querySelectorAll('.crumb').forEach(el => {
            el.addEventListener('click', (e) => { e.preventDefault(); go(el.dataset.path); });
        });
        this.container.querySelector('.up-dir-btn')?.addEventListener('click', () => {
            const parts = this.path.split('/').filter(Boolean);
            if (parts.length === 0) return;
            parts.pop();
            go(parts.join('/'));
        });
        this.container.querySelector('.home-btn')?.addEventListener('click', () => go(this.homeTarget()));
    }

    // Layout, slideshow, ⋯ and sort.
    _wireControls() {
        this.container.querySelectorAll('[data-view]').forEach(el => {
            el.addEventListener('click', () => this.setView(el.dataset.view));
        });
        this.container.querySelector('.browse-slideshow-btn')?.addEventListener('click', () => this.onSlideshowInvoke());
        const moreSlot = this.container.querySelector('.browse-more');
        if (moreSlot) {
            // One menu per pane, moved into each render, so a menu left open
            // by a switch keeps its button.
            this._menu ??= new Menu({ items: () => this._menuItems() });
            moreSlot.appendChild(this._menu.button);
        }
        const sortSelect = this.container.querySelector('.sort-field');
        sortSelect?.addEventListener('change', () => this.setSort(sortSelect.value, this.order));
        this.container.querySelector('.sort-order')?.addEventListener('click', () => this.setSort(this.sort, this.order === 'asc' ? 'desc' : 'asc'));
    }

    // Attaches img.load listeners for justified-view aspect-ratio recording on newly rendered items.
    _attachJustifiedImgLoad(start, end) {
        this.container.querySelectorAll('[data-index]').forEach(el => {
            const idx = parseInt(el.dataset.index);
            if (idx < start || idx >= end || el.dataset.type !== 'image') return;
            const img = el.querySelector('img');
            if (!img) return;
            const recordAR = () => {
                if (img.naturalWidth && img.naturalHeight) {
                    this._aspectRatios[idx] = img.naturalWidth / img.naturalHeight;
                    this._justifiedRenderer.scheduleRelayout();
                }
            };
            if (img.naturalWidth) recordAR();
            else img.addEventListener('load', recordAR);
        });
    }

    // --- Folder tool buttons ---
    //
    // The two folder-level buttons say what they would act on, and are hidden
    // or disabled only when there is genuinely nothing for them to do.

    // _menuItems is what the ⋯ menu offers now: how photos are shown, and on
    // a desk the rarer folder work. The slideshow has its own button. A phone looks; it does
    // not scan or index. The cache is cleared in Settings › Storage. A pane offers only what its host wired
    // up — Organize's source pane has no folder tools.
    _menuItems() {
        const items = [
            {
                id: 'names', label: t('Names'), switch: { on: this.showNames, labelOn: t('Shown'), labelOff: t('Hidden') },
                onChange: (on) => { this.showNames = on; this.render(); },
            },
            {
                id: 'details', label: t('Details'), switch: { on: this.showOverlays, labelOn: t('Shown'), labelOff: t('Hidden') },
                onChange: (on) => { this.showOverlays = on; this.render(); },
            },
        ];
        // A phone has no key to bring a hidden interface back.
        if (window.matchMedia('(max-width: 700px)').matches) return items;
        items.push({
            id: 'interface', label: t('Interface (H)'), switch: { on: !App.uiHidden, labelOn: t('Shown'), labelOff: t('Hidden') },
            onChange: () => App.toggleUIVisibility(),
        });
        if (!this.onToolInvoke) return items;
        items.push('separator');
        const dir = this.getFocusedDir();
        if (dir && App.mode !== 'library') {
            items.push({ id: 'make-library', label: t('Make library…'), onSelect: () => this._runFolderTool('make-library') });
        }
        if (App.mode === 'library') {
            items.push({ id: 'scan', label: t('Scan for new photos'), onSelect: () => this._runFolderTool('lib-scan-new') });
        }
        return items;
    }

    // _runFolderTool runs one of the folder tools of the ⋯ menu.
    _runFolderTool(tool) {
        if (tool === 'make-library') {
            const dir = this.getFocusedDir();
            if (dir) this.onToolInvoke({ tool, path: dir });
            return;
        }
        if (tool === 'lib-scan-new') {
            const scanPaths = this.selectedDirs.size > 0 ? Array.from(this.selectedDirs) : [this.path];
            const doNext = (i) => {
                if (i >= scanPaths.length) return;
                this.onToolInvoke({ tool, files: [], path: scanPaths[i], onDone: () => doNext(i + 1) });
            };
            doNext(0);
        }
    }

    // --- Focus change notification ---

    // A folder is a tile in the row above the photos, the same tile a
    // library draws. A folder a library covers shows what the library knows
    // of it — four photos, how many and the years — once that has come; a
    // folder no library covers shows the folder sign.
    _folderItemHTML(idx, name, focusedClass) {
        const preview = this._folderPreviews.get(name);
        const body = preview ? this._folderTileBody(name, preview) : folderSignHTML(name);
        return this._folderTileHTML(idx, name, focusedClass, body);
    }

    _folderTileHTML(idx, name, focusedClass, body) {
        return folderTileHTML(idx, name, focusedClass + (this.isMarkedForDeletion(this.fullPath(name)) ? ' marked-for-deletion' : ''), body);
    }

    _folderTileBody(name, preview) {
        return folderTileBodyHTML(this._previewLibID, name, preview);
    }

    // The tiles appear with the folder names at once; what each folder holds
    // follows from the library's index and fills them in place. path is the
    // pane's folder when asked, libPath the same folder inside the library.
    async _loadFolderPreviews(path, libPath) {
        let previews;
        try {
            const r = await fetch(`/api/library/${this._previewLibID}/folder-previews?path=${encodeURIComponent(libPath)}`);
            if (!r.ok) return;
            previews = await r.json();
        } catch { return; }
        if (path !== this.path) return;
        for (const p of previews) this._folderPreviews.set(p.name, p);
        for (const el of this.container.querySelectorAll('.folder-tile[data-name]')) {
            const p = this._folderPreviews.get(el.dataset.name);
            if (p) el.querySelector('.folder-tile-body').innerHTML = this._folderTileBody(el.dataset.name, p);
        }
    }

    _notifyFocusChange() {
        if (!this.onFocusChange) return;
        const idx = this.keyboard.focusedIndex;
        if (idx < 0 || idx >= this.entries.length) { this.onFocusChange(null, null); return; }
        const entry = this.entries[idx];
        if (entry.type === 'image') {
            this.onFocusChange(this.fullPath(entry.name), 'image');
        } else if (entry.type === 'dir') {
            this.onFocusChange(this.fullPath(entry.name), 'dir');
        } else {
            this.onFocusChange(null, null);
        }
    }

    // --- EXIF date polling ---

    _pollExifDates() {
        const pollPath = this.path;
        this._exifPollPath = pollPath;
        setTimeout(() => this._doExifPoll(pollPath), 300);
    }

    async _doExifPoll(pollPath) {
        if (this._exifPollPath !== pollPath) return;
        let data;
        try { data = await API.browseDates(pollPath); } catch { return; }
        if (this._exifPollPath !== pollPath) return;
        if (!data.ready) { setTimeout(() => this._doExifPoll(pollPath), 500); return; }
        if (data.dates && Object.keys(data.dates).length > 0) {
            for (const entry of this.entries) {
                if (entry.type === 'image' && data.dates[entry.name]) entry.exifDate = data.dates[entry.name];
            }
            if (this.sort === 'taken') this._resortAndRender();
            else if (this.view === 'list') this.render();
        }
    }

    // --- Overlay meta polling ---

    _pollOverlayMeta() {
        const pollPath = this.path;
        this._metaPollPath = pollPath;
        setTimeout(() => this._doMetaPoll(pollPath), 300);
    }

    async _doMetaPoll(pollPath) {
        if (this._metaPollPath !== pollPath) return;
        let data;
        try { data = await API.browseMeta(pollPath); } catch { return; }
        if (this._metaPollPath !== pollPath) return;
        if (!data.ready) { setTimeout(() => this._doMetaPoll(pollPath), 500); return; }
        if (data.meta) {
            this._entryMeta = data.meta;
            if (this.showOverlays) this._updateOverlays();
        }
    }

    _updateOverlays() {
        updateOverlayBadges(this._contentEl || this.container, name => this._buildOverlayBadges(name, this._entryMeta[name]));
    }

    _buildOverlayBadges(name, meta) {
        return this.showOverlays ? overlayBadgesHTML(name, meta) : '';
    }

    // --- Client-side sort ---

    _resortAndRender() {
        this.entries.sort(compareBrowseEntries(this.sort, this.order));
        this.render();
    }
}

function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}
