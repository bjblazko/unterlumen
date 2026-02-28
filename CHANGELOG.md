# Changelog

*Last modified: 2026-02-28*

All notable changes to this project are documented in this file.

## [Unreleased]

## [0.1.1] - 2026-02-28

### Added

- **`UNTERLUMEN_ROOT_PATH` environment variable** — set this to restrict navigation to a specific directory. When set, the server starts in that directory and users cannot navigate above it. Useful for self-hosted or kiosk deployments where the photo library should be confined.
- **`UNTERLUMEN_PORT` and `UNTERLUMEN_BIND` environment variables** — configure the HTTP port and bind address without CLI flags. CLI flags (`-port`, `-bind`) take precedence when provided.
- **Home directory default** — when started without arguments and without `UNTERLUMEN_ROOT_PATH`, the server now starts in the user's home directory instead of the current working directory.
- **Unrestricted navigation for cmdline arg** — when a directory is passed as a command-line argument, navigation is no longer restricted to that directory; users can navigate freely up to the filesystem root.

### Fixed

- **Viewer gap when UI hidden** — pressing H to hide the header no longer leaves a blank 48 px gap at the bottom of the screen; `#app` now expands to full viewport height when the header is hidden.

### Changed

- **Keyboard shortcuts** — `Escape` now handles navigation/dismissal (go up directory, close viewer). `Backspace` (⌫) now marks selected files for the waste bin in browse/commander mode and marks the current image for deletion in viewer mode. `Delete` (⌦) continues to work as a second shortcut for deletion. This makes culling more ergonomic on Mac laptops where Fn+⌫ was previously required for the most common action.
- **Keyboard-first culling** — Backspace, Delete, and Cmd/Ctrl+D now mark the **focused** image for deletion when nothing is selected. Navigate with arrow keys and press Backspace to mark without needing to select first.

## [0.1.0] - 2026-02-28

### Added

- **Directory browsing** — Grid and list views for photo directories with breadcrumb navigation.
- **Single image viewer** — Full-screen image display with arrow key prev/next, Escape to close.
- **Commander mode** — Dual-pane Norton Commander-style layout for copy/move between directories.
- **Thumbnail serving** — EXIF embedded thumbnail extraction with fallback server-side resize.
- **HEIF/HEIC/HIF support** — On-the-fly conversion to JPEG via ffmpeg.
- **HEIF/ffmpeg availability warning** — Dismissible banner when a directory contains HEIF files but ffmpeg is missing or lacks HEVC decoder support.
- **Sorting** — By name or date, ascending or descending; directories always sorted first.
- **Single-click selection** — Clicking an image selects it (with orange highlight), clearing any previous selection.
- **Multi-select** — Ctrl/Cmd+click to toggle, Shift+click for range selection.
- **Path traversal protection** — All API paths validated to stay within the configured root directory.
- **`.hif` extension support** — Fujifilm HEIF variant recognized alongside `.heif` and `.heic`.
- **Waste bin** — Mark unwanted photos for deletion, review them in a dedicated Marked for Deletion view, then restore or permanently delete. Non-destructive by default: files remain on disk until confirmed. Accessible as a third mode alongside Browse and Commander, with a count badge in the header.
- **Delete endpoint** — `POST /api/delete` removes files from disk, following the same pattern as copy/move with per-file results.
- **Delete in Commander** — Delete button alongside Copy/Move marks selected files for the waste bin.
- **Delete in Viewer** — Delete button in the viewer toolbar marks the current image and advances to the next.
- **Delete key** — Pressing Delete in Browse mode or the Viewer marks selected/current files for deletion.
- **Sort by size** — File size is now available as a sort option alongside Name and Date.
- **EXIF orientation support** — Photos taken in portrait mode (or other non-standard orientations) now display correctly. Thumbnails are rotated server-side; full-size images use CSS `image-orientation: from-image`. All 8 EXIF orientation values are handled.
- **HEIF orientation support** — Portrait HEIF/HIF files now display correctly. The `irot` (image rotation) box in the HEIF container is parsed and applied during conversion, since `sips`/`ffmpeg` do not apply it automatically.
- **Info panel** — Collapsible right-side panel showing file metadata and EXIF data (camera, exposure, dates, GPS location). Toggle with the `I` key or the info icon. New `GET /api/info` endpoint provides full EXIF extraction. Sections are collapsible (click to toggle, state remembered per session). Supports HEIF/HEIC/HIF files by scanning the ISOBMFF container for embedded EXIF data. Now available in the fullscreen viewer with dark theme styling — toggle via toolbar button or `I` key, panel updates automatically when navigating between images.
- **Grid keyboard navigation** — Arrow keys move a visual focus indicator through grid and list views. Up/Down jump by the current column count; Left/Right step linearly. Enter activates the focused item (navigates into a folder or opens the fullscreen viewer). Space toggles selection of the focused image without moving focus. Focus resets to the first item on directory load and syncs with mouse clicks.
- **Header logo** — The Unterlumen logo is now displayed inline to the left of the app title in the header.
- **Status bar** — Image count and selection count shown in the controls row of every Browse and Commander pane (e.g. "12 images · 3 selected"). Updates live on selection changes.
- **Keyboard shortcuts** — Comprehensive keyboard shortcut set across all views:
  - **Cmd/Ctrl+1/2/3** — switch to Browse & Cull, File Manager, or Marked for Deletion. Mode buttons show tooltips with platform-appropriate hints (⌘ on Mac, Ctrl+ elsewhere).
  - **Cmd/Ctrl+A** — select all files in the current Browse pane, active Commander pane, or Marked for Deletion.
  - **Cmd/Ctrl+D** — mark selected files for deletion (Browse & Commander); prevents browser bookmark default.
  - **F5 / F6** — copy / move selected files in Commander. Buttons show F5/F6 tooltips on hover.
  - **Arrow keys, Escape, Backspace, Tab** — navigation in viewer and commander pane switch.
