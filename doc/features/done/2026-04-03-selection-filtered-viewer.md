# Selection-Filtered Viewer

*Last modified: 2026-05-23*

## Summary

When 2 or more images are selected in the gallery, the fullscreen viewer (filmstrip, prev/next navigation) should be scoped to only those selected images. Opening from a single or no selection shows all folder images as before.

## Details

- `openViewer()` in `app.js` checks `pane.selected.size >= 2`; if true, the image list passed to the viewer is filtered to only selected paths (preserving order from the folder)
- The filmstrip, counter, and keyboard/button navigation all derive from `this.images` in `viewer.js`, so no changes were needed there
- Selection-filtered mode naturally ends when the user closes the viewer and deselects images before reopening
- A **Deselect** button is added to the status bar whenever images are selected, as an alternative to Escape

## Acceptance Criteria

- [x] Selecting 2+ images and opening the viewer shows only those images in the filmstrip
- [x] Prev/next and arrow keys navigate only through the selected images
- [x] Counter reflects the filtered count (e.g. "1 / 3" for 3 selected images)
- [x] Opening the viewer with 0 or 1 selected image shows all folder images
- [x] "Deselect" button appears in the status bar when any images are selected
- [x] Clicking "Deselect" clears all selections (equivalent to Escape)
