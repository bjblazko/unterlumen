// LibraryPane — BrowsePane subclass that serves cached library thumbnails with live fallback

class LibraryPane extends BrowsePane {
    constructor(container, libID, options = {}) {
        super(container, options);
        this._libID = libID;
    }

    thumbURL(entry, size) {
        return `/api/library/${this._libID}/thumb-by-path?path=${encodeURIComponent(this.fullPath(entry.name))}`;
    }

    thumbFallbackURL(entry, size) {
        return API.thumbnailURL(this.fullPath(entry.name), size);
    }
}
