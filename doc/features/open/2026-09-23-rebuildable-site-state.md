# Rebuildable site state: a shared album register

*Last modified: 2026-09-25*

## Summary

A website destination's album list lives in exactly one file, `site.json`,
inside that installation's output directory. Nothing can reconstruct it — not
the album folders on disk, not the XMP sidecars. Lose that file, or build the
same site from a second installation that never had it, and the regenerated
index page and sitemap silently contain only the albums the local `site.json`
happens to know.

Move the album list to the shared channel directory, one file per album, and
let each photo carry its own album membership, so `site.json` becomes a cache
that can be rebuilt rather than the single source of truth.

## Details

### Why this comes up

Unterlumen supports two installations against the same photos — a Docker/Podman
instance on the NAS and a native install on the Mac — sharing `channels.json`
through `-channels-dir` while `-lib-dir` (database, thumbnails, search index)
stays per machine ([ADR-0023](../../architecture/adr/0023-shared-channel-config-directory.md)).
Generated output deliberately stays per machine too, since it is working
output rather than configuration.

For a **share-links destination** that works out fine: each gallery is its own
folder with its own `gallery.json`, there is no shared index, and rsync runs
without `--delete` on purpose (several destinations may share one remote path,
see `rsyncArgs` in `src/internal/deploy/rsync.go`). Publishing gallery A from
the NAS and gallery B from the Mac leaves both online, both links working. The
machine that lacks a gallery simply cannot rebuild or unpublish it.

For a **website destination** it does not. `site.json` holds every album —
title, date, slug, cover, photo count *and the full photo list*, so album pages
can be rebuilt without re-exporting. Publishing album B from a machine whose
`site.json` never saw album A produces an `index.html` and a `sitemap.xml`
listing only B. The upload overwrites both. Album A's files survive (no
`--delete`), so its direct link keeps working, but it is gone from the site's
navigation and from search engines until the other machine publishes again —
which then drops B the same way.

`rebuildSite` (`src/internal/api/library/handler.go`) regenerates assets and
HTML *from* `site.json`, so it cannot help here. An album folder currently
holds `index.html`, `cover.jpg`, `thumbs/`, `photos.zip` and the image files —
everything except the facts needed to list it.

### The strategy (decided 2026-09-25)

The output directory becomes **disposable**. The truth moves to where both
installations can see it, and — as a fallback — into the photos themselves.
Re-converting on a rebuild is accepted: a website should be reconstructible at
any time as long as the source photos still exist, and moving or renaming
those photos has to stay allowed.

**Layer 1 — the album register lives in the shared channel directory.**
`-channels-dir` is already shared (ADR-0023). Albums move there as **one file
per album**, `albums/<channel>/<postID>.json`, carrying what `site.json`
records today: title, slug, `publishedAt`, `updatedAt`, `photoCount`,
`coverFile`, `hasZip`, `unlisted`, `generatedAt`, `deployedAt` and the photo
list. One file per album rather than one list for all, so two installations
never write the same file — albums are independent of each other, and the
index page is derived, not stored.

`site.json` loses its special status and becomes a build cache in the output
directory: every build writes `index.html` and `sitemap.xml` from the
register, not from whatever the local machine happens to know. That alone
removes the reported problem, with no reconstruction involved.

`outputPath` stays empty in the shared `channels.json`, so each installation
uses its own default under `-lib-dir`. A path belongs to one machine and has
no business in shared configuration.

**Layer 2 — each photo carries its own membership**, so the register itself is
rebuildable and "reconstructible from the source photos" is literally true.
Most of this exists: ADR-0027 writes `built:<slug>:<postID>` with a timestamp
and `:title` into the XMP sidecar. What is missing is the album's **slug**
(and `unlisted`). Without it a reconstruction would have to mint a new slug,
which changes a URL that was already shared — slugs must never be derived.
With it, "Rebuild album list" can walk the library and write the register from
the photos.

### Two things that have to be right first

**The sidecar has to travel with the photo.** Neither `batchrename` nor
`fileops` touched the `.xmp`; the only place outside `media/xmp.go` that knew
about sidecars was a delete. A renamed or moved photo lost its publication
history silently and left an orphan behind. Fixed 2026-09-25 with
`media.CarrySidecar`, which rides along with copy, move, rename and both
passes of the batch rename — the two passes matter, because a set of names
being permuted would otherwise make the sidecars collide exactly where the
photos do not. (The library itself was never at risk: it identifies photos by
content hash.)

**Deleting an album needs a tombstone.** As soon as the register can be
rebuilt from the photos, a deleted album would rise again on the next rebuild.
Deleting has to leave a record in the shared register *and* clear the meta
keys in the member photos' sidecars.

### Order of work

1. ~~The sidecar follows the file on copy, move and rename~~ (done
   2026-09-25).
2. The album register moves to the shared channel directory, one file per
   album; `site.json` is demoted to a cache; the build reads the register.
3. Slug and `unlisted` go into the sidecar as well; "Rebuild album list" in
   Destinations → Advanced.
4. Deleting writes a tombstone and clears the sidecars.

Steps 1 and 2 together solve the concrete problem. Steps 3 and 4 are the price
of "only the source photos have to survive" being true.

### The related path problem

While building the Destinations screen (phase 6 of the Rams redesign) a second
copy of this "state lives in one machine's output directory" problem turned up:

- A destination's output folder is picked with the folder browser, which
  returns paths **relative to the browse root** (`Users/you/Pictures/…`).
  `Store.OutputDir` handed that string straight to the filesystem, where it was
  resolved against the server process's working directory. Started from `/` it
  was right by accident; started from anywhere else it pointed at a directory
  that does not exist, and every gallery of that destination silently vanished
  from the Galleries overview. Fixed: relative paths now resolve against the
  browse root, saving normalises to an absolute path, and the API reports the
  directory actually in use as `outputDir`.
- What remains is that `outputPath` sits in the **shared** `channels.json`
  while describing a folder on **one** machine. `/Users/blazko/Pictures/…` does
  not exist inside the NAS container, and a path relative to the container's
  browse root is wrong on the Mac. Existing entries are deliberately not
  rewritten for that reason.

Both point the same way: the album list should be recoverable from the files
themselves, so it does not matter which machine holds which output directory.
A destination whose `outputPath` is left empty (each installation uses its own
default under `-lib-dir`) is the configuration this feature should make safe.

## Acceptance Criteria

- [x] A photo's XMP sidecar is carried along by copy, move, rename and batch
      rename, including when a batch permutes a set of names.
- [ ] A site destination's albums are stored one file per album in the shared
      channel directory, written on every build.
- [ ] A build writes the index and the sitemap from that register, not from
      the local `site.json`, which becomes a cache.
- [ ] Publishing album A from one installation and album B from another leaves
      an index and a sitemap containing both — covered by a Go test that
      simulates the two output directories.
- [ ] The sidecar also records the album's slug and `unlisted`, and
      Destinations → Advanced offers "Rebuild album list", which reports how
      many albums it found and what it changed.
- [ ] An album folder or photo from before this change is listed rather than
      dropped, and says what could not be read.
- [ ] Deleting an album leaves a tombstone in the register and clears the meta
      keys in its photos' sidecars, so a rebuild does not resurrect it.
- [ ] The two-installation setup is described in the README, including which
      files are shared (`channels.json`, the album register, XMP sidecars next
      to the photos) and which are per machine (`-lib-dir`, generated output).
- [ ] CHANGELOG entry under Unreleased.
