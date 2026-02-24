# Thumbnail Serving

*Last modified: 2026-02-21*

## Summary

Serve image thumbnails efficiently by extracting embedded EXIF data, with fallback to server-side resizing.

## Details

Thumbnails are served via `GET /api/thumbnail?path=<relative>`. The serving strategy depends on the file format:

### JPEG files

1. Attempt to extract the embedded EXIF thumbnail (typically 160x120, a few KB)
2. If no EXIF thumbnail exists, fall back to server-side resize

### PNG, GIF, WebP files

- No EXIF thumbnails available in these formats
- Server decodes the image and generates a nearest-neighbor resized version (max 300px)
- If the image is already smaller than 300px, it is served as-is

### HEIF/HEIC/HIF files

- Converted to JPEG via ffmpeg, then resized to the thumbnail max dimension (300px)
- Full aspect ratio is preserved — no cropping

### Caching

- All thumbnail responses include `Cache-Control: public, max-age=3600`
- No server-side thumbnail cache; thumbnails are re-extracted on each request
- Browser caching avoids redundant requests within a session

## Acceptance Criteria

- [x] JPEG EXIF thumbnails extracted and served
- [x] Fallback resize for non-EXIF formats
- [x] HEIF files converted and served
- [x] Small images served as-is without resize
- [x] Cache headers set on responses
- [x] Thumbnails load with `loading="lazy"` in the browser
- [x] Thumbnails preserve aspect ratio (no cropping)
- [x] HEIF thumbnails resized server-side instead of serving full resolution
