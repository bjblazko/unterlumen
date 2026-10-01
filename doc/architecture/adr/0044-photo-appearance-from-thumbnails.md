# ADR-0044: What a photo looks like is measured from its thumbnail

*Last modified: 2026-10-01*

## Status

Accepted.

## Context

The ideas for Statistics (`doc/ideas/statistics.md`) need values the index does not hold: a colour per photo for colour over time and the colour cloud, the entropy for the density landscape, and a similarity between photos for the relationship graph. The owner also wants to search by colour ("orange and teal photos") and to tell black-and-white photos from colour ones. All of this comes from the pixels, not from EXIF. On 2026-10-01 the owner chose:

- four groups of measurements: colour, tone and exposure, texture and structure, and a perceptual hash;
- a pass after the scan rather than work inside it, so scanning stays as fast as it was;
- a pass that fills in what is missing on its own, plus a maintenance action that measures again;
- colour stored as numbers, with colour names mapped to ranges when a query runs.

## Decision

- **The thumbnail is the source.** Every indexed photo has a 1200 px JPEG thumbnail that is already the right way up, so HEIF decoding and EXIF orientation (CLAUDE.md, Gotchas) are already done. The original is never read for this.
- **One pure package, `internal/appearance`.** It takes an image and returns a `Result`. It knows nothing of libraries or SQL. It scales the image to 512 px for sharpness and edges and to 256 px for everything else, so a value means the same for every photo.
- **OKLab for colour.** Distances in OKLab match how far apart colours look, and its hue angle names colours steadily: about 30° red, 55° orange, 195° teal, 265° blue. The palette is a k-means in OKLab with the colour axes weighted three times against lightness. Unweighted, lightness ruled and the palette split one hue into five brightnesses. The seed is fixed, so a photo always gives the same palette.
- **Mono, tinted, colour.** A photo is mono when the 99th percentile of its chroma is below 0.012, and tinted when its hues lie within 6° and that percentile stays below 0.08. Calibration on the e2e fixtures: a converted black-and-white photo has 0.000, the dullest colour photo 0.021, and a warm sunset spreads its hues over 8–14°.
- **Two tables beside the EXIF index, not in it.** `photo_appearance` holds one row per photo, with typed columns. `photo_palette` holds up to five swatches per photo, each with L, C, h, a 30° hue bin (NULL for a neutral) and its share, and an index on `(hue_bin, share)`. `exif_index` is a key–value store of strings for camera tags; these are numbers measured by Unterlumen, and a colour search needs several values of one swatch together. Deleting a photo deletes its rows in both tables.
- **Versioned.** Each row records `appearance.Version`. Raising it marks every row outdated, and the next pass measures them again. There is no migration code.
- **When it runs.** Every scan ends in `Manager.EndScan`, which starts `AnalyseMissing` for that library. At startup `AnalyseAllMissing` goes through the libraries one by one. The pass does **not** take the index lock: a first version did, and the cleanup that follows deleting a file found the lock taken and gave up silently, so the deleted photo stayed in the index. The pass now has its own guard per library instead. It does not start while a scan runs, since the scan's end starts it again. A scan may start while it runs, and a photo deleted meanwhile is passed over when the results are stored. A call while a pass runs makes it run once more. It shows in the status line as `Analysing "<library>"`, but only when there is something to measure. Two workers decode and measure, and a single writer stores 100 results per transaction. Rebuilding a thumbnail clears its measurements. "Analyse photos again" in Edit library clears them for the library and runs the pass (`POST /api/library/{id}/analyse`).
- **Pure Go.** No cgo, no new dependency, no model. It costs about 22 ms per thumbnail on an M-series Mac, decoding included.

## Consequences

- Charts and a colour search can read the tables without touching a photo. Asking for "orange and teal" is two `EXISTS` on `photo_palette` with a hue range and a minimum share and chroma each. The names can change without measuring again.
- A thumbnail that cannot be read is passed over and tried again on the next pass; it is never stored as empty.
- Values come from a JPEG thumbnail, so clipping and sharpness are relative: good for comparing photos, not for judging a raw file's exposure.
- The thresholds were set on 77 fixture photos, none of them toned. Toned photos and the owner's own libraries may move them; changing one raises `Version`.
- Embeddings (idea 4) are not part of this. They need a model and get their own decision.
