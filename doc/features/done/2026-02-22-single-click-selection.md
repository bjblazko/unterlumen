# Single-Click Image Selection

*Last modified: 2026-02-22*

## Summary

Clicking an image in the grid or list view selects it with a visual highlight (orange border), clearing any previous selection. Previously, a regular click only recorded the click index internally with no visual feedback.

## Details

The regular click handler in `web/js/browse.js` was updated to clear the current selection set, add the clicked image, re-render the view, and fire the selection change callback. This completes the selection model:

| Action | Behavior |
|--------|----------|
| Click | Clear selection, select clicked image |
| Ctrl/Cmd+Click | Toggle clicked image in selection |
| Shift+Click | Range select from last click |
| Double-click | Open image viewer |

No CSS changes were needed — the existing `.grid-item.selected` and `.list-view tr.selected` styles already apply the orange accent border/highlight.

## Acceptance Criteria

- [x] Single click on an image selects it with orange border
- [x] Single click on a different image deselects previous, selects new
- [x] Ctrl/Cmd+click multi-select still works
- [x] Shift+click range select still works
- [x] Double-click still opens the image viewer
- [x] Works in both grid and list views
- [x] Commander mode selection count updates correctly
