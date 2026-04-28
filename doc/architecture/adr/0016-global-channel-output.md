# ADR-0016: Global Channel Output Directory

*Last modified: 2026-04-28*

## Status

Accepted

## Context

Channels are publish destinations (Instagram, Mastodon, Website, custom) that define export format, quality, scale, and EXIF settings. Their configuration has always been stored globally in `~/.unterlumen/channels.json`, shared across all libraries.

However, the export output — the actual JPEG/PNG/WebP files, ZIP archives, gallery HTML, and site statefiles — was written into each library's own directory: `~/.unterlumen/libraries/<libID>/channels/<slug>/`. This meant:

- Publishing the same photo from two libraries to the same channel created two separate, unrelated output directories.
- A site-export channel's `site/index.html` and `site.json` statefile were per-library, so albums from different libraries could never appear in the same published website.
- The UI entry point for channel settings was only accessible from within an open library, even though the configuration itself was not library-specific.

## Decision

Channel export output is now written to a global directory: `~/.unterlumen/channels/<slug>/`. This is derived from the channels store's base directory (the parent of `channels.json`), exposed via `Store.OutputDir(slug)`.

Publishing still requires a library context to read photo files and EXIF data, but the output location is independent of any library. The `POST /api/library/{id}/publish` endpoint remains unchanged; only the `channelDir` construction inside it changes.

The rebuild-site endpoint moves from `POST /api/library/{id}/channels/{channel}/rebuild-site` to `POST /api/channels/{slug}/rebuild-site`, removing the library-ID requirement that was only needed to locate the (now-global) site directory.

## Consequences

**Positive:**
- Albums from any library can be published to the same site-export channel and appear together in one unified website.
- Channel settings and path navigation (copy path, reveal in Files) are accessible from the library list view without opening a specific library.
- The `Site.json` statefile is shared across libraries, so `Rebuild site` regenerates the full cross-library site index.

**Negative / Trade-offs:**
- Existing per-library channel output (`~/.unterlumen/libraries/<id>/channels/`) is not migrated; old exports remain in place but are no longer updated.
- The `Open in Commander` UI feature only works when the browse root (`-root` / `UNTERLUMEN_ROOT_PATH`) is an ancestor of `~/.unterlumen/`. In a typical dev setup (`~/Pictures`), the channel output directory is outside the boundary and the feature falls back to a descriptive alert.
