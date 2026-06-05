# Publish from Filter Results

*Last modified: 2026-06-06*

## Summary

Enable the Publish workflow from EXIF filter results — both within a single library (detail view) and across libraries (cross-library list view) — in addition to the existing folder-tree selection.

## Details

Previously the Publish button was hard-disabled whenever the EXIF filter was active in the detail view, and the cross-library list view had no Publish button at all. Users had to close the filter, manually locate photos, select them, and only then publish.

### Changes

- **Detail view filter** (`_searchPane`): Publish button now tracks the filter result selection. It enables as soon as one or more photos are selected and re-disables when the selection is cleared. Switching back to the folder tree restores the folder-selection-driven state.

- **Cross-library list view**: A new Publish… button is added to the list-view header. It is disabled by default and enables only when filter results have a selection.

- **Photo ID resolution**: Photos from `SearchResultPane` already carry their `{ libID, photoID }` in the in-memory `_photoMap`. The confirm handler uses these directly, skipping the `photoIDByPath` API call per photo (faster, no extra round-trips).

- **Multi-library publish**: When selected photos span multiple libraries, plain publish runs one API call per library group, writing all exported files to the same output path. Gallery/site export and ZIP download require all selected photos to be from a single library — a clear error message is shown if this constraint is violated.

- **Album title validation**: Publishing to a multi-album site channel without providing an album title shows an error and does not proceed.

### Files changed

- `src/web/js/library-filter.js` — Thread `onSelectionChange` option through `LibrarySearchPanel` to its inner `SearchResultPane`
- `src/web/js/library.js` — `_updateDetailPublishBtn()`, wired selection callbacks, list-view publish button, refactored `_openPublishModal()` source detection and multi-group confirm handler

## Acceptance Criteria

- [x] Selecting photos from EXIF filter results in the single-library detail view enables the Publish button
- [x] Deselecting all photos or switching back to the folder view disables the Publish button correctly
- [x] Cross-library list view shows a Publish… button that enables on filter result selection
- [x] Publishing selected filter results works end-to-end (plain publish, gallery, site export)
- [x] Multi-library selection with gallery/site export shows a clear error and does not publish
- [x] Multi-library selection with plain publish succeeds and writes all files to the same output folder
- [x] ZIP download with multi-library selection shows a clear error
- [x] Existing folder-tree publish flow is unaffected
