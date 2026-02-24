# View Popup Menu

*Last modified: 2026-02-22*

## Summary

Move sort controls and the Names toggle out of the inline toolbar into a "View" popup menu, reducing visual clutter in the controls bar.

## Details

The controls bar previously displayed Grid/List toggle, a Names button, and sort controls (label, select, order button) all inline. The sort and names options are infrequently used, so they are now tucked into a popup menu triggered by a "View" button with a sliders icon.

- A "View" button with an inline SVG sliders icon sits right-aligned in the controls bar
- Clicking it opens a dropdown popup menu with two sections:
  - **Show names** — toggle button (On/Off)
  - **Sort** — field select (Name/Date) and order button (ascending/descending)
- Clicking outside the menu closes it
- The menu stays open while interacting with controls inside it (via stopPropagation)
- After a state-changing action (sort change, names toggle), the menu closes as the view re-renders

## Acceptance Criteria

- [x] Grid/List toggle remains inline in the controls bar
- [x] Sort controls and Names button are removed from inline toolbar
- [x] "View" button with sliders icon appears in the controls bar
- [x] Clicking View opens a popup menu with Names toggle and Sort controls
- [x] Names toggle works correctly from within the popup
- [x] Sort field and order controls work correctly from within the popup
- [x] Clicking outside the popup closes it
- [x] Works in both browse mode and commander mode (both panes)
