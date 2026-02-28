# Keyboard-First Culling

*Last modified: 2026-02-28*

## Summary

Mark images for deletion using only the keyboard — no selection step required. Navigate with arrow keys and press a delete shortcut to mark the focused image.

## Details

Previously, marking a file for deletion required two steps: first select it (Space or click), then press Delete/Backspace/Cmd+D. In keyboard navigation mode this was awkward.

Now, if no files are selected, Backspace/Delete/Cmd+D fall back to the **focused** item (the image highlighted by the orange focus ring from arrow-key navigation). If files are explicitly selected, those still take precedence.

**Affected shortcuts in browse mode:**
- `Backspace` — marks selected files, or focused image if nothing selected
- `Delete` — same
- `Cmd/Ctrl+D` — same

The viewer is unchanged: it always marks the currently displayed image.

## Acceptance Criteria

- [x] Arrow-navigate to an image, press Backspace → image is dimmed and added to waste bin
- [x] Arrow-navigate to an image, press Delete → same
- [x] Arrow-navigate to an image, press Cmd+D → same
- [x] If files are selected (Space/click), those are marked instead of focused item
- [x] If focus is on a directory, delete shortcuts do nothing
- [x] If nothing is focused and nothing is selected, delete shortcuts do nothing
