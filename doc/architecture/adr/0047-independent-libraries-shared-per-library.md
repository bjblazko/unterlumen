# ADR-0047: Libraries are independent folders; sharing is per library

*Last modified: 2026-10-01*

## Status

Accepted. Supersedes the photo folder of
[ADR-0042](0042-installation-one-line-setup-in-the-browser.md) (§3, and §4 for
the installed app). Amends [ADR-0035](0035-shared-album-register.md): drafts now
name a shared library by the same ID on every installation.

## Context

The installed app asked for one *photo folder* (`photosDir` in config.json),
which did three jobs at once:

- it fenced Folders, Marked and Organize: it became the browse boundary, so a
  camera card at `/Volumes/SDCARD` could not be reached when the photo folder
  was `/Volumes/nas/Bilder`;
- every library had to lie inside it;
- its `.unterlumen-shared` was how a NAS and a Mac found the same destinations.

That super-root was the hardest part of the model to explain, and it did not fit
the hybrid setup the owner runs: two installations that have *some* libraries in
common, not all. Library indexes were never shared anyway. Each installation
keeps its own `library.db` under its own `-lib-dir`, and the two indexes of the
same folder had nothing in common but the photos' content hashes. Library IDs
were random per installation, so a gallery draft (`DraftPhoto.LibraryID`) named
a library the other installation did not know.

Decided with the owner on 2026-10-01.

## Decision

1. **No photo folder.** The installed app starts browsing in the home folder with
   the whole disk as its boundary, as a development run without a folder
   argument always did. Folders reaches every mounted disk. A library names its
   own folder, anywhere. A server keeps `UNTERLUMEN_ROOT_PATH` as its fence, and
   development and e2e runs keep their folder argument; that is deployment, not
   a setting in the app.

2. **The setup chooses the shared folder and the data folder.** Destinations and
   galleries are not bound to a library (a website mixes libraries), so they
   stay shared per installation, through one *shared folder* chosen in the setup
   (`#setup`, linked from Settings). Choosing a folder uses its
   `.unterlumen-shared`, making it if it is not there and copying this
   installation's destinations into it (`installation.Share`); choosing a
   `.unterlumen-shared` itself uses it. A disk root is refused. In the
   installed app the shared folder is never found by convention: its channels
   directory is always set (the data folder when not shared), so a
   `.unterlumen-shared` in the home folder is never picked up by accident.
   Servers and folder-argument runs keep the convention.

3. **A shared library carries a marker.** Sharing a library (Edit library →
   Share with other installations) writes `.unterlumen-library.json` into its
   folder: `{id, name, description}`, no path, because the same folder has a
   different path on every machine. Unsharing removes it. A library that is not
   shared leaves no trace in its folder. Each installation keeps its own index
   and thumbnails; SQLite is never opened over the network.

4. **The other installation gets the same library.** New library on a folder
   with a marker adds the library under the marker's ID, name and description.
   A library of that folder made before it was shared shows an offer in the
   library ("Another installation shares this folder as …"). *Join* renames the
   library's data folder to the marker's ID and takes its name and description;
   the index and thumbnails stay, and this installation's drafts are pointed at
   the new ID (`DraftStore.RekeyLibrary`). Nothing joins without a click.

5. **The marker is the shared name.** Renaming a shared library rewrites the
   marker. Every installation takes the marker's name and description when it
   lists the library, and stores them, so a rename on one shows on the other.

6. **What only the folder knows is read only for the library list and detail.**
   `Manager.Annotate` fills `missing` (the folder is not reachable, such as a
   NAS that is not mounted), `shared` and `joinOffer`. It reads the disk, so it
   stays out of `ListLibraries`, which thumbnail and path lookups call on every
   request. A missing folder is said in the library instead of keeping the app
   from starting, which is what a missing photo folder used to do.

## Migration

`installation.Migrate` runs at every start of the installed app. A config.json
with `photosDir` loses it. If no channels directory was chosen and the photo
folder has a `.unterlumen-shared`, that becomes the shared folder, so
destinations keep working on both machines without a click. A photo folder
that is not there (a NAS not mounted) is left for a later start; migrating then
would lose its shared folder. Libraries need nothing: `source_path` was always
absolute. Two installations with their own library of the same folder join per
library: share it on one, then *Join* on the other.

## Consequences

- The installed app's boundary is `/`, so paths in the API are the absolute
  path without its leading slash. Paths a browser remembers relative to the
  old photo folder (the last folder in Folders) point elsewhere once.
- The setup no longer runs on a first start; there is nothing it must know.
- An installation that adds a shared folder reads all its photos once for its
  own index. Sharing the index itself would avoid that but needs a single
  writer over SMB or NFS, which SQLite does not give.
- A marker on a read-only folder cannot be written; sharing then says why.
- Two libraries on one installation may still cover the same folder. Only one
  can carry the marker.

## Alternatives considered

- **Share the index in the library's folder.** No second scan, ratings and
  meta at once on both machines. Two writers on a network share corrupt SQLite.
  Rejected.
- **Destinations per library.** A website could no longer mix libraries.
  Rejected.
- **Find the shared folder through the shared libraries' markers.** No setting,
  but undefined when two shared libraries point at different shared folders.
  Rejected.
- **A marker in every library's folder.** Lets any installation recognise any
  library, but writes into folders (cards, read-only mounts) the owner never
  meant to share. Rejected.
