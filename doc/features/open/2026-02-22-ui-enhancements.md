# UI Enhancements: Larger Controls, Icons, Mode Names, Header

*Last modified: 2026-02-24*

## Summary

Improve usability and clarity by increasing button/control sizes, adding icons to File Manager action buttons, renaming modes to user-facing labels, and displaying the human-readable project name in the header.

## Details

- **Larger buttons and controls** — Increased padding and font sizes across `.btn`, `.btn-sm`, `.btn-action`, and `.view-menu select` for better click targets on modern displays.
- **Bold active buttons** — Active buttons now render with `font-weight: 600` (semi-bold) for clearer state indication.
- **Commander button icons** — Copy, Move, and Delete buttons in File Manager mode now include 14x14 stroke-only SVG icons (two overlapping rectangles, rectangle with arrow, trash can outline).
- **Renamed modes** — "Browse" is now "Browse & Cull"; "Commander" is now "File Manager". "Waste Bin" is unchanged.
- **Header name** — The header and browser tab title now show "Unterlumen".

## Acceptance Criteria

- [x] Header displays "Unterlumen" and browser tab title matches
- [x] Mode tabs read "Browse & Cull", "File Manager", "Waste Bin"
- [x] Buttons are visibly larger across all modes
- [x] Active mode tab renders in semi-bold orange
- [x] File Manager Copy, Move, Delete buttons show stroke-only SVG icons
- [x] Icon + label + arrow + count renders correctly when files are selected
- [x] `go vet ./...` passes
