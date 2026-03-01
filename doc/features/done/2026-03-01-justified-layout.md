# Justified Layout View

*Last modified: 2026-03-01*

## Summary

A third browse layout ("Justified") that scales images to fill each row edge-to-edge while preserving aspect ratios, with 1px gaps between photos. Similar to Flickr, Google Photos, or Lightroom Cloud.

## Details

- Frontend-only implementation — no backend changes required.
- Aspect ratios are read from `img.naturalWidth / img.naturalHeight` after thumbnail load, with a 3:2 default to minimize reflow.
- Linear partitioning algorithm: walk items left-to-right accumulating widths, close a row when accumulated width exceeds container width, compute row height as `(containerWidth - gaps) / sumOfAspectRatios`.
- Last row keeps target height (not stretched) to avoid oversized images.
- Layout recalculates on window resize (debounced via `requestAnimationFrame`).
- Uses `flex-wrap: wrap` with `gap: 1px` for spacing.
- Selection and focus use inset `box-shadow` to avoid layout shifts.
- Chunked rendering and IntersectionObserver work the same as grid/list views.
- Keyboard navigation uses variable column count per row.
- Directories are rendered in the standard grid style above the justified images for better visibility and identification.

## Acceptance Criteria

- [x] "Justified" button appears in the View menu Layout section between Grid and List
- [x] Images fill rows edge-to-edge with 1px gaps, preserving aspect ratios
- [x] Last row is not stretched
- [x] Layout reflows on window resize
- [x] Selection (click, Ctrl+click, Shift+click) works with inset highlight
- [x] Keyboard navigation (arrows, Enter, Space) works correctly
- [x] Chunked loading appends and re-layouts on scroll
- [x] Focus and marked-for-deletion states display correctly
- [x] Directories render in the standard grid style above justified images
- [x] Justified is the default view mode
