# Commander Mode

*Last modified: 2026-02-21*

## Summary

Dual-pane Norton Commander-style layout for organizing photos by copying or moving files between directories.

## Details

Commander mode splits the screen into two independent directory browsers (left and right) with action buttons in the center.

### Pane behavior

- Each pane is a full `BrowsePane` instance with its own breadcrumb, sort controls, and grid/list toggle
- Panes navigate independently — each can be in a different directory
- The active pane is highlighted with a colored border
- Clicking inside a pane makes it the active pane
- Tab key switches the active pane

### File operations

- **Copy** — copies selected files from the active pane to the other pane's current directory
- **Move** — moves selected files from the active pane to the other pane's current directory
- Buttons are in the center column between the two panes
- Button labels show the direction arrow (→ or ←) and selected file count
- Buttons are disabled when no files are selected
- A confirmation dialog shows the file count before executing
- Both panes refresh after the operation completes
- Per-file errors are shown in an alert if any files fail

### API

- `POST /api/copy` with `{files: [...], destination: "..."}`
- `POST /api/move` with `{files: [...], destination: "..."}`
- Response includes per-file success/failure results

## Acceptance Criteria

- [x] Two panes render side by side, each browsing independently
- [x] Active pane is visually indicated
- [x] Tab switches the active pane
- [x] Copy/Move buttons reflect direction and selection count
- [x] Confirmation dialog before executing operations
- [x] Both panes refresh after copy/move
- [x] Errors reported per-file
- [x] Files are not overwritten if destination already exists
