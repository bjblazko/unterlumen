# Selection-Filtered Viewer

*Last modified: 2026-04-03*

## Summary

When 2 or more images are selected in the gallery, the fullscreen viewer (filmstrip, prev/next navigation) should be scoped to only those selected images. Opening from a single or no selection shows all folder images as before.

## Details

- `openViewer()` in `app.js` checks `pane.selected.size >= 2`; if true, the image list passed to the viewer is filtered to only selected paths (preserving order from the folder)
- The filmstrip, counter, and keyboard/button navigation all derive from `this.images` in `viewer.js`, so no changes were needed there
- Selection-filtered mode naturally ends when the user closes the viewer and deselects images before reopening
- A **Deselect** button is added to the status bar whenever images are selected, as an alternative to Escape

## Acceptance Criteria

- [ ] Selecting 2+ images and opening the viewer shows only those images in the filmstrip
- [ ] Prev/next and arrow keys navigate only through the selected images
- [ ] Counter reflects the filtered count (e.g. "1 / 3" for 3 selected images)
- [ ] Opening the viewer with 0 or 1 selected image shows all folder images
- [ ] "Deselect" button appears in the status bar when any images are selected
- [ ] Clicking "Deselect" clears all selections (equivalent to Escape)
