# Folder Metadata Panel

*Last modified: 2026-05-24*

## Summary

When a folder is focused (single-click) in browse or library mode, the info panel shows folder-specific metadata instead of staying empty. The panel includes a DaisyDisk-inspired squarified treemap of subfolder sizes, a nesting-depth histogram, a file-type breakdown, and — in library mode — EXIF-based photo statistics powered by the existing library statistics API.

## Details

**New backend endpoint:**
- `GET /api/browse/folder-stats?path=<rel>` — walks the directory tree up to 10 levels deep and returns size, file count, subfolder breakdown, and file-type distribution
- `GET /api/library/{id}/folder-stats?path=<rel>` — same, resolved against the library's source path

**Info panel sections (browse mode):**
- **Folder** — name, path, modification date
- **Contents** — total files, total size, subfolder count, max nesting depth
- **Size Map** — squarified SVG treemap; each subfolder rectangle is proportional to its recursive size; click navigates into that subfolder
- **Nesting Depth** — CSS bar histogram per immediate subfolder showing max nesting depth
- **File Types** — horizontal bar chart of all file extensions found recursively

**Info panel sections (library mode, in addition to browse mode sections):**
- **Photos** — total photo count, first/last shooting date, active days
- **Formats** — bar chart of photo file formats
- **Camera** — top camera × lens combinations with photo count
- **Shooting Hours** — 24-column bar chart of shooting activity by hour of day

**Navigation:** clicking a rectangle in the Size Map navigates the browse/library pane into that subfolder.

## Acceptance Criteria

- [x] Single-clicking a folder in browse mode opens the info panel on that folder
- [x] Single-clicking a folder in library mode opens the info panel on that folder
- [x] The panel shows name, path, total size, file count, subfolder count, max depth
- [x] The Size Map shows immediate subfolders as proportional rectangles
- [x] Clicking a Size Map rectangle navigates into that subfolder
- [x] The Nesting Depth histogram shows per-subfolder bar heights
- [x] The File Types chart shows extension breakdown in browse mode
- [x] In library mode, shooting date range, formats, camera, and hours sections appear
- [x] Switching back to a photo updates the panel to photo info
- [x] No regressions in existing photo info panel behavior
