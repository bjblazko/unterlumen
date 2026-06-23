# Changelog

*Last modified: 2026-06-23*
All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- **Site logo + persistent masthead** — Multi-album site pages now show a persistent two-row header on every generated page. Row 1 (always visible): optional logo image + site name (links back to the root index on sub-pages). Row 2 (sub-pages only): back arrow + current page title (album name, "About", or "Legal Notice"). The root index keeps a single-row header with the site name as `<h1>`. An optional **site logo** can be uploaded per channel via the Website tab in channel settings (stored as `site/assets/logo.jpg`); upload and remove work the same way as the author portrait. "Rebuild site" regenerates all pages with the current logo and site name.

- **Website About, Imprint & Contact pages** — Multi-album site channels can now publish personal identity pages alongside photo albums:
  - **About page** (`about.html`) — write Markdown text in the new Website tab of channel settings. Supports an optional **author portrait** (uploaded as `assets/avatar.jpg`) displayed as a circular portrait next to the text.
  - **Legal / Imprint page** (`legal.html`) — enter Markdown text for the legal notice. Required for commercial or semi-commercial sites in many countries.
  - **Contact links in footer** — optional email address and website/social URL appear in the footer of every site page (root index, album pages, About, Imprint).
  - **Navigation** — when About or Imprint pages are configured, nav links appear in the root index header; album page footers link back to both pages.
  - Markdown is rendered to HTML at publish / rebuild time (goldmark, GFM); no client-side parser needed.
  - "Rebuild site" regenerates About and Imprint pages from the current channel config.

- **Channel settings dialog tabs** — The channel editor is reorganized into four tabs (**Export**, **Website**, **Output**, **Advanced**) to keep the growing number of options manageable. The Website tab shows a hint when the export mode is not set to "Multi-album site".

- **Add photos to existing gallery / album** — The publish dialog shows an **"Add to"** dropdown when publishing to a gallery or site channel that already has galleries. Selecting one appends the new photos to the same folder, regenerates the HTML and ZIP, and records an `updatedAt` timestamp — without changing the original `publishedAt` that controls album sort order. The dropdown label shows a date range (e.g. "Summer 2025 (12) · June – August 2025") once photos are added across months. Selecting "New gallery" creates a fresh folder as before.

- **Date field in publish dialog** — The date/time input is now a plain date picker. A **`+ Time`** button reveals the time field for cases where precise ordering within a day matters. Defaults to today (noon UTC). When "New gallery" is selected the date resets to today; when an existing gallery is selected the date also defaults to today (it becomes the `updatedAt` date on the album, not the sort date). The helper text updates to clarify which role the date plays.

- **`gallery.json` statefile for single-gallery channels** — Each published gallery folder now contains a `gallery.json` tracking the photo list, title, `publishedAt`, `updatedAt`, and count. Enables listing and adding to galleries without scanning HTML files.

- **`GET /api/channels/{slug}/galleries`** — Lists existing galleries/albums for a channel (galleryExport: reads `gallery.json` files; siteExport: reads `site.json`). Returns `[{postID, title, publishedAt, updatedAt, photoCount}]` sorted newest first.

- **Library filter — sort by date taken** — Filter results in library mode are now sorted by photo taken date (newest first) by default. The existing sort dropdown gains a "Photo Taken" option as the new default; the ↑/↓ toggle reverses the order. Photos without EXIF date sort to the bottom.

- **Library filter — date range filter** — A "Date taken" filter section with From/To date inputs is now available in the library filter panel. Restricts results to photos whose EXIF capture date falls within the chosen range. Works across all libraries.

- **Publish from filter results** — The Publish button in the single-library detail view is now active when photos are selected from EXIF filter results, not only from the folder tree. In the cross-library list view a new Publish button appears in the header; it enables whenever filter results have a selection. Photos are published using their already-resolved IDs (no extra API lookups). If a cross-library selection spans multiple libraries, publish (plain and gallery/site export) runs one call per library group. ZIP download requires all selected photos to be from one library.

- **In-app folder picker** — A new in-browser directory browser replaces the OS-native folder picker (`osascript`/`zenity`) in the export dialog. Works in desktop and server/container mode alike. New `GET /api/browse/dirs` endpoint returns subdirectories at a given path.

- **Export "Save to folder" in server mode** — The export dialog now shows the full output section (folder save + ZIP download) in server/container mode. Folder paths can be typed as root-relative (e.g. `exports/batch`); absolute paths are still accepted in desktop mode.

- **Channel publish "Download ZIP"** — The publish dialog gains a **Download ZIP** button alongside the existing **Publish** button. It exports selected photos with channel settings and delivers them as a ZIP download without writing to the channel folder or updating XMP sidecars. Available in all modes.

- **Selective cache clearing from Tools menu** — The Tools menu now has a "Cache" section with a "Clear cache" button. In browse mode it evicts all cached thumbnails for the selected file(s); if a folder is focused it recursively clears cache for every file inside. In library mode the same button is available for selected photos. New backend endpoint `POST /api/cache/evict` accepts a list of absolute paths and calls `EvictFile` per file (or walks directories recursively). Feedback: Tools button disables while running, toast shows "Clearing cache…" then "Cache cleared for N file(s)".

- **Library scan shortcuts in Tools menu** — When browsing inside a library (`App.mode === 'library'`), the Tools menu gains a "Library scan" section with "Scan new", "Re-index", and "Cleanup" buttons. These call the same SSE-backed library API endpoints as the library overview card buttons. Progress streams as a live toast ("Scanning… 15/100 · filename"); on completion a toast shows "Done — N photos". Existing "Make library" entry hides when in library mode.

