# ADR-0020: HEIC In-Place Crop via JPEG Intermediary

*Last modified: 2026-07-05*

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

The correct pattern for each decoder type at the time (superseded by the 2026-07-05 amendment below for `heifConvert`):

| Decoder | Orientation source |
|---|---|
| `sipsConvert` | `extractJPEGOrientation(outputJPEG)` — sips preserves EXIF |
| embedded JPEG stream | `extractJPEGOrientation(data)`, fallback to `heifOrientation(path)` |
| `heifConvert` | `extractJPEGOrientation(data)`; if 1 AND `irot`=1 → `heifExifOrientation(path)` |
| `ffmpegRun` | `heifOrientation(path)` directly (no EXIF in output) |

Cache keys were also bumped (`preview-v4`→`v5`, `full-v3`→`v4`, `thumbnailCacheVersion` `v2`→`v3`) to prevent stale landscape-oriented cached files from being served after the fix.

## Amendment — 2026-07-05: `heif-convert` bakes rotation but its output's EXIF lies about it

The 2026-06-30 amendment's table entry for `heifConvert` was wrong: it assumed that when the JPEG output orientation is 1, rotation was never baked. In practice, `heif-convert` (libheif) decodes the primary image plane already in final display orientation for Fujifilm-style files with no `irot` box, but copies the *source* HEIF's original EXIF orientation tag into its output JPEG unchanged and non-1. Every call site that read that tag and reapplied it (directly, or by falling back to `heifOrientation(path)` when the tag looked absent) rotated an already-correct image a second time — visible as a sideways portrait photo when viewed via the NAS/Docker installation. Confirmed by decoding a real Fujifilm X-T50 `.heic` file's `heif-convert` output with EXIF orientation ignored: the raw pixel data was already upright.

**Fix**: `heifConvert()` is now self-contained. It calls a new helper, `stripStaleHeifConvertOrientation`, which discards a non-1 orientation tag by re-encoding without any rotation (a no-op passthrough when the tag is already 1, avoiding needless quality loss for the common landscape case). No caller may read `extractJPEGOrientation`/`heifOrientation` on `heifConvert()`'s output and reapply it — the three call sites that previously did (`extractBestJPEG`, `extractPreviewFallbackJPEG`, `convertHEIFExport`) were simplified to trust its output directly. `ConvertHEIFToJPEG`'s wrapper-level "if the combined result's orientation is 1, consult `heifOrientation(path)` and reapply" logic was removed entirely and pushed down into the specific branches of `extractBestJPEG` that actually need it (embedded JPEG stream copy, `sipsConvert`, and the `ffmpegRun` fallback) — a single rule applied after the fact to a merged result cannot be correct across decoders with different baking behavior.

Updated pattern:

| Decoder | Orientation handling |
|---|---|
| `sipsConvert` | Caller applies `extractJPEGOrientation(outputJPEG)`, fallback `heifOrientation(path)`, then bakes — sips preserves EXIF but does not bake pixels |
| embedded JPEG stream copy | Same as `sipsConvert` — raw stream copy, nothing baked |
| `heifConvert` | Self-contained — always returns final, tag-clean bytes; callers do nothing further |
| `ffmpegRun` | Caller applies `heifOrientation(path)` directly — no EXIF in output, nothing baked |

Cache keys were bumped again (`full-v4`→`v5`, `thumbnailCacheVersion` `v3`→`v4`) so previously wrong-orientation cached full images and *newly generated* thumbnails self-heal without a rescan. **Already-indexed library thumbnails are not covered by this** — `ensureThumbnail` skips regeneration when a thumbnail file already exists on disk, so any portrait Fujifilm photo already thumbnailed via `heif-convert` needs the library's "Rebuild all previews" action (not a normal "Scan for new photos", which only fills in *missing* previews) to pick up the fix.
