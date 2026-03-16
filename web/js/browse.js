// Browse mode — single pane directory browser

const CHUNK_SIZE = 50;

class BrowsePane {
    constructor(container, options = {}) {
        this.container = container;
        this.path = '';
        this.entries = [];
        this.selected = new Set();
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
        this._toolsChecked = null; // null = not checked, {exiftool: bool}
        this.lastClickedIndex = -1;
        this.focusedIndex = -1;
        this._loading = false;
        this._renderedCount = 0;
        this._observer = null;
        this._exifPollPath = null;
        this._metaPollPath = null;
        this._entryMeta = {};
        this._aspectRatios = {};
        this._justifiedTargetHeight = 200;
        this._justifiedRelayoutTimer = null;
        this._resizeHandler = null;
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
        if (this.view === 'justified') {
            return Math.round(this._justifiedTargetHeight * dpr);
        }
        // list view: small thumbnails
        return Math.round(32 * dpr);
    }

    reloadThumbnails() {
        this.render();
    }

    async load(path) {
        if (this._loading) return;
        this._loading = true;
        const isReload = (path || '') === this.path;
        const scrollEl = this._contentEl || this.container;
        const savedScroll = isReload ? scrollEl.scrollTop : 0;
        this.path = path || '';
        this.selected.clear();
        this.lastClickedIndex = -1;
        this.focusedIndex = 0;
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
        if (isReload && savedScroll > 0) {
            const restoreEl = this._contentEl || this.container;
            // Render enough chunks so container has sufficient height
            while (this._renderedCount < this.entries.length &&
                   restoreEl.scrollHeight <= savedScroll + restoreEl.clientHeight) {
                this._renderNextChunk();
            }
            restoreEl.scrollTop = savedScroll;
        }
        this._notifyFocusChange();

        // Start EXIF date polling if there are images
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

    getImageEntries() {
        return this.entries.filter(e => e.type === 'image');
    }

    getSelectedFiles() {
        return Array.from(this.selected);
    }

    getActionableFiles() {
        const selected = this.getSelectedFiles();
        if (selected.length > 0) return selected;
        const focused = this.getFocusedFile();
        return focused ? [focused] : [];
    }

    getFocusedDir() {
        if (this.focusedIndex < 0 || this.focusedIndex >= this.entries.length) return null;
        const entry = this.entries[this.focusedIndex];
        if (entry.type !== 'dir') return null;
        return this.fullPath(entry.name);
    }

    getFocusedFile() {
        if (this.focusedIndex < 0 || this.focusedIndex >= this.entries.length) return null;
        const entry = this.entries[this.focusedIndex];
        if (entry.type !== 'image') return null;
        return this.fullPath(entry.name);
    }

    selectAll() {
        this.entries.filter(e => e.type !== 'dir').forEach(e => {
            this.selected.add(this.fullPath(e.name));
        });
        this.render();
    }

    fullPath(name) {
        return this.path ? `${this.path}/${name}` : name;
    }

    render() {
        this._destroyObserver();
        this._renderedCount = 0;

        // Build header (sticky)
        const header = [];
        if (this.warnings && this.warnings.length > 0) {
            header.push(this.renderWarnings());
        }
        header.push(this.renderBreadcrumb());
        header.push(this.renderControls());

        // Build content (scrollable)
        const content = [];
        if (this._loading) {
            content.push('<div class="browse-loading"><div class="browse-spinner"></div></div>');
        } else if (this.entries.length === 0) {
            content.push('<div class="empty">No images or folders found</div>');
        } else if (this.view === 'grid') {
            const end = Math.min(CHUNK_SIZE, this.entries.length);
            content.push(this._renderGridChunk(0, end));
            this._renderedCount = end;
            if (end < this.entries.length) {
                content.push('<div class="scroll-sentinel"></div>');
            }
        } else if (this.view === 'justified') {
            const end = Math.min(CHUNK_SIZE, this.entries.length);
            content.push(this._renderJustifiedChunk(0, end));
            this._renderedCount = end;
            if (end < this.entries.length) {
                content.push('<div class="scroll-sentinel"></div>');
            }
        } else {
            const end = Math.min(CHUNK_SIZE, this.entries.length);
            content.push(this._renderListChunk(0, end));
            this._renderedCount = end;
            if (end < this.entries.length) {
                content.push('<div class="scroll-sentinel"></div>');
            }
        }

        // Clean up previous resize handler
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
            this._layoutJustified();
            this._resizeHandler = () => this._scheduleJustifiedRelayout();
            window.addEventListener('resize', this._resizeHandler);
        }
    }