- **Chip autocomplete filter in library search** — The library filter panel now includes a "More filters" chip input below the existing sliders and dropdowns. Click **Add filter…** to pick a filter namespace and then a value. Multiple chips combine as AND conditions. Chips can be removed with ×, Backspace on an empty input removes the last chip, and Reset filters clears all chips. New backend endpoints: `GET /api/library/meta-keys`, `GET /api/library/meta-values`, `GET /api/library/album-titles`, `GET /api/library/exif-fields`.

- **All indexed EXIF fields available as chip namespaces** — The chip filter now exposes every distinct field present in a library's `exif_index`, not just a fixed set. Fields like `Orientation`, `Make`, `Software`, `ColorSpace`, `MeteringMode`, `SceneCaptureType`, and any Fujifilm-specific tags appear automatically. Fields already covered by numeric sliders (shutter, aperture, focal length, ISO) are excluded to avoid overlap.

- **Human-readable EXIF value labels in chip filter and info panel** — A new shared `exif-labels.js` module provides decode tables for Orientation, Flash, White Balance, Metering Mode, Exposure Program, Exposure Mode, Color Space, Scene Capture Type, and more. The chip autocomplete shows "Normal" instead of "1" for Orientation, "Fired" / "No flash" for Flash, etc. The info panel uses the same tables (eliminating duplicated decode logic). The raw stored value is still used for the backend query; only the display label changes.

- **SEO-ready static site output** — Published multi-album sites now include:
  - **Slug-based album folders** (`albums/paula-at-home/` instead of `albums/<postID>/`). Slugs are auto-derived from the album title and stored in `site.json`; albums published before this change fall back to their postID folder for backward compatibility.
  - **Static `<figure>` tags** — Image elements are now generated server-side in Go instead of injected by JavaScript at runtime. Search crawlers see all photos immediately without executing JS. The first two images per gallery use `loading="eager"` (better Largest Contentful Paint); the rest stay lazy.
  - **`<meta name="description">`** — Auto-generated from photo count and date range.
  - **JSON-LD structured data** — An `ImageGallery` (album pages) or `CollectionPage` (root index) schema block is embedded in every page `<head>`.
  - **Better image alt text** — Changed from filename to `"Album Title – Photo N of Total"`.
  - **Unique `<title>` tags on album pages** — Now formatted as "Album Title | Site Title".
  - **`robots.txt`** — Generated in the site root on every publish and rebuild. If a Site URL is configured, a `Sitemap:` line is included.
  - **`sitemap.xml`** — Generated when a Site URL is set in channel settings; lists the index page and every album URL as absolute URIs.
  - **`<link rel="canonical">` + OG tags** — Emitted when a Site URL is configured: canonical link, `og:title`, `og:description`, `og:image`.
  - **Site URL field in channel settings** — Optional `https://example.com` base URL in the channel settings panel. When set, enables canonical links, OG tags, and absolute sitemap URLs.

### Changed

- **Generated website footer** — The "Built with Unterlumen" footer now links to the product page at huepattl.de and includes an orange heart icon. A GitHub icon linking to the repository is added alongside it. Applies to multi-album site (root index and album pages) and single-gallery exports.
- **Generated website icons** — Replaced thin Unicode arrow characters with clean SVG icons: chevron arrows for lightbox navigation and back-to-albums, a download icon for the ZIP link. Applies to both site and single-gallery templates.
- **Generated site album date** — The date shown under each album card on the site index now shows a range (e.g. "June – August 2025") when photos have been added across different months, instead of only the original publish date.

### Fixed

- **Full-size gallery images shown rotated 180° in lightbox** — When exporting with ExifMode "keep" or "keep_no_gps", the pipeline correctly baked the EXIF orientation into pixels but failed to reset the Orientation tag to 1 in the output. The `exiftool -Orientation=1` invocation was silently ignored by exiftool 13.x when a PrintConv string value like `"Horizontal (normal)"` is expected instead of the integer `1`. The fix adds the `-n` flag (raw numeric values) to the exiftool args so `-Orientation=1` is written as the integer 1 ("Horizontal/normal"), matching the already-rotated pixel data.

- **Portrait HEIC/HEIF images exported as landscape** — `sips` (the macOS HEIF decoder used during export) preserves the EXIF Orientation tag in its JPEG output without baking the rotation into pixels. The export pipeline was not reading that tag from the sips output, so it decoded raw landscape pixels and exported them as-is — leaving portrait photos rotated 90°. The pipeline now reads the orientation from the sips JPEG and applies the pixel-space rotation before encoding. The same treatment is applied to the `heif-convert` (Linux) path for consistency. Additionally, when EXIF is re-injected with "keep" or "keep_no_gps" ExifMode, the `Orientation` tag is now reset to 1 after copying so it matches the already-rotated pixels and prevents double-rotation in viewers. WebP exports with a rotated source additionally had a dimension-swap bug in the ffmpeg scale filter; that is also fixed.

- **Album title written to `photo_meta` immediately at publish time** — Previously `published:<channel>:title` was only stored when the library was re-indexed (via `indexSidecar` reading the XMP). Now both the regular and gallery publish paths write the title directly to `photo_meta` at publish time, so `album:` chip filters and the album-titles autocomplete reflect new albums without requiring a manual re-index. Existing publications made before this fix can be backfilled by running Re-index from the Tools menu.

- **Chip autocomplete dropdown visibility** — The dropdown now has an accent-coloured border (matching the selection ring on photos), a layered drop shadow with a warm orange ambient glow visible on dark backgrounds, and a minimum width of 280 px so it extends beyond the narrow sidebar rather than being clipped to its width.

