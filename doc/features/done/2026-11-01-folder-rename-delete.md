# Renaming and deleting folders

*Last modified: 2026-11-01*

## Summary

Folders can be renamed and marked for deletion like photos. Deleting a photo
takes its XMP sidecar with it. A delete that leaves a folder empty asks
whether to remove that folder too.

## Details

- **Selection bar.** `Rename…` and `Mark for deletion` work on selected
  folders (`folders: true` in `SELECTION_ACTIONS`). Rename is `oneFolder`:
  with several folders selected it is disabled and says why. A folder is
  renamed in `openFolderRenameDialog` (`folder-rename-dialog.js`). The
  dialog keeps errors such as a taken name in its note and stays open.
- **Paths.** Every file operation takes paths relative to the browse
  boundary. A library pane's paths are relative to the library's folder, so
  `LibraryPane.diskPath()` converts them, and `wastebinMeta()` gives a marked
  library folder its `diskPath`. Before this, a library folder marked with
  Backspace was sent as library-relative, which named another folder.
- **Library index.** `/api/rename` of a folder moves its photos in the index
  file by file (`movedFilePairs` → `IndexMovedFiles`), so faces, things and
  notes stay. Copying or moving a folder into a library expands it into its
  files in the same way. A folder that is, or holds, a library's folder is
  refused for rename and delete.
- **Sidecars.** `media.RemoveSidecar` removes `name.xmp` after a delete,
  unless another file with the same stem (RAW, Live Photo video) still owns
  it. `/api/delete` and both library delete endpoints use it.
- **Emptied folders.** After a delete, Marked for deletion asks
  `/api/folders/emptied` which of the deleted photos' folders are empty now.
  Files a system leaves behind (`.DS_Store`, `._*`, `Thumbs.db`,
  `desktop.ini`, `.localized`) don't count. The browse root and library
  folders are never offered. `/api/folders/remove-empty` checks again before
  removing anything. Folders and Organize move out of a removed folder to its
  parent.

## Acceptance Criteria

- [x] Deleting a photo removes its sidecar; a sidecar a RAW of the same name uses stays
- [x] A selected folder can be marked for deletion from the selection bar (Folders, library)
- [x] A library folder marked for deletion deletes that folder, not one of the same relative path under the boundary
- [x] A selected folder can be renamed (Folders, Organize, library); the index keeps its photos
- [x] Several folders selected: Rename is disabled with a reason
- [x] A library's folder is not renamed or deleted, and the reason is given
- [x] Deleting every photo of a folder asks to remove it; Keep leaves it, Remove removes it
- [x] Go tests (`fileops/folders_test.go`, `media/sidecar_delete_test.go`) and e2e (`folder-delete-rename.spec.js`)
