# Sort by Photo Taken

*Last modified: 2026-03-09*

## Summary

Split the ambiguous "Date" sort option into two clearly named options: **File Modified** (filesystem mtime) and **Photo Taken** (EXIF DateTimeOriginal).

## Details

Previously, the "Date" sort option silently conflated two different dates: the filesystem modification time and the EXIF `DateTimeOriginal` (which overrode the mtime when EXIF extraction finished). This made sorting behavior non-obvious.

The new approach:

- `Entry.Date` always holds filesystem mtime — it is never overridden by EXIF data.
- `Entry.ExifDate` is a new optional field set only when EXIF extraction finds a `DateTimeOriginal`.
- **File Modified** (`sort=date`) sorts by `Entry.Date` (mtime) — deterministic, always available.
- **Photo Taken** (`sort=taken`) sorts by `Entry.ExifDate` — entries without EXIF always appear last, regardless of sort direction (asc or desc).

The frontend polls `/api/browse/dates` and stores results in `entry.exifDate`. Client-side resort on sort change uses the same null-last logic.

**Bug fix (2026-03-09):** Photos whose EXIF date happened to equal their filesystem mtime were silently omitted from the EXIF date cache due to a leftover equality guard in `extractExifBackground`. Those photos appeared as undated and sorted last in "Photo Taken" mode. The guard was removed; EXIF dates are now always stored when found.

## Acceptance Criteria

- [x] Sort dropdown shows: Name / File Modified / Photo Taken / Size
- [x] "File Modified" sorts by filesystem mtime; values do not change after EXIF loads
- [x] "Photo Taken" sorts by EXIF DateTimeOriginal; entries without EXIF go to end
- [x] "Photo Taken" descending: newest dated photo first, undated entries at end
- [x] Files without EXIF (e.g. PNGs) appear at end of "Photo Taken" sort
- [x] `go vet ./...` passes with no errors
