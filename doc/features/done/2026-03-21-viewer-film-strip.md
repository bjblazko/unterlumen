# Film Strip in Fullscreen Viewer

*Last modified: 2026-03-21*

## Summary

A horizontal film strip below the main image in the fullscreen viewer, showing thumbnails of all images in the current folder. Allows quick visual navigation to any image without leaving the viewer.

## Details

- Toolbar checkbox labeled "Film strip (F)" toggles visibility, placed between the filename and counter
- Film strip is a horizontal scrollable row of 64x56px thumbnails at the bottom of the viewer
- Clicking a thumbnail navigates directly to that image
- Active thumbnail is highlighted with an orange border at full opacity; inactive thumbnails are dimmed
- Film strip auto-scrolls to keep the active thumbnail centered
- `F` key toggles the film strip, synced with the checkbox state
- Thumbnails use `loading="lazy"` for performance with large folders
- Film strip DOM is built once in `open()` and re-appended after each `render()` to survive innerHTML wipes
- Deleting an image removes its thumbnail from the strip and re-indexes remaining thumbs
- Film strip hides when UI is hidden (`H` key / `body.ui-hidden`)
- Default state: hidden

## Acceptance Criteria

- [x] Toolbar shows "Film strip" checkbox between filename and counter
- [x] Checking the checkbox shows the film strip with thumbnails
- [x] Arrow key navigation updates the active highlight and auto-scrolls
- [x] Clicking a thumbnail jumps to that image
- [x] `F` key toggles the film strip
- [x] Deleting an image removes its thumbnail from the strip
- [x] `H` key (hide UI) also hides the film strip
- [x] Lazy loading of thumbnail images
- [x] Works with large folders (100+ images)
