# ADR-0019: Toggle sliders must carry three visible labels

*Last modified: 2026-05-23*

## Status

Accepted

## Context

Binary on/off controls that use colour or position as the sole state indicator are inaccessible to users who cannot distinguish the accent colour from the neutral colour. Unterlumen already had a `Toggle.create()` component that shows "ON" and "OFF" labels flanking the track, but several hand-rolled toggle buttons omitted those labels, and several binary boolean controls (checkboxes, two-button groups) did not use the slider component at all.

## Decision

Every binary on/off toggle slider must expose three visible labels:

1. **Purpose label** — what the control does. Can live outside the toggle element (e.g. as a `dropdown-label` sibling or a section heading) or as the leading `.toggle-label` inside a self-contained button.
2. **ON-state label** (`.toggle-label-on`) — "ON" by default; use a contextual word when the states have inherent names (e.g. "3D", "High", "35mm").
3. **OFF-state label** (`.toggle-label-off`) — "OFF" by default; analogously "2D", "Standard", "Native".

`Toggle.create()` is extended with `labelOn` / `labelOff` options (default "ON"/"OFF") to support contextual labels without forking the component.

**Multi-option selectors (3+ choices) are exempt** — button groups with three or more options are not binary toggles and are not affected by this rule.

## Consequences

- All binary boolean controls go through `Toggle.create()` or replicate its exact DOM structure.
- Hand-rolled `<button class="toggle">` elements must include both state spans; the linter/reviewer checks for `.toggle-label-on` and `.toggle-label-off`.
- The following controls were converted at the time of this decision: slideshow loop (checkbox → toggle), library-filter 35mm (checkbox → toggle), statistics focal-length (button group → toggle), info-panel map 2D/3D (button group → toggle), settings thumbnail quality (button group → toggle).
- CSS for `.info-map-controls` simplified from a connected pill-button group to a plain flex row; the `.stats-focal-toggle` box border removed since the container now wraps a standard toggle.
- `Toggle.create()` callers with no label options are unaffected (defaults remain "ON"/"OFF").
