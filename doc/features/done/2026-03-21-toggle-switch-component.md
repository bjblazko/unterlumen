# Toggle Switch Component

*Last modified: 2026-03-21*

## Summary

Reusable iOS-style toggle switch component replacing the inconsistent mix of on/off buttons and HTML checkboxes used across the UI.

## Details

A pill-shaped track (36×20px) with a circular thumb (16px) that slides left/right. OFF state shows a gray track; ON state shows the accent orange track with the thumb translated right. "OFF" and "ON" labels flank the track in 10px uppercase, styled like old radio markings.

The component is implemented as:
- `web/js/toggle.js` — JS factory (`Toggle.create()`) that generates the DOM, handles click events, and returns a control object with `state()` and `setState()` methods.
- CSS in `web/css/style.css` — Pure CSS visuals with 0.15s transitions, hover states, and a viewer toolbar override for dark backgrounds.

Four toggles unified:
1. **Show Names** (View dropdown in browse mode)
2. **Show Details** (View dropdown in browse mode)
3. **Interface** (Settings dropdown, previously "Hide Interface" button)
4. **Film Strip** (Viewer toolbar, previously an HTML checkbox)

## Acceptance Criteria

- [x] Toggle component renders as a pill-shaped track with sliding thumb
- [x] OFF state: gray track, ON state: accent orange track
- [x] Labels "OFF" and "ON" flank the track
- [x] All 4 toggles use the same component
- [x] Film strip toggle syncs with F keyboard shortcut
- [x] Viewer toolbar toggle labels readable against dark background
- [x] Accessible: role="switch" and aria-checked attributes
