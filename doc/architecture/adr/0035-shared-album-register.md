# ADR-0035: The album list of a website lives in the shared channel directory

*Last modified: 2026-09-25*

## Status

Accepted. Builds on [ADR-0023](0023-shared-channel-config-directory.md)
(shared channel directory) and [ADR-0027](0027-per-album-publication-meta-keys.md)
(album identity in sidecars and meta).

## Context

A website destination's albums were listed in one file, `site.json`, in one
installation's output directory. With two installations against the same photos
(NAS container and Mac), the second one to build wrote an `index.html` and a
`sitemap.xml` that listed only the albums it knew, and the other machine's albums
dropped out of the site's navigation and out of search engines. Nothing could
reconstruct the list — not the album folders, not the sidecars.

## Decision

**The album list is a register in the shared channel directory, one file per
album; `site.json` is a cache.**

- `<channels-dir>/albums/<channel>/<postID>.json` holds one album. Two
  installations never write the same file. `channels.Store.AlbumRegisterDir`
  names the directory; it is deliberately not under `OutputDir`, which is per
  machine.
- The API is `Upsert(album)`, `Remove(postID)`, `Delete(postID)` and `List()`,
  never "save this list". A machine that wrote the albums it knows would delete
  the other machine's albums. A build writes only the albums it touched, and the
  index and sitemap are derived from `List()`.
- A record contains no machine-local path: an album names its folder by slug and
  its files relative to the site directory. Which site directory that is stays a
  per-machine question (`OutputDir`).
- `site.json` is refreshed after every change and is read as a source once only:
  a register whose directory does not exist yet adopts it. "No albums left" is not
  "never used", or a stale cache on the other installation would bring back a
  deleted album.

**Each photo carries its album's address.** The XMP sidecar records `ul:Slug` and
`ul:Unlisted` next to the channel, post ID and title. The slug is a URL that may
already be shared, so it is never derived again; an album whose sidecars have none
(published before this decision) is listed as unrestorable, not registered under a
new address. *Rebuild album list* (Destinations → Advanced) restores albums that
are missing from the register from the sidecars, leaves registered albums as they
are, and reports what it added and what it could not read.

**Deleting is deliberate and leaves a tombstone.** `Delete` writes
`<postID>.deleted` before it removes the record, refuses later writes of that
album, and the album is taken out of the sidecars of every photo reachable from
the deleting installation. The tombstone covers the photos that are not.
`Remove` (without a tombstone) is for an album that only ran out of its list
entry through pruning and may be restored.

**A destination's output folder belongs to one installation.** `outputPath` was
picked on one machine and stored in the shared `channels.json`, where it names a
directory that does not exist on the other. It now lives in `output-paths.json`
under each installation's own `-lib-dir` (slug → absolute path; an empty value
means "the default, on purpose"). `Store.List`/`Get` lay the effective value over
`Channel.OutputPath`, so callers and the UI are unchanged. `Save` never writes a
folder to the shared file and leaves a legacy value there untouched; that legacy
value counts as this installation's only where the folder exists, so an
installation can neither take another's folder away by saving nor adopt a path
that is not its own. Nothing is rewritten on upgrade.

## Migration

Existing albums live in `site.json` and their sidecars carry no address. The
first installation that reads a channel's albums after the upgrade copies its
`site.json` into the register; installations that come later find the register
and adopt nothing (see above). So the installation that really holds the
albums — the one that publishes — has to be upgraded and opened first. Then
*Rebuild album list* writes the register's address into the sidecars of the
photos of those albums, after which they are restorable from the photos like
any album published later.

## Consequences

- The output directory is disposable: anything under it can be rebuilt from the
  register and the source photos, at the price of re-converting.
- A rename changes the register only; a rebuild from nothing brings the old title
  back. Recording the title in the sidecar would close that.
- `rebuildSiteChannel` prunes photos by the library of the installation that runs
  it, and now writes that result to shared state. An installation whose library
  lacks the build meta can shrink another installation's album (an emptied album
  is only removed, not tombstoned, so a rebuild can restore it).
- `MarkDeployed` stamps every registered album, including albums whose files were
  not part of this installation's upload.

## Alternatives considered

**One `album.json` per album folder in the output directory.** Rejected: the
output is per machine, so the second machine would still not see the first
one's albums.

**One shared `albums.json`.** Rejected: two installations would write the same
file, and a whole-list write is exactly the bug.
