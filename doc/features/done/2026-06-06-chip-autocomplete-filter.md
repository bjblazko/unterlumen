# Chip Autocomplete Filter for Library Search

*Last modified: 2026-06-06 (updated with EXIF fields expansion and label decoding)*

## Summary

Extends the library filter panel with a chip-based "Add filter…" input that sits below the existing numeric sliders and text dropdowns. Users can add AND-combined filter chips for any EXIF field, file format, publication channel, gallery album title, or user-defined photo tag — without having to type raw IDs or know backend key names.

## Details

### Chip namespaces

Fixed namespaces with short friendly labels:

| Namespace | Maps to | Source |
|-----------|---------|--------|
| `camera:` | `Model` EXIF field | `exif_index` |
| `lens:` | `LensModel` EXIF field | `exif_index` |
| `film:` | `FilmSimulation` EXIF field | `exif_index` |
| `format:` | File extension | `photos.ext` column |
| `flash:` | `Flash` EXIF field | `exif_index` |
| `wb:` | `WhiteBalance` EXIF field | `exif_index` |
| `channel:` | Published to channel | `photo_meta` EXISTS subquery |
| `album:` | Gallery/album title | `photo_meta` EXISTS subquery on `published:<ch>:title` |

Dynamic namespaces added automatically:

- **All other EXIF fields** present in `exif_index` (e.g. `Orientation`, `Make`, `ColorSpace`, `MeteringMode`, `SceneCaptureType`, …). Numeric slider fields (ExposureTime, FNumber, FocalLength, ISO) and date fields are excluded to avoid overlap with dedicated controls.
- **User-defined photo tags** (`photo_meta` keys not starting with `published:`)

### Human-readable value labels

A shared `exif-labels.js` module decodes raw stored values to human-readable labels for display in both the chip filter dropdown and the info panel. Examples: Orientation `1` → "Normal"; Flash `0` → "No flash", `1` → "Fired"; WhiteBalance `0` → "Auto". The raw value is stored on the chip and used for the backend query; only the display label changes.

### Two-phase autocomplete

1. Phase 1: namespace list (fixed EXIF + all dynamic EXIF fields + meta keys + channel slugs + album titles)
2. Phase 2: values for the selected namespace — decoded where a label map exists, raw otherwise (fetched on demand, cached until panel is closed and reopened)

### Keyboard support

- Arrow up/down navigates suggestions
- Enter confirms selection
- Escape cancels (returns to namespace phase if in value phase)
- Backspace on empty input removes the last chip

### Backend changes

- `ListPhotosOpts` struct replaces positional `ListPhotos` parameters; adds `MetaFilters`, `MetaExists`, `AlbumTitle`, `ExtFilter` fields with EXISTS subqueries
- `GET /api/library/meta-keys` — distinct photo_meta keys across selected libraries
- `GET /api/library/meta-values?key=X` — distinct values for a given meta key
- `GET /api/library/album-titles` — distinct published gallery titles
- `GET /api/library/exif-fields` — distinct EXIF field names present in `exif_index` (used to populate dynamic chip namespaces)
- `ext` special case: `GetExifFieldValues("ext")` queries `photos.ext` instead of `exif_index`
- XMP `Publication` struct gains `GalleryTitle`; `indexSidecar` writes `published:<ch>:title` to `photo_meta`; `publishPhotos` also writes `published:<ch>:title` directly at publish time (no re-index required for new publishes)

### Backward compatibility

- `album:` chips produce 0 matches for publications made before this feature; running Re-index backfills `:title` from existing XMP sidecars
- All existing filter functionality (sliders, date, Camera/Lens/Film Sim dropdowns) is untouched
- New `ListPhotosOpts` fields default to zero values, which are no-ops in the SQL query

## Acceptance Criteria

- [x] Filter panel shows a "More filters" section below existing controls
- [x] "Add filter…" chip input with two-phase autocomplete dropdown
- [x] 8 fixed namespaces: camera, lens, film, format, flash, wb, channel, album
- [x] All other EXIF fields from `exif_index` appear as dynamic namespaces
- [x] Human-readable labels for coded EXIF values (Orientation, Flash, WB, MeteringMode, ExposureProgram, ColorSpace, …)
- [x] Dynamic user-defined meta key namespaces appear if any exist
- [x] Phase 2 shows real values fetched from the indexed library
- [x] Selected chip appears as a pill with label and × remove button
- [x] Multiple chips combine as AND filter conditions
- [x] Result count updates on chip add/remove
- [x] × button removes chip
- [x] Backspace on empty input removes last chip
- [x] Reset filters clears all chips and restores full result set
- [x] `channel:X` chip filters to photos published to channel X
- [x] `album:X` chip filters to photos in gallery with title X (requires re-index for older publications)
- [x] New endpoints: `/api/library/meta-keys`, `/api/library/meta-values`, `/api/library/album-titles`
- [x] `format:` namespace uses `photos.ext` column (not `exif_index`)
- [x] Gallery title stored in XMP and written to `photo_meta` immediately at publish time
- [x] `album:` autocomplete suggestions update when filter panel is reopened after publishing
