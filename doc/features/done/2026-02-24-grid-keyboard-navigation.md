# Grid Keyboard Navigation

*Last modified: 2026-02-24*

## Summary

Add keyboard navigation to the grid and list views so users can browse without a mouse. Arrow keys move a visual focus indicator through items; Enter activates the focused item; Space toggles selection.

## Details

- A `focusedIndex` tracks which item has keyboard focus (separate from selection).
- The focused item receives a visual highlight: an orange outline (`outline: 2px solid var(--accent)`) in grid view and an inset left border with accent background in list view.
- Focus resets to the first item whenever a directory is loaded (including sort changes).
- Clicking any item (folder or image) syncs the focus indicator to that item.
- Arrow keys:
  - **Left/Right** — move focus by one item linearly.
  - **Up/Down** — move focus by one row (the current column count, measured by comparing item top positions).
  - Focus clamps at the first/last item; no wrapping.
- **Enter** — activates the focused item: navigates into a folder or opens the fullscreen viewer for an image.
- **Space** — toggles selection of the focused image (does nothing on folders). Does not move focus.
- All navigation is suppressed while the fullscreen viewer is open, so arrow keys continue to serve prev/next image navigation.
- Works in both Browse mode (single pane) and File Manager mode (active pane via `Commander.getActivePane()`).
- Input elements (INPUT, SELECT, TEXTAREA) are excluded from key capture to avoid interfering with the sort dropdown.

## Acceptance Criteria

- [ ] Arrow keys move the orange focus outline through items in grid view
- [ ] Up/Down jump by the measured column count (not a fixed number)
- [ ] Focus clamps at boundaries (no wrap)
- [ ] Focus resets to the first item on every directory load
- [ ] Clicking an item moves the focus indicator to it
- [ ] Enter on a folder navigates into it (focus resets to first item)
- [ ] Enter on an image opens the fullscreen viewer
- [ ] Space toggles selection of the focused image without moving focus
- [ ] Arrow keys in the viewer navigate prev/next image (grid navigation suppressed)
- [ ] List view shows the accent left-border focus indicator on the focused row
- [ ] Works in File Manager mode (active pane receives keyboard focus)
