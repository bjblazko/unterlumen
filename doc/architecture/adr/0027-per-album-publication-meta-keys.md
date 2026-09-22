# 0027. Album identity within a channel

*Last modified: 2026-09-20*

## Status

Accepted

## Context

A single-gallery (`galleryExport`) channel is meant to be **one host holding
many unrelated albums** — each in its own unguessable 24-hex folder, shared by
link with a different group of people, deleted when it has served its purpose.
The storage layer already supported this (`drafts.json` is an array, one
`gallery.json` per album folder), but several code paths assumed at most one
gallery per channel, which made the mode unusable as designed:

- `pending:<slug>` was keyed per channel, so collecting a photo into a second
  album of the same channel silently overwrote the first album's marker.
- `built:<slug>:title` and `built:<slug>:postid` are overwritten by every
  publish to that channel. The Info Panel read only those, so a photo in two
  albums of one channel displayed whichever album was published last.
- `indexSidecar` collapsed a channel's XMP publications to the newest one, so
  re-indexing ("Rebuild metadata") destroyed album history the sidecar still
  held.
- On add-to-existing, the publication recorded a freshly minted `postID`
  instead of the album actually written to, so sidecars and meta pointed at a
  folder that was never created.

The practical consequence was that users created **one channel per album**,
all pointing at the same rsync host — which works, but defeats the mode and
multiplies channel configuration.

## Decision

**Album membership is recorded under qualified meta keys; the unqualified keys
remain as a channel-level marker.**

| Key | Meaning |
| --- | --- |
| `built:<slug>` | this photo has been published to this channel (membership marker) |
| `built:<slug>:<postID>` | this photo is in *that* album, with its publish timestamp |
| `built:<slug>:<postID>:title` | that album's title |
| `built:<slug>:title` / `:postid` | the *most recent* album — a convenience, never the source of truth |
| `pending:<slug>` | collected into some draft of this channel (value: a draft ID) |
| `pending:<slug>:<draftID>` | collected into *that* draft, with the album title as its value |

Readers prefer the qualified keys and fall back to the unqualified pair only
when a photo carries none — which is the case for data written before this
change, and for plain-export channels that have no album at all.

`Publication.PostID` is set from the resolved album, not from a pre-minted ID,
so XMP sidecars and meta always name a folder that exists. After a successful
generate the draft's `Target.PostID` is pinned to that album, so a draft left
behind by a partial failure resumes into the same album instead of creating a
second one with the same title.

**Unlisted means different things in the two modes.** For a site album the flag
is encoded in the slug (a random token appended), so it is fixed at creation —
changing it would change the URL of a link already shared. For a single-gallery
album the folder is the random `postID` either way, so Unlisted is purely the
`noindex, nofollow` meta tag and can be toggled after publish. No `robots.txt`
is written for gallery channels: several channels may share one remote path,
and one channel's `robots.txt` would clobber another's.

## Consequences

- No data migration. Existing rows stay valid and keep rendering through the
  fallback path; the four one-album-per-channel setups that motivated this
  change are unaffected.
- The channel filter (`channel=<slug>` → `MetaExists ["built:"+slug]`) keeps
  working unchanged, because the marker key is preserved.
- Album search (`album_title=`) already matched `built:%:title` via `LIKE`, so
  it now sees per-album titles for free.
- `GetMetaKeys` hides `pending:%:%` so ephemeral per-draft keys don't flood the
  tag-chip autocomplete, matching how `built:%:postid` is already hidden.
- Deleting a publication is now two-level: removing one album's key drops that
  album only, and the channel marker goes with the last one. For site channels
  removal still means "off the site entirely" — there is no per-album removal
  there.
- `drafts.json` is excluded from rsync. It sits at the channel output root, so
  a gallery channel would otherwise serve it at `<host>/drafts.json`, exposing
  unpublished filenames and the folder names of unlisted albums.

## Alternatives considered

**Drop the unqualified keys entirely and derive everything from qualified
ones.** Cleaner on paper, but `channel=` filtering would need a `LIKE
'built:<slug>%'` scan instead of an exact-match `MetaExists`, and every
existing row would need migrating. The marker is genuinely channel-scoped
information, so keeping it is not redundancy.

**Make Unlisted immutable for gallery albums too**, mirroring site albums. The
reason it is immutable there (the slug encodes it) simply does not apply, and
forgetting the checkbox would otherwise mean rebuilding the album under a new
URL.
