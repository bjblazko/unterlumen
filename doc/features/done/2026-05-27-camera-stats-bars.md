# Camera Stats Bars

*Last modified: 2026-05-27*

## Summary

The CAMERA section in the library folder info panel now shows proportional horizontal bars instead of plain text rows, making it easy to compare shot counts across camera/lens combinations at a glance.

## Details

Previously each camera × lens entry was rendered as a plain text row (`count × label`). The new layout uses a two-line block per entry:

- **Line 1:** count label (`Nx`) left of a proportional orange bar
- **Line 2:** camera/lens string indented flush with the bar's left edge

The bar width is computed as a percentage of the highest count in the set (same approach as the Formats bar chart). Long lens strings wrap naturally without affecting the bar layout.

**Changed files:**
- `src/web/js/infopanel.js` — `_renderLibraryStats` camera section replaced with bar chart rendering
- `src/web/css/style.css` — new `.folder-cam-*` component classes added after `.folder-type-*` block

## Acceptance Criteria

- [x] Each camera/lens entry shows a horizontal bar proportional to its count
- [x] Count label appears to the left of the bar
- [x] Camera/lens name appears below the bar, indented to align with the bar's left edge
- [x] Long camera/lens strings wrap without disrupting bar layout
- [x] Bars use the same accent colour and opacity as the Formats bars
- [x] FORMATS section layout is unchanged
