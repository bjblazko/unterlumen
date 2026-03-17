# Folder Operations & Reusable Progress Dialog

*Last modified: 2026-03-17*

## Summary

Adds folder support to copy, move, and delete operations in the commander, plus a reusable modal progress dialog that shows per-item progress with cancel support for any multi-file operation.

## Details

### Folder Operations

- **Copy folder**: Enumerates the folder contents via `POST /api/list-recursive`, creates subdirectories in the destination, then copies files one-by-one with progress feedback.
- **Move folder**: Attempts a fast `os.Rename` first (same filesystem). If that fails, falls back to copy + delete.
- **Delete folder**: Shows a confirmation dialog ("Delete folder 'xyz' and all its contents? This cannot be undone."), then calls `os.RemoveAll` on the backend. Directories bypass the wastebin and are deleted immediately. File items in the same selection still go through the wastebin flow.
- **Backend changes**: `copyFile` and `moveFile` now detect directories and delegate to `copyDir` / rename+fallback. `handleDelete` uses `os.RemoveAll` for directories. `ScanCache.InvalidatePrefix` clears all cached subdirectories when a folder is deleted or moved.

### Progress Dialog

- New `ProgressDialog` class in `web/js/progress-dialog.js`, following the `LocationModal` pattern.
- Shows title, status text ("Copying 2 of 200 files..."), 4px accent-orange progress bar, current filename, and Cancel/OK button.
- Processes items sequentially, checking a cancellation flag before each iteration.
- On cancel: shows partial completion count. On error: shows expandable error list.
- Escape key and overlay clicks are blocked during operation.
- Integrated into: commander copy/move, `permanentlyDelete` (>5 files), `LocationModal._execute` / `_executeRemove` (>5 files).

### API

- `POST /api/list-recursive` — Request: `{ "path": "some/folder" }`. Response: `{ "files": [...], "dirs": [...] }`. Dirs sorted shallowest-first. Capped at 100,000 entries.

## Acceptance Criteria

- [x] `go build` and `go vet` pass cleanly
- [x] Copy a folder in commander shows per-file progress dialog
- [x] Move a folder uses fast rename on same filesystem
- [x] Delete a folder shows confirmation, removes folder and contents
- [x] Cancel button stops a multi-file operation mid-way
- [x] Progress dialog blocks background interaction
- [x] Location operations with >5 files use progress dialog
- [x] Permanent delete with >5 files uses progress dialog
- [x] Single-item operations skip the progress dialog
