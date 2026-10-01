# ADR-0048: A photo's title and fields live in its sidecar; the index copies them

*Last modified: 2026-10-01*

## Status

Accepted. Builds on [ADR-0027](0027-per-album-publication-meta-keys.md)
(publication records in sidecars) and
[ADR-0047](0047-independent-libraries-shared-per-library.md) (libraries shared
between installations, each with its own index).

## Context

A library's index (`library.db`, per installation) held everything a person
wrote about a photo. The title was also written to the XMP sidecar as
`dc:title`, but every other field from the info panel lived in the index alone.
With two installations on the same photos (ADR-0047), a field set on the Mac
never reached the NAS. Removing a title did not travel either: a scan only added
what a sidecar said and never removed what it no longer said. A sidecar that
could not be written (a read-only or unmounted share) was ignored, so the index
claimed a title the sidecar did not have.

The owner's rule, 2026-10-01: the index is an index, for working fast. What it
knows comes from the photo, its EXIF and its sidecar. Changing something in
Unterlumen changes the sidecar or the file, and the index with it.

## Decision

1. **Notes are the title and the free fields.** Every meta key is a note except
   those made by publishing (`built:`, `published:`, `pending:`), which come from
   the publication records and drafts (`library.IsNoteKey`).

2. **The sidecar is where notes live.** The title stays in `dc:title`, which
   Lightroom, Capture One and others read. Free fields go into the Unterlumen
   block as `ul:Fields`, an `rdf:Bag` of `ul:Key`/`ul:Value`, beside
   `ul:Publications`, sorted by key. Writing publications keeps the fields and
   writing a field keeps the publications; other programs' blocks are kept as
   they were (`media.WriteField`, `media.ReadNotes`).

3. **Writing goes through the sidecar first.** `Store.WriteNote` writes the
   sidecar and then the index. When the sidecar cannot be written, nothing is
   changed and the info panel says why and shows the old value again. An empty
   value removes the note.

4. **The index follows the sidecar.** Every scan already reads every photo's
   sidecar; `Store.ApplyNotes` now makes the index's notes exactly what the
   sidecar says, removals included. Opening a photo's metadata reads its sidecar
   too, so a note written by the other installation shows at once, before a scan.

5. **Moving existing notes, once.** At start, before the appearance pass,
   `Manager.MoveAllNotesToSidecars` writes each library's notes that its
   sidecars lack into them, under the library's index lock. A sidecar that
   already holds a different value keeps it, since it is what other installations
   see. A photo that is no longer on disk gets no sidecar. A library is marked
   `notes_in_sidecar` only when every note could be written; until then, and for
   a library whose folder is not connected, scans only add notes and never remove
   one the index alone holds. New libraries start marked.

6. **Adding a library says what it does to the folder.** New library lists,
   without technical terms, that the photos stay where they are, are read once,
   get a small `.xmp` file beside them when titled, given a field or published,
   that Set location and Rename change the files themselves, and that removing
   the library leaves photos and sidecars alone.

## Consequences

- A title or field set on one installation is seen on the other when the photo is
  opened, and in search and filters after the next scan.
- Writing a note needs a writable photo folder. A read-only share now says so
  instead of seeming to save.
- Sidecars appear beside photos that get fields, not only beside titled or
  published ones. They are the usual XMP sidecars other programs read and write.
- Two installations writing the same note: the later write wins, as for any file.
- The owner's libraries held no fields besides one title already in its sidecar
  (checked on both installations on 2026-10-01), so the move writes nothing there.

## Alternatives considered

- **Sidecars in a separate folder under the data folder.** Leaves the photo
  folder untouched, but each installation has its own data folder, so notes
  would not be shared at all; they would also be left behind when a photo is
  moved or renamed outside Unterlumen, and need a mapping from paths that differ
  per machine. Rejected; New library says what is written instead.
- **Map fields to standard XMP (keywords, description, rating).** Readable in
  other programs, but only for those fixed fields, not for free key/value pairs.
  Left for later, field by field.