- **Dark mode & Settings menu** — A Settings button (gear icon) appears in the header next to the mode-switcher. Clicking it opens a dropdown with a Light / Auto / Dark theme toggle. Auto follows the OS `prefers-color-scheme` setting and updates in real time. The selection is persisted in `localStorage`. Theme is applied before CSS paints to prevent flash of wrong theme.
- Press `H` to toggle interface visibility (header, info panel, viewer toolbar) for distraction-free photo viewing. State persists across reloads. A "Hide Interface (H)" button is also available in the Settings menu.
- **Commander copy/move** — if a folder is focused in the target pane, copy/move operations use that folder as the destination instead of the pane's current directory.
- **Resizable Commander panes** — a drag handle between the left pane and the center action buttons allows free resizing of the two panes. The split ratio persists across sessions via `localStorage`.
- **Fujifilm film simulation** — The info panel now shows the film simulation (e.g. Classic Chrome, Acros, Velvia) in the Camera section for Fujifilm images.
- **Loading spinner for folder navigation** — A spinner appears in the content area while the backend scans a directory, giving immediate feedback during slow EXIF extraction on large folders.
- **In-memory scan cache** — Repeat visits to a folder load instantly from an in-memory cache. Cache is invalidated automatically when files are copied, moved, or deleted, or when the directory modification time changes.
- **Deferred EXIF extraction** — Directory listings return immediately using file modification times. EXIF dates are extracted in a background goroutine and delivered to the frontend via a new `GET /api/browse/dates` polling endpoint. If sorting by date, the grid re-sorts automatically when EXIF dates arrive.
- **Chunked rendering** — Grid and list views render in batches of 50 items. Additional batches load on scroll via IntersectionObserver, keeping DOM size small for large folders. Keyboard navigation past the rendered range triggers on-demand rendering.

### Changed

- **Renamed project** — "iseefourlights" is now "Unterlumen". Binary, module path, cache directory, UI title, and all documentation updated.
- **Grid/List toggle moved to View menu** — The layout toggle (Grid/List) is now inside the View popup menu under a "Layout" section, decluttering the controls bar.
- **Larger controls** — Increased button padding and font sizes across all modes for better click targets on modern displays.
- **Bold active state** — Active mode tab and buttons now render in semi-bold (600) for clearer state indication.
- **File Manager icons** — Copy, Move, and Delete buttons in File Manager mode now include stroke-only SVG icons.
- **Renamed modes** — "Browse" is now "Browse & Cull"; "Commander" is now "File Manager".
- **Header name** — Header and browser tab title now show "Unterlumen".
- **View popup menu** — Moved sort controls and Names toggle from the toolbar into a "View" popup menu, decluttering the controls bar. Grid/List toggle remains inline.
- **Visual redesign** — Complete UI overhaul following Dieter Rams' ten principles of good design, inspired by Braun products (1961–1995). Light warm palette, functional orange accents, Helvetica-style typography, 8px grid spacing, minimal chrome.
- **Filenames hidden by default** — Grid view shows only the image. A "Names" toggle in the controls bar reveals filenames when needed. Directory names and list view are unaffected.
- **Correct aspect ratios** — Grid thumbnails render at their natural aspect ratio (no fixed height, no cropping). Landscape and portrait images are visually distinct.
- **Disk cache for HEIF conversions** — Converted JPEG data from HEIF files is cached in the OS temp directory (`os.TempDir()/unterlumen-cache`). Cache keys include file path, modification time, and purpose (full/preview). Works portably across Linux, macOS, and Windows.
- **HEIF extraction prefers embedded JPEG** — Full-size and thumbnail extraction now prefer the largest embedded JPEG preview via stream copy (instant) over HEVC tile grid decoding. Falls back to HEVC decode for simple HEIF files without previews.
- **Viewer preserves grid state** — Opening the full-screen viewer no longer destroys the browse/commander DOM. The grid is hidden while viewing and restored instantly on close, avoiding re-fetching all thumbnails.

### Fixed

- **EXIF thumbnail aspect ratio validation** — Embedded EXIF thumbnails that don't match the actual image aspect ratio (e.g., cameras storing a full-sensor 4:3 thumbnail for a 1:1 or 16:9 crop) are now rejected, falling through to server-side thumbnail generation with the correct aspect ratio.
- **HEIF/HIF fullscreen rendering** — HIF files without an embedded full-resolution JPEG preview (only a 160x120 thumbnail) now render correctly in fullscreen. Added macOS `sips` as a fallback converter before the ffmpeg HEVC decode path, which correctly assembles multi-tile grids. Cache keys bumped to invalidate previously cached quarter-images.
- **Fixed HEIF/HIF rendering** — Multi-tile HEIF files (Fujifilm HIF) now render the full assembled image instead of a single tile. Thumbnails use the embedded JPEG preview for speed. Full-size view decodes the complete tile grid.
- **Fixed image caching** — Changed `Cache-Control` from `max-age=3600` to `no-cache` on thumbnail and image endpoints. Prevents stale (cropped) images from being served from browser cache after a server update.
- **Info panel focus tracking** — The info panel now reliably shows metadata for the focused image in both grid and list view. It updates on keyboard navigation (arrow keys), mouse clicks, and directory changes. Pressing `I` to open the panel immediately loads info for the currently focused item.
- **Duplicate path on repeated folder click** — Rapidly double-clicking a folder (e.g. over a slow NAS connection) no longer produces invalid paths like `/pics/large/large`. A loading guard prevents navigation while a browse request is in flight.
