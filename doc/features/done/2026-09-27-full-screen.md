# Full screen for photos

*Last modified: 2026-09-27*

## Summary

On a phone the slideshow and the viewer still showed the browser's address bar and the system bars. Browsers offer a real full screen for a page (the Fullscreen API); the slideshow now enters it when it starts, and the viewer has a button for it.

## Details

- `Fullscreen` (`src/web/js/fullscreen.js`) wraps the API: `available()`, `active()`, `enter()` and `exit()`. The whole page goes full screen, so dialogs and menus opened on top still show.
- Browsers allow full screen only in answer to a tap or click. Starting a slideshow is that tap, so `App.openSlideshow` enters full screen there and its Close leaves it.
- The viewer has a button beside the counter, an icon with the name "Full screen" or "Leave full screen" and `aria-pressed`. It follows `fullscreenchange`, so a full screen left with Escape or the phone's back gesture shows as left.
- Whoever enters full screen leaves it: `enter()` answers whether this call entered it, and the slideshow and the viewer exit only a full screen they entered. Leaving the viewer leaves the full screen it entered.
- Safari on an iPhone allows full screen only for video: `available()` is false there, the viewer shows no button, and the slideshow starts in the window as before.
- A web app manifest (`src/web/manifest.json`, `display: standalone`) with 192 and 512px icons drawn from the logo: added to a home screen, the whole app opens without the browser's bars. `theme-color` follows light and dark. Browsers install it as an app only from a secure origin (HTTPS or localhost); over plain HTTP on a LAN address, "Add to home screen" makes a shortcut that opens in the browser.

## Acceptance criteria

- [x] A slideshow starts in full screen where the browser offers it, and Close leaves it.
- [x] The viewer has a full screen button with an accessible name that says what a press does.
- [x] The button's state holds when moving to another photo and follows a full screen left by the browser.
- [x] Leaving the viewer leaves the full screen the viewer entered.
- [x] No button where the browser offers no full screen.
- [x] A manifest opens the app standalone from the home screen, with icons that load.
- [x] e2e: `fullscreen.spec.js`, `home-screen.spec.js`.
