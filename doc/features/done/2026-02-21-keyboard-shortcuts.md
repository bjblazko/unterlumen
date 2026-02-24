# Keyboard Shortcuts

*Last modified: 2026-02-21*

## Summary

Navigate the application and perform actions using keyboard shortcuts.

## Details

### Image viewer

| Key | Action |
|-----|--------|
| Left Arrow | Previous image |
| Right Arrow | Next image |
| Escape | Close viewer |
| Backspace | Close viewer |

### Browse mode

| Key | Action |
|-----|--------|
| Backspace | Navigate to parent directory |

### Commander mode

| Key | Action |
|-----|--------|
| Tab | Switch active pane |
| Backspace | Navigate active pane to parent directory (when viewer is not open) |

### Selection (both modes)

| Modifier + Click | Action |
|------------------|--------|
| Ctrl/Cmd + Click | Toggle file selection |
| Shift + Click | Range select |

### Scope

- Viewer keyboard shortcuts are registered when the viewer opens and removed when it closes, preventing conflicts with browse/commander shortcuts
- The Backspace handler checks whether the viewer is open before navigating up

## Acceptance Criteria

- [x] Arrow keys navigate images in viewer
- [x] Escape and Backspace close the viewer
- [x] Backspace navigates up in browse mode
- [x] Tab switches panes in commander mode
- [x] Keyboard shortcuts don't conflict between modes
