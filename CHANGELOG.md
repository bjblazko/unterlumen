# Changelog

*Last modified: 2026-02-24*




All notable changes to this project are documented in this file.

## [Unreleased]

### Changed

- **Renamed project** — "iseefourlights" is now "Unterlumen". Binary, module path, cache directory, UI title, and all documentation updated.

### Added

- **Info panel** — Collapsible right-side panel showing file metadata and EXIF data (camera, exposure, dates, GPS location). Toggle with the `I` key or the info icon. New `GET /api/info` endpoint provides full EXIF extraction. Sections are collapsible (click to toggle, state remembered per session). Supports HEIF/HEIC/HIF files by scanning the ISOBMFF container for embedded EXIF data. Now available in the fullscreen viewer with dark theme styling — toggle via toolbar button or `I` key, panel updates automatically when navigating between images.
- **EXIF orientation support** — Photos taken in portrait mode (or other non-standard orientations) now display correctly. Thumbnails are rotated server-side; full-size images use CSS `image-orientation: from-image`. All 8 EXIF orientation values are handled.
- **HEIF orientation support** — Portrait HEIF/HIF files now display correctly. The `irot` (image rotation) box in the HEIF container is parsed and applied during conversion, since `sips`/`ffmpeg` do not apply it automatically.

### Fixed

- **EXIF thumbnail aspect ratio validation** — Embedded EXIF thumbnails that don't match the actual image aspect ratio (e.g., cameras storing a full-sensor 4:3 thumbnail for a 1:1 or 16:9 crop) are now rejected, falling through to server-side thumbnail generation with the correct aspect ratio.

### Changed

- **Grid/List toggle moved to View menu** — The layout toggle (Grid/List) is now inside the View popup menu under a "Layout" section, decluttering the controls bar.

### Added

- **Sort by size** — File size is now available as a sort option alongside Name and Date.

### Changed

- **Larger controls** — Increased button padding and font sizes across all modes for better click targets on modern displays.
- **Bold active state** — Active mode tab and buttons now render in semi-bold (600) for clearer state indication.
- **File Manager icons** — Copy, Move, and Delete buttons in File Manager mode now include stroke-only SVG icons.
- **Renamed modes** — "Browse" is now "Browse & Cull"; "Commander" is now "File Manager".
- **Header name** — Header and browser tab title now show "Unterlumen".

### Added

- **Waste bin** — Mark unwanted photos for deletion, review them in a dedicated Waste Bin view, then restore or permanently delete. Non-destructive by default: files remain on disk until confirmed. Accessible as a third mode alongside Browse and Commander, with a count badge in the header.
- **Delete endpoint** — `POST /api/delete` removes files from disk, following the same pattern as copy/move with per-file results.
- **Delete in Commander** — Delete button alongside Copy/Move marks selected files for the waste bin.
- **Delete in Viewer** — Delete button in the viewer toolbar marks the current image and advances to the next.
- **Delete key** — Pressing Delete in Browse mode or the Viewer marks selected/current files for deletion.

### Changed

- **View popup menu** — Moved sort controls and Names toggle from the toolbar into a "View" popup menu, decluttering the controls bar. Grid/List toggle remains inline.

### Fixed

- **HEIF/HIF fullscreen rendering** — HIF files without an embedded full-resolution JPEG preview (only a 160x120 thumbnail) now render correctly in fullscreen. Added macOS `sips` as a fallback converter before the ffmpeg HEVC decode path, which correctly assembles multi-tile grids. Cache keys bumped to invalidate previously cached quarter-images.

### Added

- **Directory browsing** — Grid and list views for photo directories with breadcrumb navigation
- **Single image viewer** — Full-screen image display with arrow key prev/next, Escape to close
- **Commander mode** — Dual-pane Norton Commander-style layout for copy/move between directories
- **Thumbnail serving** — EXIF embedded thumbnail extraction with fallback server-side resize
- **HEIF/HEIC/HIF support** — On-the-fly conversion to JPEG via ffmpeg
- **HEIF/ffmpeg availability warning** — Dismissible banner when a directory contains HEIF files but ffmpeg is missing or lacks HEVC decoder support
- **Sorting** — By name or date, ascending or descending; directories always sorted first
- **Single-click selection** — Clicking an image selects it (with orange highlight), clearing any previous selection
- **Multi-select** — Ctrl/Cmd+click to toggle, Shift+click for range selection
- **Keyboard shortcuts** — Arrow keys, Escape, Backspace, Tab (commander pane switch)
- **Path traversal protection** — All API paths validated to stay within the configured root directory
- **`.hif` extension support** — Fujifilm HEIF variant recognized alongside `.heif` and `.heic`

### Changed

- **Visual redesign** — Complete UI overhaul following Dieter Rams' ten principles of good design, inspired by Braun products (1961–1995). Light warm palette, functional orange accents, Helvetica-style typography, 8px grid spacing, minimal chrome.
- **Filenames hidden by default** — Grid view shows only the image. A "Names" toggle in the controls bar reveals filenames when needed. Directory names and list view are unaffected.
- **Correct aspect ratios** — Grid thumbnails render at their natural aspect ratio (no fixed height, no cropping). Landscape and portrait images are visually distinct.
- **Fixed HEIF/HIF rendering** — Multi-tile HEIF files (Fujifilm HIF) now render the full assembled image instead of a single tile. Thumbnails use the embedded JPEG preview for speed. Full-size view decodes the complete tile grid.
- **Fixed image caching** — Changed `Cache-Control` from `max-age=3600` to `no-cache` on thumbnail and image endpoints. Prevents stale (cropped) images from being served from browser cache after a server update.
- **Disk cache for HEIF conversions** — Converted JPEG data from HEIF files is cached in the OS temp directory (`os.TempDir()/unterlumen-cache`). Cache keys include file path, modification time, and purpose (full/preview). Works portably across Linux, macOS, and Windows.
- **HEIF extraction prefers embedded JPEG** — Full-size and thumbnail extraction now prefer the largest embedded JPEG preview via stream copy (instant) over HEVC tile grid decoding. Falls back to HEVC decode for simple HEIF files without previews.
- **Viewer preserves grid state** — Opening the full-screen viewer no longer destroys the browse/commander DOM. The grid is hidden while viewing and restored instantly on close, avoiding re-fetching all thumbnails.

### Architecture

- Go HTTP server with browser-based frontend ([ADR-0001](doc/architecture/adr/0001-go-http-server-with-browser-ui.md))
- No persistence — in-memory state only ([ADR-0002](doc/architecture/adr/0002-no-persistence.md))
- EXIF embedded thumbnails ([ADR-0003](doc/architecture/adr/0003-exif-thumbnails.md))
- HEIF via ffmpeg shell-out ([ADR-0004](doc/architecture/adr/0004-heif-via-ffmpeg.md))
- Commander-style dual-pane culling ([ADR-0005](doc/architecture/adr/0005-commander-style-culling.md))
- No authentication ([ADR-0006](doc/architecture/adr/0006-no-authentication.md))
- Vanilla HTML/JS/CSS frontend ([ADR-0007](doc/architecture/adr/0007-vanilla-frontend.md))
- Dieter Rams' ten principles of good design as governing design philosophy ([ADR-0008](doc/architecture/adr/0008-dieter-rams-design-principles.md))
- Soft delete with frontend-only waste bin ([ADR-0009](doc/architecture/adr/0009-soft-delete-waste-bin.md))
