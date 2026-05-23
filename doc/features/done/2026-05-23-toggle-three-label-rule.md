# Toggle Three-Label Rule

*Last modified: 2026-05-23*

## Summary

Audit all binary on/off controls in the UI and enforce a three-label rule: every toggle slider must display (1) a purpose label, (2) a visible ON-state label, and (3) a visible OFF-state label — so state is readable without depending on colour alone.

## Details

### Rule

Every binary toggle slider in Unterlumen must have three visible labels:

1. **Purpose label** — what the control does (can be an external `dropdown-label` sibling or the first `.toggle-label` inside the button)
2. **ON-state label** (`.toggle-label-on`) — "ON" by default, or a contextual word (e.g. "High", "3D", "35mm")
3. **OFF-state label** (`.toggle-label-off`) — "OFF" by default, or a contextual word (e.g. "Standard", "2D", "Native")

### Changes made

**Fixed — non-conforming toggles missing state labels:**
- Library list "Filter" toggle (`lib-search-btn`) — added ON/OFF labels
- Library detail "Filter" toggle (`lib-filter-btn`) — added ON/OFF labels
- Statistics film simulation toggle — relabeled "Include untagged"; added ON/OFF labels
- Settings "Interface" label — renamed to "Show interface" for clarity

**Converted — binary controls not yet using the toggle slider component:**
- Slideshow modal loop: `<input type="checkbox">` "Repeat endlessly" → `Toggle.create()` (Loop section label provides purpose)
- Library filter focal length: `<input type="checkbox">` "35mm equivalent" → `Toggle.create({ labelOn: '35mm', labelOff: 'Native' })`
- Statistics focal length chart: Native/35mm button group → `Toggle.create({ labelOn: '35mm', labelOff: 'Native' })`
- Info panel map view: 2D/3D button group → `Toggle.create({ labelOn: '3D', labelOff: '2D' })` + separate "Open" button
- Settings thumbnail quality: Standard/High button group → `Toggle.create({ labelOn: 'High', labelOff: 'Standard' })`

**Multi-option selectors (3+ choices) are not affected** — Layout (Grid/Justified/List), Theme (Light/Auto/Dark), Export format, Slideshow transition/display, Stats granularity stay as button groups.

### `Toggle.create()` extended

`toggle.js` now accepts `labelOn` and `labelOff` options (default "ON"/"OFF"). All existing callers are unaffected.

## Acceptance Criteria

- [x] Every toggle slider has `.toggle-label-on` and `.toggle-label-off` children
- [x] `Toggle.create()` accepts `labelOn` / `labelOff` options
- [x] Library Filter toggles show "FILTER ON [◉] OFF"
- [x] Settings shows "THUMBNAIL QUALITY HIGH [◉] STANDARD" (was a button group)
- [x] Settings label "Interface" renamed to "Show interface"
- [x] Slideshow Loop section shows toggle slider (was checkbox)
- [x] Stats film simulation toggle reads "Include untagged ON [◉] OFF"
- [x] Stats focal length shows Native/35mm toggle (was button group)
- [x] Info panel map shows 2D/3D toggle + separate Open button (was 3-button group)
- [x] Deployed and verified visually