- **Publish dialog — album title required for site export** — Publishing to a multi-album site channel now validates that an album title is provided. If the field is left blank, an error message is shown and publishing is blocked rather than silently falling back to a titleless export.

- **Cross-library gallery/site export** — Publishing photos selected from a cross-library search to a gallery or site album no longer fails with "Gallery export is not supported when photos span multiple libraries." Each library group is now published sequentially into the same album; subsequent groups are added to the album created by the first call. Progress shows "Library N of M:" when multiple libraries are involved.

- **Keyboard shortcuts firing inside date inputs (Safari)** — Arrow keys, number keys, H, Backspace, and Delete were incorrectly intercepted while a date input was focused in Safari. Safari's `input[type="date"]` sends `keydown` events with a retargeted `e.target` (the outer `<input>` element) rather than the internal year/month/day segment that actually has focus, so the previous `e.target.tagName === 'INPUT'` guard only worked for some segments. The input focus guard is now centralised in `GlobalKeyboard._isInputFocused(e)`, which combines a `focusin`/`focusout` flag, `e.composedPath()` traversal, and a `document.activeElement` fallback to reliably detect any focused form element across shadow DOM boundaries. All redundant per-shortcut `tagName` checks have been removed. The `input.blur()` workaround in the date filter that forced keyboard focus away after every date change is also removed.

## [0.9.4] - 2026-06-23

### Fixed

- **WebP export on macOS failing with "Encoder not found"** — The default Homebrew ffmpeg (8.x) is built without `libwebp`, causing WebP export to fail silently with a cryptic error. The fix detects ffmpeg's WebP encoder support at startup; when absent, it falls back to `cwebp` (from `brew install webp`). The cwebp path uses ffmpeg for decoding (handling HEIC/HIF multi-tile files with `-pix_fmt rgb24` to avoid 16-bit PNG incompatibility and `-vf` scale filter conflicts), then `cwebp -resize` for scaling and encoding. The export modal disables the WebP button when neither encoder is available; the deps dialog shows `cwebp` as a separate entry with per-OS install instructions when ffmpeg lacks `libwebp`.

## [0.9.3] - 2026-06-05

### Fixed

- **HEIC thumbnails failing for standard Fujifilm HEIC files** — Added `libheif-examples` (`heif-convert`) to the Docker image. Fujifilm's standard HEIC files (`DSCF*.heic`) carry no embedded JPEG preview stream that ffmpeg can parse, causing ffprobe to report "moov atom not found" and all fallback paths to fail. `heif-convert` from libheif decodes these files natively. Film Simulation Bracket files (`_DSF*.heic`) were unaffected because they embed a JPEG preview stream that ffprobe can find directly. Users running unterlumen outside Docker should install `libheif-examples` (Debian/Ubuntu) or `libheif` (Homebrew/Arch) alongside ffmpeg for full HEIF coverage.

## [0.9.2] - 2026-06-04

### Fixed

- **Photo viewer fills available space on large monitors** — The viewer's "fit" zoom mode now correctly upscales photos to fill the viewport on high-resolution displays. Previously `max-width/max-height: 100%` prevented upscaling, so photos whose native CSS-pixel dimensions were smaller than the viewport (e.g. on a 32" 1440p display with DPR=1) rendered at natural size with large margins. Changed to `width/height: 100%; object-fit: contain`, and updated the crop-tool overlay positioning to compute the rendered image rect within the `object-fit` box.

- **About dialog shows correct version in container builds** — The Docker image now receives the release tag via a build-arg (`VERSION`) and bakes it into the binary with `-X main.Version`. Previously the container always displayed "dev".

- **Library creation works in server/container mode** — Creating a library by right-clicking a folder in Browse mode (or typing a path manually in the New Library dialog) now resolves the path correctly when Unterlumen is started with a root other than `/` (e.g. `UNTERLUMEN_ROOT_PATH=/photos` in a container). Previously the submitted path was interpreted as a filesystem-absolute path, causing "sourcePath must be an existing directory" for any root-relative path. The dialog placeholder is also updated to show the expected root-relative format.

### Changed

- **Camera × lens bars in folder info panel** — The CAMERA section in the library folder info panel now shows proportional horizontal bars instead of plain text rows. Each entry displays the count label (`Nx`) in front of the bar, with the camera/lens name printed below the bar on its own line. Long lens strings wrap naturally without disrupting the bar layout.

## [0.9.1] - 2026-05-26

### Added

- **Instant folder info in library mode** — The folder info panel in library mode now loads from the indexed database instead of walking the filesystem on each click. Photo count, total size, date range, subfolder treemap, and file type breakdown are returned in milliseconds. After each scan, all folder stats are pre-computed in a background goroutine so first-access latency is zero. Browse-mode folder stats (filesystem walk with depth histogram) are unchanged. Treemap cells now show a tooltip with the full folder name, size, and item count on hover — especially useful for small cells where labels are hidden.

- **Library thumbnail fast path in organizer mode** — When browsing a folder in the organizer that is covered by a library's source path, thumbnails for already-indexed photos are served from the pre-generated library JPEG (1200 px) rather than decoded from the original file. This eliminates sips/ffmpeg processing for HEIF files on every browse-cache miss. A small library name badge appears in the breadcrumb row to indicate the current folder is tracked by a library.

- **Auto-index new files after copy/move into library folder** — When files are successfully copied or moved into a library folder, an incremental scan (`scan-new`) starts automatically in the background. Newly arrived files are indexed and thumbnailed without requiring a manual "Scan new" trigger. The scan joins the normal SSE progress stream if a client is watching.

