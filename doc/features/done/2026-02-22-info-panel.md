# Info Panel

*Last modified: 2026-02-22*

## Summary

A collapsible right-side panel that displays file metadata and EXIF data for the selected image. Available in both browse mode and the fullscreen viewer.

## Details

- Collapsed by default as a narrow strip with an info icon button
- Expands to 320px wide panel showing organized metadata sections
- Toggle via clicking the icon or pressing `I`
- Backend: new `GET /api/info?path=<relative>` endpoint returns file metadata and full EXIF data
- Sections displayed (when data exists): File, Image, Camera, Exposure, Dates, Location, Other
- Numeric EXIF values (metering mode, exposure program, flash, white balance) decoded to human-readable labels
- Panel updates on single selection, clears on multi-select or no selection
- Browse grid auto-reflows when panel opens/closes via flex layout
- Fullscreen viewer: separate InfoPanel instance with dark theme styling matching the viewer UI
- Viewer Info button in toolbar toggles panel; `I` key works in viewer
- Panel automatically updates when navigating between images in the viewer
- Panel state (expanded/collapsed) persists during navigation within the viewer
- Each viewer session gets a fresh InfoPanel instance (created on open, destroyed on close)

## Acceptance Criteria

- [x] `GET /api/info` endpoint returns file name, path, size, modified date, format, and EXIF data
- [x] Path traversal protection via `safePath()` on the info endpoint
- [x] `ExtractAllEXIF` function walks all EXIF tags, extracts GPS and dimensions
- [x] InfoPanel component with collapsed (40px) and expanded (320px) states
- [x] File section shows name, path, size, format, modified date
- [x] Camera section shows make, model, lens, software
- [x] Exposure section shows shutter speed, aperture, ISO, focal length, metering, program, flash, white balance
- [x] Dates section shows original, digitized, modified
- [x] Location section shows latitude/longitude when GPS data present
- [x] Other section shows remaining EXIF tags not covered by named sections
- [x] `I` keyboard shortcut toggles the panel in browse mode
- [x] Single selection loads info, multi/no selection clears panel
- [x] Browse grid reflows when panel opens/closes
- [x] Works gracefully with non-EXIF images (PNG, GIF) — shows file info only
- [x] Info panel available in fullscreen viewer with "Info" toolbar button
- [x] `I` keyboard shortcut toggles the panel in viewer mode
- [x] Dark theme styling for viewer info panel (dark backgrounds, light text)
- [x] Panel updates automatically when navigating between images in viewer
- [x] Closing viewer does not affect browse mode info panel state
