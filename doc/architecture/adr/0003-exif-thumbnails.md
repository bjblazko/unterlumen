# ADR-0003: EXIF Embedded Thumbnails as Primary Thumbnail Source

*Last modified: 2026-02-21*

## Status

Accepted

## Context

Displaying thumbnails for a directory of photos requires either:

1. Pre-generating and caching scaled-down versions (like most photo managers)
2. Extracting the thumbnail already embedded in the image's EXIF data
3. Serving full images and letting the browser scale them down

Option 1 requires a write-capable cache (conflicts with ADR-0002). Option 3 wastes bandwidth and is slow for large files.

## Decision

Use EXIF embedded thumbnails as the primary source. Fall back to server-side resizing (in-memory, not cached to disk) for formats without EXIF thumbnails (PNG, GIF, WebP). For HEIF files, the ffmpeg-converted JPEG is served directly as the thumbnail.

## Consequences

- **Fast** — Extracting an embedded thumbnail is a seek + read of a few KB, regardless of the full image size. No pixel decoding of the main image needed.
- **JPEG-only for EXIF** — Only JPEG files reliably contain EXIF thumbnails. PNG, GIF, and WebP fall back to server-side decode + resize, which is slower but acceptable for these typically smaller files.
- **Thumbnail quality varies** — Embedded thumbnails are typically 160×120 or similar. Quality is sufficient for grid browsing but may look soft. This is an acceptable trade-off for speed.
- **No write side-effects** — Consistent with ADR-0002; no thumbnail cache pollutes the photo directory.
