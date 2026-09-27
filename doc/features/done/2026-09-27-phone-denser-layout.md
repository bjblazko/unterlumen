# A denser phone layout

*Last modified: 2026-09-27*

## Summary

On a phone (Android, Firefox) one photo filled the width, the margins were 16px, and below the three places of the tab bar there was a wide empty band. The owner asked on 2026-09-27 for thinner margins, less space under the tab bar, and two photo tiles side by side.

## Details

- **Two photos per row.** The justified layout's row height (`justifiedRowHeight` in `browse-justified.js`) shrinks with the width below 700px, so two landscape (3:2) photos share a row; portraits come three or four to a row and grow taller. It never grows past the configured 200px, so the desktop is unchanged. The same renderer serves Folders and a library's folders.
- **Thin margins, everywhere.** Every place keeps 8px at the sides on a phone: the photo area (was 16px), a library's folders and photos (was 24px), the list of libraries and its rows (content started at 36px), Galleries and Destinations (12–24px), the place heads, and the desktop-only notice (16px). The Details switch starts at the margin too; its 16px gap to the sort controls stayed behind when those are hidden.
- **The tab bar.** Its places are 44px high (the touch-target minimum, was 48px) with a 24px icon. Measured on the owner's phone (Firefox 156, Android 16): `env(safe-area-inset-bottom)` was 42.5px although the page already ends above the navigation bar and the app sets no `viewport-fit=cover`, so the bar was 91px high with an empty band under the places. The app no longer uses the safe-area inset anywhere — tab bar, dialog foot, viewer sheet; without `viewport-fit=cover` the browser keeps the page clear of system bars itself (Safari reports 0 there anyway). The bar is now 53px, one custom property `--tabbar-h` that `#app` and the desktop-only notice use.
- **Folder tiles.** In a library, folders are tiles; the phone rule for two to a row sat above the tile's base rule and lost to its fixed 176px, so each tile took a row. It now comes after the base rule.
- **Visible height.** `body` and `#app` use `100dvh`, the height actually visible while the browser's own bar slides in and out, with `100vh` as the fallback.

## Acceptance Criteria

- [x] On a 390px-wide phone two landscape photos share a row.
- [x] The photo area's side margin is 8px.
- [x] The tab bar is 53px high, with no safe-area inset under its places.
- [x] A library shows two folder tiles to a row.
- [x] Every place's content starts 8px from the edge.
- [x] `phone.spec.js` checks the photo row, the folder tiles, the margin and the bar.
- [x] Confirmed on the owner's Android phone in Firefox (measured: bar 53px, tiles two to a row at 8px).
