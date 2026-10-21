# Select all and Select none wherever a selection is made

*Last modified: 2026-10-21*

## Summary

One component, `SelectAll` (`src/web/js/select-all.js`), puts Select all,
Select none and the count of what is selected beside every collection
whose items can be selected several at a time.

## Details

- Shows nothing while there is nothing to select; each button only while it
  would change something.
- `BrowsePane` puts it where the count was, at the right end of the
  controls row: Folders, Organize, a library, the filter's results. Select
  all takes the photos, as ⌘A does; Select none also the folders.
- `FaceGrid` puts it above the faces: a person's page and a group waiting
  for a name. The selection is by position, so it may span libraries; the
  person page groups it by library for every change and names a library
  where the change failed.
- Marked for deletion puts it under its note; the file count moved there
  from the title.
- Small checkbox lists (the libraries in Rename, the multi-select dropdown,
  which has All) are not collections and keep what they have.

## Acceptance Criteria

- [x] Folders, Organize, library, filter results: Select all / Select none
- [x] Marked for deletion: Select all / Select none
- [x] Person page and group page: Select all / Select none
- [x] A person's selection spans libraries; changes go to each library
- [x] e2e: `select-all.spec.js`, `people-everyone.spec.js`
