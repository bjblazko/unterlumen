# Home Button in Breadcrumb Navigation

*Last modified: 2026-05-24*

## Summary

Add a home icon button next to the up-dir arrow in the breadcrumb row. Clicking it jumps directly to the configured start directory (the user's home directory in default desktop mode), avoiding manual breadcrumb traversal.

## Details

- Appears in browse and commander modes only — hidden in library mode
- Navigates to `App.config.startPath` (the relative path returned by `/api/config`)
- Disabled when already at the start directory
- Hidden entirely when `startPath` is empty (i.e. when a path argument is given on the command line, making start = root)
- Same visual style as the existing up-dir button (`.btn .btn-sm`, 13×13 SVG icon)
- No backend changes required

## Acceptance Criteria

- [x] Home button appears between the up-dir button and the breadcrumb nav in browse mode
- [x] Home button appears in commander mode (both panes use BrowsePane)
- [x] Home button is absent in library mode (browsing a library)
- [x] Button is disabled when current path equals `startPath`
- [x] Clicking the button navigates to `startPath` and updates the breadcrumb
- [x] Button is not rendered when `startPath` is `""` (command-line path arg case)
