# Header Logo

*Last modified: 2026-02-24*

## Summary

Display the Unterlumen logo inline to the left of the "Unterlumen" title in the app header.

## Details

- `web/logo-96.png` rendered as a 24px-tall inline image inside the `<h1>` element.
- `<h1>` uses `inline-flex` + `align-items: center` to vertically centre logo and text.
- 8px gap between logo and text via `gap: calc(var(--unit) * 1)`.
- `alt="Unterlumen logo"` for accessibility.

## Acceptance Criteria

- [x] Logo appears immediately to the left of the "Unterlumen" text in the header.
- [x] Logo and text are vertically centred within the header bar.
- [x] Existing mode-switcher buttons are unaffected.
- [x] Image loads without 404.
