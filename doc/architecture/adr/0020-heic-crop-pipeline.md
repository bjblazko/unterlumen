# ADR-0020: HEIC In-Place Crop via JPEG Intermediary

*Last modified: 2026-06-30*

## Status

Accepted

## Context

HEIF/HEIC files store rotation metadata in two different locations depending on the camera manufacturer:

- **HEIC irot box** (ISOBMFF container level): the canonical HEIF rotation method; read by `ExtractHEIFOrientation`. Apple-device HEIC files typically use this.
- **Embedded JPEG EXIF orientation**: Fujifilm cameras embed a JPEG preview stream inside the HEIC and store rotation in that JPEG's own EXIF tag, leaving the HEIC irot box absent or at 0.

The original HEIF crop implementation used `sips --cropToHeightWidth H W --cropOffset Y X`. Two problems were discovered during testing with Fujifilm HEIC files:

**Problem 1 — Ambiguous sips coordinate space.**
`sips --cropOffset` uses visual (orientation-applied) coordinates, but `sips -g pixelWidth/pixelHeight` returns stored (encoded, pre-rotation) pixel dimensions for Fujifilm HEIC files. Multiplying visual fractions by stored landscape dimensions and passing the result to `--cropOffset` (which expects visual portrait coordinates) placed the crop in the wrong region.

**Problem 2 — sipsConvert does not bake orientation for Fujifilm HEIC.**
`sips -s format jpeg` preserves the EXIF orientation tag in the JPEG output rather than rotating the pixels, when the HEIC's rotation is stored in the EXIF item (not the irot box). Go's `jpeg.Decode` ignores EXIF orientation, so decoding the sipsConvert output without explicitly applying the orientation yielded a crop in stored (landscape) pixel space. The result appeared rotated 90° clockwise when displayed as portrait.

The coordinate space that `sips --cropOffset` uses is fundamentally tied to how each camera manufacturer stores rotation, making it impossible to pass correct coordinates without first knowing the manufacturer convention and reading the appropriate metadata source.

## Decision

Replace the `sips --cropToHeightWidth/--cropOffset` path with a three-step decode-crop-encode pipeline:

1. **Decode to visual pixels**: Call `sipsConvert` to get a JPEG. Read the JPEG's EXIF orientation via `extractJPEGOrientation` and apply it with `applyOrientation`, producing a Go `image.Image` in the correct visual (display) orientation — regardless of whether the HEIC stored rotation in irot or EXIF.

2. **Crop in Go image space**: Call `cropRect` on the oriented image. Coordinates are unambiguous here: they are visual pixel fractions [0,1] of the displayed image, matching what the frontend sends. Encode the cropped result as JPEG (no EXIF orientation tag; orientation is baked into the pixel layout).

3. **Re-encode to HEIC**: Call `sips -s format heic cropped.jpg --out output.heic` to produce a valid HEIC. Copy original metadata (GPS, lens, MakerNotes, etc.) with exiftool, excluding dimension tags and clearing orientation (`-Orientation=`).

## Consequences

- **Correct for all HEIC variants**: Works for irot-based (Apple) and EXIF-based (Fujifilm, possibly others) HEIC files because orientation is applied in Go before cropping, without relying on sips internal coordinate conventions.
- **Two sips invocations per crop**: The pipeline runs sips twice (HEIC→JPEG decode, then JPEG→HEIC encode) instead of once. Crop is an infrequent, user-initiated operation; the additional latency (~1–3 s) is acceptable.
- **One JPEG re-encode cycle**: The HEVC image is decoded to JPEG at quality 92 by sips, cropped in memory, and re-encoded at JPEG quality 92 before HEVC re-encoding. This is one generation of lossy compression beyond the original — the same trade-off made by all non-lossless crop tools.
- **sips dependency preserved**: HEIC crop still requires macOS. Returns an error on non-macOS platforms; other formats (JPEG, PNG, WebP) are unaffected.
- **New helper**: `extractJPEGOrientation(data []byte) int` added to the `media` package reads EXIF orientation from raw JPEG bytes without a temp-file round-trip, keeping the decode path in memory.
- **Key diagnostic for future debugging**: if a sips-based image operation produces a result that is rotated or has misaligned coordinates, check whether the HEIC stores rotation in the irot box (checked by `ExtractHEIFOrientation`) or in the EXIF item of the HEIC container (checked by reading EXIF after sipsConvert). These two sources are independent and sips does not always unify them.

## Amendment — 2026-06-30: unified `heifOrientation` for the full pipeline

Portrait HEIF/HIF images from Fujifilm cameras were displayed in landscape orientation when running on Docker/Linux (where `sips` is unavailable). The fallback decoders (`heif-convert`, `ffmpegRun`) were only considering the `irot` box via `ExtractHEIFOrientation`, which returns 1 for Fujifilm files. Without a JPEG EXIF tag in the output (stripped by `ffmpegRun`, or omitted by some `heif-convert` builds), no rotation was applied.

**Fix**: added two helpers to `media/exif.go`:

- `heifExifOrientation(path)` — reads the EXIF orientation tag from the HEIF container's embedded EXIF block (distinct from the `irot` box).
- `heifOrientation(path)` — the canonical orientation lookup for HEIF files. Prefers `irot`; falls back to `heifExifOrientation` when `irot` is unset. **All HEIF conversion and thumbnail paths must use this function instead of calling `ExtractHEIFOrientation` directly.**

The correct pattern for each decoder type:

| Decoder | Orientation source |
|---|---|
| `sipsConvert` | `extractJPEGOrientation(outputJPEG)` — sips preserves EXIF |
| embedded JPEG stream | `extractJPEGOrientation(data)`, fallback to `heifOrientation(path)` |
| `heifConvert` | `extractJPEGOrientation(data)`; if 1 AND `irot`=1 → `heifExifOrientation(path)` |
| `ffmpegRun` | `heifOrientation(path)` directly (no EXIF in output) |

Cache keys were also bumped (`preview-v4`→`v5`, `full-v3`→`v4`, `thumbnailCacheVersion` `v2`→`v3`) to prevent stale landscape-oriented cached files from being served after the fix.
