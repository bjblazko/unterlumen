# Batch Rename

*Last modified: 2026-03-21*

## Summary

Pattern-based batch renaming of photos using EXIF metadata placeholders. Accessible from the Tools dropdown ("Batch (Metadata)") in browse mode and the Rename dropdown in commander mode. Includes a live preview of resulting filenames, progress indication during execution, and a simple single-file rename option alongside.

## Details

- Curly-brace pattern syntax with placeholders for date (`{YYYY}`, `{MM}`, `{DD}`, `{hh}`, `{mm}`, `{ss}`), camera metadata (`{make}`, `{model}`, `{lens}`, `{filmsim}`, `{iso}`, `{aperture}`, `{focal}`, `{shutter}`), original filename (`{original}`), and auto-incrementing counter (`{seq}`, `{seq:N}`).
- Missing EXIF values resolve to `unknown`.
- Filenames are sanitized for SMB-safe portability: spaces become hyphens, unsafe characters removed, consecutive hyphens/underscores collapsed.
- File extension is always preserved from the original (lowercased).
- Duplicate resulting names get `_001`, `_002` etc. suffixes automatically.
- Two-pass rename on disk (temp names first, then final) handles circular renames safely.
- Backend validates no collisions with existing files outside the rename set.
- Modal UI with color-coded, draggable token pills (date=blue, camera=purple, exposure=amber, file=green) with tooltips showing example values. Tokens can be clicked to append or dragged into the pattern input at a precise position with a visual drop marker.
- Colored highlight overlay in the pattern input mirrors token colors inline.
- Debounced live preview with horizontally scrollable filename list, conflict highlighting, and per-row error display.
- Progress bar with status text during rename execution, with error summary on completion.
- Simple single-file rename ("Single") available alongside batch rename in both browse and commander modes. Disabled when multiple files are selected.
- Rename button in commander mode is a dropdown offering both options.

## Acceptance Criteria

- [x] `POST /api/batch-rename/preview` returns resolved filenames for a given pattern
- [x] `POST /api/batch-rename/execute` renames files on disk using two-pass strategy
- [x] Pattern placeholders resolve from EXIF data; missing values show "unknown"
- [x] Filename sanitization produces SMB-safe portable names
- [x] Duplicate names get auto-increment suffixes
- [x] Modal accessible from Tools dropdown in browse mode
- [x] Commander Rename dropdown opens batch rename or single rename
- [x] Live preview updates on pattern change (debounced)
- [x] Conflicts highlighted in preview
- [x] Rename button disabled until preview loaded with no errors
- [x] Token pills are color-coded by category with tooltips
- [x] Token pills are draggable into the pattern input
- [x] Progress bar shown during rename execution
- [x] Simple rename available for single files; disabled for multi-select
- [x] `go vet ./...` passes
