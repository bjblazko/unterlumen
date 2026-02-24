# HEIF Disk Cache & Viewer State Preservation

*Last modified: 2026-02-24*

## Summary

Cache HEIF-to-JPEG conversion results to disk using the OS temp directory, and preserve the browse/commander grid when opening the full-screen viewer so returning does not require re-fetching all thumbnails.

## Details

### Disk cache for HEIF conversions

- Converted JPEG data is stored in `os.TempDir()/unterlumen-cache` (portable across Linux, macOS, Windows).
- Cache keys are SHA256 hashes of `path | modtime | purpose`, producing unique filenames per source file and conversion type.
- Both `ConvertHEIFToJPEG` (full-size) and `ExtractHEIFPreview` (thumbnail) check the cache before running ffmpeg.
- Cache is invalidated automatically when the source file's modification time changes.
- Cache directory uses `0700` permissions; cached files use `0600`.

### HEIF extraction prefers embedded JPEG

- `extractBestJPEG` probes the HEIF container for embedded MJPEG streams (ffmpeg probe output).
- Selects the largest embedded JPEG preview (skips 160x120 thumbnails).
- Extracts via stream copy (`-c copy`), which is instant — no re-encoding.
- Falls back to HEVC decode (`-vcodec mjpeg`) for simple HEIF/HEIC files without embedded previews.

### Viewer preserves grid state

- Opening the full-screen viewer hides the browse/commander DOM (`display: none`) instead of replacing it.
- The viewer is created in a separate container appended alongside the hidden content.
- On close, the viewer container is removed and the original content is restored (`display: ''`).
- This avoids re-fetching all thumbnails for large directories (800+ files).

## Acceptance Criteria

- [x] HEIF conversions are cached to OS temp directory
- [x] Cache keys incorporate file path, modification time, and purpose
- [x] Embedded JPEG preview is preferred over HEVC tile decode
- [x] Fallback to HEVC decode works for HEIF files without embedded previews
- [x] Opening and closing the viewer does not re-fetch thumbnails
- [x] Grid state (scroll position, loaded images) is preserved across viewer open/close
- [x] Cache directory is portable (Linux, macOS, Windows)
