# A phone on its side

*Last modified: 2026-11-13*

## Summary

Turned sideways, a phone is wider than 700 px. Unterlumen took it for a desk:
sidebar, search label, a 73 px viewer toolbar whose "Mark for deletion"
wrapped over three lines, and the info column — the photo was 292 × 264 px on
an 844 × 390 screen, about a quarter of it. There is no H key on a phone to
hide the interface.

Now a phone stays a phone in both directions, and its viewer on its side shows
the photo alone.

## Details

- **What a phone is.** `PHONE_MEDIA` in `src/web/js/phone.js` —
  `(max-width: 700px), (max-height: 500px) and (pointer: coarse)` — with
  `onPhone()`. Every script that asked for `(max-width: 700px)` asks
  `onPhone()`; every CSS phone block carries the same query (a media query
  cannot be named in CSS). A tablet on its side is taller than 500 px and a
  low desktop window has a fine pointer, so neither becomes a phone.
- **Photos.** On a phone on its side a justified row aims at a third of the
  screen's height (`justifiedRowHeight`), so two rows stand under the head
  instead of one.
- **Viewer.** Under `(max-height: 500px) and (pointer: coarse) and
  (orientation: landscape)` the viewer is fixed over the whole screen, above
  the search and the tab bar; the toolbar and the arrows lie over the photo
  and are hidden. A tap on the photo (a touch that moves less than 10 px, not
  on a button or link) toggles `.viewer--controls`; the choice lasts while
  the viewer is open, and opening it again starts with the photo alone. The
  film strip is not shown there. Swipe and the info sheet are unchanged.
- **Not done, and why.**
  - Entering the browser's full screen on rotation: `requestFullscreen` needs
    a tap, a rotation is none; Safari on an iPhone allows it only for video.
    Where the browser offers it, the Full screen button in the toolbar stays.
  - Locking the orientation (`screen.orientation.lock`): only inside full
    screen on Android, not at all on an iPhone; nothing here needs it.
  - `viewport-fit=cover`: a 3:2 or 4:3 photo on a 390 px high screen is
    narrower than the space between the notch and the edge, so the photo
    would not grow; the tab bar and every place would need safe-area insets,
    which the phone layout deliberately has none of.

## Acceptance Criteria

- [x] A phone on its side (844 × 390, touch) shows the tab bar, not the sidebar; a tap on a photo shows no selection bar.
- [x] A justified row on a phone on its side is lower than 40 % of the screen.
- [x] The viewer on its side covers the whole screen; the toolbar is hidden until a tap and hidden again by the next.
- [x] A swipe moves to the next photo and keeps the toolbar as it was; reopening the viewer starts without it.
- [x] Info opens as a sheet from below.
- [x] e2e: `e2e/specs/phone-landscape.spec.js`.
