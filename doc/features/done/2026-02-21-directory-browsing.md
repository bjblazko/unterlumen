# Directory Browsing

*Last modified: 2026-02-21*

## Summary

Browse a directory of photos and subdirectories in a single-pane view with grid and list layouts.

## Details

The browse mode displays the contents of a directory within the configured root. Entries are either subdirectories or supported image files.

### Grid view

- Thumbnails displayed in a responsive CSS grid (auto-fill, min 180px)
- Directory entries shown with a folder icon
- Image entries show their EXIF thumbnail (or a generated fallback)
- Filename displayed below each thumbnail

### List view

- Table layout with columns: icon, name, date, size
- Directory entries shown with a folder icon
- Image entries show a small 32x32 preview
- Date column shows date taken (EXIF) or filesystem modification time

### Navigation

- Breadcrumb bar at the top shows the current path as clickable segments
- Clicking "Root" navigates to the configured root directory
- Double-clicking a directory navigates into it
- Backspace key navigates to the parent directory

### API

- `GET /api/browse?path=<relative>&sort=<field>&order=<asc|desc>` returns a JSON array of entries with `name`, `type` (dir/image), `date`, and `size` fields.

## Acceptance Criteria

- [x] Grid view renders thumbnails for all supported image formats
- [x] List view shows filename, date, and size
- [x] Breadcrumb navigation works at all directory depths
- [x] Double-click on directory navigates into it
- [x] Hidden files (dot-prefix) are excluded
- [x] Empty directories show an "empty" message
