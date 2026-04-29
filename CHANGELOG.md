# Changelog

*Last modified: 2026-04-29*

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- **Lazy loading for search/filter results** — Scrolling through EXIF-filtered or text-searched results now loads all matching photos, not just the first 100. When the user approaches the bottom of the currently loaded set, the next page is fetched from the server and appended to the grid seamlessly. Works in both the cross-library search panel and the per-library filter view. Backend limits raised to match: cross-library search now accepts up to 500 results per page, and the per-library internal fetch window scales with the requested offset so any page is reachable.

- **EXIF filter panel** — Slider-based filtering by shutter speed, aperture, focal length, ISO, camera, lens, and film simulation, available in two places:
  - **Per-library** — "Filter" button in the library detail header opens a sidebar filter panel to the left of the photo grid. Ranges are scoped to the current library.
  - **Cross-library search** — "Search" button on the library list opens the same sidebar with a library selector (defaults to "All libraries"), searching across the full indexed collection.
  - Numeric EXIF values are normalised at index time into canonical floats (`numeric_value` column in `exif_index`), handling mixed camera formats (`"1/500 s"`, `"0.004 sec"`, `"f/2.8"`, `"50/1"`, etc.). Sliders use log scale for shutter speed, aperture, and ISO (matching photographic stops); linear for focal length.
  - Custom dual-handle drag slider — no browser `<input type="range">`; handles are shaped triangles that snap correctly at the track edges.
  - Text filters (camera, lens, film sim) use exact value matching after stripping any surrounding quotes from the EXIF index.
  - 300 ms debounce; matched photo count shown in the sidebar. Results reuse the full grid/list/justified view with viewer and metadata panel.
  - New API endpoints: `GET /api/library/{id}/exif-ranges`, `GET /api/library/search`, `GET /api/library/exif-ranges` (global), `GET /api/library/exif-values` (distinct text values).

- **Global channel output** — Channel export output moves from `~/.unterlumen/libraries/<id>/channels/<slug>/` to `~/.unterlumen/channels/<slug>/`, shared across all libraries. Albums from any library now publish into the same channel directory, and site-export channels accumulate albums from all libraries into one unified site. Publishing still reads photos from a specific library; only the output path is now global.

- **Channel path buttons** — Each channel row in the Channels dialog now shows three new buttons:
  - *Copy path* — copies the channel output directory to the clipboard.
  - *Show in Files* — opens the directory in Finder (macOS), Explorer (Windows), or the default file manager (Linux); creates the directory if it doesn't exist yet.
  - *Open in Commander* — closes the dialog and navigates the left Commander pane to the channel directory.

- **Channels accessible from library list** — "Channels" button added to the library list header alongside "New library", so channel config is reachable without first opening a specific library. Rebuild site also no longer requires a library context.

- **Favicon & dock icon** — SVG favicon with dark-mode adaptive bar; 180×180 apple-touch-icon for Safari "Add to Dock".

- **Multi-Album Static Website** — Channels now offer a third export mode: *Multi-album site*. Each publish adds a new album subfolder (`site/albums/<postID>/`) with full-res photos, thumbnails, a download ZIP, and a standalone `index.html`. A `site.json` statefile records every published album (including the full photo list so pages can be rebuilt without re-exporting); from it the root `site/index.html` is regenerated on each publish. Albums are ordered newest-first by publish date — inserting an older album (via the date picker) places it correctly in the grid. Old album folders can be deleted locally after rsyncing — the statefile remembers them. Transfer with `rsync -avz site/ user@host:/var/www/` and only deltas are sent.

- **Light/Dark Theme Toggle** — All generated gallery and website pages now include a theme toggle button. Single-gallery pages default to dark; multi-album site pages default to whatever theme is configured on the channel. Visitor preference is stored in `localStorage` under the key `ul-theme` and shared across all pages of a site, including when navigating back via the browser's back/forward cache. The website uses a single shared `assets/style.css` with CSS custom properties so both themes are defined once; `assets/toggle.js` is fully static (reads the default from a `data-default-theme` attribute) and safe to cache indefinitely.