    renderWarnings() {
        const items = this.warnings.map((w, i) =>
            `<div class="warning-banner" data-warning-index="${i}">
                <span class="warning-message">${w.message}</span>
                <button class="btn btn-sm warning-dismiss" data-warning-index="${i}" title="Dismiss">&times;</button>
            </div>`
        );
        return items.join('');
    }

    renderBreadcrumb() {
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

    renderControls() {
        const imageCount = this.getImageEntries().length;
        const selectedCount = this.selected.size;
        const statusText = selectedCount > 0
            ? `${imageCount} images · ${selectedCount} selected`
            : `${imageCount} images`;

        return `<div class="controls">
            <div class="controls-left">
            <div class="view-menu-wrap">
                <button class="btn btn-sm view-menu-btn" title="View options">
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><line x1="2" y1="3.5" x2="12" y2="3.5"/><line x1="2" y1="7" x2="12" y2="7"/><line x1="2" y1="10.5" x2="12" y2="10.5"/><circle cx="5" cy="3.5" r="1.5" fill="currentColor" stroke="none"/><circle cx="9" cy="7" r="1.5" fill="currentColor" stroke="none"/><circle cx="6" cy="10.5" r="1.5" fill="currentColor" stroke="none"/></svg>
                    View
                </button>
                <div class="view-menu" style="display:none">
                    <div class="view-menu-section">
                        <label class="view-menu-label">Layout</label>
                        <div class="view-menu-toggle">
                            <button class="btn btn-sm ${this.view === 'grid' ? 'active' : ''}" data-view="grid">Grid</button>
                            <button class="btn btn-sm ${this.view === 'justified' ? 'active' : ''}" data-view="justified">Justified</button>
                            <button class="btn btn-sm ${this.view === 'list' ? 'active' : ''}" data-view="list">List</button>
                        </div>
                    </div>
                    <div class="view-menu-section">
                        <label class="view-menu-label">Show names</label>
                        <button class="btn btn-sm toggle-names ${this.showNames ? 'active' : ''}">
                            ${this.showNames ? 'On' : 'Off'}
                        </button>
                    </div>
                    <div class="view-menu-section">
                        <label class="view-menu-label">Show details</label>
                        <button class="btn btn-sm toggle-overlays ${this.showOverlays ? 'active' : ''}">
                            ${this.showOverlays ? 'On' : 'Off'}
                        </button>
                    </div>
                    <div class="view-menu-section">
                        <label class="view-menu-label">Sort</label>
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
            <div class="tools-menu-wrap">
                <button class="btn btn-sm tools-menu-btn" title="Tools">
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"><path d="M8.5 1.5l4 4-7.5 7.5H1v-4z"/><path d="M7 3l4 4"/></svg>
                    Tools
                </button>
                <div class="tools-menu" style="display:none">
                    <div class="tools-menu-loading" style="display:none">Checking...</div>
                    <div class="tools-menu-message" style="display:none">Requires exiftool. Install it to use tools.</div>
                    <div class="tools-menu-items" style="display:none">
                        <div class="view-menu-section">
                            <label class="view-menu-label tools-geo-label">Geolocation</label>
                            <div class="view-menu-toggle">
                                <button class="btn btn-sm tool-item" data-tool="set-location">Set</button>
                                <button class="btn btn-sm tool-item" data-tool="remove-location">Remove</button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            </div>
            <span class="status-bar">${statusText}</span>
        </div>`;
    }

    _renderGridChunk(start, end) {
        const thumbSize = this._getThumbnailSize();
        const items = [];
        for (let idx = start; idx < end; idx++) {
            const entry = this.entries[idx];
            const focusedClass = idx === this.focusedIndex ? ' focused' : '';
            if (entry.type === 'dir') {
                items.push(`<div class="grid-item dir-item${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="dir">
                    <div class="dir-icon"><svg width="32" height="26" viewBox="0 0 32 26" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10l2-3h16v22H2z"/></svg></div>
                    <div class="item-name">${entry.name}</div>
                </div>`);
            } else {
                const fp = this.fullPath(entry.name);
                const selectedClass = this.selected.has(fp) ? ' selected' : '';
                const markedClass = App.isMarkedForDeletion(fp) ? ' marked-for-deletion' : '';
                const nameHtml = this.showNames ? `<div class="item-name">${entry.name}</div>` : '';
                const badgesHtml = this._buildOverlayBadges(entry.name, this._entryMeta[entry.name]);
                items.push(`<div class="grid-item image-item${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="image" data-path="${fp}">
                    <img src="${API.thumbnailURL(fp, thumbSize)}" alt="${entry.name}" loading="lazy" onload="this.classList.add('img-loaded')">${badgesHtml}${nameHtml}
                </div>`);
            }
        }
        if (start === 0) {
            return `<div class="grid">${items.join('')}</div>`;
        }
        return items.join('');
    }

    _renderListChunk(start, end) {
        const thumbSize = this._getThumbnailSize();
        const rows = [];
        for (let idx = start; idx < end; idx++) {
            const entry = this.entries[idx];
            const date = formatDate(entry.date);
            const focusedClass = idx === this.focusedIndex ? ' focused' : '';
            if (entry.type === 'dir') {
                rows.push(`<tr class="dir-row${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="dir">
                    <td class="list-icon"><svg width="32" height="26" viewBox="0 0 32 26" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10l2-3h16v22H2z"/></svg></td>
                    <td class="list-name">${entry.name}</td>
                    <td class="list-date">${date}</td>
                    <td class="list-size"></td>
                </tr>`);
            } else {
                const fp = this.fullPath(entry.name);
                const selectedClass = this.selected.has(fp) ? ' selected' : '';
                const markedClass = App.isMarkedForDeletion(fp) ? ' marked-for-deletion' : '';
                const size = entry.size ? formatSize(entry.size) : '';
                const badgesHtml = this._buildOverlayBadges(entry.name, this._entryMeta[entry.name]);
                rows.push(`<tr class="image-row${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="image" data-path="${fp}">
                    <td class="list-icon"><img src="${API.thumbnailURL(fp, thumbSize)}" alt="" loading="lazy"></td>
                    <td class="list-name">${entry.name}${badgesHtml ? `<span class="list-badges">${badgesHtml}</span>` : ''}</td>
                    <td class="list-date">${date}</td>
                    <td class="list-size">${size}</td>
                </tr>`);
            }
        }
        if (start === 0) {
            return `<table class="list-view">
                <thead><tr><th></th><th>Name</th><th>Date</th><th>Size</th></tr></thead>
                <tbody>${rows.join('')}</tbody>
            </table>`;
        }
        return rows.join('');
    }

    _renderJustifiedChunk(start, end) {
        const thumbSize = this._getThumbnailSize();
        const dirItems = [];
        const imageItems = [];
        for (let idx = start; idx < end; idx++) {
            const entry = this.entries[idx];
            const focusedClass = idx === this.focusedIndex ? ' focused' : '';
            if (entry.type === 'dir') {
                dirItems.push(`<div class="grid-item dir-item${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="dir">
                    <div class="dir-icon"><svg width="32" height="26" viewBox="0 0 32 26" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h10l2-3h16v22H2z"/></svg></div>
                    <div class="item-name">${entry.name}</div>
                </div>`);
            } else {
                const fp = this.fullPath(entry.name);
                const selectedClass = this.selected.has(fp) ? ' selected' : '';
                const markedClass = App.isMarkedForDeletion(fp) ? ' marked-for-deletion' : '';
                const nameHtml = this.showNames ? `<div class="item-name">${entry.name}</div>` : '';
                const badgesHtml = this._buildOverlayBadges(entry.name, this._entryMeta[entry.name]);
                const ar = this._aspectRatios[idx] || 1.5;
                imageItems.push(`<div class="justified-item image-item${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="image" data-path="${fp}" style="width:${Math.round(this._justifiedTargetHeight * ar)}px;height:${this._justifiedTargetHeight}px">
                    <img src="${API.thumbnailURL(fp, thumbSize)}" alt="${entry.name}" loading="lazy" data-jidx="${idx}">${badgesHtml}${nameHtml}
                </div>`);
            }
        }
        if (start === 0) {
            const parts = [];
            if (dirItems.length > 0) {
                parts.push(`<div class="grid justified-dirs">${dirItems.join('')}</div>`);
            }
            parts.push(`<div class="justified">${imageItems.join('')}</div>`);
            return parts.join('');
        }
        return imageItems.join('');
    }

    _layoutJustified() {
        const container = this.container.querySelector('.justified');
        if (!container) return;
        const containerWidth = container.clientWidth;
        if (containerWidth <= 0) return;

        const items = container.querySelectorAll('.justified-item');
        if (items.length === 0) return;

        const gap = 1;
        let rowStart = 0;
        let rowAspectSum = 0;

        for (let i = 0; i <= items.length; i++) {
            if (i < items.length) {
                const idx = parseInt(items[i].dataset.index);
                const ar = this._aspectRatios[idx] || 1.5;
                const itemWidth = ar * this._justifiedTargetHeight;
                const rowGaps = (i - rowStart) * gap;

                if (rowAspectSum > 0 && rowAspectSum * this._justifiedTargetHeight + itemWidth + rowGaps + gap > containerWidth) {
                    // Close the current row
                    this._setJustifiedRow(items, rowStart, i, containerWidth, rowAspectSum, gap);
                    rowStart = i;
                    rowAspectSum = ar;
                } else {
                    rowAspectSum += ar;
                }
            } else {
                // Last row — keep target height, don't stretch
                for (let j = rowStart; j < i; j++) {
                    const jIdx = parseInt(items[j].dataset.index);
                    const ar = this._aspectRatios[jIdx] || 1.5;
                    items[j].style.width = Math.round(this._justifiedTargetHeight * ar) + 'px';
                    items[j].style.height = this._justifiedTargetHeight + 'px';
                }
            }
        }
    }

    _setJustifiedRow(items, start, end, containerWidth, aspectSum, gap) {
        const gaps = (end - start - 1) * gap;
        const rowHeight = (containerWidth - gaps) / aspectSum;
        let usedWidth = 0;

        for (let i = start; i < end; i++) {
            const idx = parseInt(items[i].dataset.index);
            const ar = this._aspectRatios[idx] || 1.5;
            if (i === end - 1) {
                // Last item gets remaining pixels to avoid rounding gaps
                const w = containerWidth - usedWidth - (end - start - 1) * gap;
                items[i].style.width = Math.round(w) + 'px';
            } else {
                const w = Math.round(ar * rowHeight);
                items[i].style.width = w + 'px';
                usedWidth += w;
            }
            items[i].style.height = Math.round(rowHeight) + 'px';
        }
    }

    _scheduleJustifiedRelayout() {
        if (this._justifiedRelayoutTimer) return;
        this._justifiedRelayoutTimer = requestAnimationFrame(() => {
            this._justifiedRelayoutTimer = null;
            this._layoutJustified();
        });
    }

    // Keep old renderGrid/renderList as aliases for compatibility (used nowhere now)
    renderGrid() { return this._renderGridChunk(0, this.entries.length); }
    renderList() { return this._renderListChunk(0, this.entries.length); }

    _setupObserver() {
        if (!this._contentEl) return;
        const sentinel = this._contentEl.querySelector('.scroll-sentinel');
        if (!sentinel) return;
        this._observer = new IntersectionObserver((entries) => {
            if (entries[0].isIntersecting) {
                this._renderNextChunk();
            }
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
            if (grid) {
                grid.insertAdjacentHTML('beforeend', this._renderGridChunk(start, end));
            }
        } else if (this.view === 'justified') {
            const justified = ct.querySelector('.justified');
            if (justified) {
                justified.insertAdjacentHTML('beforeend', this._renderJustifiedChunk(start, end));
                this._scheduleJustifiedRelayout();
            }
        } else {
            const tbody = ct.querySelector('tbody');
            if (tbody) {
                tbody.insertAdjacentHTML('beforeend', this._renderListChunk(start, end));
            }
        }

        this._renderedCount = end;
        this._attachItemEvents(start, end);

        // Remove sentinel if we've rendered everything
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

    updateMarkedForDeletion() {
        this.container.querySelectorAll('[data-path]').forEach(el => {
            const path = el.getAttribute('data-path');
            if (App.isMarkedForDeletion(path)) {
                el.classList.add('marked-for-deletion');
            } else {
                el.classList.remove('marked-for-deletion');
            }
        });
    }

    updateSelectionClasses() {
        this.container.querySelectorAll('[data-type="image"]').forEach(el => {
            el.classList.toggle('selected', this.selected.has(el.dataset.path));
        });
        const statusEl = this.container.querySelector('.status-bar');
        if (statusEl) {
            const imageCount = this.getImageEntries().length;
            const selectedCount = this.selected.size;
            statusEl.textContent = selectedCount > 0
                ? `${imageCount} images · ${selectedCount} selected`
                : `${imageCount} images`;
        }
    }

    attachEvents() {
        // Warning dismiss
        this.container.querySelectorAll('.warning-dismiss').forEach(el => {
            el.addEventListener('click', () => {
                const idx = parseInt(el.dataset.warningIndex);
                this.warnings.splice(idx, 1);
                this.render();
            });
        });

        // Breadcrumb navigation
        this.container.querySelectorAll('.crumb').forEach(el => {
            el.addEventListener('click', (e) => {
                e.preventDefault();
                const path = el.dataset.path;
                this.load(path);
                if (this.onNavigate) this.onNavigate(path);
            });
        });

        // View toggle
        this.container.querySelectorAll('[data-view]').forEach(el => {
            el.addEventListener('click', () => this.setView(el.dataset.view));
        });

        // View menu toggle
        const viewMenuBtn = this.container.querySelector('.view-menu-btn');
        const viewMenu = this.container.querySelector('.view-menu');
        if (viewMenuBtn && viewMenu) {
            const closeMenu = () => {
                viewMenu.style.display = 'none';
                document.removeEventListener('click', closeMenu);
            };
            viewMenuBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                const open = viewMenu.style.display !== 'none';
                if (open) {
                    closeMenu();
                } else {
                    viewMenu.style.display = '';
                    document.addEventListener('click', closeMenu);
                }
            });
            viewMenu.addEventListener('click', (e) => {
                e.stopPropagation();
            });
        }

        // Tools menu toggle
        const toolsMenuBtn = this.container.querySelector('.tools-menu-btn');
        const toolsMenu = this.container.querySelector('.tools-menu');
        if (toolsMenuBtn && toolsMenu) {
            const closeToolsMenu = () => {
                toolsMenu.style.display = 'none';
                document.removeEventListener('click', closeToolsMenu);
            };
            toolsMenuBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                const open = toolsMenu.style.display !== 'none';
                if (open) {
                    closeToolsMenu();
                } else {
                    toolsMenu.style.display = '';
                    document.addEventListener('click', closeToolsMenu);
                    this._updateToolsGeoLabel();
                    this._checkToolsAvailability();
                }
            });
            toolsMenu.addEventListener('click', (e) => {
                e.stopPropagation();
            });

            // Tool item clicks
            toolsMenu.querySelectorAll('.tool-item').forEach(btn => {
                btn.addEventListener('click', () => {
                    const files = this.getActionableFiles();
                    if (files.length === 0) {
                        alert('No images selected.');
                        return;
                    }
                    const tool = btn.dataset.tool;
                    const params = {};
                    if (tool === 'rotate') params.angle = parseInt(btn.dataset.angle);
                    if (this.onToolInvoke) this.onToolInvoke({ tool, files, ...params });
                    closeToolsMenu();
                });
            });
        }

