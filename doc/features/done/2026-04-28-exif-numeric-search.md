*Last modified: 2026-04-29*

# EXIF Filter Panel

## Summary

Sidebar filter panel for the four core numeric EXIF fields (shutter speed, aperture, focal
length, ISO) and three text fields (camera, lens, film simulation), available both per-library
and across all libraries from the library list.

## Details

### Problem

EXIF values for numeric fields arrive in many formats from different cameras and scanning
software: `"1/500"`, `"1/500 s"`, `"0.004 sec"` (shutter speed), `"28/10"`, `"f/2.8"`
(aperture), `"50/1"`, `"50 mm"` (focal length). Text LIKE queries cannot do range matching
across these representations. Having filters stacked vertically above the photo grid wastes
significant vertical screen space.

### Approach

Normalize numeric EXIF values at **index time** into a canonical float and store them in a
`numeric_value REAL` column in `exif_index`. Sliders then do `BETWEEN` queries on this
column. The filter panel sits in a left sidebar beside the photo grid rather than stacking
above it.

### Backend

- `src/internal/media/normalize.go` — four parsers + `NormalizeExifNumbers()`:
  - `ParseExposureSeconds` — handles rationals, decimals, unit suffixes
  - `ParseFNumber` — handles rationals and `f/2.8` prefix forms
  - `ParseFocalLengthMM` — handles rationals, mm suffix; skips zoom ranges like "24-70"
  - `ParseISO` — handles plain integers and "ISO 400" prefix forms
- `src/internal/library/store.go`:
  - `exif_index.numeric_value REAL` column + migration for existing databases
  - `UpsertExifIndex` stores numeric values alongside text values
  - `NumericFilter` type + extended `ListPhotos()` with `BETWEEN` range support
  - `ExifRange` type + `GetExifRanges()` for slider bounds
  - `GetExifFieldValues()` returns distinct text values with double-TRIM for quoted strings
  - Text filters use exact match (`TRIM(TRIM(e.value, '"')) = ?`) rather than LIKE
- `src/internal/api/library/handler.go`:
  - `GET /api/library/{id}/exif-ranges` — returns `{ field: { min, max } }` per field
  - `GET /api/library/{id}/photos` — accepts `Field_min` / `Field_max` query params
  - `GET /api/library/exif-ranges` — global ranges across all libraries
  - `GET /api/library/exif-values?field=F&ids=ID` — distinct text values for dropdowns
  - `GET /api/library/search` — cross-library photo search with EXIF filters
  - `parseNumericFilters` uses pointer-based bounds; only sends a filter when min or max
    is explicitly moved (avoids false zero-bounds when only one side is set)

### Frontend

- `src/web/js/library-filter.js` — `LibrarySearchPanel`:
  - Fetches `/exif-ranges` on open; hides sliders when min === max (no useful range)
  - Log-scale sliders for shutter speed, aperture, ISO (matching photographic stops)
  - Linear slider for focal length
  - Custom dual-handle drag slider (shaped triangle handles, no `<input type="range">`)
  - Text filter dropdowns (camera, lens, film sim) — only shown when >1 distinct value
  - 300 ms debounce on change; shows matched photo count in sidebar
  - Library selector dropdown (all libs or single lib); reloads ranges and text values on change
  - Resets all filters and re-queries on "Reset filters"
- `src/web/js/library.js`:
  - **Per-library**: "Filter" button in detail header → sidebar beside the photo grid
  - **Cross-library**: "Search" button on list view → sidebar with library selector
  - Both use `LibrarySearchPanel`; detail view passes `onResults` callback, list view
    passes `resultsContainer` so the grid renders in the main content area
  - `div.lib-search-body` wrapper enables the sidebar flex layout in both views
- `src/web/css/style.css`:
  - `.lib-search-body` — flex-row wrapper; `:has(> .lib-search-panel.visible)` shows it
  - Sidebar panel: 210 px wide, scrollable, border-right separator
  - Filter groups stack vertically inside sidebar (overrides horizontal default)
  - Detail view always shows `lib-search-body`; sidebar slides in when filter opens

## Acceptance Criteria

- [x] After re-indexing, `exif_index.numeric_value` is populated for all four fields on
  photos that have those EXIF tags
- [x] Old library databases gain the column automatically on next launch (migration)
- [x] `GET /api/library/{id}/exif-ranges` returns correct min/max for each field present
- [x] `GET /api/library/{id}/photos?ExposureTime_min=0.002&ExposureTime_max=0.008` returns
  only photos whose shutter speed falls in that range
- [x] Filter panel appears as a left sidebar when "Filter" (detail) or "Search" (list) is clicked
- [x] Sliders are hidden for fields with no useful range (all photos the same value)
- [x] Moving a slider shows the matched photo count within ~300 ms
- [x] Shutter speed display formats as "1/500" or "2 s", not raw floats
- [x] Cross-library search works from the library list with the library selector
- [ ] E2E test covers slider interaction and result count change