- **Rebuild Site** — A "Rebuild site" button appears in the channel list for site-export channels. Clicking it regenerates `assets/style.css`, `assets/toggle.js`, every album's `index.html`, and the root `index.html` from the existing `site.json` statefile — no photo re-export needed. Albums published before photo metadata was added to the statefile are handled by scanning the album directory on disk. Use this after changing the channel's default theme or site title.

- **Static Website Gallery Export** — Any channel can opt in to HTML gallery generation via a "Generate HTML gallery on publish" toggle in Channel Settings (the built-in Website channel has it enabled by default). When publishing to a gallery-capable channel, a `<title>` field appears in the Publish modal; the backend generates a self-contained `index.html` alongside the exported photos in a per-publish subfolder (`channels/<slug>/<postID>/`). The gallery uses static HTML with native `loading="lazy"` for SEO-friendly lazy loading without JavaScript. The folder can be transferred directly to any web host via `scp` or `rsync`.

- **Publish to Channels** — From library mode, select photos and publish them to a named channel (Instagram, Mastodon, Website, or custom). Each publish action:
  - Writes an XMP sidecar (`.xmp` alongside the original) using a custom `xmlns:ul` namespace to record channel, account, post ID, and timestamp — non-destructive and portable. Merge-safe: existing sidecar namespaces (darktable, Lightroom, etc.) are preserved.
  - Caches the publish record in the library DB (`photo_meta` keys `published:<channel>`, `published:<channel>:account`, `published:<channel>:postid`) for fast search; rebuilt automatically from sidecars on re-index.
  - Exports a platform-optimised copy to `~/.unterlumen/channels/<channel>/` with filename `<channel>_<datetime>_<basename>.<ext>`.
  - Multi-photo publishes share a **PostID** (24-char hex) linking all photos as one grouped post (e.g. Instagram carousel).
  - Three built-in channel presets (Instagram 1080px JPEG, Mastodon 1920px JPEG, Website 2400px JPEG), all fully editable.
  - **Accounts** — channels support named sub-accounts (e.g. two Mastodon logins). The publish modal shows an account dropdown when a channel has sub-accounts.
  - **Channel management UI** — "Channels" button in the library header opens a settings modal to add, edit, or delete channels. Each channel has format, quality, scale, EXIF mode, an optional handler identifier (for future upload automation), free-form handler config (key→value), and zero or more named accounts (each with their own key→value config). Channel configurations are stored globally in `~/.unterlumen/channels.json`.
  - Publish button in the library toolbar becomes active when photos are selected; a modal lets you choose channel, account, and date/time (defaults to now, can be back-dated).

- **DAM Libraries** — New "Libraries" tab (4th mode) adds optional Digital Asset Management. A library indexes a photo folder recursively into `~/.unterlumen/libraries/<id>/library.db` (SQLite via `modernc.org/sqlite` — no CGo, no external process). Features:
  - Create a library from any folder; background indexing walks all subfolders.
  - Photos are identified by SHA-256 content hash: metadata survives external renames.
  - Fast re-scan path using mtime + file size to skip unchanged files.
  - HQ thumbnails (1200px, JPEG 85) stored on disk keyed by content hash, sharded into `thumbs/<prefix>/` subdirectories.
  - EXIF metadata stored in an EAV table (`exif_index`) for full-text and field-filtered search.
  - User-defined key/value annotations per photo (`photo_meta` EAV table); inline editable in the annotation panel.
  - Missing photos (deleted from source) are marked `status='missing'`; metadata and thumbnails are preserved.
  - Re-indexing progress streamed via Server-Sent Events.
  - `--lib-dir` CLI flag and `UNTERLUMEN_LIB_DIR` environment variable override the library root (default: `~/.unterlumen`); useful for testing with temporary directories.

### Changed