### Fixed

- **Navigation no longer blocked during folder load** — Clicking a subfolder while a large directory is still loading now works immediately. The previous in-flight request is cancelled (browser shows it as cancelled in DevTools) and the new folder loads right away. Previously the click was silently ignored until the current load completed.

## [0.9.0] - 2026-05-24

### Added

- **About dialog** — Clicking the Unterlumen logo/title in the header opens an About dialog with the GitHub repository link, author contact, and legal disclaimer.

- **Folder metadata panel** — Clicking a folder in browse or library mode now loads it in the info panel. The panel shows folder name, path, total size, file count, subfolder count, and max nesting depth. A squarified SVG treemap visualises immediate subfolders by recursive size (click any segment to navigate into it). A nesting-depth histogram shows how deeply each subfolder branches. In browse mode a file-type breakdown lists all extensions found recursively. In library mode, EXIF-based sections are added: shooting date range, format breakdown, top camera × lens combinations, and a 24-hour shooting-activity chart.

- **Home button in breadcrumb navigation** — In browse and commander modes, a home icon button appears next to the up-dir arrow in the breadcrumb row. Clicking it navigates to the OS home directory when the app can reach it (desktop mode; boundary is `/`), or to the boundary root when the app is restricted to a specific folder. The button is disabled when already at the home target. The backend exposes the resolved home path via a new `homePath` field in `/api/config`.

- **Slideshow multi-track built-in music** — The "Built-in" audio option now shows a checklist instead of a single dropdown. Any combination of the three built-in tracks can be selected; they play sequentially and loop forever. An "In order / Shuffled" sub-toggle controls playback order. Track selection and order preference are persisted across sessions.

- **Film simulation "Show photos without" toggle** — The Film Simulation chart now has a pill toggle (off by default) that includes or excludes photos with no Fujifilm film simulation tag. Hiding the dominant "None" bar makes the distribution of actual simulations much easier to read.

- **Export folder picker** — A `…` button next to the destination path input opens the native OS folder chooser dialog (macOS: system dialog via osascript; Linux: zenity or kdialog). The selected path is filled into the input automatically; cancelling leaves it unchanged.

### Changed

- **Crop handles — larger hit target** — Resize handles on the crop selection box are now 12×12 px (up from 10×10 px) to make them easier to click and drag for fine adjustments.

- **Toggle sliders — three-label rule enforced** — Every binary on/off toggle now shows three visible labels: a purpose label, an ON-state label, and an OFF-state label, so state is readable without depending on colour alone. Concretely: the library "Filter" toggles and the statistics film-simulation toggle gain ON/OFF labels; the settings "Interface" label is renamed to "Show interface"; the slideshow loop checkbox, library filter 35mm-equivalent checkbox, statistics focal-length Native/35mm selector, info-panel map 2D/3D selector, and settings thumbnail-quality Standard/High selector are all converted to proper toggle sliders. `Toggle.create()` gains `labelOn`/`labelOff` options for contextual state labels.

- **Statistics snapshot layout** — Time of Day (radial clock) is now in row 1 next to Format, giving the most-glanceable charts top billing. Film Simulation moves to a full-width row below; it is hidden entirely when no Fujifilm film simulation data exists (e.g. iPhone-only libraries), reducing unnecessary scrolling.

- **Design system** — Applied Hüpattl! Design System v1 to the UI. Tokens now use OKLCH colour space with warm-neutral backgrounds, orange accent (`#d35400`), and IBM Plex Mono as the UI typeface. Light and dark themes are fully token-driven; the theme toggle is unchanged.

- **Library card spacing** — Cards in the library list are spaced further apart (32 px gap instead of 16 px), aligning with the design system 8 px grid.
- **Filter toggle switches** — The "Filter" button in both the library list view (cross-library) and the library detail view now use the design system's pill toggle switch (track + sliding thumb, turns orange when active) instead of a text button with a dot indicator.
- **Cross-library panel renamed to "Filter"** — The toggle button in the library list view that opens the cross-library EXIF panel is now labelled "Filter" (was "Search"), matching the equivalent button in the library detail view. The results breadcrumb likewise reads "Filter results".
- **Viewer toolbar button groups** — Zoom controls (zoom-out, Fit, zoom-in, reset) and action buttons (Crop, Delete) are now visually connected groups: 1 px separator, shared border, flush inner buttons, matching the design system button-group pattern used elsewhere in the UI.
- **Viewer info button removed** — The "Info" toolbar button is removed; the `i` keyboard shortcut and the collapsed-panel toggle are the primary controls. The collapsed panel now shows a stroked SVG ⓘ icon instead of the italic serif "i".
- **SVG navigation icons** — The Back, Previous, and Next buttons in the large-photo viewer now use stroke-based SVG chevrons instead of typographic characters (`← Back`, `‹`, `›`). The up-directory button in all grid views likewise uses an SVG arrow with even padding.
- **Photo navigation as hover overlays** — Previous and Next chevrons are now absolutely-positioned overlays that fade in on mouse-over and are invisible otherwise, so the photo fills the full window width at all zoom levels and when the UI is hidden with `h`.
- **macOS release artifact names** — macOS downloads are now named `macos_intel` and `macos_apple_silicon` instead of `darwin_amd64` / `darwin_arm64`.

### Fixed

- **Crop — stale thumbnail after apply** — After cropping an image, the server-side disk cache (thumbnails, HEIF conversions) is now evicted so subsequent requests return the updated image. The directory scan cache is also invalidated. The film strip thumbnail refreshes immediately without a page reload.

