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
        this.showNames = false;
        this.showOverlays = true;
        this.onToolInvoke = options.onToolInvoke || null;
        this.onSlideshowInvoke = options.onSlideshowInvoke || null;
        this._toolsChecked = null;
        this._loading = false;
        this._renderedCount = 0;
        this._observer = null;
        this._exifPollPath = null;
        this._metaPollPath = null;
        this._entryMeta = {};
        this._aspectRatios = {};
        this._justifiedTargetHeight = 200;
        this._resizeHandler = null;
        this._contentEl = null;

        this.selection = new SelectionManager((files) => {
            if (this.onSelectionChange) this.onSelectionChange(files);
        });
        this.keyboard = new BrowseKeyboard(this);
        this._gridRenderer = new GridRenderer(this);
        this._listRenderer = new ListRenderer(this);
        this._justifiedRenderer = new JustifiedRenderer(this);
    }

    // Getters so external code (commander.js, renderers) can access sub-object state via the pane directly
    get focusedIndex() { return this.keyboard.focusedIndex; }
    set focusedIndex(v) { this.keyboard.focusedIndex = v; }
    get selected() { return this.selection.selected; }

    // --- Public API ---

    async load(path) {
        if (this._loading) return;
        this._loading = true;
        const isReload = (path || '') === this.path;
        const scrollEl = this._contentEl || this.container;
        const savedScroll = isReload ? scrollEl.scrollTop : 0;
        this.path = path || '';
        this.selection.clear();
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
            data = await API.browse(this.path, this.sort, this.order);
        } catch (err) {
            this._loading = false;
            this.container.innerHTML = `<div class="error">Failed to load: ${err.message}</div>`;
            return;
        }

        this._loading = false;
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

        if (this.onLoad) this.onLoad();
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

    getFocusedDir()    { return this.keyboard.getFocusedDir(); }
    getFocusedFile()   { return this.keyboard.getFocusedFile(); }
    getFocusedEntry()  { return this.keyboard.getFocusedEntry(); }
    moveFocus(delta)   { this.keyboard.moveFocus(delta); }
    activateFocused()  { this.keyboard.activateFocused(); }
    getColumnCount()   { return this.keyboard.getColumnCount(); }

    toggleFocusedSelection() { this.keyboard.toggleFocusedSelection(); }

    selectAll() {
        this.selection.selectAll(this.entries, n => this.fullPath(n));
        this.render();
    }

    fullPath(name) {
        return this.path ? `${this.path}/${name}` : name;
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

    // --- Rendering ---

    render() {
        this._destroyObserver();
        this._renderedCount = 0;

        const header = [];
        if (this.warnings && this.warnings.length > 0) header.push(this._renderWarnings());
        header.push(this._renderBreadcrumb());
        header.push(this._renderControls());

        const content = [];
        if (this._loading) {
            content.push('<div class="browse-loading"><div class="browse-spinner"></div></div>');
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
        this.attachEvents();
        this._setupObserver();

        if (this.view === 'justified') {
            this._justifiedRenderer.layout();
            this._resizeHandler = () => this._justifiedRenderer.scheduleRelayout();
            window.addEventListener('resize', this._resizeHandler);
        }
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
        let crumbs = `<a href="#" class="crumb${isAtRoot ? ' crumb-current' : ''}" data-path="">Root</a>`;
        let accumulated = '';
        for (let i = 0; i < parts.length; i++) {
            const part = parts[i];
            accumulated = accumulated ? `${accumulated}/${part}` : part;
            const isCurrent = i === parts.length - 1;
            crumbs += `<span class="crumb-sep"> / </span><a href="#" class="crumb${isCurrent ? ' crumb-current' : ''}" data-path="${accumulated}">${part}</a>`;
        }
        return `<nav class="breadcrumb">${crumbs}</nav>`;
    }

    _renderControls() {
        const imageCount = this.getImageEntries().length;
        const selectedCount = this.selection.selected.size;
        const statusText = selectedCount > 0
            ? `${imageCount} images · ${selectedCount} selected`
            : `${imageCount} images`;

        return `<div class="controls">
            <div class="controls-left">
            <div class="dropdown-wrap">
                <button class="btn btn-sm dropdown-btn view-menu-btn" title="View options">
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><line x1="2" y1="3.5" x2="12" y2="3.5"/><line x1="2" y1="7" x2="12" y2="7"/><line x1="2" y1="10.5" x2="12" y2="10.5"/><circle cx="5" cy="3.5" r="1.5" fill="currentColor" stroke="none"/><circle cx="9" cy="7" r="1.5" fill="currentColor" stroke="none"/><circle cx="6" cy="10.5" r="1.5" fill="currentColor" stroke="none"/></svg>
                    View
                    <svg class="dropdown-chevron" width="8" height="8" viewBox="0 0 8 8" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3l2 2 2-2"/></svg>
                </button>
                <div class="dropdown-menu view-menu" style="display:none">
                    <div class="dropdown-section">
                        <label class="dropdown-label">Layout</label>
                        <div class="dropdown-toggle">
                            <button class="btn btn-sm ${this.view === 'grid' ? 'active' : ''}" data-view="grid">Grid</button>
                            <button class="btn btn-sm ${this.view === 'justified' ? 'active' : ''}" data-view="justified">Justified</button>
                            <button class="btn btn-sm ${this.view === 'list' ? 'active' : ''}" data-view="list">List</button>
                        </div>
                    </div>
                    <div class="dropdown-section">
                        <label class="dropdown-label">Show names</label>
                        <div class="toggle-names-wrap"></div>
                    </div>
                    <div class="dropdown-section">
                        <label class="dropdown-label">Show details</label>
                        <div class="toggle-overlays-wrap"></div>
                    </div>
                    <div class="dropdown-section">
                        <label class="dropdown-label">Sort</label>
                        <select class="sort-field">
                            <option value="name" ${this.sort === 'name' ? 'selected' : ''}>Name</option>
                            <option value="date" ${this.sort === 'date' ? 'selected' : ''}>File Modified</option>
                            <option value="taken" ${this.sort === 'taken' ? 'selected' : ''}>Photo Taken</option>
                            <option value="size" ${this.sort === 'size' ? 'selected' : ''}>Size</option>
                        </select>
                        <button class="btn btn-sm sort-order" title="Toggle order">${this.order === 'asc' ? '↑' : '↓'}</button>
                    </div>
                </div>
            </div>
            <div class="dropdown-wrap">
                <button class="btn btn-sm dropdown-btn tools-menu-btn" title="Tools">
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"><path d="M8.5 1.5l4 4-7.5 7.5H1v-4z"/><path d="M7 3l4 4"/></svg>
                    Tools
                    <svg class="dropdown-chevron" width="8" height="8" viewBox="0 0 8 8" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3l2 2 2-2"/></svg>
                </button>
                <div class="dropdown-menu tools-menu" style="display:none">
                    <div class="tools-menu-loading" style="display:none">Checking...</div>
                    <div class="tools-menu-items">
                        <div class="dropdown-section tools-geo-section" style="display:none">
                            <label class="dropdown-label tools-geo-label">Geolocation</label>
                            <div class="dropdown-toggle">
                                <button class="btn btn-sm tool-item" data-tool="set-location">Set</button>
                                <button class="btn btn-sm tool-item" data-tool="remove-location">Remove</button>
                            </div>
                        </div>
                        <div class="dropdown-section">
                            <label class="dropdown-label">Rename</label>
                            <div class="dropdown-toggle">
                                <button class="btn btn-sm tool-item" data-tool="rename">Single</button>
                                <button class="btn btn-sm tool-item" data-tool="batch-rename">Batch (Metadata)</button>
                            </div>
                        </div>
                        <div class="dropdown-section">
                            <label class="dropdown-label">Export</label>
                            <div class="dropdown-toggle">
                                <button class="btn btn-sm tool-item" data-tool="export">Convert &amp; Export</button>
                            </div>
                        </div>
                        <div class="dropdown-section tools-library-section" style="display:none">
                            <label class="dropdown-label">Library</label>
                            <div class="dropdown-toggle">
                                <button class="btn btn-sm tool-item" data-tool="make-library">Make library</button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            <button class="btn btn-sm slideshow-btn" title="Slideshow">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                    <polygon points="3,2 12,7 3,12" fill="currentColor" stroke="none"/>
                    <line x1="1" y1="2" x2="1" y2="12"/>
                </svg>
                Slideshow
            </button>
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
        this._attachItemEvents(start, end);

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
        if (localStorage.getItem('thumbnail-quality') !== 'high') return null;
        const dpr = window.devicePixelRatio || 1;
        if (this.view === 'grid') {
            const grid = this.container.querySelector('.grid');
            if (grid) {
                const item = grid.querySelector('.grid-item');
                if (item) return Math.round(item.offsetWidth * dpr);
            }
            return Math.round(200 * dpr);
        }
        if (this.view === 'justified') return Math.round(this._justifiedTargetHeight * dpr);
        return Math.round(32 * dpr);
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

        this.container.querySelectorAll('[data-view]').forEach(el => {
            el.addEventListener('click', () => this.setView(el.dataset.view));
        });

        const viewMenuBtn = this.container.querySelector('.view-menu-btn');
        const viewMenu = this.container.querySelector('.view-menu');
        if (viewMenuBtn && viewMenu) Dropdown.init(viewMenuBtn, viewMenu);

        const toolsMenuBtn = this.container.querySelector('.tools-menu-btn');
        const toolsMenu = this.container.querySelector('.tools-menu');
        if (toolsMenuBtn && toolsMenu) {
            const { close: closeToolsMenu } = Dropdown.init(toolsMenuBtn, toolsMenu, {
                onOpen: () => {
                    this._updateToolsGeoLabel();
                    this._updateToolsRenameState();
                    this._updateToolsLibraryState();
                    this._checkToolsAvailability();
                },
            });
            toolsMenu.querySelectorAll('.tool-item').forEach(btn => {
                btn.addEventListener('click', () => {
                    const tool = btn.dataset.tool;
                    if (tool === 'make-library') {
                        const dir = this.getFocusedDir();
                        if (!dir) return;
                        if (this.onToolInvoke) this.onToolInvoke({ tool, path: dir });
                        closeToolsMenu();
                        return;
                    }
                    const files = this.getActionableFiles();
                    if (files.length === 0) { alert('No images selected.'); return; }
                    const params = {};
                    if (tool === 'rotate') params.angle = parseInt(btn.dataset.angle);
                    if (this.onToolInvoke) this.onToolInvoke({ tool, files, ...params });
                    closeToolsMenu();
                });
            });
        }

        const slideshowBtn = this.container.querySelector('.slideshow-btn');
        if (slideshowBtn) {
            slideshowBtn.addEventListener('click', () => {
                if (this.onSlideshowInvoke) this.onSlideshowInvoke();
            });
        }

        const namesWrap = this.container.querySelector('.toggle-names-wrap');
        if (namesWrap) Toggle.create(namesWrap, {
            initial: this.showNames,
            onChange: (on) => { this.showNames = on; this.render(); }
        });

        const overlaysWrap = this.container.querySelector('.toggle-overlays-wrap');
        if (overlaysWrap) Toggle.create(overlaysWrap, {
            initial: this.showOverlays,
            onChange: (on) => { this.showOverlays = on; this.render(); }
        });

        const sortField = this.container.querySelector('.sort-field');
        if (sortField) sortField.addEventListener('change', () => this.setSort(sortField.value, this.order));

        const sortOrder = this.container.querySelector('.sort-order');
        if (sortOrder) sortOrder.addEventListener('click', () => this.setSort(this.sort, this.order === 'asc' ? 'desc' : 'asc'));

        const gridEl = this.container.querySelector('.grid, .justified, .list-view');
        if (gridEl) {
            gridEl.addEventListener('click', (e) => {
                if (e.target !== gridEl) return;
                if (this.selection.selected.size === 0) return;
                this.selection.clear();
                this.selection.updateClasses(this.container);
                if (this.onSelectionChange) this.onSelectionChange([]);
            });
        }

        this._attachItemEvents(0, this._renderedCount);
    }

    _attachItemEvents(start, end) {
        this.container.querySelectorAll('[data-index]').forEach(el => {
            const idx = parseInt(el.dataset.index);
            if (idx < start || idx >= end) return;

            if (el.dataset.type === 'dir') {
                el.addEventListener('click', () => {
                    this.keyboard.focusedIndex = idx;
                    this.keyboard.updateFocusClass();
                    this._notifyFocusChange();
                });
                el.addEventListener('dblclick', () => {
                    const path = this.fullPath(el.dataset.name);
                    this.load(path);
                    if (this.onNavigate) this.onNavigate(path);
                });
            } else if (el.dataset.type === 'image') {
                el.addEventListener('click', (e) => {
                    const fp = el.dataset.path;
                    this.keyboard.focusedIndex = idx;
                    this.keyboard.updateFocusClass();
                    this._notifyFocusChange();
                    this.selection.handleImageClick(e, idx, fp, this.entries, n => this.fullPath(n));
                    this.selection.updateClasses(this.container);
                });
                el.addEventListener('dblclick', () => {
                    if (this.onImageClick) this.onImageClick(el.dataset.path);
                });

                if (this.view === 'justified') {
                    const img = el.querySelector('img');
                    if (img) {
                        const recordAR = () => {
                            if (img.naturalWidth && img.naturalHeight) {
                                this._aspectRatios[idx] = img.naturalWidth / img.naturalHeight;
                                this._justifiedRenderer.scheduleRelayout();
                            }
                        };
                        if (img.naturalWidth) recordAR();
                        else img.addEventListener('load', recordAR);
                    }
                }
            }
        });
    }

    // --- Tools menu helpers ---

    async _checkToolsAvailability() {
        const loading = this.container.querySelector('.tools-menu-loading');
        const geoSection = this.container.querySelector('.tools-geo-section');
        if (!loading || !geoSection) return;
        if (this._toolsChecked !== null) {
            loading.style.display = 'none';
            geoSection.style.display = this._toolsChecked.exiftool ? '' : 'none';
            return;
        }
        loading.style.display = '';
        try { this._toolsChecked = await API.toolsCheck(); }
        catch { this._toolsChecked = { exiftool: false }; }
        loading.style.display = 'none';
        geoSection.style.display = this._toolsChecked.exiftool ? '' : 'none';
    }

    _updateToolsGeoLabel() {
        const label = this.container.querySelector('.tools-geo-label');
        if (!label) return;
        const count = this.getActionableFiles().length;
        label.textContent = count > 0
            ? `Geolocation (${count} image${count !== 1 ? 's' : ''})`
            : 'Geolocation';
    }

    _updateToolsRenameState() {
        const btn = this.container.querySelector('[data-tool="rename"]');
        if (btn) btn.disabled = this.getActionableFiles().length !== 1;
    }

    _updateToolsLibraryState() {
        const section = this.container.querySelector('.tools-library-section');
        if (!section) return;
        section.style.display = this.getFocusedDir() ? '' : 'none';
    }

    // --- Focus change notification ---

    _notifyFocusChange() {
        if (!this.onFocusChange) return;
        const idx = this.keyboard.focusedIndex;
        if (idx < 0 || idx >= this.entries.length) { this.onFocusChange(null); return; }
        const entry = this.entries[idx];
        if (entry.type !== 'image') { this.onFocusChange(null); return; }
        this.onFocusChange(this.fullPath(entry.name));
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

    _buildOverlayBadges(name, meta) {
        if (!this.showOverlays) return '';
        const badges = [];
        if (meta && meta.hasGPS) {
            badges.push(`<span class="overlay-badge overlay-badge-gps"><svg width="10" height="10" viewBox="0 0 24 24" fill="rgba(255,255,255,0.85)" stroke="none"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5a2.5 2.5 0 1 1 0-5 2.5 2.5 0 0 1 0 5z"/></svg></span>`);
        }
        const ft = this._getFileTypeBadge(name);
        if (ft) badges.push(`<span class="overlay-badge" style="background:${ft.color}">${ft.label}</span>`);
        if (meta && meta.filmSimulation) {
            const fs = this._getFilmSimBadge(meta.filmSimulation);
            if (fs) badges.push(`<span class="overlay-badge" style="background:${fs.color}">${fs.label}</span>`);
        }
        if (meta && meta.aspectRatio) {
            const icon = this._aspectRatioIcon(meta.aspectRatio);
            badges.push(`<span class="overlay-badge" style="background:#5a6872;display:inline-flex;align-items:center;gap:3px">${icon}${meta.aspectRatio}</span>`);
        }
        if (badges.length === 0) return '';
        return `<div class="overlay-badges">${badges.join('')}</div>`;
    }

    _getFileTypeBadge(name) {
        const ext = name.split('.').pop().toLowerCase();
        const types = {
            jpg: { label: 'JPEG', color: '#c27833' }, jpeg: { label: 'JPEG', color: '#c27833' },
            heif: { label: 'HEIF', color: '#4a8c5c' }, heic: { label: 'HEIF', color: '#4a8c5c' }, hif: { label: 'HEIF', color: '#4a8c5c' },
            png: { label: 'PNG', color: '#4a6fa5' }, gif: { label: 'GIF', color: '#8c6b4a' }, webp: { label: 'WebP', color: '#7b5299' },
        };
        return types[ext] || null;
    }

    _getFilmSimBadge(sim) {
        if (!sim) return null;
        const colors = {
            'Provia': '#3a7ca5', 'Astia': '#5a9ab5', 'Velvia': '#b5443a', 'Classic Chrome': '#8a7d3a',
            'Classic Neg.': '#b07040', 'Eterna': '#3a8a8a', 'Nostalgic Neg.': '#a05050', 'Reala Ace': '#3a8a5a',
            'Pro Neg. Std': '#6a6a7a', 'Pro Neg. Hi': '#7a6a8a', 'Bleach Bypass': '#8a8a8a',
            'Monochrome': '#404040', 'Monochrome + R': '#5a3030', 'Monochrome + Ye': '#5a5a30', 'Monochrome + G': '#305a30',
            'Acros': '#333333', 'Acros + R': '#4a2828', 'Acros + Ye': '#4a4a28', 'Acros + G': '#284a28', 'Sepia': '#6a5038',
        };
        return { label: sim, color: colors[sim] || '#6a6a7a' };
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
