# ADR-0024: Published Galleries Overview

*Last modified: 2026-09-05*

## Status

Accepted

## Context

Publishing is per-channel and per-album: each `SiteExport` channel tracks its published albums in `site.json`, and each `GalleryExport` channel tracks one `gallery.json` per subfolder. The only existing view of "what's published" is `GET /api/channels/{slug}/galleries`, scoped to a single channel, surfaced in the frontend as a per-channel "Albums" popup (`channels.js`'s `_showAlbums`). There is no way to see everything ever published across every channel in one place, and no way to notice that a previously deployed gallery has since gone dark — DNS change, deleted remote directory, expired certificate, a rsync target that moved — without opening each channel's Albums popup individually and clicking through every link. A successful `LastDeployOK: true` on the channel only proves the rsync transfer succeeded at that moment; it says nothing about whether the site is still reachable afterward.

## Decision

Add a read-only aggregate endpoint, `GET /api/channels/galleries`, that iterates every configured channel and merges its `site.json`/`gallery.json`-derived rows (via the same `collectGalleryItems` helper the existing per-channel endpoint uses) into one flat, newest-first `[]PublishedGallery` list — computed on every request directly from the statefiles, with no new persisted cross-channel index. Each row carries a resolved public URL when one is computable: the channel's `SiteURL`, or (for `rsync`-handler channels without one) a best-effort guess of `https://<rsync host>`.

Pair this with an opt-in, client-initiated reachability sweep, `POST /api/channels/galleries/reachability`, that accepts the exact list of `{channelSlug, postID, url}` targets the client is currently displaying and streams one Server-Sent Event per completed HEAD-request result, bounded to 8 concurrent requests with a 5-second per-request timeout, terminated by a `{"complete":true}` sentinel. This mirrors the one existing streaming idiom in the codebase (`fetch()` + manual `data:` line parsing, used by `buildStream` for publish/build progress) rather than the browser `EventSource` API, which nothing else here uses.

A new top-level "Published" tab (`#mode-published`, `PublishedGalleriesPane`) renders the aggregate table and kicks off the reachability sweep automatically on load, updating each row's status independently as its SSE event arrives rather than blocking the table on the slowest target.

## Consequences

**Positive:**
- A single place to audit everything ever published, across every channel, without opening each channel's config individually.
- The reachability check surfaces silent breakage that `LastDeployOK` cannot: a deploy can succeed and the site can still go unreachable afterward for reasons entirely outside that deploy (DNS, TLS, a deleted remote directory, a changed rsync target on a later, unrelated deploy).
- Purely additive: no `channels.json`, `site.json`, or `gallery.json` schema changes; `collectGalleryItems` is a behavior-preserving extraction shared by the existing per-channel endpoint and the new aggregate one.
- No new persisted state — the overview is always current because it reads the statefiles directly on each request, unlike the cached `LastDeployedAt`/`LastDeployOK` fields on `Channel`.

**Negative / Trade-offs:**
- The reachability sweep makes outbound HTTP requests, initiated by the server process, to user-configured (or guessed) third-party URLs. In a network-restricted or air-gapped container deployment this is server-initiated egress per published gallery — worth flagging for anyone running Unterlumen that way.
- The 5-second timeout × 8-way concurrency bound means a very large published-gallery collection can still take noticeable wall-clock time to finish a full sweep; the UI treats this as expected (rows update progressively, no blocking spinner over the whole table) rather than as a problem to eliminate.
- URL-resolution logic now exists in two places — a new Go port (`resolveGalleryURL`) for the aggregate endpoint and reachability targets, and the pre-existing JS version (`_deployBaseURL` in `channels.js`) for the per-channel Albums popup — until a follow-up has the Albums popup consume the new endpoint's resolved `url`/`urlGuessed` fields instead of recomputing them client-side.