- **Internal refactoring (ADR-0015)** — No user-visible behaviour changes; structural cleanup only.
  - Go: `src/internal/api/` flat package split into domain subpackages: `browse`, `export`, `fileops`, `location`, `batchrename`. Path-traversal guard extracted to `src/internal/pathguard`.
  - Go: `src/internal/media/exif.go` (735 lines) split into `exif.go`, `orientation.go`, `thumbnail.go`, `fujifilm.go`, `aspectratio.go`. `export.go` extraction of `decodeSourceImage`, `scaleImage`, `encodeToFormat`.
  - Go: Unit tests added for `pathguard`, `batchrename` (pattern, sanitize, conflict), `media` (EXIF date parsing, aspect ratio labels, export dimension modes).
  - JS: `browse.js` (1 174 lines) split into `browse.js` (orchestration), `browse-selection.js`, `browse-keyboard.js`, `browse-grid.js`, `browse-list.js`, `browse-justified.js`.
  - JS: `app.js` (767 lines) split into `app.js` (orchestration), `app-theme.js`, `app-wastebin.js`, `app-keyboard.js`.

### Added

- **E2E integration tests** — Playwright test suite covering browse, image viewer, EXIF overlay badges, and wastebin mark/restore workflow. Tests run headlessly via `cd e2e && npm test` and are integrated into GitHub Actions CI (`.github/workflows/e2e.yml`), triggering on every push and pull request.

- **Slideshow** — New "Slideshow" button in the browse toolbar (between Tools and the status bar). Operates on selected images, or all images in the current folder if nothing is selected. An options dialog lets you set the delay between images (1–60 s), choose a transition effect (Fade, Slide, Zoom, or Instant), and pick a display style: Single image, Ken Burns (slow animated pan and zoom), 2-up (two images side by side), or 4-up (2×2 grid). Optional audio: choose a local audio file or a folder of tracks (shuffled, looping). The player shows a minimal HUD with Prev / Pause-Play / Next / Close controls that autohides after 3 seconds; keyboard shortcuts Space (pause/resume), ← / → (navigate), and Esc (close) are supported.

- **Dependency check** — New "Check dependencies" entry in the Settings menu opens a modal listing the status of all required external tools (ffmpeg, exiftool, sips on macOS). Missing or misconfigured tools are shown with a plain-language explanation and platform-specific install instructions.
- **Container image** — Docker image published to GHCR (`ghcr.io/bjblazko/unterlumen`) for `linux/amd64` and `linux/arm64`. Image bundles ffmpeg and exiftool; defaults to server mode with `/photos` as the root.

## [0.5.0] - 2026-04-08

### Added

- **Convert & Export** — Export selected images to JPEG, PNG, or WebP from the Tools menu. Options include target quality (JPEG/WebP), flexible scaling (original size, percentage, or max width/height), and EXIF metadata control (strip all, keep all, or keep all except GPS location). The modal shows per-file estimated output size and output pixel dimensions, with a toggle between fast heuristic and exact in-memory encoding. Exact estimation runs file-by-file with a progress bar labeled "Calculating exact sizes…", a counter, and an Abort button. If any output dimension exceeds the source, an upscale warning badge (!) with a tooltip is shown. Estimation errors (e.g. ffmpeg failures for WebP) are displayed inline in the file row. Totals row sums all input and output bytes, aligned under the file list columns. In local mode (no `UNTERLUMEN_ROOT_PATH`), files can be saved directly to a chosen folder or downloaded as a ZIP. In server mode (`UNTERLUMEN_ROOT_PATH` set), ZIP download is the only output option. JPEG and PNG use Go's native encoder with CatmullRom resampling; WebP uses ffmpeg with Lanczos. GPS stripping requires exiftool.
- **`POST /api/export/estimate`** — Returns per-file estimated output size for given format/quality/scale options, using either heuristic formulas or actual in-memory encoding.
- **`POST /api/export/zip`** — Converts and streams all selected files as a ZIP archive.
- **`POST /api/export/save`** — Converts and writes files to a local directory on disk (local mode only).

- **Selection-filtered viewer** — When 2 or more images are selected in the gallery, opening the fullscreen viewer scopes the filmstrip and prev/next navigation to only the selected images. Opening from a single or no selection retains the previous behavior (all images in the folder). A "Deselect" button is now visible in the status bar whenever images are selected, as an alternative to pressing Escape.

