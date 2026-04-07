# Image Export

*Last modified: 2026-04-07*

## Summary

Convert and export selected images to web-friendly formats (JPEG, PNG, WebP) from the Tools menu. Supports quality control, flexible scaling, EXIF metadata management, per-file size and dimension estimates, and progress feedback.

## Details

Accessible via **Tools > Convert & Export**. Works on any selection of images (including HEIF/HEIC source files).

### Format & Quality
- Output formats: **JPEG**, **PNG**, **WebP**
- Quality slider (1–100) for JPEG and WebP; PNG is always lossless
- JPEG and PNG encoded with Go's native image library using CatmullRom resampling
- WebP encoded via ffmpeg with Lanczos resampling for high photo quality

### Scaling
- **Original size** — no resizing
- **Percentage** — scale by any percentage (e.g. 50% for half resolution)
- **Maximum dimension** — constrain maximum width or maximum height, keeping aspect ratio

### EXIF Metadata
- **Strip all** — output contains no metadata
- **Keep all** — copies all EXIF from source to output (requires exiftool)
- **Keep, remove GPS** — copies all EXIF except location data (requires exiftool)

### Size Estimation
The modal shows estimated output file size and output pixel dimensions per image with two methods:
- **~** (heuristic) — instant formula based on dimensions × quality factor; shown by default
- **◦** (exact) — encodes each file in memory and returns the real byte count

Exact estimation runs per-file with a progress bar labeled **"Calculating exact sizes…"**, a `N of M` counter, and an **Abort** button to cancel mid-run.

Totals row (input → output) sums all files and aligns under the file list columns.

### Upscale Warning
When any output dimension exceeds the source dimension, an **!** badge appears next to the dimensions. Hovering shows a tooltip explaining that upscaling cannot recover detail and may cause artefacts.

### Error Display
If a file fails during exact estimation (e.g. ffmpeg error for WebP), the error is shown inline in the file row. The last line of the error message is displayed as text; the full error is available in the tooltip.

### Output (derived from `UNTERLUMEN_ROOT_PATH`)
- **Local mode** (no `UNTERLUMEN_ROOT_PATH`, or started with a CLI path argument):
  - Save to folder: writes converted files directly to a chosen local directory
  - Download as ZIP: bundles all exports into a single archive download
- **Server mode** (`UNTERLUMEN_ROOT_PATH` set — navigation locked to a root):
  - ZIP download only; folder save option is hidden

### Reusability
The core conversion logic lives in `internal/media/export.go` as `ExportImage()` and `EstimateSize()`, designed for reuse in other contexts (e.g. bulk processing, CLI tools).

## Acceptance Criteria

- [x] "Convert & Export" button appears in the Tools dropdown for any image selection
- [x] Export button is always visible regardless of exiftool availability
- [x] Format tabs switch between JPEG, PNG, WebP; quality slider hides for PNG
- [x] Scale modes all work correctly: none, percent, max dimension
- [x] EXIF modes: strip produces no metadata; keep preserves all; keep_no_gps removes location
- [x] "Keep, remove GPS" option is disabled when exiftool is not available
- [x] Heuristic estimate is shown by default; toggles to exact on click
- [x] Estimates auto-refresh when format, quality, or scale changes (debounced 400 ms)
- [x] Exact estimation shows progress bar labeled "Calculating exact sizes…" with N of M counter and Abort button
- [x] Per-file output pixel dimensions shown; upscale warning badge (!) with tooltip when output exceeds source
- [x] Estimation errors shown inline in the file row (last error line as text, full error in tooltip)
- [x] Totals row aligns with file list columns; sums both input and output bytes
- [x] Local mode: "Save to folder" writes files with correct names and extensions
- [x] Local mode: "Download as ZIP" triggers browser ZIP download
- [x] Server mode (`UNTERLUMEN_ROOT_PATH` set): Output section hidden, ZIP-only
- [x] HEIC/HEIF sources export correctly to all three formats
- [x] Output filenames use the correct extension for the chosen format
- [x] `go vet ./...` passes with no errors
