# Tools Dropdown — Set Location

*Last modified: 2026-03-16*

## Summary

A "Tools" dropdown in the browse controls bar (directly adjacent to the View button) provides image-editing tools that operate on selected or focused images via exiftool. The dropdown checks for exiftool availability on first open and shows a message if it is missing.

**Set Location** — Manually set GPS coordinates on images using an interactive map picker. Preserves all existing EXIF data including maker notes.

## Details

- **Tools dropdown** appears in the browse controls bar directly adjacent to the View button
- **exiftool check**: `GET /api/tools/check` returns availability, cached via `sync.Once`
- **Set Location**: Modal with MapLibre GL map, click-to-place marker, editable lat/lon fields, confirmation step warning about overwrite, per-file success/error summary
- **API endpoint**: `POST /api/set-location` with `{files, latitude, longitude}`, following existing `fileOpResult` pattern
- **Orientation label**: Info panel Orientation field shows human-readable names instead of raw EXIF integers

## Acceptance Criteria

- [x] Tools dropdown appears in browse mode controls, adjacent to View button
- [x] exiftool missing message shown when exiftool is not installed
- [x] Set Location: map picker, editable fields, confirmation step, GPS written via exiftool
- [x] Set Location works on HEIF files
- [x] Orientation field in info panel shows readable labels (e.g. "Normal", "Rotated 90° CW")
- [x] `go vet ./...` passes
