# Thumbnail Quality Setting

*Last modified: 2026-03-04*

## Summary

Add a "Thumbnails" setting (Standard / High) that controls thumbnail resolution. When set to High, thumbnails are generated at the actual display size multiplied by the device pixel ratio, using the full image decode path with bicubic resampling instead of the low-resolution EXIF embedded thumbnail.

## Details

- **Backend**: The `/api/thumbnail` endpoint accepts an optional `size` query parameter (integer, 50–1024, default 300). When `size` exceeds 300, the EXIF thumbnail fast path is skipped and the full image is decoded and resized with `draw.CatmullRom` (bicubic).
- **Frontend**: When "High" is selected, the browse pane computes the target thumbnail size per view mode (grid column width, justified row height, or list icon size) multiplied by `devicePixelRatio`, and passes it as the `size` parameter.
- **Resize algorithm**: Both `GenerateThumbnail` and `ResizeJPEGBytes` now use `draw.CatmullRom` instead of nearest-neighbor, and JPEG quality is bumped from 80 to 85.
- **Setting persistence**: Stored in `localStorage` under `thumbnail-quality` (`'standard'` or `'high'`). Default is `'standard'` (existing behavior).
- **UI**: A "Thumbnail quality" section in the Settings dropdown with Standard/High toggle buttons, matching the Theme section pattern. Buttons include tooltips describing the trade-off.

## Acceptance Criteria

- [x] `/api/thumbnail?size=N` parameter accepted and clamped to 50–1024
- [x] `size > 300` skips EXIF thumbnail extraction
- [x] Resize uses bicubic (`draw.CatmullRom`) instead of nearest-neighbor
- [x] JPEG encode quality bumped to 85
- [x] Settings menu shows Thumbnails section with Standard/High buttons
- [x] Setting persists in localStorage and survives page reload
- [x] Switching to High reloads thumbnails at DPR-aware resolution
- [x] Standard mode preserves existing behavior (no size param sent)
- [x] HEIF thumbnails also respect the size parameter
- [x] `go vet ./...` passes