        // Names toggle
        const namesToggle = this.container.querySelector('.toggle-names');
        if (namesToggle) {
            namesToggle.addEventListener('click', () => {
                this.showNames = !this.showNames;
                this.render();
            });
        }

        // Overlays toggle
        const overlaysToggle = this.container.querySelector('.toggle-overlays');
        if (overlaysToggle) {
            overlaysToggle.addEventListener('click', () => {
                this.showOverlays = !this.showOverlays;
                this.render();
            });
        }

        // Sort field
        const sortField = this.container.querySelector('.sort-field');
        if (sortField) {
            sortField.addEventListener('change', () => {
                this.setSort(sortField.value, this.order);
            });
        }

        // Sort order toggle
        const sortOrder = this.container.querySelector('.sort-order');
        if (sortOrder) {
            sortOrder.addEventListener('click', () => {
                this.setSort(this.sort, this.order === 'asc' ? 'desc' : 'asc');
            });
        }

        // Click on void area clears selection
        const gridEl = this.container.querySelector('.grid, .justified, .list-view');
        if (gridEl) {
            gridEl.addEventListener('click', (e) => {
                if (e.target !== gridEl) return;
                if (this.selected.size === 0) return;
                this.selected.clear();
                this.updateSelectionClasses();
                if (this.onSelectionChange) this.onSelectionChange([]);
            });
        }

