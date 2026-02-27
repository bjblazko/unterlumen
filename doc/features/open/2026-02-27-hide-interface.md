# Hide Interface Toggle

*Last modified: 2026-02-27*

## Summary

Add an `H` key toggle (and Settings menu item) that hides all chrome — header bar, info sidebar, and viewer toolbar — for distraction-free photo viewing and real fullscreen use.

## Details

- Pressing `H` anywhere (including inside the viewer) toggles interface visibility.
- When hidden, a brief toast hint appears: "Press H to show the interface again".
- A "Hide Interface (H)" button in the Settings dropdown provides a discoverability path.
- State persists across reloads via `localStorage` (`ui-hidden` key).
- Implementation: CSS class `ui-hidden` on `<body>` hides `.header`, `.info-panel`, and `.viewer-toolbar`.

## Acceptance Criteria

- [ ] Press `H` in browse/commander/wastebin mode → header and info panel disappear; hint shows for ~3 seconds
- [ ] Press `H` again → interface reappears
- [ ] Open Settings → "Hide Interface (H)" button is present; clicking it toggles and closes the menu
- [ ] Open viewer → press `H` → viewer toolbar disappears; hint appears
- [ ] Close viewer (Esc) → interface hidden state persists as set
- [ ] Reload page → hidden state restored from localStorage; hint shown if hidden
