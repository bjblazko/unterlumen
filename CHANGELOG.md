# Changelog

*Last modified: 2026-03-09*

All notable changes to this project are documented in this file.

## [Unreleased]

### Fixed

- **"Photo Taken" sort order** — Photos whose EXIF date happened to equal their filesystem mtime were silently omitted from the EXIF date map and treated as undated, causing them to sort last. EXIF dates are now always stored regardless of whether they match the mtime.
- **Date display in list view** — Dates in list view and the info panel "Modified" row were formatted using the browser locale (e.g. `01.03.2024, 15:04:05`). They are now displayed in ISO format (`2024-03-01 15:04:05`), consistent with the EXIF dates section in the info panel.
- **EXIF date formatting** — Info panel Dates section now displays dates as `2016-07-16 20:24:53` instead of the raw EXIF format `2016:07:16 20:24:53`. When offset tags are present, the UTC offset is appended (e.g. `2016-07-16 20:24:53 +09:00`). Raw date and offset tags are suppressed from the Other section.
- **Deselect on Escape** — Pressing Escape now clears the current selection instead of navigating up a directory when photos are selected (both browse and commander modes). A second Escape with no selection navigates up as before.
- **Deselect on void click** — Clicking an empty gap in the grid, justified, or list view now clears the selection.

### Added

- **"Photo Taken" sort** — New sort option in the view menu sorts photos by EXIF `DateTimeOriginal`. Photos without EXIF data always appear last regardless of sort direction. The existing "Date" option is renamed "File Modified" to clearly indicate it sorts by filesystem modification time.

### Changed

- **Commander direction arrow** — The Copy/Move buttons no longer show an arrow or selection count in their labels. Direction is now conveyed by a large translucent arrow SVG in the center actions panel, which flips left/right based on the active pane.
- **Inline SVG logo** — The header logo is now an inline SVG (two horizontal bars + orange triangle), removing the dependency on an external PNG file and allowing the logo to adapt to the current text color.
- **Workflow-oriented UI** — Replaced the tab-style mode switcher with a connected chevron stepper: three arrow-shaped buttons (flat left, pointed right) that interlock gaplessly. Modes are renamed and reordered to reflect the photographer's natural workflow — Select (1), Review (2), Organize (3) — each with a representative icon (photo grid, trash can, dual-pane). Active step is orange with white text; completed steps are gray; future steps are off-white. Mode switches animate with a directional slide. The Review step shows a count badge when photos are marked for deletion. Keyboard shortcuts updated to match: 2=Review, 3=Organize.
- **Thumbnail overlays on by default** — The "Show details" overlay badges (file type, GPS, film simulation) are now enabled by default.

### Added

- **Aspect ratio display** — Images now show their aspect ratio (e.g. `3:2`, `16:9`) as a badge on thumbnails (when "Show details" is on) and as a row in the info panel below Dimensions. An inline proportional-rectangle icon gives an instant visual hint of the image shape. Uses approximate matching (1.5% tolerance) for real-world sensor dimensions; unusual crops show "Custom Crop" with a dashed icon.
- **Thumbnail overlays** — New "Show details" toggle in the View menu displays colored metadata badges on thumbnails: file type (JPEG, HEIF, PNG, GIF, WebP), GPS location pin, and Fujifilm film simulation name. Each category has a unique vibrant color. File type badges appear immediately; GPS and film simulation badges load asynchronously via background EXIF extraction. Works in grid, justified, and list views. The same color scheme is used in the info panel for Format and Film Simulation values.

### Fixed

- **Commander buttons showing stale count** — Action buttons (Copy, Move, Delete) no longer show "(1)" when no images are explicitly selected; the count now reflects only Ctrl+click selections, matching the grid status bar.
- **Sticky header in commander and waste bin views** — The breadcrumb and controls now stay pinned at the top of each commander pane and the waste bin view while scrolling, matching the existing browse view behavior.
- **Commander copy no longer resets source pane scroll** — After a copy operation, only the destination pane reloads; the source pane is left untouched, preserving its scroll position.
- **Escape navigates up in commander mode** — Pressing Escape in commander mode now navigates the active pane to its parent directory, matching the existing browse-mode behavior.
- **Justified grid not resizing after info panel close** — Closing the info panel no longer leaves the justified grid at the narrower width; it now relays out immediately to fill the full available space.
- **Info panel in fullscreen mode** — The info panel (I key) now works in full UI-hidden mode (H key), allowing photo metadata to be viewed without leaving fullscreen.
- **Map zoom controls** — The location map now has +/- zoom buttons for reliable zooming across all input methods.
- **HEIF date extraction** — Background EXIF date extraction now handles HEIF/HEIC/HIF files via embedded EXIF fallback, fixing missing dates for HEIF images when sorting by date.

