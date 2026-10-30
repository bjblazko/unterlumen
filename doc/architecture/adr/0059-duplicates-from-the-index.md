# ADR-0059: Duplicates found from the index

*Last modified: 2026-10-30*

## Status

Accepted.

## Context

The owner asked for a search for duplicates over everything in the
libraries: in one library and across them. It should tell three cases apart
where that is simple: the exact same file, the same picture scaled, and a
very similar one. Every version in a group must show its pixels, bytes,
format, file name and path, and the versions not wanted are marked for
deletion.

When asked, the owner chose:
- **Near-identical copies only.** Bursts stay out.
- **Marks go to the existing Marked for deletion.**
- **The largest version is named, but nothing is marked on its own.**
- **A place of its own in the sidebar.**

The index already holds what is needed:
- **`photos.id`** is the file's SHA-256.
- **`path_cache`** has a row for every path of a content.
- **`photo_appearance.dhash`** is a 64-bit difference hash of the thumbnail (ADR-0044).
- **`exif_index`** holds the pixel size and `SubSecTimeOriginal`.

## Decision

1. **Nothing is measured again.** `Manager.Duplicates` reads each library's
   paths with their size, format, time taken, sub-second, pixel size and
   dHash. That is one query per library, with joins on primary keys and no
   correlated subquery (ADR-0045). The result is merged in memory. A path in
   two libraries that overlap is one file, not a duplicate. Videos are left
   out.
2. **Three kinds of group** (`internal/library/duplicates*.go`):
   - **Identical**: one SHA-256 at two or more paths.
   - **Same picture**: dHashes at most 2 bits apart, the same shape (long
     side over short, so an EXIF turn still matches), and the same moment.
     A version is *scaled* when it has fewer pixels than the largest, *saved
     again* when it has as many.
   - **Similar**: dHashes at most 10 bits apart and taken in the same
     second, with the same fraction of a second where both have one. An
     edit, an export or a crop keeps its time.
3. **A burst stays apart.** On the owner's real libraries (50 000 photos),
   linking by the hash alone joined:
   - the shots of a burst, seconds apart;
   - a whole photo booth series in front of one wall.

   So the same picture also needs the same moment, unless one of the two
   has no time. A picture without a time is linked only to its closest
   dated one: a scaled copy of one burst shot must not join the burst
   together. In one second, the fraction tells the shots of a burst apart
   (400 against 650). Exports often shorten it (640 to 64), so fractions
   are compared to the digits both have. A second shared by more than 20
   pictures is a date without its time and links nothing.
4. **Near pairs are found by bands, not by comparing everything.** The hash
   is split into four bands of 16 bits; two hashes 3 bits or fewer apart
   agree in one of them. Similar pictures are compared only within their
   second. The groups are closed with a union-find. All of this takes
   30 ms for 50 000 photos.
5. **The files decide what is shown.** Each version in a group is checked
   on disk, and its pixel size is read from the file's header. After an
   edit made elsewhere, EXIF can keep the old size: a cropped fixture still
   said 7728 × 5152. EXIF stays only for a format Go cannot read. On a
   network share each look takes about 8 ms, so 16 run at once.
6. **A mark names a path, and deletion goes by path.** Deleting by photo ID
   removes the file at `path_hint` and drops the row. For two copies in one
   library, that can be the wrong copy. So a version marked in Duplicates is
   kept in Marked for deletion with `byPath`, and deleted through
   `DELETE /api/library/{id}/file?path=`:
   - It deletes only a path the library holds.
   - It deletes the sidecar only when no other file shares it (a RAW beside
     its JPEG).
   - It leaves the rest to `Manager.ForgetFiles`, which keeps the other copy
     in the index.
7. **No cache.** On the owner's libraries the search takes about 2 s
   locally and about 6 s with the NAS. The place shows that through
   Activity, and it is opened rarely. A cache would have to follow every
   scan, analysis and deletion. Each visit reads again, so a file deleted
   meanwhile is never offered.
8. **Desk only.** Deciding what to delete is culling, as in Marked for
   deletion. The place is not among the phone's places.

## Consequences

- A photo whose look is not measured yet takes part only as an identical
  file. The place says how many there are.
- The thresholds are constants in `duplicates_link.go`. They were chosen on
  copies of the owner's libraries with the probe in
  `probe_duplicates_test.go` (`go test -tags probe`). On those libraries the
  search found 866 identical, 676 same-picture and 110 similar groups.
- A copy cropped to another shape and saved without its EXIF time is not
  found. Neither the hash and shape nor the time join it.
- In the installed Mac app the search over the NAS takes about a minute,
  where the same code run from a terminal takes 13 s. Almost all of it is
  opening files on the share. A program started as an app bundle reached
  the share far more slowly than the same program from a terminal (400
  files: 0.8 s against more than two minutes). It is not the background
  band: `taskpolicy -b/-B` did not change the app's speed. This is open,
  and it slows everything the Mac app reads from the NAS. Each version is
  opened once, never looked at twice.
- Marks made in Duplicates are kept by absolute path. Marks made in a
  library pane are kept by the path inside the library. The same file
  marked in both places is two entries in Marked for deletion.
