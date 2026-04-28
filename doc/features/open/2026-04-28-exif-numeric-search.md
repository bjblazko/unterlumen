*Last modified: 2026-04-28*

# EXIF Numeric Search (Slider Filters)

## Summary

Range-slider filters for the four core numeric EXIF fields — shutter speed, aperture,
focal length, and ISO — so users can narrow a library to photos taken under specific
exposure conditions without typing anything or knowing what format the EXIF data is in.

## Details

### Problem

EXIF values for numeric fields arrive in many formats from different cameras and scanning
software: `"1/500"`, `"1/500 s"`, `"0.004 sec"` (shutter speed), `"28/10"`, `"f/2.8"`
(aperture), `"50/1"`, `"50 mm"` (focal length). Text LIKE queries cannot do range matching
across these representations.

### Approach

Normalize numeric EXIF values at **index time** into a canonical float and store them in a
new `numeric_value REAL` column in `exif_index`. Sliders then do `BETWEEN` queries on this
column — users never deal with the raw format.

### Backend (done)

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
- `src/internal/api/library/handler.go`:
  - `GET /api/library/{id}/exif-ranges` — returns `{ field: { min, max } }` per field
  - `GET /api/library/{id}/photos` — now accepts `Field_min` / `Field_max` query params

### Frontend (done)

- `src/web/js/library-filter.js` — `LibraryFilterPanel`:
  - Fetches `/exif-ranges` on open; hides sliders when min === max (no useful range)
  - Log-scale sliders for shutter speed, aperture, ISO (matching photographic stops)
  - Linear slider for focal length
  - 300 ms debounce on change; shows matched photo count
  - Toggle via "Filter" button in the library detail header
- `src/web/css/style.css` — filter bar and slider panel styles

## Acceptance Criteria

- [ ] After re-indexing, `exif_index.numeric_value` is populated for all four fields on
  photos that have those EXIF tags
- [ ] Old library databases gain the column automatically on next launch (migration)
- [ ] `GET /api/library/{id}/exif-ranges` returns correct min/max for each field present
- [ ] `GET /api/library/{id}/photos?ExposureTime_min=0.002&ExposureTime_max=0.008` returns
  only photos whose shutter speed falls in that range
- [ ] The Filter panel appears when the Filter button is clicked
- [ ] Sliders are hidden for fields with no useful range (all photos the same value)
- [ ] Moving a slider shows the matched photo count within ~300 ms
- [ ] Shutter speed display formats as "1/500" or "2 s", not raw floats
- [ ] E2E test covers slider interaction and result count change
