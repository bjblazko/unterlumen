# Statistics & Library Performance Optimisations

*Last modified: 2026-05-10*

## Summary

Significantly reduce the time needed to open the Statistics modal for large libraries (20k+ photos) by eliminating expensive per-row JSON extraction, optimising format distribution queries, raising the SQLite page cache, and adding an in-memory result cache that is invalidated on scan start/completion.

## Details

**Root causes addressed:**

1. **Per-row JSON extraction** — All shooting-hours, shooting-days, and timeline queries used `json_extract(exif_json, '$.dateTaken')` on every photos row, triggering SQLite's JSON parser for each of 20k+ records on each Statistics or Timeline request.

2. **Full filename scan for format distribution** — All filenames were fetched into Go and the extension was parsed in a loop; for 20k photos this transferred 20k rows just to count 5–10 distinct format strings.

3. **No result caching** — Identical statistics were recomputed from scratch on every modal open.

**Changes made:**

- Added `date_taken TEXT` column to the `photos` table. Populated at index time from `exifData.DateTaken`. Backfilled for existing photos via `UPDATE photos SET date_taken = json_extract(exif_json, '$.dateTaken')` migration. Indexed with `photos_date_taken_idx`.
- Added `ext TEXT` column to the `photos` table. Populated at index time (normalised lowercase extension, e.g. `jpg→jpeg`, `hif/heic→heif`). Backfilled via SQL CASE statement. Indexed with `photos_ext_idx ON photos(status, ext)`.
- Rewrote all `json_extract(exif_json,'$.dateTaken')` references in `Statistics()` and `Timeline()` to use the `date_taken` column directly.
- Replaced the full-filename-fetch format query with `SELECT ext, COUNT(*) … GROUP BY ext`.
- Raised the SQLite page cache to 64 MB (`PRAGMA cache_size = -65536`) and enabled in-memory temp tables (`PRAGMA temp_store = MEMORY`).
- Added in-memory statistics and timeline caches in `Manager` (`sync.Map`), keyed by (sorted library IDs, path prefix, granularity). Cache is invalidated at `StartScan` (before indexing) and `EndScan` (after indexing), ensuring results are always fresh after a scan completes.

## Acceptance Criteria

- [x] Statistics modal loads noticeably faster for a 20k+ photo library
- [x] Second open of Statistics modal is instant (served from cache)
- [x] Running a re-index invalidates the cache; next open recomputes
- [x] `go vet ./...` passes
- [x] E2E tests pass
- [x] CHANGELOG updated
