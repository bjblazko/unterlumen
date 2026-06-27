# Generated Site — Mobile UX Improvements

*Last modified: 2026-06-27*

## Summary

The published photo-album website (site-channel mode) received a set of mobile-specific improvements to eliminate horizontal scrolling, make navigation touch-friendly, and give the lightbox a native-app full-screen feel.

## Details

### Back button — pill with touch target

The `← Back` link on album pages was restyled as a pill button (`display: inline-flex`, `min-height: 44px`, `border-radius: 14px`, card background, border). This meets the recommended 44 px minimum touch target and is visually distinct on both light and dark themes.

### Overflow menu on mobile (≤ 600 px)

On narrow screens the "Download all photos" button and theme toggle move into a `⋯` overflow dropdown:

- A `.menu-btn` trigger appears only on touch-width screens (`@media (max-width: 600px)`).
- The existing `.header-actions-inner` div (containing the download link and theme-toggle button) becomes an absolutely-positioned dropdown that opens/closes via JS.
- On desktop (> 600 px) the menu button is hidden and the items remain inline — no layout change.
- `toggle.js` is unchanged: `id="theme-toggle"` stays on the same button element.

### Full-screen lightbox with swipe navigation

The lightbox was redesigned for a native-app feel on mobile:

- `#lb-img` is now `width: 100%; height: 100%; object-fit: contain` — the image fills the entire screen in one dimension (full-width portrait, full-height landscape) with black letterboxing in the other, identical to Apple Photos.
- Background changed to pure `#000` (was 93 % opacity).
- Border-radius and box-shadow removed for a clean edge-to-edge look.
- Arrow buttons (`.lb-nav`) are hidden on touch devices via `@media (pointer: coarse)`.
- Swipe navigation added via `touchstart` / `touchend` listeners: a horizontal swipe of ≥ 40 px (dominant axis) calls `next()` or `prev()`. Works in portrait and landscape.
- `viewport-fit=cover` added to the album page's `<meta name="viewport">` tag so the lightbox reaches the edges of iPhones with a notch or Dynamic Island.
- The ✕ close button and photo counter remain always visible.
- Keyboard arrow keys and Escape still work on desktop.

### Horizontal scroll eliminated

Root cause: the footer's `.footer-contact` contained a long URL (`https://snaps-by-timo-boewing.de`) that could not shrink below its text width in a no-wrap flex layout, pushing the body wider than the viewport. On mobile Safari, `margin: 0 auto` on the body then centred the oversized content, allowing a few pixels of scroll in both directions.

Fixes applied:
- `footer` gains `flex-wrap: wrap` so items wrap gracefully on narrow screens.
- `.footer-contact` gains `flex-wrap: wrap`, `min-width: 0`, and `max-width: 100%`.
- `.footer-contact a` gains `overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 100%` — long URLs are truncated rather than forcing overflow.
- `html { overflow-x: hidden; }` added as a safety net.

## Acceptance Criteria

- [x] Back button is visually distinct, ≥ 44 px tall, correct hover colour on both themes
- [x] On ≤ 600 px viewport, only the `⋯` button is visible in the header; tapping it reveals download + theme toggle
- [x] Dropdown closes when tapping outside it
- [x] On > 600 px viewport, download button and theme toggle are visible inline; no `⋯` button
- [x] Lightbox image fills the full screen; no visible margin or frame
- [x] Portrait photo fills full width on portrait phone; landscape photo fills full width with bars top/bottom
- [x] Swipe left/right navigates to next/previous photo (no arrows needed)
- [x] Arrow buttons not visible on touch devices; visible on desktop
- [x] ✕ close button and "N / total" counter visible in lightbox on all devices
- [x] No horizontal scroll on any album page at any viewport width
- [x] Lightbox reaches screen edges on iPhone (no safe-area gap)
