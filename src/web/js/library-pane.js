// LibraryPane — BrowsePane subclass that reads folder contents from the library DB.
// No browse API calls are made, so no background EXIF extraction runs over NAS.

// "412 photos · 2021–2026": how much a folder holds and the years it spans.
function folderTileMeta(preview) {
    const n = preview.photoCount;
    const parts = [`${n} photo${n === 1 ? '' : 's'}`];
    const first = (preview.firstTaken || '').slice(0, 4);
    const last = (preview.lastTaken || '').slice(0, 4);
    if (first && last) parts.push(first === last ? first : `${first}–${last}`);
    return parts.join(' · ');
}

class LibraryPane extends BrowsePane {
    constructor(container, libID, options = {}) {
        super(container, options);
        this._libID = libID;
        this._sourcePath = options.sourcePath || '';
        this._onStatistics = options.onStatistics || null;
        this._photoMap = new Map(); // relPath → { photoID }
        this._folderPreviews = new Map(); // subfolder name → preview
        this.sort = 'taken';
        this.order = 'desc';
    }

    // A phone's library head has no room for Statistics, so it joins the ⋯.
    _menuItems() {
        const items = super._menuItems();
        if (this._onStatistics && window.matchMedia('(max-width: 700px)').matches) {
            items.push({ id: 'statistics', label: 'Statistics', onSelect: this._onStatistics });
        }
        return items;
    }

    async load(path) {
        if (this._loading) return;
        this._loading = true;
        const isReload = (path || '') === this.path;
        const savedScroll = isReload ? (this._contentEl || this.container).scrollTop : 0;
        this.path = path || '';
        this.selection.clear();
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
            this.container.innerHTML = `<div class="error">Failed to load: ${err.message}</div>`;
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
        if (folderEntries.length) this._loadFolderPreviews(this.path);
    }

    // The tiles appear with the folder names at once; what each folder holds
    // follows from the index and fills them in place.
    async _loadFolderPreviews(path) {
        let previews;
        try {
            const r = await fetch(`/api/library/${this._libID}/folder-previews?path=${encodeURIComponent(path)}`);
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

    // A library knows what a folder holds, so the folder shows it: its four
    // newest photos, how many there are and the years they span.
    _folderItemHTML(idx, name, focusedClass) {
        const markedClass = this.isMarkedForDeletion(this.fullPath(name)) ? ' marked-for-deletion' : '';
        return `<button class="folder-tile dir-item${focusedClass}${markedClass}" data-index="${idx}" data-name="${escapeHtml(name)}" data-type="dir">
            <span class="folder-tile-body">${this._folderTileBody(name, this._folderPreviews.get(name))}</span>
        </button>`;
    }

    _folderTileBody(name, preview) {
        const ids = preview?.photoIds || [];
        const cells = [0, 1, 2, 3].map(i => ids[i]
            ? `<img src="${LibraryAPI.thumbURL(this._libID, ids[i])}" alt="" loading="lazy">`
            : '<span></span>').join('');
        return `<span class="folder-tile-mosaic${ids.length === 1 ? ' folder-tile-mosaic--one' : ''}" aria-hidden="true">${cells}</span>
            <span class="item-name">${escapeHtml(name)}</span>
            <span class="folder-tile-meta">${preview ? escapeHtml(folderTileMeta(preview)) : '&nbsp;'}</span>`;
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

    homeLabel() { return 'Top of this library'; }

    organizeBtnHint() {
        return 'Select photos, or one folder, to show in Organize';
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
