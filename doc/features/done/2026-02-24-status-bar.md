# Status Bar: Image Count & Selection Status

*Last modified: 2026-02-24*

## Summary

A persistent status indicator in the controls row of every Browse and Commander pane showing how many images are in the current directory and how many are currently selected.

## Details

- Rendered as a `<span class="status-bar">` inside `.controls`, positioned to the right via `justify-content: space-between`.
- Text format: `"N images"` when nothing is selected; `"N images · M selected"` when one or more images are selected.
- Computed in `renderControls()` using `getImageEntries().length` and `this.selected.size`.
- Updated live (without full re-render) in `updateSelectionClasses()`, which is called on every ctrl+click, shift+click, and keyboard selection change.
- Applies automatically to Browse mode and both panes of Commander mode because they all use `BrowsePane`.
- Styled with `font-size: 11px`, `color: var(--text-sec)`, `letter-spacing: 0.02em` — unobtrusive and consistent with the Dieter Rams palette.

## Acceptance Criteria

- [x] Navigating to a folder shows e.g. "12 images" in the controls bar.
- [x] Ctrl+clicking images updates the count live to e.g. "12 images · 3 selected".
- [x] Shift+click range selection updates the count.
- [x] Switching between Grid and List views preserves the count.
- [x] Both Commander panes each show their own independent status bar.
- [x] Navigating to a sub-folder resets the count to the new folder's image count.
- [x] No count shown for directories (only images counted).
