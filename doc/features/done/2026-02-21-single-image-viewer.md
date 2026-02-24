# Single Image Viewer

*Last modified: 2026-02-21*

## Summary

Full-screen image view with keyboard and button navigation for stepping through images in a directory.

## Details

Double-clicking a thumbnail in browse or commander mode opens the image viewer. The viewer displays the full-resolution image fitted to the viewport.

### Controls

- **Back button** (top-left) — returns to the previous browse/commander view
- **Previous/Next buttons** — large chevrons on left/right sides of the image
- **Arrow keys** — Left/Right for prev/next
- **Escape / Backspace** — close viewer and return to browse

### Display

- Toolbar shows the filename and a position counter (e.g. "3 / 42")
- Image is displayed with `object-fit: contain` to fit within the viewport without cropping
- Previous/Next buttons are disabled (greyed out) at the start/end of the image list

### Image list

The viewer receives the full list of images in the current directory (in the current sort order). Navigation steps through this list, not just adjacent files.

### API

- `GET /api/image?path=<relative>` serves the full-size image
- HEIF/HEIC files are converted to JPEG on-the-fly via ffmpeg

## Acceptance Criteria

- [x] Double-clicking a thumbnail opens the viewer
- [x] Full image is displayed fitted to viewport
- [x] Arrow keys navigate prev/next
- [x] Escape/Backspace closes the viewer
- [x] Filename and position counter are shown
- [x] Navigation buttons disable at boundaries
- [x] Returning from viewer restores the previous browse/commander state