- **Film strip in viewer** — Horizontal thumbnail strip below the main image in fullscreen viewer mode. Toggle via toolbar checkbox or `F` key. Click any thumbnail to jump directly to that image. Auto-scrolls to keep the current image visible. Thumbnails are lazy-loaded for performance. Hidden by default; also hides when UI is hidden (`H` key).
- **Batch Rename** — Pattern-based batch renaming of photos using EXIF metadata. Accessible from the Tools dropdown ("Batch (Metadata)") and the commander Rename dropdown. Supports placeholders for date (`{YYYY}`, `{MM}`, `{DD}`, etc.), camera metadata (`{make}`, `{model}`, `{lens}`, `{filmsim}`, `{iso}`, `{aperture}`, `{focal}`, `{shutter}`), original filename (`{original}`), and auto-incrementing counter (`{seq}`). Features color-coded draggable token pills with tooltips, a colored highlight overlay in the pattern input, a horizontally scrollable live preview of resulting filenames, conflict detection with automatic suffix resolution, progress bar during execution, and SMB-safe filename sanitization.
- **Simple Rename** — Single-file rename option ("Single") in the Tools dropdown and commander Rename dropdown. Automatically disabled when multiple files are selected.
- **`POST /api/batch-rename/preview`** — Returns resolved filenames for a given pattern and file list.
- **`POST /api/batch-rename/execute`** — Renames files on disk using a two-pass strategy (temp names first) to handle circular renames safely.

### Changed

- **Reusable toggle switch** — All boolean toggles (Show Names, Show Details, Interface, Film Strip) now use a unified iOS-style pill toggle component with OFF/ON labels, replacing the inconsistent mix of buttons and checkboxes. CSS-only animation with accent orange for the on state.
- **Unified dropdown system** — Settings, View, Tools, and commander Rename menus now share a single `Dropdown.init()` utility and consistent CSS classes (`dropdown-wrap`, `dropdown-btn`, `dropdown-menu`, etc.), eliminating duplicated toggle logic and styling. Dropdown menus size to their content (`width: max-content`) and overflow narrow parent containers gracefully.

## [0.4.0] - 2026-03-17

### Added

- **Tools dropdown** — New "Tools" dropdown in the browse controls bar, directly adjacent to the View button. Operates on selected (or focused) images. Checks for exiftool availability on first open and shows a message if missing.
- **Set Location** — Interactive map picker (MapLibre GL) to manually set GPS coordinates on images. Click the map to place a marker, or type coordinates directly. Shows a confirmation step before writing. Preserves all existing EXIF data including maker notes. The map opens pre-centered on the image's existing GPS (if any), then falls back to a remembered user location, then requests browser geolocation once (cached in `localStorage`), then world view.
- **Remove Geolocation** — "Remove" button in the Tools → Geolocation row strips GPS EXIF tags from selected images via exiftool. Shows a confirmation modal before writing; reports success or failure inline.
- **Geolocation row in Tools menu** — The Tools menu shows a "Geolocation" label with "Set" / "Remove" buttons. The label updates to show the count of actionable images (e.g. "Geolocation (3 images)") when the menu is opened.
- **exiftool availability check** — `GET /api/tools/check` endpoint. Tools dropdown shows a message when exiftool is not installed.
- **Folder operations** — Copy, move, and delete now work on entire folders in the commander. Copying a folder enumerates its contents and shows per-file progress; moving a folder uses fast rename when on the same filesystem, with copy+delete fallback. Deleting a folder shows a confirmation dialog and removes the folder and all its contents immediately (bypasses the wastebin).
- **Progress dialog** — A reusable modal progress dialog shows per-item progress with cancel support for multi-file operations (copy, move, delete, set/remove location). Displays a progress bar, current filename, and error summary. Only shown for operations with more than 1 item (copy/move) or more than 5 items (delete, location).
- **Commander: New Folder** — "Folder" button in the commander actions column creates a subfolder in the active pane's current directory.
- **Commander: Rename** — "Rename" button renames the focused item in the active pane. Enabled when an item is focused.
- **Commander: panel captions** — Each pane now shows a "From" or "To" label indicating the active (source) and inactive (destination) pane.
- **"Photo Taken" sort** — New sort option in the view menu sorts photos by EXIF `DateTimeOriginal`. Photos without EXIF data always appear last regardless of sort direction. The existing "Date" option is renamed "File Modified".
- **Aspect ratio display** — Images show their aspect ratio (e.g. `3:2`, `16:9`) as a badge on thumbnails (when "Show details" is on) and as a row in the info panel. Unusual crops show "Custom Crop".
- **Thumbnail overlays** — New "Show details" toggle in the View menu displays colored metadata badges on thumbnails: file type (JPEG, HEIF, PNG, GIF, WebP), GPS location pin, and Fujifilm film simulation name. Badges load asynchronously. Works in grid, justified, and list views.
- **Orientation label in info panel** — The Orientation field now shows a human-readable name (e.g. "Normal", "Rotated 90° CW") instead of the raw EXIF integer.
- **`POST /api/mkdir`** — Creates a directory at the given path within the root boundary.
- **`POST /api/rename`** — Renames a file or directory to a new base name within the same parent directory.
- **`POST /api/list-recursive`** — Recursively lists all files and subdirectories under a given path. Used for per-file copy/move progress on folders.

