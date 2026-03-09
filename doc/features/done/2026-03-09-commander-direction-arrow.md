# Commander Direction Arrow

*Last modified: 2026-03-09*

## Summary

Replace per-button direction arrows and selection counts with a large background SVG arrow in the center actions panel that flips to indicate which pane is the source.

## Details

Previously the Copy/Move buttons showed `→`/`←` and a selection count (e.g. `Copy → (3)`) to indicate operation direction. This cluttered the button labels and duplicated information already implied by the active pane highlight.

The new approach:

- **Buttons** show plain labels: `Copy`, `Move`, `Delete` (icon + text, no arrow, no count).
- **Direction arrow** — a large near-triangular SVG arrow (short shaft, wide arrowhead) is absolutely positioned as a background element inside the center actions panel. It is accent-colored at 10% opacity so it is visible but unobtrusive.
- **Flips on pane switch** — when the right pane is active (source), the arrow points left via a `scaleX(-1)` CSS transform with a 0.2s ease transition. When the left pane is active it points right (default).
- The arrow updates whenever the active pane changes (Tab key, pane click, focus change).

## Acceptance Criteria

- [x] Center panel shows a large right-pointing arrow when the left pane is active
- [x] Arrow flips to point left when the right pane is active
- [x] Arrow transitions smoothly (0.2s ease) on pane switch
- [x] Copy/Move/Delete button labels show no arrow and no count
- [x] Copy/Move operations still work correctly in both directions
- [x] Selecting files still enables the buttons
