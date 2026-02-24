# EXIF Orientation Support

*Last modified: 2026-02-22*

## Summary

Photos taken in portrait mode (or at other non-standard orientations) now display correctly in both the grid thumbnail view and the fullscreen viewer. The app reads the EXIF orientation tag and applies the appropriate rotation.

## Details

- **Thumbnails (server-side rotation)**: The EXIF orientation tag (values 1–8) is extracted before serving thumbnails. Embedded EXIF thumbnails and generated thumbnails are rotated on the server, since Go's `image.Decode()` and EXIF-embedded thumbnails don't respect orientation metadata.
- **Full-size images (CSS)**: Raw image files are served with EXIF data intact. The browser handles correct display via the CSS `image-orientation: from-image` property, applied to grid, list, and viewer image elements.
- **HEIF orientation (server-side rotation)**: HEIF files store rotation in an `irot` (image rotation) box in the ISOBMFF container, not in EXIF. The `irot` box is parsed directly from the file header and mapped to EXIF-compatible orientation values. Rotation is applied to both thumbnails and full-size converted JPEGs before caching.
- **Thumbnail aspect ratio validation**: Embedded EXIF thumbnails are validated against the actual image dimensions (via `PixelXDimension`/`PixelYDimension` EXIF tags). When the aspect ratios differ by more than 10% (e.g., camera stores a full-sensor 4:3 thumbnail for an in-camera 1:1 crop), the EXIF thumbnail is rejected and a correct thumbnail is generated from the full image.
- **No new dependencies**: The 8 EXIF orientation values are mapped to pixel coordinate remappings using the existing standard library. HEIF `irot` parsing uses `encoding/binary`.

## Acceptance Criteria

- [ ] `ExtractOrientation` reads EXIF orientation tag, defaults to 1 on error
- [ ] `applyOrientation` correctly handles all 8 orientation values (identity, flips, rotations, transposes)
- [ ] Embedded EXIF thumbnails are rotated server-side before serving
- [ ] Generated thumbnails are rotated server-side before serving
- [ ] The "small enough, serve as-is" fast path re-encodes when orientation > 1
- [ ] Full-size images display correctly via CSS `image-orientation: from-image`
- [ ] Portrait photos (orientation 6 or 8) display upright in grid view
- [ ] Portrait photos display upright in fullscreen viewer
- [ ] Landscape photos (orientation 1) are unaffected
- [ ] HEIF portrait photos display upright in grid and fullscreen views
- [ ] HEIF landscape photos are unaffected
- [ ] EXIF thumbnails with mismatched aspect ratio (e.g., 4:3 thumb for 1:1 image) are rejected
- [ ] Standard EXIF thumbnails with matching aspect ratio still use the fast path
- [ ] `go vet ./...` passes
- [ ] `go build` succeeds
