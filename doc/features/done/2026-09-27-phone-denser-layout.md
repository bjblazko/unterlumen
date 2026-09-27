# A denser phone layout

*Last modified: 2026-09-27*

## Summary

On a phone (Android, Firefox) one photo filled the width, the margins were 16px, and below the three places of the tab bar there was a wide empty band. The owner asked on 2026-09-27 for thinner margins, less space under the tab bar, and two photo tiles side by side.

## Details

- **Two photos per row.** The justified layout's row height (`justifiedRowHeight` in `browse-justified.js`) shrinks with the width below 700px, so two landscape (3:2) photos share a row; portraits come three or four to a row and grow taller. It never grows past the configured 200px, so the desktop is unchanged. The same renderer serves Folders and a library's folders.
- **Thin margins.** The photo area keeps 8px at the sides (was 16px); the heads of Libraries, a library, Galleries and Destinations keep 12px.
- **The tab bar.** Its places are 44px high (the touch-target minimum, was 48px) with a 24px icon. The system's bottom inset now replaces the bar's 4px bottom padding instead of adding to it — Firefox on Android, drawn edge to edge, reports one for the gesture bar, which is the likely source of the empty band. The bar's height is one custom property, `--tabbar-h`, that `#app` and the desktop-only notice use.
- **Visible height.** `body` and `#app` use `100dvh`, the height actually visible while the browser's own bar slides in and out, with `100vh` as the fallback.

## Acceptance Criteria

- [x] On a 390px-wide phone two landscape photos share a row.
- [x] The photo area's side margin is 8px.
- [x] The tab bar is at most 56px high where there is no system inset.
- [x] The bottom inset is not added on top of the bar's padding.
- [x] `phone.spec.js` checks the row, the margin and the bar height.
- [ ] Confirmed on the owner's Android phone in Firefox.
