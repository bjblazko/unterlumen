# HEIF/ffmpeg Availability Warning

*Last modified: 2026-02-21*

## Summary

When a directory contains HEIF/HEIC files and ffmpeg is not installed or lacks HEIF support, show the user a clear warning explaining the issue and how to resolve it.

## Details

### Detection

- On startup or first use, check whether `ffmpeg` is available on `$PATH` and whether it supports HEIF decoding (`ffmpeg -decoders` includes `hevc`)
- Cache the result for the lifetime of the process (no need to re-check per request)

### API

- The `GET /api/browse` response includes a `warnings` array when relevant
- When the directory contains HEIF/HEIC files and ffmpeg is unavailable or lacks HEIF support, a warning object is included with a `type` and `message`

### Frontend

- When the browse response contains warnings, a dismissible banner is shown at the top of the pane
- The banner explains what's wrong and how to fix it (install ffmpeg, install with HEIF support)
- HEIF files still appear in the listing but their thumbnails will fail gracefully

## Acceptance Criteria

- [x] ffmpeg availability is checked once and cached
- [x] Warning appears when HEIF files exist in a directory but ffmpeg is missing
- [x] Warning appears when ffmpeg is present but lacks HEIF/HEVC decoder
- [x] Warning message tells the user what to install
- [x] Warning is dismissible
- [x] HEIF files still show in the file listing regardless
