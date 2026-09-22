# Many galleries per single-gallery channel

*Last modified: 2026-09-22*

## Summary

Make a `galleryExport` channel work as designed: **one channel = one host**,
holding **many unrelated, unlisted albums**, each under its own unguessable
24-hex URL, each independently publishable, editable and deletable, with every
photo tagged by both channel and album.

Until now several code paths assumed one gallery per channel, so the only
workable pattern was one channel per album — all pointing at the same host.

## Details

### The mode contract

| | Multi-album site (`siteExport`) | Single gallery (`galleryExport`) |
| --- | --- | --- |
| Purpose | a real, growing website | sharing one album by link |
| Albums | related, navigable from an index | unrelated, no index anywhere |
| URL | human-readable slug | random 24-hex `postID` |
| Discoverable | yes, sitemap + index | no — unguessable URL + `noindex` |
| Unlisted | fixed at creation (slug encodes it) | default on, toggleable after publish |
| `robots.txt` | written | never written (channels may share a host) |

### What changed

- **Album identity** — membership is recorded per album (`built:<slug>:<postID>`),
  not just per channel. See [ADR-0027](../../architecture/adr/0027-per-album-publication-meta-keys.md).
- **Info Panel** shows one Publications card per album, so a photo published to
  two albums of one channel lists both instead of only the newest.
- **Reindex / "Rebuild metadata"** keeps every album a sidecar records, instead
  of collapsing to the most recent publication per channel.
- **Correct album ID** on add-to-existing: the publication, the XMP sidecar and
  the API response all name the album actually written to.
- **Resume after partial failure** — a draft is pinned to the album it produced,
  so retrying the photos that failed completes that album instead of building a
  second one with the same title.
- **noindex for single-gallery albums** via an Unlisted checkbox that now also
  appears for gallery channels (default on), editable afterwards from the
  Published tab.
- **Base URL** is configurable for gallery channels, not just site channels, so
  the Published tab shows real links; draft rows show the target address rather
  than "No URL configured".
- **Distinct rows per pending album** — draft rows carry a `rowKey`, so two
  albums pending in one channel are no longer indistinguishable in the UI.
- **One gallery per collect**, even when the selection spans several libraries
  (it previously created one gallery per library).
- **No silent degradation** — if a channel's existing galleries fail to load,
  the collect dialog says so instead of quietly offering only "New gallery".
- **`drafts.json` is no longer deployed** — it sat at the channel output root
  and would be served at `<host>/drafts.json`.
- **Deploy status survives a settings save** (it was wiped by the settings form).
- **No duplicate photos** when the same selection is collected twice.

### Out of scope

No migration of existing one-album-per-channel setups, and no "move gallery to
another channel" action — their live links must keep working untouched. No
`dc:subject`/IPTC keyword writing; publication data stays in the private `ul:`
XMP namespace.

## Acceptance Criteria

- [x] Two albums can be collected and published in one gallery channel, each in
      its own `postID` folder with its own `gallery.json`
- [x] Adding photos to an existing album creates no second folder, and the
      complete event reports the real album ID
- [x] A photo in two albums of one channel carries both albums' qualified meta
      keys, and both survive a reindex
- [x] The XMP sidecar records one publication per album with the real `postID`
- [x] An unlisted single-gallery page emits `noindex, nofollow`; a listed one
      emits no robots tag; no `robots.txt` is written for gallery channels
- [x] Unlisted can be toggled after publish for gallery albums, and is refused
      for site albums
- [x] Two pending drafts in one channel produce two rows with distinct row keys
- [x] Draft rows show the channel's target URL instead of "No URL configured"
- [x] `rsyncArgs` excludes `drafts.json`
- [x] Saving channel settings preserves the last-deploy status
- [x] Collecting a cross-library selection into a new gallery creates one draft
- [x] e2e coverage for the two-album flow, add-to-existing, per-album tagging
      across a reindex, the noindex tag and two distinct pending rows
      (`e2e/specs/gallery-channel-multi-album.spec.js`)
