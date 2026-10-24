// LibraryPane — BrowsePane subclass that reads folder contents from the library DB.
// No browse API calls are made, so no background EXIF extraction runs over NAS.

// "412 photos · 2021–2026", as HTML: how much a folder holds and the years
// it spans, the years never broken apart.
function folderTileMeta(preview) {
    const first = (preview.firstTaken || '').slice(0, 4);
    const last = (preview.lastTaken || '').slice(0, 4);
    const years = first && last ? `<span class="nowrap"> · ${first === last ? first : `${first}–${last}`}</span>` : '';
    return `<span class="nowrap">${countText(preview.photoCount, 'photo')}</span>${years}`;
}

class LibraryPane extends BrowsePane {
    constructor(container, libID, options = {}) {
        super(container, options);
        this._libID = libID;
        this._previewLibID = libID;
        this._sourcePath = options.sourcePath || '';
        this._libraryItems = options.libraryItems || null;
        this._onLeave = options.onLeave || null;
        this._photoMap = new Map(); // relPath → { photoID }
        this._folderPreviews = new Map(); // subfolder name → preview
        this.sort = 'taken';
        this.order = 'desc';
    }

    // What concerns the whole library joins the ⋯ after the view.
    _menuItems() {
        const items = super._menuItems();
        if (this._libraryItems) items.push('separator', ...this._libraryItems());
        return items;
    }

    async load(path) {
        if (this._loading) return;
        this._loading = true;
        const isReload = (path || '') === this.path;
        const savedScroll = isReload ? (this._contentEl || this.container).scrollTop : 0;
        this.path = path || '';
        const hadSelection = this.selection.selected.size + this.selectedDirs.size > 0;
        this.selection.clear();
        this.selectedDirs.clear();
        if (hadSelection && this.onSelectionChange) this.onSelectionChange([]);
        this.keyboard.focusedIndex = 0;
        this.warnings = [];
        this.entries = [];
        this._exifPollPath = null;
        this._metaPollPath = null;
        this._entryMeta = {};
        this._aspectRatios = {};
        this._photoMap = new Map();
        this._folderPreviews = new Map();
        this.render();

        let data;
        try {
            const r = await fetch(
                `/api/library/${this._libID}/browse?path=${encodeURIComponent(this.path)}`
            );
            if (!r.ok) throw new Error(await r.text());
            data = await r.json();
        } catch (err) {
            this._loading = false;
            this.container.innerHTML = `<div class="error">${t('Failed to load: {error}', { error: err.message })}</div>`;
            return;
        }

        this._loading = false;
        const folderEntries = (data.subfolders || []).map(name => ({
            name, type: 'dir', date: new Date(0).toISOString(),
        }));
        const photoEntries = (data.photos || []).map(photo => {
            const relPath = this.path ? `${this.path}/${photo.filename}` : photo.filename;
            this._photoMap.set(relPath, { photoID: photo.id });
            if (photo.exif) {
                const m = {};
                if (photo.exif.GPSLatitude)    m.hasGPS = true;
                if (photo.exif.FilmSimulation) m.filmSimulation = photo.exif.FilmSimulation;
                if (photo.exif.AspectRatio)    m.aspectRatio = photo.exif.AspectRatio;
                if (Object.keys(m).length > 0) this._entryMeta[photo.filename] = m;
            }
            return {
                name: photo.filename,
                type: 'image',
                date: photo.indexedAt, // already an ISO string from the API
                exifDate: photo.dateTaken || null,
                size: photo.fileSize,
            };
        });
        this.entries = [...folderEntries, ...photoEntries];
        this._resortAndRender();
        this.keyboard.updateFocusClass();
        if (isReload && savedScroll > 0) {
            (this._contentEl || this.container).scrollTop = savedScroll;
        }
        this._notifyFocusChange();
        this._applyPendingPreselect();
        if (this.onLoad) this.onLoad();
        if (folderEntries.length) this._loadFolderPreviews(this.path, this.path);
    }

