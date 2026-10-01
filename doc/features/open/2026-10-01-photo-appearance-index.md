# Photo appearance in the index

*Last modified: 2026-10-01*

## Summary

Each indexed photo has its colours, tones, texture and a perceptual hash measured from its thumbnail. The results are stored in two tables of the library's database, as the ground for colour statistics, a colour search ("orange and teal") and finding look-alikes. This document covers the collection only; charts and search follow in their own work. Decisions: [ADR-0044](../../architecture/adr/0044-photo-appearance-from-thumbnails.md).

## Details

What is measured (`src/internal/appearance`):

| Group | Values |
|---|---|
| Colour | mono / tinted / colour, the tint's hue, the average colour (OKLCh), colourfulness (Hasler–Süsstrunk), warmth (−1 blue … +1 orange), up to five swatches with their share and a 30° hue bin |
| Tone | mean, median, 5th and 95th percentile of lightness, contrast, key (low / normal / high), shares of pure white and pure black, a 16-bin histogram |
| Texture | entropy, sharpness (variance of the Laplacian), edge density, where the light sits (centroid) |
| Hash | 64-bit dHash |

Where it is stored: `photo_appearance` (one row per photo) and `photo_palette` (one row per swatch), both in the library's SQLite database.

When it runs:

- after every scan of a library;
- at startup, for photos without values or with values of an older `appearance.Version`;
- with "Analyse photos again" in Edit library → Maintenance.

The pass shows in the sidebar's status line as `Analysing "<library>"` with "x of y photos", and only when there is something to measure.

Calibration on the e2e fixtures (77 JPEGs) with `CALIBRATE_DIR=../e2e/fixtures/photos go test -tags calibrate -run Calibrate -v ./internal/appearance/`:

- The one black-and-white photo has a 99th-percentile chroma of 0.000. The dullest colour photo has 0.021, so the threshold for mono is 0.012.
- Warm sunsets and green foliage keep their hues within 8–14°, so a photo counts as tinted only within 6°. The fixtures hold no toned photo.
- Measuring takes about 22 ms per 1200 px thumbnail, decoding included, on an M-series Mac.

## Acceptance Criteria

- [x] `internal/appearance` measures colour, tone, texture and a hash from an image, with tests on synthetic images with known answers
- [x] Two tables in the library database; deleting a photo deletes its rows
- [x] The pass runs after each scan and at startup and measures only what is missing or outdated
- [x] Rebuilding a thumbnail makes its photo measured again
- [x] "Analyse photos again" in Edit library measures the whole library again, with progress
- [x] e2e spec `library-appearance.spec.js`
- [ ] Checked against the owner's NAS library: share of mono and tinted photos plausible, time per photo noted
- [ ] A toned photo (sepia, Acros with a filter) is classed tinted
