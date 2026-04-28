// SearchResultPane — BrowsePane subclass that renders library search results
// with all view layouts, selection, slideshow and overlay badges.

class SearchResultPane extends BrowsePane {
    constructor(container, options = {}) {
        super(container, options);
        this._photoMap = new Map(); // pathHint → { libID, photoID }
    }

    // Load an array of LibraryPhoto objects (from cross- or single-library search).
    loadResults(photos, multiLib = false) {
        this.path = '';
        this.selection.clear();
        this.keyboard.focusedIndex = 0;
        this.warnings = [];
        this._entryMeta = {};
        this._aspectRatios = {};

        this._photoMap.clear();
        this.entries = photos.map(p => {
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
                size: p.fileSize,
            };
        });

        this.render();
        this.keyboard.updateFocusClass();
        this._notifyFocusChange();
    }

    // Return { libID, photoID } for a given pathHint — used by info panel meta lookup.
    getPhotoInfo(pathHint) {
        return this._photoMap.get(pathHint);
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
        const n = this.entries.length;
        return `<nav class="breadcrumb"><span class="crumb-current">Search results · ${n} photo${n !== 1 ? 's' : ''}</span></nav>`;
    }

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
