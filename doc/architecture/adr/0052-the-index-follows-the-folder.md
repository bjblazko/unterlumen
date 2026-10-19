# ADR-0052: The index follows the folder; a scan only reads it

*Last modified: 2026-10-19*

## Status

Accepted. Generalises [ADR-0048](0048-notes-live-in-the-sidecar.md) (notes
live in the sidecar, the index copies them) to everything a library knows
about a photo, and serves [ADR-0047](0047-independent-libraries-shared-per-library.md)
(two installations, each with its own index of a shared folder).

## Context

On 2026-10-19 the owner fixed the location of a photo taken in Hagen that
said Austin, Texas. The file was right; the Map still showed Texas after a
rescan, on the installation that made the change. Looking for the cause
turned up a class of faults, not one:

- **A file changed in place stayed in the index twice.** A photo's ID is the
  SHA-256 of its file. Setting a location or cropping rewrites the file, so a
  scan found new content and added a photo — and kept the old row, with the
  old EXIF, because removing photos only asked whether the old row's path
  still existed. It did: it is the same path. No scan, reindex or cleanup
  ever took the old row away.
- **Publications only ever arrived.** A scan added the `built:` keys of every
  publication in the sidecar but never removed one the sidecar no longer had,
  so a gallery left on one installation stayed claimed on the other.
- **A share that was not mounted emptied the index.** The walk of a library's
  folder skipped what it could not read, the folder itself included, and
  reported no error. A full index or a cleanup of a library whose NAS was not
  connected then saw no files and removed every photo.
- **The other installation learnt of changes only when someone scanned.**
  Nothing scanned on its own; the background scan existed but nothing
  started it.
- An index of the owner's carried 3 753 photos left marked missing by an
  older version, none of whose files exist, and 2 015 paths no file answers to.

The owner's question was broader than the fault: with two installations
changing the same library and scanning at different times, can a wrong order
destroy data?

## Decision

1. **Two rules, for every library, shared or not.**
   - *A scan only reads the photo folder.* It never writes a file or a
     sidecar; it only changes the index. So the order in which installations
     scan cannot destroy anything: an index can be out of date, and the next
     scan corrects it.
   - *A change writes only itself,* into the file or into the sidecar as it
     is on disk at that moment — a field, a publication, a location — never
     the index's whole idea of the photo. So a change made on the other
     installation in between is kept. Later writes to the same value win, as
     for any file.

2. **The index becomes exactly what the folder says, removals included.**
   Notes already did (ADR-0048); publications now do too
   (`publicationKeys`, `Store.replaceBuiltMeta`). A key that already holds its
   value is not written again.

3. **A file changed in place is one photo with its new content.** When a scan
   finds new content at a path the index knew with other content
   (`Indexer.retire`), the old row goes with its EXIF, thumbnail and
   measurements, which belonged to the old content, and the new content is
   indexed in full. Unless the old content is still at another path — a
   byte-identical copy — which then keeps it. What only the index knew moves
   over where the sidecar does not already say it: the "in a draft" markers,
   and notes in a library not yet marked `notes_in_sidecar`. The index
   remembers the old ID in `photo_replaced`, so a gallery draft or the shared
   album register naming it finds the photo (`Store.CurrentPhotoID`, used by
   `GetPhoto`, `GetPhotoPathHint`, `GetPhotoThumbPath` and publishing); the
   shared files are not rewritten by a scan.

4. **A photo leaves only when a complete look at the folder misses it.**
   `Indexer.settle` runs once at the end of a full index or a cleanup and
   decides by every path the index knows a photo by: a photo none of whose
   paths has a file leaves; a path whose file now belongs to other content no
   longer keeps the old photo; paths no file answers to are forgotten; a photo
   found at another of its paths takes that one. Settling is skipped when the
   walk was not complete: the library's folder is not there (a share not
   mounted), something below it could not be read (`errIncomplete`), or the
   library's folder shows no files at all while the library knows photos. The
   run then says that nothing was taken out of the library. A folder inside a
   reachable library that was deleted counts as empty. What Unterlumen
   deletes or moves away itself it takes out directly (`Manager.ForgetFiles`):
   it knows which file is gone, so even the last photo of a library leaves,
   and a photo moved within the library stays at its new path.

5. **Shared libraries are checked on their own.** At start, and then every 30
   minutes, each shared library on this installation is indexed against its
   folder (`Manager.KeepSharedLibrariesCurrent`): new and changed files are
   read, sidecars applied, gone photos removed. It shows in the status line as
   "Checking …" and leaves a library that is already being scanned alone. A
   library that is not shared changes only through this installation and its
   owner's scans.

6. **A scan looks at each file once.** The walk notes every sidecar it
   passes, so a photo without one costs no further question to the disk — on
   a network share each is a round trip. A photo whose row is already right
   is not written. Measured on 3 218 photos on the NAS from a Mac over SMB,
   nothing changed: 53 s before, 19 s after with a cold cache, 4.5 s warm.
7. **A sidecar is replaced whole, never rewritten in place.** Every sidecar
   write goes through `media.changeSidecar` (`atomicfile.Change`): it reads the sidecar, makes the
   change from what is there now, writes the result beside it under a hidden
   name and renames it over the sidecar. A scan on the other installation
   reads the whole old or the whole new file; rewriting in place left an
   empty or half file for a moment, which a scan took for "no names" and a
   writer on the other installation could write back, losing the rest of the
   sidecar. Just before the rename it reads the sidecar again; if it changed,
   it starts over from the new content, so a change the other installation
   made meanwhile stays. There is no lock: locks across SMB and NFS between a
   Mac and a Linux container are unreliable, and a lock file left by a killed
   app blocks for good. Two installations writing the same photo's sidecar in
   the same microseconds can still lose one change, never the file.

## Consequences

- The owner's Austin photo, and every photo edited in place, is listed once,
  with the file's data, after the next full index or check. The first check
  of each shared library with this version also removes the rows earlier
  versions left; libraries that are not shared are cleaned by Rebuild or a
  cleanup.
- A thumbnail, measurements and faces of an edited photo are made anew: they
  describe the pixels, which may have changed (a crop).
- Deleting every photo of a library outside Unterlumen leaves them in
  the index until one is back or the library is removed: an empty library
  folder is indistinguishable from a share that is not there.
- Hiding faces, and anything else that only one installation knows, stays on
  that installation by design: the sidecar carries only what is true of the
  photo (who is in it), not what one installation decided to ignore.
- Checking a large shared library from a Mac over SMB takes minutes every
  half hour; on the NAS's own disk it takes seconds.

## Alternatives considered

- **Identify a photo by its path, or by a hash of its pixels only.** A path
  breaks on every rename, the case content hashing was chosen for; a pixel
  hash would survive a location fix but not a crop, and needs a decoder per
  format, RAW included. Rejected; the replacement is recorded instead.
- **Rewrite the shared drafts and album register on a replacement.** Keeps
  them current without a lookup, but makes a scan write shared files — the
  first rule. Rejected.
- **A lock file beside each sidecar, or `flock`.** Not honoured across SMB
  from a Mac and NFS from a container alike, and a lock left by a killed app
  needs a timeout guess to break. Rejected for replace-and-recheck (rule 7).
- **Watch the folder for changes.** File system events do not cross SMB or
  NFS reliably, and not at all from the other machine. Rejected for a
  periodic check.
