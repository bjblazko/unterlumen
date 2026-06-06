# Library Filter: Date Taken Sort & Date Range Filter

*Last modified: 2026-06-06*

## Summary

The library filter ("all filters") view gains two date-related capabilities: results are sorted by photo taken date by default, and a date range filter lets users narrow results to photos taken between two dates.

## Details

**Sort by date taken (default)**
- `SearchResultPane` now defaults to `sort = 'taken'`, `order = 'desc'` (newest first), matching `LibraryPane`.
- The sort dropdown already included a "Photo Taken" option inherited from `BrowsePane`; it was just not wired up.
- `SearchResultPane._mapPhoto()` now exposes `exifDate` from `p.dateTaken`.
- `SearchResultPane.setSort()` gains a `'taken'` case with nulls-last behaviour.
- The backend `ListPhotos()` query now returns `date_taken` and orders by it (`CASE WHEN date_taken IS NULL … END, date_taken DESC`).
- The cross-library merge sort (`sortLibraryPhotos`) now sorts by parsed `DateTaken` (nulls last).
- Photos without EXIF date always appear at the bottom, regardless of sort direction.

**Date range filter**
- New "Date taken" section in `LibrarySearchPanel` with From/To `<input type="date">` fields.
- Sends `date_taken_min` / `date_taken_max` query params to `/api/library/search`.
- Backend uses `SUBSTR(p.date_taken, 1, 10)` for comparison, which is timezone-safe.
- Reset button clears the date inputs alongside other filter state.

## Acceptance Criteria

- [x] Filter results appear sorted newest-first by default
- [x] "Photo Taken" is the selected option in the sort dropdown when opening filter results
- [x] ↑/↓ toggle reverses sort order (oldest first)
- [x] Photos without EXIF date appear at the bottom regardless of sort direction
- [x] Date range inputs appear in the filter panel
- [x] Entering a From date excludes photos taken before that date
- [x] Entering a To date excludes photos taken after that date
- [x] Both dates together narrow results correctly
- [x] Reset filters button clears the date inputs and removes the date constraint
- [x] Works correctly in both single-library and cross-library (all libraries) mode
