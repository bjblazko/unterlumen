# Thumbnail Overlays

*Last modified: 2026-03-09*

## Summary

Display colored metadata badges on thumbnails in browse mode, showing file type, GPS presence, and Fujifilm film simulation at a glance without opening the info panel.

## Details

A "Show details" toggle in the View menu controls all overlay badges. When enabled:

- **File type badges** appear immediately (derived from filename extension): JPEG, HEIF, PNG, GIF, WebP — each with a distinct, vibrant color.
- **GPS icon** — a small map pin icon appears (always first) for images with GPS coordinates.
- **Film simulation badges** — Fujifilm film simulation names (e.g. Classic Chrome, Velvia, Acros) appear with unique colors per simulation.

GPS and film simulation badges appear after a short delay as they require background EXIF extraction via the new `/api/browse/meta` polling endpoint.

The backend `ExtractDateAndMeta()` function performs a single EXIF decode pass for date, GPS, and film simulation, with HEIF fallback — fixing a gap where `ExtractDateTaken()` didn't handle HEIF files.

Overlays work in grid, justified, and list views.

The same color scheme is used in the info panel: the Format value and Film Simulation value are displayed as colored badges matching the thumbnail overlay colors.

## Acceptance Criteria

- [x] "Show details" toggle appears in View menu after "Show names"
- [x] File type badges appear immediately on toggle (no backend needed)
- [x] GPS pin icon appears after EXIF polling completes
- [x] Film simulation badges appear after EXIF polling completes
- [x] Badges render correctly in grid, justified, and list views
- [x] Badge colors are vibrant and unique per category (file type, film simulation)
- [x] GPS badge always appears first
- [x] Info panel shows matching colored badges for Format and Film Simulation
- [x] `/api/browse/meta` endpoint returns GPS and film simulation data
- [x] `ExtractDateAndMeta()` handles HEIF files via embedded EXIF fallback
- [x] `go vet ./...` passes
