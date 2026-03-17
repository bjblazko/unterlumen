# Commander UI Enhancements

*Last modified: 2026-03-17*

## Summary

Restructured the commander (dual-pane) actions column and added two new file operations: New Folder and Rename.

## Details

- **Panel captions** — each pane displays a "From" or "To" label using a CSS `::before` pseudo-element driven by a `data-pane-label` attribute. The active pane is "From"; the inactive pane is "To". Labels update whenever the active pane changes.
- **Restructured actions column** — the actions column is split into two sub-sections:
  - Top (`cmd-top-actions`): Delete, Folder, Rename — non-directional operations
  - Bottom (`cmd-dir-actions`): the directional arrow SVG + Copy + Move
- **New Folder** — prompts for a folder name and calls `POST /api/mkdir`. The active pane reloads after creation.
- **Rename** — enabled when an item is focused in the active pane. Prompts for a new name and calls `POST /api/rename`. The active pane reloads after renaming.
- **Backend** — two new handlers in `internal/api/fileops.go`:
  - `handleMkdir`: validates path via `safePath`, calls `os.Mkdir`, invalidates cache.
  - `handleRename`: validates path and new base name (no path separators, not `.` or `..`), checks destination doesn't exist, calls `os.Rename`, invalidates cache.

## Acceptance Criteria

- [x] Each pane shows "From" or "To" label
- [x] Labels swap when the active pane changes
- [x] Delete, Folder, Rename appear at the top of the actions column
- [x] Copy, Move remain grouped with the directional arrow
- [x] "Folder" button is always enabled; creates a directory and refreshes the pane
- [x] "Rename" button is enabled only when an item is focused
- [x] Rename prompts with the current name pre-filled; renames and refreshes the pane
- [x] Arrow still flips when the active pane changes
- [x] Copy/Move continue to work correctly
- [x] `go vet ./...` passes