### Changed

- **Workflow-oriented UI** — Replaced the tab-style mode switcher with a connected chevron stepper reflecting the photographer's natural workflow: Select (1), Review (2), Organize (3), each with a representative icon. Active step is orange; mode switches animate with a directional slide. The Review step shows a count badge when photos are marked for deletion.
- **Commander direction arrow** — Direction is now conveyed by a large translucent arrow SVG in the center actions panel, which flips left/right based on the active pane. Copy/Move button labels no longer show a selection count.
- **Commander: restructured actions** — Delete, Folder, and Rename buttons are grouped at the top of the actions column (non-directional). Copy and Move buttons remain near the directional arrow.
- **Inline SVG logo** — The header logo is now an inline SVG, removing the dependency on an external PNG file.
- **Thumbnail overlays on by default** — The "Show details" overlay badges (file type, GPS, film simulation) are now enabled by default.

### Fixed

- **Commander: Tools (Set/Remove Location) not working** — In commander mode, the Set Location modal never opened and Remove Location had no effect. The `Commander` class was not passing `onToolInvoke` to its `BrowsePane` instances, so tool invocations were silently dropped.
- **Grid overlay badges refresh after location operations** — After Set Location or Remove Location completes, GPS pin badges on grid items update immediately in-place. The info panel also refreshes if the focused file was changed.
- **Cache invalidation for location operations** — `handleSetLocation` and `handleRemoveLocation` now correctly invalidate the scan cache after writing.
- **Tools menu geolocation label wrapping** — The "Geolocation (N images)" label in the Tools menu no longer wraps across two lines.
- **Set Location duplicate alert** — After confirming Set Location, a redundant browser `alert()` no longer appears; the result is shown inline in the modal footer.
- **"Photo Taken" sort order** — Photos whose EXIF date equalled their filesystem mtime were silently treated as undated, causing them to sort last. EXIF dates are now always stored.
- **Date display in list view** — Dates in list view and the info panel "Modified" row are now displayed in ISO format (`2024-03-01 15:04:05`).
- **EXIF date formatting** — Info panel Dates section now displays dates as `2016-07-16 20:24:53` (was `2016:07:16 20:24:53`). UTC offset appended when present.
- **Commander buttons showing stale count** — Action buttons no longer show "(1)" when no images are explicitly selected.
- **Sticky header in commander and waste bin views** — The breadcrumb and controls stay pinned at the top while scrolling.
- **Commander copy no longer resets source pane scroll** — After a copy operation, only the destination pane reloads.
- **Escape navigates up in commander mode** — Pressing Escape navigates the active pane to its parent directory.
- **Deselect on Escape** — Pressing Escape clears the current selection before navigating up (both browse and commander modes).
- **Deselect on void click** — Clicking an empty gap in the grid, justified, or list view clears the selection.
- **Justified grid not resizing after info panel close** — Closing the info panel now immediately relays out the justified grid to fill the full available space.
- **Info panel in fullscreen mode** — The info panel (I key) now works in full UI-hidden mode (H key).
- **Map zoom controls** — The location map now has +/− zoom buttons for reliable zooming across all input methods.
- **HEIF date extraction** — Background EXIF date extraction now handles HEIF/HEIC/HIF files via embedded EXIF fallback, fixing missing dates when sorting by date.

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
