# Focal Length 35mm Equivalent Filter

*Last modified: 2026-04-29*

## Summary

A "35mm equivalent" checkbox added to the focal length range slider in the library search/filter panel. When enabled, filtering uses the `FocalLengthIn35mmFilm` EXIF tag (tag 0xA405) rather than the native `FocalLength` tag, making comparisons meaningful across different sensor sizes (iPhone, APS-C, full-frame, etc.).

## Details

- The checkbox appears below the focal length range slider label.
- When checked, the slider range and filter query switch to the `FocalLength35` virtual field, which resolves to `FocalLengthIn35mmFilm` where available and falls back to `FocalLength` for photos that lack the 35mm equivalent EXIF tag.
- The fallback is handled in SQL at query time, so no re-indexing is required for existing photos.
- A startup migration backfills `numeric_value` for `FocalLengthIn35mmFilm` from the already-stored string value in `exif_index`, so existing libraries work immediately.
- Resetting filters clears the checkbox back to the standard focal length mode.

## Acceptance Criteria

- [x] A "35mm equivalent" checkbox appears below the focal length slider in the search/filter panel.
- [x] When checked, the slider range updates to reflect 35mm-equivalent values in the library.
- [x] Filtering by 35mm-equivalent range returns photos whose `FocalLengthIn35mmFilm` is within range.
- [x] Photos without `FocalLengthIn35mmFilm` fall back to `FocalLength` for the filter.
- [x] "Reset filters" unchecks the checkbox.
- [x] Existing libraries work without re-indexing (startup migration backfills numeric values).
