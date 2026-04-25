*Last modified: 2026-04-25*

# Feature: Static Website Gallery Export

## Summary

When publishing photos to a gallery-capable channel, Unterlumen generates a self-contained `index.html` alongside the exported photos. The result is a complete folder the user can copy to any web host (`scp`, rsync, etc.) without any server-side software.

Each publish creates a new subfolder (named by post ID) under the channel's output directory. The folder is self-contained: HTML, CSS, and all images — no CDN, no external resources.

---

## Details

### HTML Generation

The gallery uses **static HTML with native `loading="lazy"`** on every `<img>` element:

- All image tags are visible in the HTML source → full SEO benefit (no JavaScript execution required by crawlers).
- Native browser lazy loading handles deferred image loading with many photos.
- Zero external dependencies, works offline.
- `width` and `height` attributes are set from the exported image dimensions to prevent layout shift (CLS).

### Output Structure

```
~/.unterlumen/libraries/<uuid>/channels/website/
  <postID>/
    index.html
    website_20260425T143000Z_IMG_1234.jpg
    website_20260425T143000Z_IMG_5678.jpg
```

### Gallery Title

The user enters a display name in the Publish modal when a gallery-capable channel is selected. The title becomes the `<h1>` heading and `<title>` of the page.

### Channel Configuration

Any channel can opt in to gallery export via the **"Generate HTML gallery on publish"** toggle in Channel Settings. The built-in **Website** preset has this enabled by default.

### Gallery Design

The gallery HTML uses its own clean, minimal CSS grid layout — entirely separate from Unterlumen's interface design. Future work: let users choose from multiple gallery themes.

---

## Future Expansion

This feature is designed to grow into a complete static website generator:
- Multiple gallery publishes could become individual pages of a site.
- A site index page linking all galleries.
- User-selectable themes.

---

## Acceptance Criteria

- [ ] Channel Settings shows "Generate HTML gallery on publish" toggle; saved to channel JSON.
- [ ] Built-in Website channel has `galleryExport: true` by default.
- [ ] Publish modal shows "Gallery title" input only when the selected channel has `galleryExport` enabled.
- [ ] Publishing to a gallery channel creates `channels/<slug>/<postID>/` subfolder with photos and `index.html`.
- [ ] `index.html` contains the gallery title as `<h1>`, all exported photos with `loading="lazy"`, correct `width`/`height` attributes.
- [ ] HTML is self-contained (no external CSS/JS/fonts).
- [ ] Publishing to a non-gallery channel writes to the existing flat `channels/<slug>/` folder (no regression).
- [ ] Toast after gallery publish shows the output folder path.
- [ ] `GenerateGallery()` unit tests pass.