        // Item events for the initial chunk
        this._attachItemEvents(0, this._renderedCount);
    }

    _attachItemEvents(start, end) {
        const els = this.container.querySelectorAll(`[data-index]`);
        els.forEach(el => {
            const idx = parseInt(el.dataset.index);
            if (idx < start || idx >= end) return;
            const type = el.dataset.type;

            if (type === 'dir') {
                el.addEventListener('click', () => {
                    this.focusedIndex = idx;
                    this.updateFocusClass();
                    this._notifyFocusChange();
                });
                el.addEventListener('dblclick', () => {
                    const name = el.dataset.name;
                    const path = this.fullPath(name);
                    this.load(path);
                    if (this.onNavigate) this.onNavigate(path);
                });
            } else if (type === 'image') {
                el.addEventListener('click', (e) => {
                    const fp = el.dataset.path;
                    this.focusedIndex = idx;
                    this.updateFocusClass();
                    this._notifyFocusChange();

                    if (e.ctrlKey || e.metaKey) {
                        if (this.selected.has(fp)) {
                            this.selected.delete(fp);
                        } else {
                            this.selected.add(fp);
                        }
                        this.lastClickedIndex = idx;
                        this.updateSelectionClasses();
                        if (this.onSelectionChange) this.onSelectionChange(this.getSelectedFiles());
                    } else if (e.shiftKey && this.lastClickedIndex >= 0) {
                        const rangeStart = Math.min(this.lastClickedIndex, idx);
                        const rangeEnd = Math.max(this.lastClickedIndex, idx);
                        for (let i = rangeStart; i <= rangeEnd; i++) {
                            const entry = this.entries[i];
                            if (entry.type === 'image') {
                                this.selected.add(this.fullPath(entry.name));
                            }
                        }
                        this.updateSelectionClasses();
                        if (this.onSelectionChange) this.onSelectionChange(this.getSelectedFiles());
                    } else {
                        this.selected.clear();
                        this.selected.add(fp);
                        this.lastClickedIndex = idx;
                        this.updateSelectionClasses();
                        if (this.onSelectionChange) this.onSelectionChange(this.getSelectedFiles());
                    }
                });

                el.addEventListener('dblclick', () => {
                    const fp = el.dataset.path;
                    if (this.onImageClick) this.onImageClick(fp);
                });

                // Record aspect ratio on thumbnail load for justified layout
                if (this.view === 'justified') {
                    const img = el.querySelector('img');
                    if (img) {
                        if (img.naturalWidth && img.naturalHeight) {
                            this._aspectRatios[idx] = img.naturalWidth / img.naturalHeight;
                            this._scheduleJustifiedRelayout();
                        } else {
                            img.addEventListener('load', () => {
                                this._aspectRatios[idx] = img.naturalWidth / img.naturalHeight;
                                this._scheduleJustifiedRelayout();
                            });
                        }
                    }
                }
            }
        });
    }

    getColumnCount() {
        if (this.view === 'list') return 1;
        if (this.view === 'justified') {
            const focused = this.container.querySelector(`[data-index="${this.focusedIndex}"]`);
            if (!focused) return 1;
            // If focused item is a dir, use the grid column count from the dirs grid
            if (focused.classList.contains('dir-item')) {
                const dirGrid = this.container.querySelector('.justified-dirs');
                if (!dirGrid) return 1;
                const dirItems = dirGrid.querySelectorAll('.grid-item');
                if (dirItems.length < 2) return 1;
                const firstTop = dirItems[0].getBoundingClientRect().top;
                let cols = 0;
                for (const item of dirItems) {
                    if (item.getBoundingClientRect().top !== firstTop) break;
                    cols++;
                }
                return cols > 0 ? cols : 1;
            }
            // Image in justified layout — count items sharing the same top
            const focusedTop = Math.round(focused.getBoundingClientRect().top);
            const items = this.container.querySelectorAll('.justified-item');
            let cols = 0;
            for (const item of items) {
                if (Math.round(item.getBoundingClientRect().top) === focusedTop) cols++;
            }
            return cols > 0 ? cols : 1;
        }
        const items = this.container.querySelectorAll('.grid-item');
        if (items.length < 2) return 1;
        const firstTop = items[0].getBoundingClientRect().top;
        let cols = 0;
        for (const item of items) {
            if (item.getBoundingClientRect().top !== firstTop) break;
            cols++;
        }
        return cols > 0 ? cols : 1;
    }

    updateFocusClass() {
        this.container.querySelectorAll('[data-index]').forEach(el => {
            el.classList.toggle('focused', parseInt(el.dataset.index) === this.focusedIndex);
        });
    }

    scrollFocusedIntoView() {
        const el = this.container.querySelector(`[data-index="${this.focusedIndex}"]`);
        if (el) el.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    }

    async _checkToolsAvailability() {
        const loading = this.container.querySelector('.tools-menu-loading');
        const message = this.container.querySelector('.tools-menu-message');
        const items = this.container.querySelector('.tools-menu-items');
        if (!loading || !message || !items) return;

        if (this._toolsChecked !== null) {
            // Already checked
            loading.style.display = 'none';
            message.style.display = this._toolsChecked.exiftool ? 'none' : '';
            items.style.display = this._toolsChecked.exiftool ? '' : 'none';
            return;
        }

        loading.style.display = '';
        message.style.display = 'none';
        items.style.display = 'none';

        try {
            this._toolsChecked = await API.toolsCheck();
        } catch {
            this._toolsChecked = { exiftool: false };
        }

        loading.style.display = 'none';
        message.style.display = this._toolsChecked.exiftool ? 'none' : '';
        items.style.display = this._toolsChecked.exiftool ? '' : 'none';
    }

    _updateToolsGeoLabel() {
        const label = this.container.querySelector('.tools-geo-label');
        if (!label) return;
        const count = this.getActionableFiles().length;
        label.textContent = count > 0
            ? `Geolocation (${count} image${count !== 1 ? 's' : ''})`
            : 'Geolocation';
    }

    _notifyFocusChange() {
        if (!this.onFocusChange) return;
        if (this.focusedIndex < 0 || this.focusedIndex >= this.entries.length) {
            this.onFocusChange(null);
            return;
        }
        const entry = this.entries[this.focusedIndex];
        if (entry.type !== 'image') {
            this.onFocusChange(null);
            return;
        }
        this.onFocusChange(this.fullPath(entry.name));
    }

    moveFocus(delta) {
        const count = this.entries.length;
        if (count === 0) return;
        let next = this.focusedIndex + delta;
        if (next < 0) next = 0;
        if (next >= count) next = count - 1;
        this.focusedIndex = next;
        this._ensureRenderedUpTo(next);
        this.updateFocusClass();
        this.scrollFocusedIntoView();
        this._notifyFocusChange();
    }

    activateFocused() {
        if (this.focusedIndex < 0 || this.focusedIndex >= this.entries.length) return;
        const entry = this.entries[this.focusedIndex];
        const path = this.fullPath(entry.name);
        if (entry.type === 'dir') {
            this.load(path);
            if (this.onNavigate) this.onNavigate(path);
        } else {
            if (this.onImageClick) this.onImageClick(path);
        }
    }

    toggleFocusedSelection() {
        if (this.focusedIndex < 0 || this.focusedIndex >= this.entries.length) return;
        const entry = this.entries[this.focusedIndex];
        if (entry.type !== 'image') return;
        const fp = this.fullPath(entry.name);
        if (this.selected.has(fp)) this.selected.delete(fp);
        else this.selected.add(fp);
        this.updateSelectionClasses();
        if (this.onSelectionChange) this.onSelectionChange(this.getSelectedFiles());
    }

    // EXIF date polling
    _pollExifDates() {
        const pollPath = this.path;
        this._exifPollPath = pollPath;

        setTimeout(() => this._doExifPoll(pollPath), 300);
    }

    async _doExifPoll(pollPath) {
        // Guard: user navigated away
        if (this._exifPollPath !== pollPath) return;

        let data;
        try {
            data = await API.browseDates(pollPath);
        } catch {
            return;
        }

        // Guard again after await
        if (this._exifPollPath !== pollPath) return;

        if (!data.ready) {
            setTimeout(() => this._doExifPoll(pollPath), 500);
            return;
        }

        // Apply dates if any differ
        if (data.dates && Object.keys(data.dates).length > 0) {
            for (const entry of this.entries) {
                if (entry.type === 'image' && data.dates[entry.name]) {
                    entry.exifDate = data.dates[entry.name];
                }
            }
            if (this.sort === 'taken') {
                this._resortAndRender();
            }
        }
    }

    // Overlay meta polling
    _pollOverlayMeta() {
        const pollPath = this.path;
        this._metaPollPath = pollPath;
        setTimeout(() => this._doMetaPoll(pollPath), 300);
    }

    async _doMetaPoll(pollPath) {
        if (this._metaPollPath !== pollPath) return;

        let data;
        try {
            data = await API.browseMeta(pollPath);
        } catch {
            return;
        }

        if (this._metaPollPath !== pollPath) return;

        if (!data.ready) {
            setTimeout(() => this._doMetaPoll(pollPath), 500);
            return;
        }

        if (data.meta) {
            this._entryMeta = data.meta;
            if (this.showOverlays) {
                this._updateOverlays();
            }
        }
    }

    async notifyFilesChanged(fullPaths) {
        try {
            await API.browse(this.path, this.sort, this.order);
        } catch { /* ignore */ }
        this._pollOverlayMeta();
    }

    _updateOverlays() {
        const ct = this._contentEl || this.container;
        ct.querySelectorAll('[data-type="image"]').forEach(el => {
            const name = el.dataset.name;
            const badges = this._buildOverlayBadges(name, this._entryMeta[name]);

            if (el.tagName === 'TR') {
                // List view: badges go inside .list-name td > .list-badges span
                const nameCell = el.querySelector('td.list-name');
                if (!nameCell) return;
                let listBadges = nameCell.querySelector('.list-badges');
                if (badges) {
                    if (!listBadges) {
                        listBadges = document.createElement('span');
                        listBadges.className = 'list-badges';
                        nameCell.appendChild(listBadges);
                    }
                    listBadges.innerHTML = badges;
                } else if (listBadges) {
                    listBadges.remove();
                }
            } else {
                // Grid/justified view: badges overlay the image item
                const existing = el.querySelector('.overlay-badges');
                if (existing) existing.remove();
                if (badges) el.insertAdjacentHTML('beforeend', badges);
            }
        });
    }

    _getFileTypeBadge(name) {
        const ext = name.split('.').pop().toLowerCase();
        const types = {
            jpg: { label: 'JPEG', color: '#c27833' },
            jpeg: { label: 'JPEG', color: '#c27833' },
            heif: { label: 'HEIF', color: '#4a8c5c' },
            heic: { label: 'HEIF', color: '#4a8c5c' },
            hif: { label: 'HEIF', color: '#4a8c5c' },
            png: { label: 'PNG', color: '#4a6fa5' },
            gif: { label: 'GIF', color: '#8c6b4a' },
            webp: { label: 'WebP', color: '#7b5299' },
        };
        return types[ext] || null;
    }

    _getFilmSimBadge(sim) {
        if (!sim) return null;
        const colors = {
            'Provia': '#3a7ca5',
            'Astia': '#5a9ab5',
            'Velvia': '#b5443a',
            'Classic Chrome': '#8a7d3a',
            'Classic Neg.': '#b07040',
            'Eterna': '#3a8a8a',
            'Nostalgic Neg.': '#a05050',
            'Reala Ace': '#3a8a5a',
            'Pro Neg. Std': '#6a6a7a',
            'Pro Neg. Hi': '#7a6a8a',
            'Bleach Bypass': '#8a8a8a',
            'Monochrome': '#404040',
            'Monochrome + R': '#5a3030',
            'Monochrome + Ye': '#5a5a30',
            'Monochrome + G': '#305a30',
            'Acros': '#333333',
            'Acros + R': '#4a2828',
            'Acros + Ye': '#4a4a28',
            'Acros + G': '#284a28',
            'Sepia': '#6a5038',
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

    _buildOverlayBadges(name, meta) {
        if (!this.showOverlays) return '';
        const badges = [];

        // GPS always first
        if (meta && meta.hasGPS) {
            badges.push(`<span class="overlay-badge overlay-badge-gps"><svg width="10" height="10" viewBox="0 0 24 24" fill="rgba(255,255,255,0.85)" stroke="none"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5a2.5 2.5 0 1 1 0-5 2.5 2.5 0 0 1 0 5z"/></svg></span>`);
        }

        const ft = this._getFileTypeBadge(name);
        if (ft) {
            badges.push(`<span class="overlay-badge" style="background:${ft.color}">${ft.label}</span>`);
        }

        if (meta && meta.filmSimulation) {
            const fs = this._getFilmSimBadge(meta.filmSimulation);
            if (fs) {
                badges.push(`<span class="overlay-badge" style="background:${fs.color}">${fs.label}</span>`);
            }
        }

        if (meta && meta.aspectRatio) {
            const icon = this._aspectRatioIcon(meta.aspectRatio);
            badges.push(`<span class="overlay-badge" style="background:#5a6872;display:inline-flex;align-items:center;gap:3px">${icon}${meta.aspectRatio}</span>`);
        }

        if (badges.length === 0) return '';
        return `<div class="overlay-badges">${badges.join('')}</div>`;
    }

    _resortAndRender() {
        // Client-side sort: dirs first, then by current sort field/order
        this.entries.sort((a, b) => {
            if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
            let less;
            switch (this.sort) {
                case 'date':
                    less = new Date(a.date) < new Date(b.date);
                    break;
                case 'taken': {
                    const aDate = a.exifDate ? new Date(a.exifDate) : null;
                    const bDate = b.exifDate ? new Date(b.exifDate) : null;
                    if (!aDate && !bDate) return 0;
                    if (!aDate) return 1;  // null always last
                    if (!bDate) return -1;
                    less = aDate < bDate;
                    break;
                }
                case 'size':
                    less = (a.size || 0) < (b.size || 0);
                    break;
                default:
                    less = a.name.toLowerCase() < b.name.toLowerCase();
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
