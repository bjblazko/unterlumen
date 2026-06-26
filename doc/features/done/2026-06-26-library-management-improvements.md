# Library Management Improvements

*Last modified: 2026-06-26*

## Summary
Five improvements to the library overview that make it more informative and easier to manage.

## Details

### 1 & 2. Library sort order
The library overview can now be sorted automatically by most-recent additions (newest first) or reordered manually using ↑/↓ arrow buttons. Sort mode is persisted server-side in `settings.json` (via `GET/PATCH /api/settings`). Manual order is stored as a `sort_position` prop in each library's SQLite database (written in bulk via `PUT /api/library-order`), so the order survives browser resets and works across devices.

The "recent additions" sort key (`lastNewPhotos`) is a new backend field that updates only when a scan actually finds new photos — a maintenance reindex does not change it.

### 3. Parent folder in scan progress
Scan progress now shows the parent folder of the file being processed: `"foo.jpg in "bar" · 42/102"`. This helps users understand which part of the library is being scanned, especially for large libraries with many subfolders. The `Progress` struct gained a `Parent` field (JSON: `parent`) populated with `filepath.Base(filepath.Dir(absPath))`.

### 4. Edit library properties
A new Edit button on each library card opens a dialog to rename the library or update its description. Source path remains read-only. Changes are persisted via `PATCH /api/library/{id}`.

### 5. New additions badge
A small orange dot appears next to a library name when new photos were added since the user last visited the library overview. The badge clears automatically when the user navigates to the library overview (tracked via `localStorage['library.lastOverviewVisit']`).

## Acceptance Criteria
- [x] Library list can be sorted by most-recent additions (auto mode)
- [x] Library list can be sorted manually with ↑/↓ buttons (manual mode)
- [x] Sort mode persists server-side in `settings.json`; manual order persists as `sort_position` in each library's DB
- [x] Scan progress shows `"filename in 'folder' · done/total"` format
- [x] Edit button on library card opens dialog with pre-filled name and description
- [x] PATCH /api/library/{id} endpoint updates name and description
- [x] Orange dot badge appears when lastNewPhotos > lastOverviewVisit
- [x] Badge clears when user visits the library overview
- [x] Existing installations self-migrate without data loss (library_props new keys only)
- [x] ADR-0021 documents the migration strategy
