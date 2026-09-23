# Rebuildable site state: one state file per album

*Last modified: 2026-09-23*

## Summary

A website destination's album list lives in exactly one file, `site.json`,
inside that installation's output directory. Nothing can reconstruct it — not
the album folders on disk, not the XMP sidecars. Lose that file, or build the
same site from a second installation that never had it, and the regenerated
index page and sitemap silently contain only the albums the local `site.json`
happens to know.

Write a small state file per album folder, so `site.json` becomes a cache that
can be rebuilt from what is on disk rather than the single source of truth.

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

### The shape of the fix

Write an `album.json` next to each album's `index.html`, carrying what
`site.json` stores for that album today: `postID`, `slug`, `title`,
`publishedAt`, `updatedAt`, `photoCount`, `coverFile`, `hasZip`, `unlisted`,
`generatedAt`, `deployedAt` and the photo list. `site.json` keeps its role as
the fast path; a "Rebuild album list" action (Destinations → Advanced) scans
the album folders and rewrites `site.json` from them, and a site build that
finds no `site.json` at all reconstructs it rather than starting empty.

Open questions for the design:

- Does a build reconcile automatically when `site.json` knows fewer albums
  than the folder holds, or only on request? Automatic is friendlier; on
  request is honest about touching a file the user did not ask about.
- What happens to an album folder whose `album.json` is missing (published
  before this change)? Probably: list it with what the folder reveals and mark
  it as incomplete rather than dropping it.
- Should the reconstruction also consult the XMP sidecars, which hold the
  publication record per photo ([ADR-0027](../../architecture/adr/0027-per-album-publication-meta-keys.md))?
  They know which album a photo belongs to, but not the album's title or date.

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

- [ ] Each album folder of a site destination carries an `album.json` with
      everything `site.json` records for it, written on every build.
- [ ] A site build whose `site.json` is missing or incomplete reconstructs the
      album list from the album folders instead of publishing an index that
      drops albums.
- [ ] Destinations → Advanced offers "Rebuild album list", which reports how
      many albums it found and what it changed.
- [ ] An album folder without an `album.json` (published before this change)
      is listed rather than dropped, and says what could not be read.
- [ ] Publishing album A from one installation and album B from another leaves
      an index and a sitemap containing both — covered by a Go test that
      simulates the two output directories.
- [ ] The two-installation setup is described in the README, including which
      files are shared (`channels.json`, XMP sidecars next to the photos) and
      which are per machine (`-lib-dir`, generated output).
- [ ] CHANGELOG entry under Unreleased.