- **Crop — coordinate mismatch for EXIF-rotated images** — The crop overlay previously computed its bounds from `naturalWidth`/`naturalHeight`, which is browser-inconsistent for EXIF-rotated JPEGs. The overlay now uses `getBoundingClientRect()` directly, which always reflects the visual image area. Entering crop mode also resets the zoom level to fit, preventing the overlay from being placed partially off-screen when the user was zoomed in.

- **Crop (HEIC) — wrong region and 90° rotation after apply** — HEIF/HEIC crops now use a decode-crop-encode pipeline (sipsConvert → apply EXIF orientation in Go → cropRect → sips JPEG→HEIC) instead of `sips --cropOffset`. The previous approach was unreliable because `sips --cropOffset` uses visual coordinates while `sips -g pixelWidth/Height` returns stored dimensions; for cameras like Fujifilm that store rotation in the embedded JPEG's EXIF rather than the HEIC irot box, this mismatch placed the crop in the wrong region and produced a 90° rotated result. Additionally, `sips -s format jpeg` preserves the EXIF orientation tag rather than baking it into pixels for these files, so orientation must be applied explicitly with `extractJPEGOrientation` + `applyOrientation` before cropping. See ADR-0020.

- **Slideshow ghost image on portrait photos** — When a portrait photo followed a landscape photo, the old image remained faintly visible in the letterbox areas during the fade transition. The `.ss-frame` container now carries a `background: #000` so transparent regions in portrait (contain-fit) frames are solid black, hiding the outgoing frame behind them. Fix applies to all display modes and transition types.

- **Wastebin view not refreshing after permanent delete** — When more than five photos were selected for permanent deletion, the ProgressDialog ran asynchronously and the wastebin grid was re-rendered before any files were actually removed, leaving deleted photos still visible. The grid now re-renders only after all deletions complete, in both the single-batch (≤ 5 files) and ProgressDialog (> 5 files) code paths.

- **Statistics format legend truncation** — Format labels (JPEG, HEIF, …) were clipped by the SVG viewBox boundary. The donut SVG is now wider so all labels fit without overflow.

- **Library filter reset race** — Clicking "Reset filters" while a debounced slider or dropdown query was in flight no longer causes the reset result to be silently overridden. The reset now cancels any pending debounce before issuing its own query.
- **Overlay badges missing in library folder view** — GPS pin, film-simulation, and aspect-ratio badges now appear in library folder thumbnails when "Show details" is enabled. The library browse API now fetches GPS, film simulation, and image dimensions from the SQLite DB, and the library pane populates the badge overlay data directly from the response instead of leaving it empty.
- **Keyboard navigation in cross-library search** — Arrow keys, `i` (info panel), and all other shortcuts now work in the list-view cross-library search results pane. Previously, `getActivePaneForKeyboard()` did not know about the list-view search pane and returned `null`, silently dropping all keyboard events.
- **Focus ring invisible in dark/light mode** — Hovering or keyboard-focusing a photo in grid or justified view now shows a soft-orange pulsing ring (4 px inset, 0.9 s breathing animation) that is visible in both themes. Previously the indicator used `--border`, a token designed to match the background, making it effectively invisible.
- **Library export "invalid path"** — Exporting photos from library mode no longer fails with "invalid path". The library's source directory is now passed alongside the export request so the backend resolves file paths against the correct root instead of the browse boundary.
- **Tools menu in library search/filter results** — Set Geolocation, Export, and other tool actions now fire correctly when invoked from filter results within a library or from cross-library search results. Previously the tools menu appeared but clicking items had no effect because the search-result panes were wired without an `onToolInvoke` callback.
- **Export from library search/filter results** — Exporting photos selected in filter or cross-library search result grids no longer fails with "invalid path". Library photos are tracked by absolute filesystem paths; the export handler now accepts absolute paths in non-server-role (desktop) mode, validated by file existence.

### Security

- **golang.org/x/image** updated from v0.39.0 to v0.41.0, picking up fixes for a TIFF out-of-memory amplification (CVE-2026-33809), an SFNT/font parsing OOM (CVE-2026-33812), and a WebP panic on 32-bit platforms (CVE-2026-33813). None of the changed decoders are used by Unterlumen directly; the update is precautionary.

## [0.8.1] - 2026-05-16

### Added

- **Up-directory button** — A `↑` button now appears before the breadcrumb in every grid view (browse, library folder browser, both commander panes). The button is disabled at the root and navigates up one level on click.

### Fixed

- **Library keyboard navigation** — Arrow keys, Enter, Space, Escape, Ctrl+A, and `I` no longer stop working after the search/filter panel is opened and then closed. The active pane resolver now checks DOM visibility before returning the search-results pane, so keyboard events correctly target the visible library pane.
- **Library Escape key** — Pressing Escape in library mode now clears selections first, then navigates up one directory level (matching browse and commander behaviour). When the fullscreen viewer is open, Escape is correctly delegated to the viewer.
- **Desktop mode Chrome prompts** — Chrome no longer shows "Make this your default browser?" or the search-engine choice screen when launched via `-desktop`. Added `--no-first-run`, `--no-default-browser-check`, and `--disable-search-engine-choice-screen` flags on startup.

## [0.8.0] - 2026-05-15

### Added

