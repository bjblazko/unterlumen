# Published Galleries Overview

*Last modified: 2026-09-05*

## Summary

A new top-level "Published" tab aggregates every published gallery across every export channel — both single-gallery and multi-album site exports — into one table, with an automatic live reachability check for galleries whose public URL is known.

## Details

### The problem

Each channel tracks its own published galleries independently (`site.json` for site-export channels, `gallery.json` per gallery-export channel), and the only existing view was the per-channel "Albums" popup (`channels.js`'s `_showAlbums`, via `GET /api/channels/{slug}/galleries`). There was no way to see everything ever published in one place, and no way to notice a previously-deployed gallery had gone dark since — a successful past deploy (`LastDeployOK: true`) says nothing about whether the site is still reachable now.

### Aggregate endpoint

`GET /api/channels/galleries` iterates every configured channel, loads its published galleries via the same `collectGalleryItems` helper the per-channel endpoint uses (factored out of `listGalleries`), and merges them into one flat, newest-first list. Each row (`PublishedGallery`) carries the channel's slug/name and a resolved public URL when computable: the channel's `SiteURL`, or — for `rsync`-handler channels without one — a best-effort guess of `https://<rsync host>` (flagged via `urlGuessed`). A broken or unreadable statefile on one channel is skipped rather than failing the whole request.

### Live reachability check

`POST /api/channels/galleries/reachability` accepts the exact `{channelSlug, postID, url}` targets the client is currently displaying and streams one Server-Sent Event per completed `HEAD` request, bounded to 8 concurrent requests with a 5-second per-request timeout, ending with a `{"complete":true}` sentinel. This follows the codebase's existing SSE idiom (`fetch()` + manual `data:` parsing, as used by publish/build progress) rather than the browser `EventSource` API. Rows without a resolvable URL are never probed and show a static "No URL configured"/"—" status instead.

### Frontend

A new `#mode-published` tab (keyboard shortcut `5`) mounts `PublishedGalleriesPane`, which renders the table, then automatically starts the reachability sweep on load — each row's status cell updates independently (pending → Live / Unreachable) as its SSE event arrives, without blocking the rest of the table.

See [ADR-0024](../architecture/adr/0024-published-galleries-overview.md) for the full architectural rationale, and [Unlisted galleries](2026-08-02-unlisted-galleries.md) for the Unlisted flag surfaced in this table.

### Edit and delete (added 2026-09-05)

Each row has **Edit** and **Delete** actions:

- **Edit** renames the gallery's title only (`PATCH /api/channels/{slug}/galleries/{postID}`). The album's Slug/URL never changes — Unlisted galleries stay immutable by design (see [Unlisted galleries](2026-08-02-unlisted-galleries.md)), since changing the slug would break any link already shared. Renaming a site-export album regenerates that album's page and the site index/sitemap (via the shared `rebuildSiteChannel` helper, factored out of the existing "Rebuild site" handler); renaming a gallery-export gallery regenerates just its own `index.html`.
- **Delete** (`DELETE /api/channels/{slug}/galleries/{postID}`) always removes the statefile entry and the local output folder. It optionally also attempts a **remote delete over SSH** (`deploy.DeleteRemote`, opt-in via `{"deleteRemote": true}` in the request body) for `rsync`-handler channels — this is a new capability, since `deploy.Deploy` deliberately never uses rsync `--delete` (multiple channels can share one `RemotePath`, and `--delete` would let deploying one wipe another's content). Remote delete instead runs a scoped `rm -rf` for exactly one album's subfolder, with the subpath validated against `..`/absolute-path traversal before it's ever placed in a remote shell command. A failed remote delete is reported back but does not undo the local delete.

### Channel list links (added 2026-09-05)

The channel row (in the Channels settings list, `channels.js`) gained two links, for any channel with `siteExport`/`galleryExport` and/or a resolvable URL:

- **Visit site** — opens the channel's resolved public URL (`SiteURL`, or the guessed rsync-host URL) in a new tab.
- **Published** — switches to the Published tab pre-filtered to just that channel's rows (`App.showPublishedForChannel`), with a "Show all channels" control to clear the filter.

## Acceptance Criteria

- [x] `GET /api/channels/galleries` returns a merged, newest-first list across all channels
- [x] Rows include both `SiteExport` and `GalleryExport` channels
- [x] A broken statefile on one channel does not fail the aggregate request
- [x] URL resolution prefers `SiteURL`, falls back to a guessed rsync-host URL, and is empty otherwise
- [x] `POST /api/channels/galleries/reachability` streams one result per target plus a final `complete` event, bounded to 8 concurrent requests
- [x] The "Published" tab renders the unified table and kicks off the reachability check automatically on load
- [x] Rows with no resolvable URL are excluded from the reachability sweep and show a static status
- [x] Go unit tests cover the aggregate endpoint, URL resolution, and the reachability sweep (including its concurrency bound)
- [x] e2e coverage exercises both channel types end-to-end, including a deliberately unreachable URL resolving to "Unreachable" in the UI
- [x] Edit renames a gallery's title and regenerates its HTML (and site index/sitemap for site-export) without changing its slug/URL
- [x] Delete removes the statefile entry and local folder for both channel types
- [x] Delete's optional remote-delete never uses a bare/unscoped path and rejects `..`/absolute-path traversal in the target subpath
- [x] Delete's remote-delete option is only offered/attempted for `rsync`-handler channels; other handlers report a clear error without touching the local delete's success
- [x] Channel row's "Visit site" link and "Published" (filtered) link are covered by e2e tests
