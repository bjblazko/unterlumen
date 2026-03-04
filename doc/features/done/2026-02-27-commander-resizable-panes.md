# Commander Resizable Panes

*Last modified: 2026-03-04*

## Summary

In Commander (dual-pane) mode, add a draggable vertical divider between the left file pane and the center action buttons so users can freely adjust the width of each pane.

## Details

- A thin `<div class="commander-resizer">` element is inserted between `.left-pane` and `.commander-actions` in the Commander DOM template.
- On `mousedown` the resizer captures the current widths of both panes, switches them from `flex: 1` to explicit pixel widths, and tracks mouse movement to resize them in real time.
- Neither pane can be dragged below a minimum of 100px.
- On `mouseup` the split ratio (left-width / total-width) is saved to `localStorage` under the key `commander-split` and the drag state is cleaned up.
- On Commander initialisation the saved ratio is restored via `requestAnimationFrame` to ensure layout dimensions are available.
- A `destroy()` method removes all event listeners so no stale handlers remain when the user switches away from Commander mode.
- Visually the handle is 5px wide with a 13px hit-target (via `::before` pseudo-element). It turns orange (`var(--accent)`) on hover and while dragging.

## Acceptance Criteria

- [x] A thin vertical strip is visible between the left pane and the action buttons in Commander mode.
- [x] Hovering over the strip changes the cursor to `col-resize` and the strip colour to orange.
- [x] Dragging left/right resizes both panes smoothly without either pane collapsing below 100px.
- [x] Releasing the mouse ends the drag; cursor and user-select are restored.
- [x] Refreshing the page restores the previous split ratio.
- [x] Switching to Browse mode and back produces no JS errors and no stuck `col-resize` cursor.
- [x] `go vet ./...` passes with no errors.