- **Desktop mode** — `-desktop` flag opens the app in a Chrome/Chromium app window (no URL bar, no tab strip). The server shuts down automatically when the window is closed. Falls back to the default system browser if Chrome is not installed.
- **Desktop install** — `-desktop-install` flag runs an interactive wizard that copies the binary into a native app launcher (macOS `.app` bundle in `~/Applications`, Linux `.desktop` entry, Windows Start Menu shortcut) with an icon, so the app can be launched from Spotlight/Launchpad, the application grid, or the Start Menu without a terminal.
- `UNTERLUMEN_CACHE_DIR` environment variable and `-cache-dir` flag to configure the thumbnail and conversion cache directory (defaults to OS cache conventions; falls back to `/tmp/unterlumen-cache` when unavailable)

## [0.7.0] - 2026-05-14

### Changed

- **E2E fixture source** — Test fixtures are no longer downloaded from external URLs. `npm run setup` now copies `src/examples` (79 real-world images, 2004–2026, multiple cameras) into `e2e/fixtures/photos/`. New specs cover folder navigation, HIF thumbnails, GPS add/remove, export ZIP, crop API, and aspect-ratio rendering.

- **Focus vs. selection visual distinction** — The keyboard cursor (focused item) now looks identical to mouse hover: a subtle gray border. Selected items show a uniform 2px inset orange ring with a light orange tint, consistent across all grid positions. Previously both states used the accent orange, making them indistinguishable.

### Added

- **Cache management** — A "Cache" section in the Settings dropdown shows how much disk space the thumbnail cache occupies (in MB) and its location. A "Clear cache" button deletes all cached files immediately, with the size display refreshing afterwards.

- **Slideshow folder selection** — Clicking a folder entry in the browse or library grid now selects it (orange border; Ctrl+click for multi-select). Triggering the slideshow with folders selected plays all photos from those folders recursively — including nested subfolders. Selecting a photo clears the folder selection and vice versa.

- **Slideshow button disabled state** — The Slideshow button is now greyed out and unclickable when the current folder contains no photos and no folder entry is selected.

- **Timeline statistics** — A "Timeline" tab in the Statistics modal showing how shooting habits evolved over time. Six D3 charts: Camera usage (stacked bar with top-5 cameras), Focal length drift (median + IQR band), ISO evolution (log-scale area chart), Aperture usage (normalised heatmap), Aspect ratio mix (100% stacked area), and Megapixel timeline (max step-line + avg). Granularity auto-detects from library date span (month ≤ 4 years, year otherwise), with a manual Month/Year/Auto toggle. Timeline data is lazy-loaded on first tab click. Backed by a new `GET /api/library/timeline` endpoint.

- **Library scan modes** — A "Scan new and changed" combo button with a dropdown arrow replaces the standalone "Re-index" button on every library card. "Scan new and changed" (primary action) walks the source directory and adds new photos, updates changed ones, and re-links renamed files — without removing anything. "Re-index (full)" (dropdown) performs a full rescan that also purges deleted files. "Cleanup deleted" (dropdown) removes indexed photos whose source files are gone, without re-scanning or re-hashing. All three actions share live SSE progress; clicking while a scan is running joins the ongoing stream instead of erroring. Only one scan can run per library at a time.

- **Crop tool** — An interactive crop tool in the fullscreen viewer. Click "Crop" in the toolbar to enter crop mode, draw a rectangle on the photo, choose from free, standard (1:1, 4:3, 3:2, 16:9, 9:16 and their portrait variants), or cinema (1.85:1, 2.35:1, 2.39:1) aspect ratios, and apply. Crops are saved in-place. All metadata — including Fujifilm film simulation and MakerNotes — is preserved via exiftool. JPEG and GIF are re-encoded at high quality; PNG is lossless; WebP uses ffmpeg; HEIF/HEIC uses `sips` (macOS). Keyboard: Enter to apply, Escape to cancel.

- **Statistics modal** — A context-aware "Statistics" button in the library header shows stats for all libraries (list view), the current library (library root), or the current subfolder. Eight D3.js charts cover formats, film simulation, focal length (with 35mm-equivalent toggle), aperture, ISO, camera × lens, time of day, and a shooting calendar heatmap. D3.js v7 is bundled locally — no CDN dependency. The stats API returns deduplicated `{value, count}` pairs for histogram data instead of raw float arrays, significantly reducing response size for large libraries. A `path_hint` index is added on first startup to speed up folder-scoped queries.

- **E2E test coverage for library search and filter** — Added two new Playwright spec files (`library.spec.js`, `library-search.spec.js`) and a shared `reindexLibrary` helper covering library card UI, search panel open/close, EXIF range sliders, text filter dropdowns, the 35mm focal length toggle, the detail-view filter panel, and six API contract tests. Applied `waitForAppReady` guard to the statistics spec to eliminate a race condition with the app initialisation sequence.

- **Clean sweep after re-index** — Photos that have been deleted or moved out of a library's source directory are now fully removed from the database (along with their EXIF index, metadata, and path cache rows) and their thumbnail files are deleted from disk. Previously, missing photos accumulated as invisible dead records.

- **35mm equivalent focal length filter** — The focal length range slider in the library search panel now has a "35mm equivalent" checkbox. When enabled, the slider operates on the `FocalLengthIn35mmFilm` EXIF value instead of the native focal length — useful for comparing photos across cameras with different sensor sizes. Photos without 35mm-equivalent EXIF data fall back to the native focal length automatically. Existing libraries are migrated on startup without re-indexing.

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

- **E2E integration tests** — Playwright test suite covering browse, image viewer, EXIF overlay badges, and wastebin mark/restore workflow. Tests run headlessly via `cd e2e && npm test` and are integrated into GitHub Actions CI (`.github/workflows/e2e.yml`), triggering on every push and pull request.

