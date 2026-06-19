*Last modified: 2026-06-20*

# Feature: Site Logo + Persistent Site Masthead

## Summary

Adds a persistent two-row masthead to multi-album site pages so the site identity is always visible, and introduces an optional site logo image upload.

## Details

### Persistent header

Previously, album, about, and legal pages replaced the site name entirely with the album title or page name. Now every page has a two-row header:

- **Row 1** (persistent): optional logo + site name (links back to `index.html` on sub-pages)
- **Row 2** (page-specific, sub-pages only): back arrow + current album title / "About" / "Legal Notice"

The root index continues to show the site name as a large `<h1>` (single row, no back link needed).

### Site logo

An optional logo image can be uploaded per channel (stored as `site/assets/logo.jpg`). When present, it appears left of the site name in the header on every page. Upload / remove via the channel settings Website tab, using the same multipart-file API pattern as the author avatar.

Logo URLs are relative: `assets/logo.jpg` for root-level pages, `../../assets/logo.jpg` for album pages.

### CSS

- `header` updated to `align-items: center` + `flex-wrap: wrap` (allows row 2 to wrap below row 1 on sub-pages)
- New classes: `.site-masthead` (full-width row 1 wrapper on sub-pages), `.site-brand` (logo + name flex row), `.site-logo`, `.site-name` (1rem persistent brand link), `.page-title` (1.4rem page-specific title on sub-pages)

## Acceptance Criteria

- [x] Channel settings Website tab shows a "Site logo" upload/remove section
- [x] Logo upload stores file at `site/assets/logo.jpg`
- [x] Logo remove deletes the file; rebuild shows no broken img
- [x] Root `index.html`: row 1 shows logo (if set) + site name as `<h1>`; no second row
- [x] Album `index.html`: row 1 shows logo + site name as link back to root; row 2 shows back arrow + album title
- [x] `about.html`: row 1 shows logo + site name as link back to root; row 2 shows back arrow + "About"
- [x] `legal.html`: row 1 shows logo + site name as link back to root; row 2 shows back arrow + "Legal Notice"
- [x] "Rebuild site" regenerates all pages with current logo presence and site name
- [x] Without a logo, layout is intact (no gap or broken img)
- [x] All e2e tests pass