    // A library knows what a folder holds, so the folder shows it from the
    // start: a mosaic waiting for its four photos.
    _folderItemHTML(idx, name, focusedClass) {
        return this._folderTileHTML(idx, name, focusedClass, this._folderTileBody(name, this._folderPreviews.get(name)));
    }

    // Organize sorts one folder, so a selection has to name one. Photos in a
    // library are all in the folder that is open, so that folder is the
    // answer and they arrive selected; a folder picked on its own is the
    // folder itself.
    getOpenInCommanderTarget() {
        const files = this.getSelectedFiles();
        if (files.length) {
            return {
                dir: this._sourcePath + (this.path ? '/' + this.path : ''),
                names: files.map(p => p.split('/').pop()),
            };
        }
        if (this.selectedDirs.size !== 1) return null;
        const relDir = Array.from(this.selectedDirs)[0];
        const dir = this._sourcePath + (relDir ? '/' + relDir : '');
        return { dir, names: [] };
    }

    // A library has no filesystem home to go to; its top is its own root.
    homeTarget() { return ''; }

    // The way out of the library is the first crumb: Libraries / Pictures / 2017.
    crumbsBefore() {
        return this._onLeave ? `<a href="#libraries" class="crumb-out">${t('Libraries')}</a><span class="crumb-sep"> / </span>` : '';
    }

    attachEvents() {
        super.attachEvents();
        this.container.querySelector('.crumb-out')?.addEventListener('click', (e) => {
            if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
            e.preventDefault();
            this._onLeave();
        });
    }

    homeLabel() { return t('Top of this library'); }

    organizeBtnHint() {
        return t('Select photos, or one folder, to show in Organize');
    }

    // Library EXIF data lives in the SQLite DB — no need to poll the browse API.
    _pollExifDates() {}
    _pollOverlayMeta() { if (this.showOverlays) this._updateOverlays(); }

    getLibraryMeta(path) {
        const info = this._photoMap.get(path);
        return info ? { libID: this._libID, photoID: info.photoID } : null;
    }

    thumbURL(entry, size) {
        const info = this._photoMap.get(this.fullPath(entry.name));
        return info
            ? LibraryAPI.thumbURL(this._libID, info.photoID)
            : API.thumbnailURL(this.fullPath(entry.name), size);
    }

    thumbFallbackURL(entry, size) {
        return API.thumbnailURL(this.fullPath(entry.name), size);
    }

    viewerImageURL(path) {
        const info = this._photoMap.get(path);
        return info ? LibraryAPI.photoURL(this._libID, info.photoID) : API.imageURL(path);
    }

    // The library's stored preview, shown while the full-size photo loads.
    viewerPreviewURL(path) {
        const info = this._photoMap.get(path);
        return info ? LibraryAPI.thumbURL(this._libID, info.photoID) : null;
    }

    viewerThumbURL(path) {
        const info = this._photoMap.get(path);
        return info ? LibraryAPI.thumbURL(this._libID, info.photoID) : API.thumbnailURL(path, 80);
    }

    viewerLoadInfo(path, infoPanel) {
        const info = this._photoMap.get(path);
        if (info) {
            infoPanel.loadFromURL(
                `/api/library/${this._libID}/photo/${info.photoID}/info`,
                `lib:${this._libID}:${info.photoID}`
            );
        } else {
            infoPanel.loadInfo(path);
        }
    }

    async fetchRecursivePhotoPaths(dirPath) {
        const r = await fetch(
            `/api/library/${this._libID}/browse-recursive?path=${encodeURIComponent(dirPath)}`
        );
        if (!r.ok) return [];
        const data = await r.json();
        return (data.photos || []).map(photo => {
            this._photoMap.set(photo.relPath, { photoID: photo.id });
            return photo.relPath;
        });
    }
}