- **Slideshow** — New "Slideshow" button in the browse toolbar (between Tools and the status bar). Operates on selected images, or all images in the current folder if nothing is selected. An options dialog lets you set the delay between images (1–60 s), choose a transition effect (Fade, Slide, Zoom, or Instant), and pick a display style: Single image, Ken Burns (slow animated pan and zoom), 2-up (two images side by side), or 4-up (2×2 grid). Optional audio: choose a local audio file or a folder of tracks (shuffled, looping). The player shows a minimal HUD with Prev / Pause-Play / Next / Close controls that autohides after 3 seconds; keyboard shortcuts Space (pause/resume), ← / → (navigate), and Esc (close) are supported.

- **Dependency check** — New "Check dependencies" entry in the Settings menu opens a modal listing the status of all required external tools (ffmpeg, exiftool, sips on macOS). Missing or misconfigured tools are shown with a plain-language explanation and platform-specific install instructions.

- **Container image** — Docker image published to GHCR (`ghcr.io/bjblazko/unterlumen`) for `linux/amd64` and `linux/arm64`. Image bundles ffmpeg and exiftool; defaults to server mode with `/photos` as the root.

### Changed

- **Search panel performance** — Opening the library search/filter panel is significantly faster for large libraries. EXIF range queries are batched from 6 serial SQL statements into 2 (one GROUP BY for all scalar fields, one combined query for the 35mm focal-length range). EXIF ranges and distinct text values (camera, lens, film sim) are now cached in memory and invalidated automatically when a scan starts or completes — making every panel open after the first instant. The panel also shows "Loading filters…" immediately on first open instead of appearing as a blank box.

- **Statistics and timeline performance** — Opening the Statistics modal is significantly faster for large libraries (20k+ photos). Key improvements: a `date_taken` column is now stored directly in the photos table (backfilled from `exif_json` on startup), eliminating per-row JSON extraction in all shooting hours, shooting days, and timeline queries; format distribution is computed with a `GROUP BY` on a pre-computed `ext` column instead of fetching all filenames; the SQLite page cache is raised to 64 MB. Statistics and timeline results are cached in memory and invalidated automatically when a scan starts or completes.

- **Filter query performance** — EXIF filter queries now use JOINs instead of correlated EXISTS subqueries, allowing SQLite to start from the small set of indexed EXIF rows rather than iterating every photo. The unused `q` / LIKE wildcard search parameter (which was never sent by any UI element and prevented index use) has been removed.

- **"New library" button redesigned as circle-plus glyph** — The `+` affordance in the Libraries tab button is now an SVG circle-with-plus icon. When the Libraries tab is active the glyph inverts: a filled white circle with an orange plus inside. On inactive tabs it renders as a thin outlined circle. The SVG eliminates the font-baseline centering offset that made the previous text character appear slightly off.

- **Library toolbar refinements** — Several visual improvements to the library UI:
  - "New library" button removed from the library list header and merged into the top-right "Libraries" nav button as a small `+` affordance (separated by a thin line). Clicking `+` switches to library mode and immediately opens the new-library dialog.
  - "Search" (library list) and "Filter" (library detail) toggle buttons now show a small orange dot when their panel is open, rather than filling solid orange. The button returns to its default appearance when the panel is closed.
  - Vertical separator added between the toggle buttons (Search/Filter) and dialog-opening buttons (Channels) in both the list and detail headers.
  - "Channels" button now carries a `›` suffix in both the list and detail views to signal it opens a dialog.

- **Internal refactoring (ADR-0015)** — No user-visible behaviour changes; structural cleanup only.
  - Go: `src/internal/api/` flat package split into domain subpackages: `browse`, `export`, `fileops`, `location`, `batchrename`. Path-traversal guard extracted to `src/internal/pathguard`.
  - Go: `src/internal/media/exif.go` (735 lines) split into `exif.go`, `orientation.go`, `thumbnail.go`, `fujifilm.go`, `aspectratio.go`. `export.go` extraction of `decodeSourceImage`, `scaleImage`, `encodeToFormat`.
  - Go: Unit tests added for `pathguard`, `batchrename` (pattern, sanitize, conflict), `media` (EXIF date parsing, aspect ratio labels, export dimension modes).
  - JS: `browse.js` (1 174 lines) split into `browse.js` (orchestration), `browse-selection.js`, `browse-keyboard.js`, `browse-grid.js`, `browse-list.js`, `browse-justified.js`.
  - JS: `app.js` (767 lines) split into `app.js` (orchestration), `app-theme.js`, `app-wastebin.js`, `app-keyboard.js`.

### Fixed

- **"Make library" hidden in Tools menu while in library mode** — The Tools → "Make library" button is no longer visible when already in library mode.
- **"Organise: jump to folder" now works correctly when server root is `/`** — Navigating from a library folder to the Organise view (and vice-versa via "Jump to library…") was always landing at root instead of the correct subfolder when the server was started with `/` as the photo directory. Fixed boundary-stripping logic in `app.js` and `commander.js` to correctly compute the relative path for root-anchored servers.
- **"Organise: jump to folder" button now enables on folder selection** — In the library browse view, the button previously enabled as soon as any photos were selected. It now enables only when a single folder entry is selected, and clicking it opens that folder in the Organise view.

