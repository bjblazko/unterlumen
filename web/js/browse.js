// Browse mode — single pane directory browser

const CHUNK_SIZE = 50;

class BrowsePane {
    constructor(container, options = {}) {
        this.container = container;
        this.path = '';
        this.entries = [];
        this.selected = new Set();
        this.view = 'grid'; // 'grid' or 'list'
        this.sort = 'name';
        this.order = 'asc';
        this.onNavigate = options.onNavigate || null;
        this.onImageClick = options.onImageClick || null;
        this.onSelectionChange = options.onSelectionChange || null;
        this.onFocusChange = options.onFocusChange || null;
        this.showNames = false;
        this.lastClickedIndex = -1;
        this.focusedIndex = -1;
        this._loading = false;
        this._renderedCount = 0;
        this._observer = null;
        this._exifPollPath = null;
    }

    async load(path) {
        if (this._loading) return;
        this._loading = true;
        this.path = path || '';
        this.selected.clear();
        this.lastClickedIndex = -1;
        this.focusedIndex = 0;
        this.warnings = [];
        this.entries = [];
        this._exifPollPath = null;
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
        this._notifyFocusChange();

        // Start EXIF date polling if there are images
        if (this.entries.some(e => e.type === 'image')) {
            this._pollExifDates();
        }
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

    getFocusedDir() {
        if (this.focusedIndex < 0 || this.focusedIndex >= this.entries.length) return null;
        const entry = this.entries[this.focusedIndex];
        if (entry.type !== 'dir') return null;
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

        const html = [];

        // Warnings
        if (this.warnings && this.warnings.length > 0) {
            html.push(this.renderWarnings());
        }

        // Breadcrumb
        html.push(this.renderBreadcrumb());

        // Controls
        html.push(this.renderControls());

        if (this._loading) {
            html.push('<div class="browse-loading"><div class="browse-spinner"></div></div>');
        } else if (this.entries.length === 0) {
            html.push('<div class="empty">No images or folders found</div>');
        } else if (this.view === 'grid') {
            const end = Math.min(CHUNK_SIZE, this.entries.length);
            html.push(this._renderGridChunk(0, end));
            this._renderedCount = end;
            if (end < this.entries.length) {
                html.push('<div class="scroll-sentinel"></div>');
            }
        } else {
            const end = Math.min(CHUNK_SIZE, this.entries.length);
            html.push(this._renderListChunk(0, end));
            this._renderedCount = end;
            if (end < this.entries.length) {
                html.push('<div class="scroll-sentinel"></div>');
            }
        }

        this.container.innerHTML = html.join('');
        this.attachEvents();
        this._setupObserver();
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
        let crumbs = `<a href="#" class="crumb" data-path="">Root</a>`;
        let accumulated = '';
        for (const part of parts) {
            accumulated = accumulated ? `${accumulated}/${part}` : part;
            crumbs += ` / <a href="#" class="crumb" data-path="${accumulated}">${part}</a>`;
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
                        <label class="view-menu-label">Sort</label>
                        <select class="sort-field">
                            <option value="name" ${this.sort === 'name' ? 'selected' : ''}>Name</option>
                            <option value="date" ${this.sort === 'date' ? 'selected' : ''}>Date</option>
                            <option value="size" ${this.sort === 'size' ? 'selected' : ''}>Size</option>
                        </select>
                        <button class="btn btn-sm sort-order" title="Toggle order">${this.order === 'asc' ? '↑' : '↓'}</button>
                    </div>
                </div>
            </div>
            <span class="status-bar">${statusText}</span>
        </div>`;
    }

    _renderGridChunk(start, end) {
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
                items.push(`<div class="grid-item image-item${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="image" data-path="${fp}">
                    <img src="${API.thumbnailURL(fp)}" alt="${entry.name}" loading="lazy" onload="this.classList.add('img-loaded')">${nameHtml}
                </div>`);
            }
        }
        if (start === 0) {
            return `<div class="grid">${items.join('')}</div>`;
        }
        return items.join('');
    }

    _renderListChunk(start, end) {
        const rows = [];
        for (let idx = start; idx < end; idx++) {
            const entry = this.entries[idx];
            const date = entry.date ? new Date(entry.date).toLocaleString() : '';
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
                rows.push(`<tr class="image-row${selectedClass}${markedClass}${focusedClass}" data-index="${idx}" data-name="${entry.name}" data-type="image" data-path="${fp}">
                    <td class="list-icon"><img src="${API.thumbnailURL(fp)}" alt="" loading="lazy"></td>
                    <td class="list-name">${entry.name}</td>
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

    // Keep old renderGrid/renderList as aliases for compatibility (used nowhere now)
    renderGrid() { return this._renderGridChunk(0, this.entries.length); }
    renderList() { return this._renderListChunk(0, this.entries.length); }

    _setupObserver() {
        const sentinel = this.container.querySelector('.scroll-sentinel');
        if (!sentinel) return;
        this._observer = new IntersectionObserver((entries) => {
            if (entries[0].isIntersecting) {
                this._renderNextChunk();
            }
        }, { rootMargin: '200px' });
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

        if (this.view === 'grid') {
            const grid = this.container.querySelector('.grid');
            if (grid) {
                grid.insertAdjacentHTML('beforeend', this._renderGridChunk(start, end));
            }
        } else {
            const tbody = this.container.querySelector('tbody');
            if (tbody) {
                tbody.insertAdjacentHTML('beforeend', this._renderListChunk(start, end));
            }
        }

        this._renderedCount = end;
        this._attachItemEvents(start, end);

        // Remove sentinel if we've rendered everything
        if (end >= this.entries.length) {
            const sentinel = this.container.querySelector('.scroll-sentinel');
            if (sentinel) sentinel.remove();
            this._destroyObserver();
        }
    }

    _ensureRenderedUpTo(index) {
        while (this._renderedCount <= index && this._renderedCount < this.entries.length) {
            this._renderNextChunk();
        }
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

        // Names toggle
        const namesToggle = this.container.querySelector('.toggle-names');
        if (namesToggle) {
            namesToggle.addEventListener('click', () => {
                this.showNames = !this.showNames;
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
            }
        });
    }

    getColumnCount() {
        if (this.view !== 'grid') return 1;
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
                    entry.date = data.dates[entry.name];
                }
            }
            if (this.sort === 'date') {
                this._resortAndRender();
            }
        }
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

function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}
