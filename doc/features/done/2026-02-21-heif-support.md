# HEIF/HEIC Format Support

*Last modified: 2026-02-24*


## Summary

Display HEIF/HEIC/HIF images by converting them to JPEG on-the-fly using ffmpeg.

## Details

HEIF/HEIC/HIF files (`.heif`, `.heic`, `.hif` extensions) are detected by `media.IsHEIF()`. The `.hif` extension is used by Fujifilm cameras. Since browsers cannot render HEIF natively and no pure-Go decoder exists, the server converts them to JPEG by shelling out to ffmpeg.

### Extraction strategy

Both full-size and thumbnail endpoints use `extractBestJPEG`, which probes the HEIF container for embedded MJPEG streams. The largest embedded JPEG preview (typically 1920x1280 for Fujifilm HIF) is extracted via stream copy (`-c copy`), which is instant. If no embedded preview is found, ffmpeg decodes the HEVC stream to JPEG as a fallback. Tiny thumbnails (160x120) are skipped.

Note: ffmpeg's tile grid assembly (`-map 0:g:0`) does not reliably produce full-frame output for multi-tile Fujifilm HIF files — it outputs a single tile instead. The embedded JPEG preview is the preferred source. When no large embedded JPEG exists, macOS `sips` is used as a fallback — it uses Apple's native HEIF decoder which correctly assembles multi-tile grids. The ffmpeg HEVC decode remains as a last-resort fallback for non-macOS systems.

### Disk caching

Converted JPEG data is cached to the OS temp directory (`os.TempDir()/unterlumen-cache`). Cache keys are SHA256 hashes of file path, modification time, and purpose (full vs preview). This avoids re-running ffmpeg for the same file.

### Where conversion is used

- `GET /api/image` — best available JPEG extracted and served
- `GET /api/thumbnail` — same extraction, then resized server-side

### Graceful degradation

- If ffmpeg is not installed, HEIF requests return an HTTP 500 error
- All other image formats continue to work without ffmpeg
- The browse listing still shows HEIF files in the directory (they just fail to display)

## Acceptance Criteria

- [x] HEIF/HEIC/HIF files appear in directory listings
- [x] Full-size HEIF images display in the viewer (when ffmpeg is available)
- [x] Multi-tile HEIF (Fujifilm HIF) assembles the full image, not a single tile
- [x] HEIF thumbnails use embedded JPEG previews when available
- [x] No temporary files created during conversion
- [x] Fails gracefully when ffmpeg is not installed
- [x] File paths are not vulnerable to shell injection
