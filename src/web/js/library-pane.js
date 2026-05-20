// LibraryPane — BrowsePane subclass that reads folder contents from the library DB.
// No browse API calls are made, so no background EXIF extraction runs over NAS.

class LibraryPane extends BrowsePane {
    constructor(container, libID, options = {}) {
        super(container, options);
        this._libID = libID;
        this._sourcePath = options.sourcePath || '';
        this._photoMap = new Map(); // relPath → { photoID }
        this.sort = 'taken';
        this.order = 'desc';
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
            name, type: 'dir', date: new Date(0),
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
                date: new Date(photo.indexedAt),
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
    }

    getOpenInCommanderTarget() {
        if (this.selectedDirs.size !== 1) return null;
        const relDir = Array.from(this.selectedDirs)[0];
        const dir = this._sourcePath + (relDir ? '/' + relDir : '');
        return { dir, names: [] };
    }

    commanderBtnHint() {
        return 'Select a folder to open in Organise view';
    }

    // Library EXIF data lives in the SQLite DB — no need to poll the browse API.
    _pollExifDates() {}
    _pollOverlayMeta() { if (this.showOverlays) this._updateOverlays(); }

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