- **Library HEIF images load from cache when source volume is offline** — Opening a HEIF/HEIC photo in the library viewer no longer returns HTTP 500 when the source NAS is unmounted. Previously the conversion cache key included the file's modification time, which changed to an empty string when `os.Stat` failed (volume offline), causing a cache miss on every request. The key is now path-stable so a previously converted image is served from cache regardless of volume status. Converted JPEGs are now stored in the OS persistent cache directory (`~/Library/Caches/unterlumen/` on macOS via `os.UserCacheDir()`) instead of the OS temp directory, so cached conversions survive reboots. When a HEIF file has never been cached and the source is unreachable, the response is now 404 instead of 500.

- **Info panel opens correctly from search/filter results** — Pressing I while viewing library search or filter results now loads the focused photo's metadata immediately. Previously the panel showed "Select an image to view info" because the keyboard handler was notifying the browse pane instead of the active search-results pane, causing `infoPanel.clear()` to cancel the correct load.

- **Info panel now loads immediately when opened** — In browse mode, pressing I to open the info panel while a photo is focused now loads that photo's metadata straight away. Previously, the panel stayed at "Select an image to view info" until the user navigated to another photo and back. In the fullscreen viewer, a defensive guard ensures the info loads even if the initial trigger was missed due to a race condition.

- **Library overview header height** — The library overview header ("Libraries" with Search, Statistics, Channels buttons) now has the same fixed 48px height as the in-library detail header. Previously, it used 24px vertical padding instead of a fixed height, making it roughly 70px tall.

- **Statistics reflect all photos correctly** — The statistics modal now shows the true total across all fully-indexed libraries. When a library is actively being scanned, an amber banner informs the user how many photos are still being indexed. Libraries that could not be read (e.g. due to a database lock during heavy indexing) now surface a warning instead of being silently dropped from the totals. The Camera × Lens chart now uses a LEFT JOIN on `LensModel`, so cameras without a lens tag (smartphones, film scanners) appear as "(no lens)" rather than being excluded entirely. Histogram charts (Focal length, Aperture, ISO) show a "N of M photos" subtitle when not all photos carry that EXIF field.

- **Malformed `exif_json` no longer breaks statistics** — Photos whose EXIF could not be extracted were stored with an empty string in `exif_json` rather than valid JSON. SQLite's `json_extract()` throws "malformed JSON" on empty strings (unlike `NULL`), causing the entire Statistics and Timeline query pipeline to fail for any library containing such photos. The indexer now writes `{}` for photos without parsable EXIF. Existing affected rows are repaired on the fly.

- **Interrupted re-index no longer shows 0 photos** — If a full re-index was cancelled mid-way (network drop, power-save, crash), the library overview showed 0 photos because the cached photo count was only written at the very end of a successful scan. The count is now always updated on exit, so the overview reflects however many photos were successfully indexed before the interruption.

- **Browse mode no longer stalls on large HIF film rolls** — Standard-quality browse thumbnails now send an explicit requested size, so the HEIF/HIF thumbnail path can choose the smallest embedded JPEG preview that still fits the UI instead of repeatedly extracting a larger preview. Uncached thumbnail work is also bounded and deduplicated on the server, which prevents large folders from pegging all CPU cores during first load. Opening the fullscreen viewer no longer eagerly loads the whole filmstrip, so the selected image can appear while background thumbnails are still pending.

- **Deployed service can now find Homebrew tools (ffmpeg, exiftool)** — The launchd plist (`com.unterlumen.app`) was launched without `EnvironmentVariables`, so macOS gave the process only the bare system PATH (`/usr/bin:/bin:/usr/sbin:/sbin`). `exec.LookPath` therefore reported ffmpeg and exiftool as missing even though both were installed. The plist now explicitly sets `PATH` to include `/opt/homebrew/bin` and `/opt/homebrew/sbin`, matching the user session environment.

- **Thumbnail quality mode restored for HEIF browsing** — The browse thumbnail endpoint now receives the user's selected quality mode explicitly. In **Standard** mode, HEIF/HEIC thumbnails use the fast preview-based JPEG path and only resize when the preview is still larger than the requested thumbnail; resized results are cached by file and size. In **High** mode, HEIF thumbnails are generated from the full decoded source image and cached by size. Source-generated thumbnails for non-HEIF images are now cached as well, so repeated browsing no longer re-renders the same thumbnails on every request.

- **Slow first photo open in library mode on NAS** — The library pane previously called the browse API to list folder contents, which spawned a background goroutine reading every file over SMB to extract EXIF metadata. This saturated NAS bandwidth and delayed opening the first photo by up to 10 minutes. The library pane now reads folder contents and photo IDs directly from the SQLite database via a new `/api/library/{id}/browse` endpoint, with no filesystem reads during normal browsing. Thumbnails use pre-cached local files; full images stream on demand when a photo is opened.

- **Slow library overview load with large libraries** — Loading the library list (`GET /api/library/`) was doing a full table scan (`SELECT COUNT(1) FROM photos WHERE status='ok'`) on a table with fat `exif_json` TEXT rows. With 50 000 photos across two libraries this took 5–6 seconds. Fixed by: (1) adding an index on `photos.status`; (2) caching the photo count in `library_props` after each re-index so the overview reads a single key-value row instead of counting all photos. Existing databases are migrated automatically on startup.

- **Library folder browse path scoped to library root** — The `/api/library/{id}/browse` endpoint previously resolved the `path` parameter relative to the server's photo root directory. This meant libraries on a NAS or any path outside the server root would return empty results, and the breadcrumb showed the full native path (e.g., `Volumes / nas / Timo / Bilder / Fotos / 2024`). The endpoint now resolves paths relative to each library's own `source_path`, and the frontend always starts navigation at the library root. The breadcrumb shows a clean relative path (e.g., `Root / 2024 / June`) and updir navigation stops at the library root.

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
