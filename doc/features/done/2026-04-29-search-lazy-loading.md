# Search/Filter Lazy Loading

*Last modified: 2026-04-29*

## Summary

Scrolling through EXIF-filtered or text-searched results in Library mode now loads all matching photos instead of stopping at the first 100.

## Details

Previously, both the per-library filter panel and the cross-library search panel always fetched `limit=100` and never requested more. BrowsePane's existing client-side IntersectionObserver chunked those 100 results into 50-item scroll increments, but no further API calls were made.

The fix adds server-side pagination to `SearchResultPane`. When the user scrolls near the bottom of the currently loaded set, the next page is fetched from the server and appended to `this.entries`. BrowsePane's existing chunk rendering then continues naturally through the new entries. A stale-result guard (`_fetchGeneration`) ensures that changing a filter mid-scroll discards any in-flight fetch from the previous query.

The backend was updated to support this:
- Cross-library search (`/api/library/search`) now accepts up to 500 results per page (raised from 200).
- The per-library internal fetch limit in `SearchLibraries` is now `offset + limit` instead of a hardcoded 200, so any pagination depth is reachable.

Both the integrated `SearchResultPane` (cross-library list view) and the callback-based path (`onResults` in the per-library detail view) receive the full pagination context.

## Acceptance Criteria

- [x] Scrolling to the bottom of a filter result set with >100 matches loads the next page automatically
- [x] Works in both the cross-library search panel and the per-library filter view
- [x] Changing a filter resets pagination to page 1
- [x] The breadcrumb shows the server total count from the first page
- [x] No visible flicker or re-render when appending new pages
