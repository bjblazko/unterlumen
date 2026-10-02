# The full view's ⋯ menu

*Last modified: 2026-10-02*

## Summary

A photo opened from the Map, Statistics or the Timeline lay in a read-only
full view: none of what a library's overview does to a selection could be done
to it. The full view now has a ⋯ menu with those actions for the photo on
screen, and Show in library, which opens the library at the photo's folder
with the photo selected.

## Details

- `ViewerMenu` (`src/web/js/viewer-menu.js`) builds the items with `Menu`.
  The viewer takes `libraryRef: (key) => { lib, id }`; library panes, filter
  results, the Map's and Statistics' photo column and the Timeline pass it.
- A library photo: Add to gallery…, Export…, Rename…, Set location…, Show in
  Organize, Show in library. Its path is read from the library when an action
  is chosen and turned into a path under the browse boundary.
- A photo in Folders or Organize: Export…, Rename…, Set location…, Show in
  Organize (not from Organize itself).
- Download and Delete stay buttons. Marking for deletion is not offered from
  the Map, Statistics or the Timeline, which look at photos rather than cull
  them.
- A rename closes the full view: the photo is gone under its old name, and
  the place it came from shows the new one.
- While a dialog or the menu is open over the photo, the full view takes no
  keys, so Escape closes the dialog and not the photo.
- Fixed with it: a library's Rename… and Set location… sent paths relative to
  the library's folder where the server expects paths under the served folder
  (`App._boundaryPaths`); right only when the two were the same folder.

## Acceptance Criteria

- [x] A photo from the Map, Statistics or the Timeline has the library actions in ⋯.
- [x] Show in library opens the library's subfolder with the photo selected and in view.
- [x] A photo in Folders has Export, Rename, Set location and Show in Organize.
- [x] Escape in a dialog opened from the menu leaves the photo open.
- [x] Set location from a library in a subfolder of the served folder reaches the right file.
- [x] e2e (`viewer-menu`).
