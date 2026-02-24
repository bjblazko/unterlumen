# Keyboard Shortcuts

*Last modified: 2026-02-24*

*Completed: 2026-02-24*

## Summary

A cohesive set of keyboard shortcuts for all major views, covering view switching, selection, deletion marking, and file operations in the commander.

## Details

### View switching (Cmd/Ctrl+1/2/3)
- `Cmd+1` / `Ctrl+1` — Browse & Cull
- `Cmd+2` / `Ctrl+2` — File Manager (Commander)
- `Cmd+3` / `Ctrl+3` — Waste Bin
- Mode buttons show tooltips with platform-appropriate shortcut hints.
- Implemented via `isMac` property (`/Mac|iPhone|iPad|iPod/.test(navigator.platform)`) on the `App` object.

### Select all (Cmd/Ctrl+A)
- In Browse & Cull: selects all non-directory files in the current directory.
- In Commander: selects all non-directory files in the active pane.
- In Waste Bin: selects all entries.
- `BrowsePane.selectAll()` method iterates `entries`, filters out `dir` types, adds each full path to `this.selected`, and re-renders.

### Mark for deletion (Cmd/Ctrl+D)
- In Browse & Cull (no viewer open): marks all selected files for deletion and clears selection.
- In Commander: calls `commander.doDelete()` (marks active pane selection for deletion).
- No-op in Waste Bin (items are already marked).
- Prevents browser "Bookmark this tab" default in browse/commander contexts.

### Copy and Move in Commander (F5 / F6)
- `F5` — Copy selected files from the active pane to the other pane (same as the Copy button).
- `F6` — Move selected files from the active pane to the other pane (same as the Move button).
- Copy and Move buttons show `title="Copy (F5)"` / `title="Move (F6)"` tooltips.

## Files Changed

- `web/js/browse.js` — `BrowsePane.selectAll()` method
- `web/js/commander.js` — F5/F6 tooltips on Copy/Move buttons
- `web/js/app.js` — `isMac` property, button tooltips in `init()`, all shortcut handlers in `handleGlobalKey()`

## Acceptance Criteria

- [ ] Hovering over mode buttons shows shortcut hints (⌘1/2/3 on Mac, Ctrl+1/2/3 elsewhere).
- [ ] Cmd+1/2/3 (Mac) or Ctrl+1/2/3 switches views from any context.
- [ ] Cmd+A / Ctrl+A selects all files in Browse, Commander active pane, and Waste Bin.
- [ ] Cmd+D / Ctrl+D marks selected files for deletion in Browse and Commander; no-op in Waste Bin.
- [ ] F5 triggers copy in Commander (same as Copy button); tooltip visible on hover.
- [ ] F6 triggers move in Commander (same as Move button); tooltip visible on hover.
- [ ] All shortcuts are inert when no items are selected (no errors).
- [ ] `go vet ./...` passes with no errors.
