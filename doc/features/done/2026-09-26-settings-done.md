# Settings has a way back

*Last modified: 2026-09-26*

## Summary

Settings is a place ([ADR-0033](../../architecture/adr/0033-dialogs-and-places.md)), reached from the sidebar or with the comma key. Once there, the only way on was to pick another place in the sidebar: wherever you had been, you had to find it again, and the desktop app's window has no back button. The owner asked on 2026-09-26 for a way out and chose a Done button.

## Details

- The Settings head carries a framed **Done** button. It leads back to the place Settings was opened from; opened directly (a reload on `#settings`), it leads to Folders.
- **Escape** in Settings does the same as Done.
- Done is not orange: settings take effect at once, so there is nothing to commit — Done only leaves. The screen still has no primary action.
- `App.setMode` remembers the place before Settings; `App.leaveSettings()` returns to it through `setMode`, so the address and the history move as for any other change of place.

## Acceptance Criteria

- [x] Done returns to the place Settings was opened from, and the address follows.
- [x] Escape in Settings does the same.
- [x] Opened directly, Done leads to Folders.
- [x] `navigation.spec.js` covers all three.
