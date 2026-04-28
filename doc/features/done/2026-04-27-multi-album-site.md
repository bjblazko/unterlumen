# Multi-Album Static Website

*Last modified: 2026-04-27*

## Summary

Extend the channel gallery export into a full multi-album static website. Each publish adds a new album subfolder; a local statefile tracks all albums ever published so the root index can be regenerated without the previous album files. The output folder is ready to rsync to any static host.

## Details

### Channel configuration

A channel gains new fields alongside the existing `galleryExport`:

- **Export mode** (select in channel editor):
  - *Standard* — files only (unchanged)
  - *Single gallery* — existing `galleryExport` behavior, one `index.html` per publish; defaults to dark theme
  - *Multi-album site* — new `siteExport` mode; builds an incrementally-managed website
- **Site title** — shown in the root `index.html` heading (e.g. "My Photography")
- **Default theme** — Light or Dark; visitors can always switch via the toggle button

### Output structure

```
~/.unterlumen/libraries/<id>/channels/<slug>/site/
  index.html              ← album browser (regenerated on every publish or rebuild)
  site.json               ← statefile (source of truth for all albums)
  assets/
    style.css             ← shared CSS with both light/dark themes via custom properties
    toggle.js             ← fully static theme toggle (reads default from data-default-theme)
  albums/
    <postID>/
      index.html          ← album gallery page, links to ../../assets/
      thumbs/
      cover.jpg           ← first thumbnail, used by root index card
      photo1.jpg …
      photos.zip
```

### Statefile

`site.json` persists an array of `SiteAlbum` entries: postID, title, publish date, photo count, cover filename, zip presence, and the full photo/thumb filename list. Albums are always sorted newest-first by publish date, so inserting a backdated album places it correctly in the grid.

The statefile is the only local state required. Old album folders can be deleted locally after rsyncing — the statefile remembers them and the rebuild command can regenerate the HTML from disk if needed.

### Theme system

All generated pages (single gallery, site root, site album pages) share the same `localStorage` key `ul-theme` (`"light"` or `"dark"`). Pages use opposite CSS class conventions:

| Page type | Default | Toggle class |
|---|---|---|
| Single gallery | dark | `html.theme-light` |
| Website pages | configured per channel | `html.theme-dark` |

`toggle.js` is fully static — the default is read from `data-default-theme` on `<html>`, not baked into the JS file. This means the file can be cached indefinitely and theme changes only require updating the HTML pages, not the JS. A `pageshow` listener handles browser back/forward cache restoration.

### Rebuild site

The "Rebuild site" button in the channel list regenerates — without re-exporting any photos:
1. `assets/style.css` and `assets/toggle.js`
2. Every album's `index.html` (using stored photo list or disk scan for older albums)
3. Root `index.html`

Use after changing the channel's default theme or site title.

### Rsync workflow

```bash
rsync -avz ~/.unterlumen/libraries/<id>/channels/<slug>/site/ user@host:/var/www/photos/
```

rsync transfers only changed/new files. After adding a second album, only `index.html` and the new `albums/<postID>/` subtree are sent. After a theme rebuild, only the updated `index.html` files and `assets/` are sent.

## Acceptance Criteria

- [x] Channel editor shows Export mode selector with Standard / Single gallery / Multi-album site options
- [x] Selecting Multi-album site reveals Site title and Default theme inputs
- [x] Publishing with site mode creates `site/albums/<postID>/` with `index.html`, `thumbs/`, `cover.jpg`, full-res photos, and `photos.zip`
- [x] `site/site.json` is created/updated with the new album entry (including photo list)
- [x] `site/assets/style.css` and `site/assets/toggle.js` are written on each publish
- [x] `site/index.html` is regenerated listing all known albums, newest first by publish date
- [x] Albums inserted with an older date appear in the correct position (sorted by date, not insertion order)
- [x] Publishing a second album appends to the statefile and lists both albums in the root index
- [x] Deleting local album folders does not remove them from the root index on next publish
- [x] Publish dialog shows "Album title" label when site mode channel is selected
- [x] Progress shows "Updating site index…" SSE step during site mode publish
- [x] Completion toast shows site path for site mode publishes
- [x] Theme toggle button appears on all gallery and website pages
- [x] Single gallery defaults to dark theme; website pages default to the channel-configured theme
- [x] Visitor theme preference persists via localStorage across root and all album pages
- [x] Back/forward navigation picks up the current theme preference (pageshow handler)
- [x] "Rebuild site" button in channel list regenerates all pages and assets without re-exporting photos
- [x] Rebuild works for albums published before photo list was stored (falls back to disk scan)
- [x] Changing default theme and rebuilding updates all album pages correctly
