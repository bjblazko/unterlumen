# Duplicates

*Last modified: 2026-10-30*

## Summary

A place under Cull, `#duplicates`, that finds the versions of one photo
anywhere in the libraries, side by side, so the ones not wanted can be
marked for deletion. See
[ADR-0059](../../architecture/adr/0059-duplicates-from-the-index.md).

## Details

- **Three kinds of group**:
  - **Identical**: the same file at several paths.
  - **Same picture**: scaled, or saved again at the same size.
  - **Similar**: an edited, exported or cropped copy taken in the same
    moment.

  Bursts stay apart.
- **One group** is a row of its versions, the largest first ("Largest").
  Each version shows:
  - its relation to the largest (Identical file, Scaled, Saved again,
    Similar);
  - its pixels, file size and format;
  - its file name and folder;
  - its libraries, when it is in more than one.
- **Show** in the head: All · Identical · Same picture · Similar.
- **Mark for deletion** on a version puts it into Marked for deletion, by
  its path. It is deleted there, and only that copy goes.
- **Opening**: clicking a thumbnail opens the group in the viewer.
- **Not analysed yet**: the sentence under the title counts the photos
  whose look is not measured yet.

## Acceptance Criteria

- [x] Identical files are found in one library and across libraries
- [x] One path in two overlapping libraries is not a duplicate
- [x] A scaled copy and an edited copy of the same moment are found; a burst is not
- [x] Each version shows its pixels, file size, format, name and path
- [x] The largest version comes first and is named so
- [x] Marking a version adds it to Marked for deletion; deleting it there removes only that copy
- [x] The place reads the libraries again on every visit
- [x] e2e spec `e2e/specs/duplicates.spec.js`
