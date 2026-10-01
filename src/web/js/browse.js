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
        this.onNavigate = options.onNavigate || null;
        this.onImageClick = options.onImageClick || null;
        this.onSelectionChange = options.onSelectionChange || null;
        this.onFocusChange = options.onFocusChange || null;
        this.onLoad = options.onLoad || null;
        // What the place is, shown at the top folder only (placeLede HTML).
        this.lede = options.lede || '';
        this.showNames = false;
        this.showOverlays = true;
        this.onToolInvoke = options.onToolInvoke || null;
        this.onSlideshowInvoke = options.onSlideshowInvoke || null;
        this._toolsChecked = null;
        this._loading = false;
        this._loadController = null;
        this._renderedCount = 0;
        this._observer = null;
        this._exifPollPath = null;
        this._metaPollPath = null;
        this._entryMeta = {};
        this._aspectRatios = {};
        this._justifiedTargetHeight = 200;
        this._resizeHandler = null;
        this._contentEl = null;
        this.selectedDirs = new Set();
        this._pendingPreselect = null;
        this._libraries = [];

        this.selection = new SelectionManager((files) => {
            if (this.onSelectionChange) this.onSelectionChange(files);
        });
        this.keyboard = new BrowseKeyboard(this);
        this._gridRenderer = new GridRenderer(this);
        this._listRenderer = new ListRenderer(this);
        this._justifiedRenderer = new JustifiedRenderer(this);

        this._attachDelegatedEvents();
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
        // The aborted load() will receive an AbortError and return silently.
        if (this._loadController) this._loadController.abort();
        const controller = new AbortController();
        this._loadController = controller;

        this._loading = true;
        const isReload = (path || '') === this.path;
        const scrollEl = this._contentEl || this.container;
        const savedScroll = isReload ? scrollEl.scrollTop : 0;
        this.path = path || '';
        const hadSelection = this.selection.selected.size + this.selectedDirs.size > 0;
        this.selection.clear();
        this.selectedDirs.clear();
        // The bar counts folders too: going into the one selected by the
        // first click of a double click must take the bar away with it.
        if (hadSelection && this.onSelectionChange) this.onSelectionChange([]);
        this.keyboard.focusedIndex = 0;
        this.warnings = [];
        this.entries = [];
        this._exifPollPath = null;
        this._metaPollPath = null;
        this._entryMeta = {};
        this._aspectRatios = {};
        this.render();

        let data;
        try {
            data = await API.browse(this.path, this.sort, this.order, controller.signal);
        } catch (err) {
            if (err.name === 'AbortError' || this._loadController !== controller) return;
            this._loading = false;
            this._loadController = null;
            this.container.innerHTML = `<div class="error">Failed to load: ${err.message}</div>`;
            return;
        }

        if (this._loadController !== controller) return;
        this._loading = false;
        this._loadController = null;
        this.entries = data.entries || [];
        this.warnings = data.warnings || [];
        this.render();
        this.keyboard.updateFocusClass();

        if (isReload && savedScroll > 0) {
            const restoreEl = this._contentEl || this.container;
            while (this._renderedCount < this.entries.length &&
                   restoreEl.scrollHeight <= savedScroll + restoreEl.clientHeight) {
                this._renderNextChunk();
            }
            restoreEl.scrollTop = savedScroll;
        }
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

    homeLabel() { return 'Home folder'; }

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
            }
        } catch { /* ignore */ }
    }

    _updateLibraryBadge() {
        const badge = this.container.querySelector('.browse-library-badge');
        if (!badge) return;
        // Says which libraries catalog this folder, and goes there.
        const libs = this._libraries;
        if (libs.length) {
            badge.innerHTML = `In ${libs.length === 1 ? 'library' : 'libraries'} ${libs.map(libraryLink).join(', ')}`;
            badge.style.display = '';
        } else {
            badge.style.display = 'none';
        }
    }

    // --- Rendering ---

    render() {
        this._destroyObserver();
        this._renderedCount = 0;

        const header = [];
        if (this.warnings && this.warnings.length > 0) header.push(this._renderWarnings());
        header.push(this._renderBreadcrumb());
        header.push(this._renderControls());

        const content = [];
        if (!this._loading && this.entries.length > 0) content.push(this._renderFolderTitle());
        if (this._loading) {
            content.push('<div class="browse-loading"></div>');
        } else if (this.entries.length === 0) {
            content.push('<div class="empty">No images or folders found</div>');
        } else {
            const end = Math.min(CHUNK_SIZE, this.entries.length);
            content.push(this._renderChunk(0, end));
            this._renderedCount = end;
            if (end < this.entries.length) content.push('<div class="scroll-sentinel"></div>');
        }

        if (this._resizeHandler) {
            window.removeEventListener('resize', this._resizeHandler);
            this._resizeHandler = null;
        }

        this.container.innerHTML =
            `<div class="browse-header">${header.join('')}</div>` +
            `<div class="browse-content">${content.join('')}</div>`;
        this._contentEl = this.container.querySelector('.browse-content');
        const loadingEl = this._contentEl.querySelector('.browse-loading');
        if (loadingEl) Activity.in(loadingEl, 'Reading the folder…', { area: true });
        this.attachEvents();
        this._setupObserver();

        if (this.view === 'justified') {
            this._justifiedRenderer.layout();
            this._resizeHandler = () => this._justifiedRenderer.scheduleRelayout();
            window.addEventListener('resize', this._resizeHandler);
        }
    }

    // The folder's name, and what is in it. The counts are what the pane
    // already knows; nothing is fetched for this.
    _renderFolderTitle() {
        const name = this.path ? this.path.split('/').filter(Boolean).pop() : 'Root';
        const images = this.getImageEntries().length;
        const dirs = this.entries.filter(e => e.type === 'dir').length;
        const bytes = this.entries.reduce((sum, e) => sum + (e.size || 0), 0);
        const parts = [
            `${images} photo${images !== 1 ? 's' : ''}`,
            dirs > 0 ? `${dirs} folder${dirs !== 1 ? 's' : ''}` : null,
            bytes > 0 ? formatSize(bytes) : null,
        ].filter(Boolean);
        return `<div class="folder-title">
            <h1>${escapeHtml(name)}</h1>
            <span class="folder-title-meta">${parts.join(' · ')}</span>
            ${this.path ? '' : this.lede}
        </div>`;
    }

    _renderChunk(start, end) {
        if (this.view === 'grid')      return this._gridRenderer.renderChunk(start, end);
        if (this.view === 'justified') return this._justifiedRenderer.renderChunk(start, end);
        return this._listRenderer.renderChunk(start, end);
    }

    _renderWarnings() {
        return this.warnings.map((w, i) =>
            `<div class="warning-banner" data-warning-index="${i}">
                <span class="warning-message">${w.message}</span>
                <button class="btn btn-sm warning-dismiss" data-warning-index="${i}" title="Dismiss">&times;</button>
            </div>`
        ).join('');
    }

    _renderBreadcrumb() {
        const parts = this.path ? this.path.split('/') : [];
        const isAtRoot = parts.length === 0;
        const isAtHome = this.path === this.homeTarget();
        const upBtn = `<button class="btn btn-sm up-dir-btn" title="Go up"${isAtRoot ? ' disabled' : ''}><svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><line x1="6.5" y1="10.5" x2="6.5" y2="2.5"/><polyline points="3 6 6.5 2.5 10 6"/></svg></button>`;
        const homeBtn = `<button class="btn btn-sm home-btn" title="${escapeHtml(this.homeLabel())}" aria-label="${escapeHtml(this.homeLabel())}"${isAtHome ? ' disabled' : ''}><svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="1.5 6.5 6.5 1.5 11.5 6.5"/><path d="M3 6V11H10V6"/></svg></button>`;
        let crumbs = `<a href="#" class="crumb${isAtRoot ? ' crumb-current' : ''}" data-path="">Root</a>`;
        let accumulated = '';
        for (let i = 0; i < parts.length; i++) {
            const part = parts[i];
            accumulated = accumulated ? `${accumulated}/${part}` : part;
            const isCurrent = i === parts.length - 1;
            crumbs += `<span class="crumb-sep"> / </span><a href="#" class="crumb${isCurrent ? ' crumb-current' : ''}" data-path="${accumulated}">${part}</a>`;
        }
        return `<div class="breadcrumb-row">${homeBtn}${upBtn}<nav class="breadcrumb">${crumbs}</nav><span class="browse-library-badge" style="display:none"></span><span class="browse-row-end">${this._slideshowButtonHTML()}<span class="browse-more"></span></span></div>`;
    }

    // The slideshow is always on screen where a pane offers one, beside the
    // ⋯ menu: a frame with a play mark, and its name.
    _slideshowButtonHTML() {
        if (!this.onSlideshowInvoke) return '';
        const noPhotos = this.getImageEntries().length === 0 && this.selectedDirs.size === 0;
        return `<button class="btn btn-sm browse-slideshow-btn" aria-label="Slideshow"${noPhotos ? ' disabled title="This folder holds no photos"' : ''}><svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><rect x="1" y="2" width="11" height="9" rx="1"/><path d="M5.25 4.75v3.5l3-1.75z" fill="currentColor"/></svg><span class="browse-slideshow-label">Slideshow</span></button>`;
    }

    _renderControls() {
        const imageCount = this.getImageEntries().length;
        const selectedCount = this.selection.selected.size;
        // While the folder is read, the count is not known yet.
        const statusText = this._loading ? ''
            : imageCountLabel(imageCount, selectedCount);

        return `<div class="controls">
            <div class="controls-left">
            <!-- The layout is a view you switch while looking at photos, so
                 it is a visible control rather than an entry in a menu. -->
            <div class="seg" role="group" aria-label="Layout">
                ${['justified', 'grid', 'list'].map(v => `
                    <button aria-pressed="${this.view === v}" data-view="${v}">${v[0].toUpperCase() + v.slice(1)}</button>`).join('')}
            </div>
            <select class="btn btn-sm select-btn sort-field" aria-label="Sort photos">
                ${[['taken', 'Photo taken'], ['name', 'Name'], ['date', 'File modified'], ['size', 'Size']].map(([v, label]) => `
                    <option value="${v}" ${this.sort === v ? 'selected' : ''}>${label}</option>`).join('')}
            </select>
            <button class="btn btn-sm sort-order" title="${this.order === 'asc' ? 'Sorting oldest first' : 'Sorting newest first'}" aria-label="Reverse the order">${this.order === 'asc' ? '↑' : '↓'}</button>
            </div>
            <span class="status-bar">${statusText}</span>
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
                const idx = parseInt(el.dataset.warningIndex);
                this.warnings.splice(idx, 1);
                this.render();
            });
        });

        this.container.querySelectorAll('.crumb').forEach(el => {
            el.addEventListener('click', (e) => {
                e.preventDefault();
                const path = el.dataset.path;
                this.load(path);
                if (this.onNavigate) this.onNavigate(path);
            });
        });

        const upBtn = this.container.querySelector('.up-dir-btn');
        if (upBtn) {
            upBtn.addEventListener('click', () => {
                const parts = this.path.split('/').filter(Boolean);
                if (parts.length === 0) return;
                parts.pop();
                const parentPath = parts.join('/');
                this.load(parentPath);
                if (this.onNavigate) this.onNavigate(parentPath);
            });
        }

        const homeBtn = this.container.querySelector('.home-btn');
        if (homeBtn) {
            homeBtn.addEventListener('click', () => {
                const path = this.homeTarget();
                this.load(path);
                if (this.onNavigate) this.onNavigate(path);
            });
        }

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
        if (sortSelect) sortSelect.addEventListener('change', () => this.setSort(sortSelect.value, this.order));

        const sortOrder = this.container.querySelector('.sort-order');
        if (sortOrder) sortOrder.addEventListener('click', () => this.setSort(this.sort, this.order === 'asc' ? 'desc' : 'asc'));

        if (this.view === 'justified') {
            this._attachJustifiedImgLoad(0, this._renderedCount);
        }
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
    // not scan, index or clear caches. A pane offers only what its host wired
    // up — Organize's source pane has no folder tools.
    _menuItems() {
        const items = [
            {
                id: 'names', label: 'Names', switch: { on: this.showNames, labelOn: 'Shown', labelOff: 'Hidden' },
                onChange: (on) => { this.showNames = on; this.render(); },
            },
            {
                id: 'details', label: 'Details', switch: { on: this.showOverlays, labelOn: 'Shown', labelOff: 'Hidden' },
                onChange: (on) => { this.showOverlays = on; this.render(); },
            },
        ];
        if (!this.onToolInvoke || window.matchMedia('(max-width: 700px)').matches) return items;
        items.push('separator');
        const dir = this.getFocusedDir();
        if (dir && App.mode !== 'library') {
            items.push({ id: 'make-library', label: 'Make library…', onSelect: () => this._runFolderTool('make-library') });
        }
        if (App.mode === 'library') {
            items.push({ id: 'scan', label: 'Scan for new photos', onSelect: () => this._runFolderTool('lib-scan-new') });
        }
        items.push({ id: 'clear-cache', ...this._clearCacheLabel(), onSelect: () => this._runFolderTool('clear-cache') });
        return items;
    }

    // _clearCacheLabel says what Clear cache would clear, or why it cannot.
    _clearCacheLabel() {
        const files = this.getActionableFiles();
        if (files.length > 0) {
            return { label: `Clear cache · ${files.length} file${files.length !== 1 ? 's' : ''}`, title: 'Clear the cached previews of the selected files' };
        }
        if (this.getFocusedDir()) {
            return { label: 'Clear cache · folder', title: 'Clear the cached previews of the focused folder' };
        }
        return { label: 'Clear cache', disabled: true, title: 'Select photos or a folder whose cached previews to clear' };
    }

    // _runFolderTool runs one of the folder tools of the ⋯ menu.
    _runFolderTool(tool) {
        if (tool === 'make-library') {
            const dir = this.getFocusedDir();
            if (dir) this.onToolInvoke({ tool, path: dir });
            return;
        }
        if (tool === 'clear-cache') {
            const files = this.getActionableFiles();
            const dir = this.getFocusedDir();
            if (files.length === 0 && !dir) return;
            this.onToolInvoke({ tool, files, path: dir || this.path, onDone: () => {} });
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
    // library draws. A plain folder has no index to say what it holds, so it
    // shows the folder sign where a library shows four photos.
    _folderItemHTML(idx, name, focusedClass) {
        const markedClass = this.isMarkedForDeletion(this.fullPath(name)) ? ' marked-for-deletion' : '';
        return `<button class="folder-tile dir-item${focusedClass}${markedClass}" data-index="${idx}" data-name="${escapeHtml(name)}" data-type="dir">
            <span class="folder-tile-body">
                <span class="folder-tile-icon" aria-hidden="true"><svg width="40" height="32" viewBox="0 0 18 14" fill="none" stroke="currentColor" stroke-width="1" stroke-linejoin="round"><path d="M1 2.5v10h16v-8.5H8L6.5 2.5H1z"/></svg></span>
                <span class="item-name">${escapeHtml(name)}</span>
            </span>
        </button>`;
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
        const ct = this._contentEl || this.container;
        ct.querySelectorAll('[data-type="image"]').forEach(el => {
            const name = el.dataset.name;
            const badges = this._buildOverlayBadges(name, this._entryMeta[name]);
            if (el.tagName === 'TR') {
                const nameCell = el.querySelector('td.list-name');
                if (!nameCell) return;
                let listBadges = nameCell.querySelector('.list-badges');
                if (badges) {
                    if (!listBadges) { listBadges = document.createElement('span'); listBadges.className = 'list-badges'; nameCell.appendChild(listBadges); }
                    listBadges.innerHTML = badges;
                } else if (listBadges) {
                    listBadges.remove();
                }
            } else {
                const existing = el.querySelector('.overlay-badges');
                if (existing) existing.remove();
                if (badges) el.insertAdjacentHTML('beforeend', badges);
            }
        });
    }

    // --- Overlay badge builders ---

    // All overlay chips look the same: a dark chip with mono text over the
    // photo. Colour is a signal in this UI (ADR-0030), and a colour per file
    // format and per film simulation would be eight decorative colours
    // competing with the photograph itself. Every value from before is still
    // shown, in the same order.
    _buildOverlayBadges(name, meta) {
        if (!this.showOverlays) return '';
        const badges = [];
        if (meta && meta.hasGPS) {
            badges.push(`<span class="overlay-badge overlay-badge-gps" title="Has GPS coordinates"><svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5a2.5 2.5 0 1 1 0-5 2.5 2.5 0 0 1 0 5z"/></svg></span>`);
        }
        const fileType = this._fileTypeLabel(name);
        if (fileType) badges.push(`<span class="overlay-badge">${fileType}</span>`);
        if (meta && meta.filmSimulation) {
            badges.push(`<span class="overlay-badge">${escapeHtml(meta.filmSimulation)}</span>`);
        }
        if (meta && meta.aspectRatio) {
            const icon = this._aspectRatioIcon(meta.aspectRatio);
            badges.push(`<span class="overlay-badge overlay-badge-ratio">${icon}${escapeHtml(meta.aspectRatio)}</span>`);
        }
        if (badges.length === 0) return '';
        return `<div class="overlay-badges">${badges.join('')}</div>`;
    }

    _fileTypeLabel(name) {
        const labels = {
            jpg: 'JPEG', jpeg: 'JPEG',
            heif: 'HEIF', heic: 'HEIF', hif: 'HEIF',
            png: 'PNG', gif: 'GIF', webp: 'WebP',
        };
        return labels[name.split('.').pop().toLowerCase()] || null;
    }

    _aspectRatioIcon(ratioStr) {
        const isCustom = ratioStr === 'Custom Crop';
        let ratio = 1;
        if (!isCustom) {
            const parts = ratioStr.split(':');
            if (parts.length === 2) ratio = parseFloat(parts[0]) / parseFloat(parts[1]);
        }
        let rw, rh;
        if (ratio > 14 / 10) { rw = 14; rh = 14 / ratio; }
        else { rh = 10; rw = 10 * ratio; }
        const x = ((16 - rw) / 2).toFixed(1);
        const y = ((12 - rh) / 2).toFixed(1);
        const dash = isCustom ? ' stroke-dasharray="2 1"' : '';
        return `<svg width="16" height="12" viewBox="0 0 16 12" fill="none" stroke="rgba(255,255,255,0.9)" stroke-width="1.5"${dash}><rect x="${x}" y="${y}" width="${rw.toFixed(1)}" height="${rh.toFixed(1)}" rx="0.5"/></svg>`;
    }

    // --- Client-side sort ---

    _resortAndRender() {
        this.entries.sort((a, b) => {
            if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
            let less;
            switch (this.sort) {
                case 'date': less = new Date(a.date) < new Date(b.date); break;
                case 'taken': {
                    const aDate = a.exifDate ? new Date(a.exifDate) : null;
                    const bDate = b.exifDate ? new Date(b.exifDate) : null;
                    if (!aDate && !bDate) return 0;
                    if (!aDate) return 1;
                    if (!bDate) return -1;
                    less = aDate < bDate;
                    break;
                }
                case 'size': less = (a.size || 0) < (b.size || 0); break;
                default: less = a.name.toLowerCase() < b.name.toLowerCase();
            }
            return this.order === 'desc' ? (less ? 1 : -1) : (less ? -1 : 1);
        });
        this.render();
    }
}

function formatDate(iso) {
    if (!iso) return '';
    return iso.replace('T', ' ').replace(/Z$/, '').replace(/([+-]\d{2}:\d{2})$/, ' $1').trim();
}

function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}
