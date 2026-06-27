// SearchResultPane — BrowsePane subclass that renders library search results
// with all view layouts, selection, slideshow and overlay badges.

class SearchResultPane extends BrowsePane {
    constructor(container, options = {}) {
        super(container, options);
        this.sort = 'taken';
        this.order = 'desc';
        this._photoMap = new Map(); // pathHint → { libID, photoID }
        this._serverTotal = 0;
        this._serverOffset = 0;
        this._serverFetchPage = null;
        this._isFetching = false;
        this._fetchGeneration = 0;
        this._onClose = options.onClose || null;
    }

    // Load an array of LibraryPhoto objects (from cross- or single-library search).
    // options.total: server-reported total match count
    // options.fetchPage: async (offset, limit) => LibraryPhoto[] for subsequent pages
    loadResults(photos, multiLib = false, { total, fetchPage } = {}) {
        this.path = '';
        this.selection.clear();
        this.keyboard.focusedIndex = 0;
        this.warnings = [];
        this._entryMeta = {};
        this._aspectRatios = {};

        this._fetchGeneration++;
        this._isFetching = false;
        this._serverTotal = total ?? photos.length;
        this._serverOffset = photos.length;
        this._serverFetchPage = fetchPage ?? null;
        this._serverMultiLib = multiLib;

        this._photoMap.clear();
        this.entries = photos.map(p => this._mapPhoto(p, multiLib));

        this.render();
        this.keyboard.updateFocusClass();
        this._notifyFocusChange();
    }

    _mapPhoto(p, multiLib) {
        this._photoMap.set(p.pathHint, { libID: p.libraryID, photoID: p.id });
        const exif = p.exif || {};
        this._entryMeta[p.pathHint] = {
            hasGPS: !!(exif.GPSLatitude),
            filmSimulation: exif.FilmSimulation || null,
            aspectRatio: null,
        };
        return {
            name: p.pathHint,
            type: 'image',
            label: multiLib ? `${p.filename} (${p.libraryName || p.libraryID})` : p.filename,
            date: p.indexedAt,
            exifDate: p.dateTaken || null,
            size: p.fileSize,
        };
    }

    // Return { libID, photoID } for a given pathHint — used by info panel meta lookup.
    getPhotoInfo(pathHint) {
        return this._photoMap.get(pathHint);
    }

    getLibraryMeta(path) {
        return this._photoMap.get(path) || null;
    }

    // Returns { dir, names } if all selected photos share the same parent folder, else null.
    getOpenInCommanderTarget() {
        if (this.selection.selected.size === 0) return null;
        const paths = Array.from(this.selection.selected);
        const dirs = paths.map(p => p.substring(0, p.lastIndexOf('/')));
        const firstDir = dirs[0];
        if (!dirs.every(d => d === firstDir)) return null;
        return { dir: firstDir, names: paths.map(p => p.split('/').pop()) };
    }

    commanderBtnHint() {
        return this.selection.selected.size > 0
            ? 'Select photos from the same folder to open in Commander'
            : 'Select photos to open in Commander';
    }

    // pathHint is already an absolute path — no prefix needed.
    fullPath(name) {
        return name;
    }

    thumbURL(entry, size) {
        const info = this._photoMap.get(entry.name);
        if (info) return LibraryAPI.thumbURL(info.libID, info.photoID);
        return API.thumbnailURL(entry.name, size);
    }

    thumbFallbackURL(entry, size) {
        return API.thumbnailURL(entry.name, size);
    }

    viewerImageURL(path) {
        const info = this._photoMap.get(path);
        return info ? LibraryAPI.photoURL(info.libID, info.photoID) : API.imageURL(path);
    }

    viewerThumbURL(path) {
        const info = this._photoMap.get(path);
        return info ? LibraryAPI.thumbURL(info.libID, info.photoID) : API.thumbnailURL(path, 80);
    }

    viewerLoadInfo(path, infoPanel) {
        const info = this._photoMap.get(path);
        if (info) {
            infoPanel.loadFromURL(`/api/library/${info.libID}/photo/${info.photoID}/info`, `lib:${info.libID}:${info.photoID}`);
        } else {
            infoPanel.loadInfo(path);
        }
    }

    // EXIF and overlay data come from the search result payload — no polling needed.
    _pollExifDates() {}
    _pollOverlayMeta() {}

    _renderBreadcrumb() {
        const n = this._serverTotal || this.entries.length;
        const closeBtn = this._onClose
            ? `<button class="search-close-btn" title="Close filter results">×</button>`
            : '';
        return `<nav class="breadcrumb search-breadcrumb"><span class="crumb-current">Filter results · ${n} photo${n !== 1 ? 's' : ''}</span>${closeBtn}</nav>`;
    }

    attachEvents() {
        super.attachEvents();
        const btn = this.container.querySelector('.search-close-btn');
        if (btn && this._onClose) {
            btn.addEventListener('click', () => this._onClose());
        }
    }

    // --- Lazy loading from server ---

    _renderNextChunk() {
        const remaining = this.entries.length - this._renderedCount;
        if (remaining <= CHUNK_SIZE && this._serverOffset < this._serverTotal && !this._isFetching) {
            this._fetchNextPage();
        }
        super._renderNextChunk();
    }

    async _fetchNextPage() {
        if (this._isFetching || !this._serverFetchPage) return;
        if (this._serverOffset >= this._serverTotal) return;

        this._isFetching = true;
        const gen = this._fetchGeneration;
        const offset = this._serverOffset;

        try {
            const photos = await this._serverFetchPage(offset, 100);
            if (this._fetchGeneration !== gen) return; // filter changed, discard

            const newEntries = photos.map(p => this._mapPhoto(p, this._serverMultiLib));
            this.entries = [...this.entries, ...newEntries];
            this._serverOffset = offset + photos.length;

            // If all previously loaded entries were already rendered, re-add sentinel
            const ct = this._contentEl || this.container;
            if (!ct.querySelector('.scroll-sentinel')) {
                ct.insertAdjacentHTML('beforeend', '<div class="scroll-sentinel"></div>');
                this._setupObserver();
            }

            // Update status bar count without a full re-render
            const statusBar = this.container.querySelector('.status-bar');
            if (statusBar) {
                const imageCount = this.getImageEntries().length;
                const selectedCount = this.selection.selected.size;
                statusBar.textContent = selectedCount > 0
                    ? `${imageCount} images · ${selectedCount} selected`
                    : `${imageCount} images`;
            }
        } catch { /* ignore transient errors */ }
        finally {
            if (this._fetchGeneration === gen) this._isFetching = false;
        }
    }

    // ---

    setSort(sort, order) {
        this.sort = sort;
        this.order = order;
        this._aspectRatios = {};
        const asc = order === 'asc';
        this.entries = [...this.entries].sort((a, b) => {
            let av, bv;
            if (sort === 'size') {
                av = a.size || 0; bv = b.size || 0;
            } else if (sort === 'name') {
                av = (a.label || a.name).toLowerCase();
                bv = (b.label || b.name).toLowerCase();
            } else if (sort === 'taken') {
                const aDate = a.exifDate ? new Date(a.exifDate) : null;
                const bDate = b.exifDate ? new Date(b.exifDate) : null;
                if (!aDate && !bDate) return 0;
                if (!aDate) return 1;  // nulls always last
                if (!bDate) return -1;
                av = a.exifDate; bv = b.exifDate;
            } else {
                av = a.date || ''; bv = b.date || '';
            }
            if (av < bv) return asc ? -1 : 1;
            if (av > bv) return asc ? 1 : -1;
            return 0;
        });
        this.render();
    }
}
