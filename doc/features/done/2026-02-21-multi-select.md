# Multi-Select

*Last modified: 2026-02-21*

## Summary

Select multiple image files for bulk copy/move operations using standard click modifiers.

## Details

### Selection methods

- **Ctrl+Click** (Cmd+Click on macOS) — toggle selection of a single file
- **Shift+Click** — select a range from the last clicked item to the current item
- **Regular click** — records the click position (for shift-click ranges) but does not change selection

### Visual feedback

- Selected items in grid view get a colored border (red accent)
- Selected rows in list view get a highlighted background
- The commander mode action buttons show the count of selected files

### Scope

- Selection is tracked per pane (each `BrowsePane` instance has its own `selected` set)
- Selection is stored as a `Set` of full relative paths
- Selection is cleared when navigating to a different directory
- Only image files can be selected, not directories

### Integration with Commander mode

- Copy/Move operates on the active pane's selection
- After a copy/move operation, selections are cleared (both panes reload)

## Acceptance Criteria

- [x] Ctrl/Cmd+Click toggles individual file selection
- [x] Shift+Click selects a range of files
- [x] Selected files are visually highlighted in both grid and list views
- [x] Selection count is reflected in commander action buttons
- [x] Selection is cleared on directory navigation
- [x] Only images are selectable, not directories
