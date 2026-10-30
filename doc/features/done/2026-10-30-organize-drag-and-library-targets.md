# Organize: dragging, library targets, a fixed key for Mark for deletion

*Last modified: 2026-10-30*

## Summary

Three wishes of the owner after 0.21.0, with a keyboard fix in Folders.
Recorded in the addenda of
[ADR-0032](../../architecture/adr/0032-organize-one-source-many-targets.md).

## Details

- **Mark for deletion** stands first in the targets and is always key 0; the
  folders keep 1–9.
- **Drag and drop**: photos dragged onto a target are sent there, as with its
  number key; ⌥ on dropping copies.
- **Add from a library…** opens the folder picker on its Libraries list; a
  library opens its folder, where Select takes it or a folder in it.
- **Arrow keys** move by what is on screen: Up and Down to the nearest tile
  in the row above or below, across folders and photos; Enter on a folder
  tile opens the folder the arrows moved to.

## Acceptance Criteria

- [x] Mark for deletion is the first target and keeps key 0 when targets are added
- [x] A photo dragged onto a target moves there
- [x] Add from a library… opens the picker on the libraries
- [x] Down from a folder goes to the tile below it; Up comes back
- [x] Enter opens the folder the arrow keys moved to
