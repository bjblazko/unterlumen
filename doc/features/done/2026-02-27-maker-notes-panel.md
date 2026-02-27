# Fujifilm Film Simulation in Camera Info Panel

*Last modified: 2026-02-27*

## Summary

Show the Fujifilm film simulation (e.g. Classic Chrome, Acros, Velvia) as a "Film Simulation" row in the Camera section of the info panel for Fujifilm images.

## Details

Fujifilm stores the film simulation in its proprietary MakerNote IFD, which is not decoded by the `goexif/mknote` package. Two tags are relevant:

- **0x1401 FilmMode** (INT16U) — color simulations (Provia, Velvia, Classic Chrome, Eterna, etc.)
- **0x1003 Saturation** (INT16U) — B&W and Acros simulations (values ≥ 0x300 indicate a film mode)

The Fujifilm MakerNote is little-endian, prefixed by an 8-byte "FUJIFILM" ASCII header followed by a 4-byte IFD offset.

**Backend (`internal/media/exif.go`)**:
- Removed `mknote` import and `init()` registration (no longer needed).
- Added `extractFujiFilmSimulation(x *exif.Exif) string` that reads the raw MakerNote bytes, validates the "FUJIFILM" header, walks the IFD entries, and returns a human-readable simulation name.
- Helper functions `fujiColorSimName` and `fujiBWSimName` map tag values to names.
- Result stored as `data.Tags["FilmSimulation"]` in `ExtractAllEXIF`.

**Frontend (`web/js/infopanel.js`)**:
- Added `FilmSimulation` tag after `LensModel` in the Camera section.
- Removed the "Maker Notes" section entirely (makerPrefixes, makerRows, MakerNote handling).

## Acceptance Criteria

- [x] Build passes (`go build -o unterlumen .`)
- [x] Vet passes (`go vet ./...`)
- [x] Fujifilm JPEG (X-T3, X-T50, GFX, etc.): "Film Simulation" appears in Camera section with correct name
- [x] Fujifilm Acros shot: "Acros" (or "Acros + R" etc.) appears
- [x] Non-Fujifilm JPEG: no "Film Simulation" row appears
- [x] Image with no EXIF: no errors, panel shows normally
- [x] "Maker Notes" section no longer appears for any image