## [0.3.1] - 2026-03-06

### Fixed

- **Map "Open" button** — The location map's "Open" button now links to OpenStreetMap instead of OpenFreeMap, which has no map viewer UI.
- **Map attribution clutter** — The location map's attribution text now starts collapsed, showing only the info icon. Click to expand.
- **Sticky browse header** — Breadcrumb navigation, View button, and image count bar now remain fixed at the top of the browse view instead of scrolling away with the content.
- **Viewer not closing on tab switch** — Switching tabs (File Manager, Waste Bin) while viewing an image now properly closes the viewer before transitioning.
- **Waste bin thumbnail distortion** — Waste bin thumbnails now use the same `onload` handler and size parameter as the browse grid, fixing distorted aspect ratios and low-quality previews.

## [0.3.0] - 2026-03-04

### Added

- **Location map** — Photos with GPS EXIF data now show an interactive map in the Info panel's Location section, powered by OpenFreeMap and MapLibre GL JS. Includes 2D/3D view switching and a link to open the location on OpenStreetMap.
- **High-quality thumbnails** — New "Thumbnails" setting (Standard / High) in the Settings menu. High mode generates thumbnails at the actual display size × device pixel ratio using bicubic resampling, producing visibly sharper thumbnails on retina displays. Standard mode preserves the fast EXIF thumbnail behavior.

### Changed

- **Improved thumbnail resize quality** — Thumbnail generation now uses bicubic (Catmull-Rom) interpolation instead of nearest-neighbor, and JPEG quality is bumped from 80 to 85.
- **Architecture documentation** — Added ADR-0011 (scan cache and deferred EXIF), ADR-0012 (client-side settings via localStorage), ADR-0013 (MapLibre GL for location maps), ADR-0014 (thumbnail quality tiers). Updated arc42 to reflect current caching, performance, and settings architecture. Updated ADR-0005 to reflect removal of confirmation dialogs.

## [0.2.0] - 2026-03-02

### Changed

- **Copy/move without confirmation** — Copy (F5) and Move (F6) in File Manager mode now execute immediately without a confirmation dialog, reducing friction during photo culling workflows.
- **Clearer deletion mark visual** — Images marked for deletion now show a dark semi-transparent overlay with a waste bin icon instead of the previous subtle opacity reduction, making the deletion state immediately obvious.
- **File Manager default layout** — Left pane now defaults to grid view and right pane to list view, with a 60/40 width split favoring the left pane for a better photo culling workflow.

### Fixed

- **Marking for deletion no longer causes grid jump** — Marking or unmarking images for deletion in browse mode now toggles classes in-place instead of re-rendering the entire container, preserving scroll position across grid, justified, and list views.
- **Scroll position preserved** — Closing the fullscreen viewer and reloading a pane after copy/move now restore the browse grid to the same scroll position instead of jumping to the top. Works across grid, justified, and list views.
- **Mode switching preserves state** — Browse and File Manager views are created once and hidden/shown on mode switch instead of being destroyed and rebuilt. Scroll position, loaded thumbnails, selections, and folder state are all preserved. File Manager opens both panes in the folder you were browsing; switching back restores the active pane's folder.
- **Uniform File Manager button widths** — Copy, Move, and Delete buttons in the File Manager center column now stretch to equal width instead of sizing to their label.

### Added

- **Justified layout** — New default browse view that scales images to fill each row edge-to-edge while preserving aspect ratios, with 1px gaps between photos. Directories render in the standard grid style above the justified images. Available in the View menu alongside Grid and List. Layout reflows on window resize. Similar to Flickr or Google Photos.

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
