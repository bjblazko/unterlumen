*Last modified: 2026-06-20*

# Feature: Website About, Imprint & Contact Pages

## Summary

Extends multi-album website channels with personal identity pages: an About page, a Legal/Imprint page, and global contact links in the footer of every site page. The channel settings dialog is reorganized into tabs to keep the growing number of options manageable.

## Details

### Channel settings tabs

The channel editor form is split into four tabs:

- **Export** — format, quality, scale, EXIF mode, export mode (standard / single gallery / multi-album site)
- **Website** — all site-specific settings: title, theme, URL, about text, imprint text, contact info, author photo
- **Output** — output mode (save / download), custom output folder
- **Advanced** — handler, handler config key-value pairs, named accounts

### About page (`about.html`)

Stored as Markdown text in the channel config (`siteAbout` field). Rendered to HTML using [goldmark](https://github.com/yuin/goldmark) at publish or rebuild time — visitors see static HTML with no JavaScript required for content display.

An optional **author portrait** can be uploaded via the Website tab. The image is stored as `site/assets/avatar.jpg` and displayed as a circular portrait next to the About text.

### Legal / Imprint page (`legal.html`)

Stored as Markdown text (`siteImprint` field). Generated the same way as the About page. Required by law in many countries for commercial or semi-commercial websites.

### Contact info in footer

Two optional fields:

- **Contact email** (`siteContactEmail`) — rendered as `mailto:` link
- **Website / social URL** (`siteContactURL`) — rendered as external link

Both appear in the `<footer>` of every site page (root index, album pages, about, imprint). Album pages link back via `../../about.html` and `../../legal.html`.

### Navigation

When About or Imprint pages are configured, a `<nav>` element with links is added to the root `index.html` header. Album pages link to these pages from their footer.

### Markdown rendering

Uses `github.com/yuin/goldmark` with GitHub Flavored Markdown extensions and HTML passthrough enabled. No client-side markdown parser is needed — content is baked into the static HTML at publish/rebuild time.

## Acceptance Criteria

- [x] Channel settings dialog shows four tabs: Export / Website / Output / Advanced
- [x] Website tab is accessible regardless of export mode; shows a hint when export mode is not "Multi-album site"
- [x] `siteAbout`, `siteImprint`, `siteContactEmail`, `siteContactURL` fields save to and load from channel JSON
- [x] Publishing to a site-mode channel generates `site/about.html` when `siteAbout` is non-empty
- [x] Publishing to a site-mode channel generates `site/legal.html` when `siteImprint` is non-empty
- [x] Markdown headings, paragraphs, links, lists render correctly in about.html and legal.html
- [x] Avatar upload stores file at `site/assets/avatar.jpg`; about.html includes the portrait
- [x] Avatar remove deletes the file; about.html renders without portrait after rebuild
- [x] Root `index.html` header shows "About" and "Legal" nav links when those pages exist
- [x] Album pages footer shows About/Legal links and contact info when configured
- [x] "Rebuild site" regenerates about.html and legal.html from current channel config
- [x] Pages with no avatar or no about text skip the respective elements (no broken img tags)
- [x] All e2e tests pass
