# Large Folder Performance

*Last modified: 2026-02-28*

## Summary

Loading large folders (500+ images) is slow because `ScanDirectory` synchronously extracts EXIF dates from every file before returning, and the frontend renders all DOM elements at once. Three changes address this: an in-memory scan cache, deferred EXIF extraction with a polling endpoint, and chunked DOM rendering.

## Details

### In-memory scan cache

A `ScanCache` stores directory scan results keyed by absolute path. Cache entries are validated against the directory's modification time and evicted when stale. File mutations (copy, move, delete) invalidate affected directories. Repeat visits to a folder return instantly from cache.

### Deferred EXIF extraction

`ScanDirectoryFast` returns entries using file modification times only (no EXIF calls). A background goroutine extracts EXIF dates and stores them in the cached entry. A new `GET /api/browse/dates?path=...` endpoint lets the frontend poll for completion. When dates arrive and the user is sorting by date, the grid re-sorts client-side.

### Chunked rendering

Grid and list views render in batches of 50 items. An `IntersectionObserver` on a sentinel element triggers loading the next batch as the user scrolls. Keyboard navigation past the rendered range calls `_ensureRenderedUpTo()` to render items on demand.

## Acceptance Criteria

- [x] `go vet ./...` passes with no errors
- [ ] Navigating into a 500+ image folder shows the grid within ~200ms
- [ ] Only ~50 grid items are in the DOM initially; scrolling loads more
- [ ] Sorting by date re-sorts when EXIF dates arrive in the background
- [ ] Navigating away and back loads the folder instantly from cache
- [ ] Copy/move/delete invalidates the cache; next visit rescans
- [ ] Keyboard arrow-down past the rendered chunk renders items on demand
- [ ] Works in both grid and list views
