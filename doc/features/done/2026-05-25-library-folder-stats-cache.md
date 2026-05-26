# Library Folder Stats Cache

*Last modified: 2026-05-25*

## Summary

Replace the per-request filesystem walk in `libraryFolderStats` with a DB-backed query and in-memory cache, eliminating the main performance bottleneck when clicking folders in library mode.

## Details

When a folder is selected in library mode, the info panel loads two data sources:
- `GET /api/library/{id}/folder-stats` — structural stats (size, count, subfolders)
- `GET /api/library/statistics` — EXIF stats (cameras, formats, shooting hours)

The second endpoint was already cached (`Manager.statsCache`). The first was calling `media.WalkFolderStats()` — a full recursive filesystem walk — on every request, with no cache.

**Changes:**

- New `LibraryFolderStats` and `LibSubfolder` model types in `library/model.go`.
- New `Store.FolderStats(folderAbs string)` method in `store.go`: derives photo count, total size, format breakdown, subfolder list and sizes, and date range entirely from the indexed `photos` table. No filesystem access.
- New `Manager.FolderStats(id, relPath string)` with lazy `folderStatsCache` (keyed `"<libID>|<absPath>"`).
- New `Manager.prewarmFolderStats(id string)`: collects all unique ancestor directories from `path_hint` values and pre-computes `FolderStats` for each. Called in a background goroutine from `EndScan`, so after every scan all folder stats are ready immediately.
- `InvalidateStatsCache` extended to also clear `folderStatsCache` entries for the library.
- `libraryFolderStats` handler simplified: delegates entirely to `mgr.FolderStats`.
- `infopanel.js`: `renderFolderData` detects the new `LibraryFolderStats` shape (via `photoCount` field) and delegates to a new `_renderLibraryFolderData` method. Shows photo count, total size, date range, size map treemap, and file types. No depth histogram (not in DB). Browse-mode rendering unchanged.

**Scope:** indexed photos only (no .xmp sidecars or unindexed files). Consistent with the rest of library mode.

## Acceptance Criteria

- [ ] Clicking a folder in library mode populates the info panel in <200ms (after first scan)
- [ ] Info panel shows photo count, total size, date range, subfolder treemap, file types in library mode
- [ ] After a scan, all folder stats are pre-computed — zero latency on first access
- [ ] Running a scan clears the folder stats cache; stats refresh correctly on next access
- [ ] Browse mode (non-library) folder info panel unchanged: still shows filesystem stats with depth histogram
- [ ] `go vet ./...` passes
- [ ] E2E tests pass
