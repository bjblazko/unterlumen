# Search Panel Performance

*Last modified: 2026-05-11*

## Summary

Reduce the time to open the library search/filter panel from ~20 s to near-instant on large libraries (20k+ photos).

## Details

**Root causes addressed:**

1. **No caching of EXIF ranges and text values** — Every panel open recomputed `AggregateExifRanges` and `AggregateExifFieldValues` from scratch. These are expensive aggregation queries over a large `exif_index` table, and results change only when a new scan completes.

2. **Excessive DB round-trips** — `GetExifRanges` issued 6 separate `SELECT MIN/MAX` queries per library (one per EXIF field). The `FocalLength35` query was especially slow: a double-JOIN across `photos` and two aliases of `exif_index` to compute a per-photo COALESCE.

3. **Blank panel** — While the 5 parallel HTTP requests resolved (serialising at the single SQLite connection), the panel appeared as an empty box with no indication of progress.

**Changes made:**

- Added `exifRangesCache sync.Map` and `exifFieldValuesCache sync.Map` to `Manager`. Cache key for ranges: sorted library IDs. Cache key for field values: sorted library IDs + `|` + field name. Both caches are invalidated in `InvalidateStatsCache` (called at scan start and scan end), keeping the same lifecycle as `statsCache` and `timelineCache`.
- Refactored `GetExifRanges` in `store.go`: all scalar fields (`ExposureTime`, `FNumber`, `FocalLength`, `FocalLengthIn35mmFilm`, `ISOSpeedRatings`) are now fetched with a single `GROUP BY field` query using the existing `exif_index_field_numeric` index. The `FocalLength35` virtual field uses a simple `WHERE field IN ('FocalLengthIn35mmFilm','FocalLength')` query instead of the double-JOIN.
- Added "Loading filters…" text to `_toggle()` in `library-filter.js` so the panel gives immediate visual feedback on first open.

## Acceptance Criteria

- [x] Search panel on a 20k+ photo library opens noticeably faster than before
- [x] Second open (without re-indexing) is instant (cache hit)
- [x] Running a re-index invalidates the cache; next open recomputes correctly
- [x] `TestFocalLength35Range` still passes (semantics preserved for test data)
- [x] `go vet ./...` passes
- [x] `go test ./internal/library/...` passes
- [x] CHANGELOG updated
