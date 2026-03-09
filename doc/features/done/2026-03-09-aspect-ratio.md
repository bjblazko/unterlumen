# Feature: Aspect Ratio Display

*Last modified: 2026-03-09*

## Summary

Display the aspect ratio of images (e.g. `3:2`, `16:9`) as a badge on thumbnails and as a row in the info panel, with a proportional-rectangle SVG icon that visually reinforces the shape.

## Details

- **Algorithm**: Approximate-ratio matching (1.5% tolerance) handles real camera sensors where pixel counts don't reduce to clean integers (e.g. Nikon 7360×4912 ≈ 3:2). Falls back to "Custom Crop" for unrecognised ratios.
- **Known ratios**: `1:2`, `9:16`, `2:3`, `3:4`, `4:5`, `1:1`, `5:4`, `4:3`, `3:2`, `7:5`, `16:10`, `5:3`, `16:9`, `2:1`, `21:9`.
- **Icon**: Inline SVG rectangle (16×12 viewBox, max 14×10 inner rect) proportionally scaled to the ratio. Dashed stroke for "Custom Crop".
- **Backend**: `AspectRatioLabel(w, h int) string` in `internal/media/exif.go`; `AspectRatio` field on `EntryMeta`; extracted during background EXIF scan.
- **Frontend**: Aspect ratio badge in thumbnail overlays (4th badge, after GPS/format/film-sim); "Aspect Ratio" row in the info panel Image section below Dimensions.

## Acceptance Criteria

- [x] Thumbnail overlay shows aspect ratio badge (e.g. `[▬] 3:2`) when "Show Details" is on
- [x] Badge disappears when "Show Details" is off
- [x] Info panel "Aspect Ratio" row appears below "Dimensions" with matching icon + label
- [x] Portrait images show ratio in portrait form (e.g. `2:3`)
- [x] Unusual-ratio images show "Custom Crop" with dashed icon
- [x] `go vet ./...` passes with no errors
