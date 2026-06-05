# Selective Cache Eviction & Library Scan in Tools Menu

*Last modified: 2026-06-05*

## Summary

Adds targeted cache-clearing and library scan operations to the Tools menu, accessible without leaving the current view. Prompted by a production issue where HEIC thumbnails failed to generate in a container, but "Scan new and changed" didn't re-compute them because file mtimes hadn't changed.

## Details

### Clear cache (browse mode + library pane)

A new "Cache" section appears in the Tools menu at all times (browse, commander, library). The "Clear cache" button:

- **Files selected**: evicts cached thumbnails for each selected file via `POST /api/cache/evict`
- **Folder focused, no files selected**: recursively evicts cache for every file inside that folder
- **Feedback**: Tools button disables while running; toast shows "Clearing cache…" then "Cache cleared for N file(s)"

Backend: new `handleCacheEvict(boundary)` handler in `src/internal/api/cache.go` wraps the existing `media.EvictFile()` function and adds directory-recursive walk. Paths are validated against the root boundary to prevent traversal attacks.

### Library scan (library pane only)

When browsing inside a specific library (`App.mode === 'library'`), a "Library scan" section appears with three buttons:

- **Scan new** — incremental scan for new/changed files (`POST /api/library/{id}/scan-new`)
- **Re-index** — full re-index of all photos (`POST /api/library/{id}/reindex`)
- **Cleanup** — remove index entries for deleted files (`POST /api/library/{id}/cleanup`)

All three use the existing SSE-based library API. Progress streams as a live toast updating in real time; final toast shows "Done — N photos". The "Make library" option in the Tools menu is automatically hidden when in library mode (existing behavior unchanged).

### Shared infrastructure

`App.showToast(msg)` is now a method on the global `App` object, consolidating the previously duplicated `_showToast` function in `library.js` and the `_showUIHint` pattern in `app.js`.

## Acceptance Criteria

- [x] Tools menu "Cache" section visible in browse mode
- [x] "Clear cache" evicts cache for selected file(s), count reported in toast
- [x] "Clear cache" on focused folder recursively evicts all files inside
- [x] Paths correctly constructed as absolute (boundary-prefixed for browse, sourcePath-prefixed for library)
- [x] Path traversal guard: out-of-boundary paths silently skipped
- [x] Tools button disabled while operation runs, re-enabled on completion
- [x] Tools menu "Library scan" section visible only when `App.mode === 'library'`
- [x] "Make library" section hidden when in library mode (pre-existing behavior, verified unchanged)
- [x] Cache section visible in library mode as well
- [x] Scan new / Re-index / Cleanup stream live progress to toast
- [x] Final toast shows "Done — N photos" after library scan completes
