# Library Sync (Incremental Indexing)

*Last modified: 2026-05-05*

## Summary

Replace the standalone "Re-index" button on each library card with a combo split button offering three scan modes and fix a bug where an interrupted scan left the library overview showing 0 photos.

## Details

### Interrupted scan count bug

`Run()` only wrote `photo_count` to `library_props` at the very end of a successful scan. If the scan was interrupted (network drop, power-save, crash), the cached count remained stale and the overview showed 0. Fixed by deferring the count update so it runs even on interruption.

### Scan modes

| Label | Endpoint | What it does |
|---|---|---|
| **Scan new and changed** | `POST /api/library/{id}/scan-new` | Adds new photos, updates changed ones, re-links renamed files (SHA-256). Does not remove anything. |
| **Re-index (full)** | `POST /api/library/{id}/reindex` | Same as Scan new and changed, plus marks missing photos and purges deleted files at the end. |
| **Cleanup deleted** | `POST /api/library/{id}/cleanup` | Checks each indexed photo's `path_hint` on disk; marks absent ones missing and purges them. No re-hashing. Run Scan new and changed first if files were renamed. |

"Scan new and changed" replaces the old standalone "Re-index" as the primary action. Rename detection (SHA-256) works in both Scan new and changed and Re-index — Cleanup is path-based only.

### Broadcaster (join in-progress scan)

All three endpoints share a `Broadcaster` that fans out progress to multiple SSE subscribers. If a scan is already running and the user clicks any scan button, the response streams the live progress of the ongoing scan instead of returning 409. Scans run on `context.Background()` so they continue even if the browser tab is closed.

### UI

Each library card gets a combo split button:
- Left: **Scan new and changed** (primary action)
- Right: dropdown arrow revealing **Re-index (full)** and **Cleanup deleted**

The combo is only on the library overview cards; the detail view has no scan buttons.

## Acceptance Criteria

- [ ] After an interrupted re-index, refreshing the library overview shows the partial count already indexed (not 0)
- [ ] "Scan new and changed" adds new photos, updates changed ones, and re-links renamed files without removing anything
- [ ] "Scan new and changed" is significantly faster than "Re-index" for large libraries with few changes (path-cache fast-path skips unchanged files without re-hashing)
- [ ] "Re-index (full)" detects and removes photos for files deleted from disk
- [ ] "Cleanup deleted" removes photos whose path_hint no longer exists, without re-hashing
- [ ] Clicking a scan button while a scan is running shows live progress of the ongoing scan (no 409 error)
- [ ] All scan buttons are disabled while a scan is running; only one scan can run per library at a time
