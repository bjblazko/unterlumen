# arc42 Architecture Documentation — Unterlumen

*Last modified: 2026-10-04*

## 1. Introduction and Goals

### 1.1 Requirements Overview

Unterlumen is a photo browser and culler. It allows users to:

- Browse directories of photos in grid or list view
- View individual photos full-screen with prev/next navigation
- Organize photos by moving or copying them from one folder into a list of target folders
- Sort by filename or date taken

It explicitly does **not** support image editing, RAW file processing, tagging, rating, or persistent metadata.

### 1.2 Quality Goals

| Priority | Goal | Description |
|----------|------|-------------|
| 1 | Simplicity | Single binary, no database, no config files, no build toolchain for the frontend |
| 2 | Speed | Thumbnails served from embedded EXIF data; no generation step |
| 3 | Portability | Pure Go binary runs on any OS; browser-based UI works everywhere |
| 4 | Safety | Path traversal prevention; localhost-only by default |

### 1.3 Stakeholders

| Role | Expectations |
|------|-------------|
| Photographer | Fast photo browsing and efficient culling workflow |
| Self-hoster | Easy deployment on a NAS or server, no complex setup |

## 2. Constraints

### 2.1 Technical Constraints

| Constraint | Rationale |
|------------|-----------|
| Go for the backend | Single binary deployment, strong stdlib for HTTP |
| No JavaScript framework | No build step, minimal frontend complexity |
| No database | Filesystem is the source of truth (see [ADR-0002](adr/0002-no-persistence.md)) |
| ffmpeg for HEIF | No mature pure-Go HEIF decoder available (see [ADR-0004](adr/0004-heif-via-ffmpeg.md)) |

### 2.2 Organizational Constraints

| Constraint | Rationale |
|------------|-----------|
| No authentication | Simplicity; network-level access control is the user's responsibility (see [ADR-0006](adr/0006-no-authentication.md)) |

## 3. Context and Scope

### 3.1 Business Context

```
┌──────────────┐         HTTP          ┌────────────────────┐
│              │ ◄──────────────────── │                    │
│    Browser   │ ────────────────────► │  Unterlumen        │
│   (User)     │   JSON API + static   │  (Go HTTP server)  │
│              │   files               │                    │
└──────────────┘                       └────────┬───────────┘
                                                │
                                       ┌────────▼───────────┐
                                       │   Filesystem       │
                                       │   (photo dirs)     │
                                       └────────────────────┘
                                                │ (optional)
                                       ┌────────▼───────────┐
                                       │   ffmpeg           │
                                       │   (HEIF→JPEG)      │
                                       └────────────────────┘
```

| Neighbor | Description |
|----------|-------------|
| Browser | User's web browser; renders the UI, makes API calls |
| Filesystem | The root directory tree containing photos; read for browsing, written to for copy/move/delete |
| ffmpeg | External process invoked for HEIF/HEIC to JPEG conversion |

### 3.2 Technical Context

| Interface | Protocol | Format |
|-----------|----------|--------|
| `/api/config` | HTTP GET | JSON (server configuration, e.g. startPath) |
| `/api/browse` | HTTP GET | JSON (directory listing) |
| `/api/thumbnail` | HTTP GET | JPEG/PNG binary |
| `/api/image` | HTTP GET | JPEG/PNG/GIF/WebP binary |
| `/api/copy` | HTTP POST | JSON request/response |
| `/api/move` | HTTP POST | JSON request/response |
| `/api/info` | HTTP GET | JSON (file metadata + EXIF) |
| `/api/delete` | HTTP POST | JSON request/response |
| `/api/browse/dates` | HTTP GET | JSON (deferred EXIF dates for a directory) |
| `/api/browse/folder-stats` | HTTP GET | JSON (recursive size/count/depth stats for a folder) |
| `/api/library/{id}/folder-stats` | HTTP GET | JSON (same, resolved against library source path) |
| `PATCH /api/library/{id}` | HTTP PATCH | JSON — update library name and description |
| `PUT /api/library-order` | HTTP PUT | JSON `{order:[ids]}` — set `sort_position` on all libraries in bulk |
| `GET /api/jobs/stream` | HTTP GET, SSE | Every running or recently finished job, then each change ([ADR-0036](adr/0036-activity-and-progress.md)) |
| `GET /api/settings` | HTTP GET | JSON — global app settings (e.g. `librarySortMode`) |
| `PATCH /api/settings` | HTTP PATCH | JSON — update one or more global settings fields |
| `/` (static) | HTTP GET | HTML/CSS/JS files |

## 4. Solution Strategy

| Goal | Approach |
|------|----------|
| Fast thumbnails | Extract embedded EXIF thumbnails rather than decoding full images ([ADR-0003](adr/0003-exif-thumbnails.md)) |
| Simple culling | One source folder and a list of targets, one key each ([ADR-0032](adr/0032-organize-one-source-many-targets.md), supersedes ADR-0005) |
| Easy deployment | Single Go binary, static files served from `web/` directory ([ADR-0001](adr/0001-go-http-server-with-browser-ui.md)) |
| No state management | Filesystem is the only store; no database ([ADR-0002](adr/0002-no-persistence.md)) |
| HEIF support | Shell out to ffmpeg ([ADR-0004](adr/0004-heif-via-ffmpeg.md)) |
| Large-folder performance | In-memory scan cache, deferred EXIF extraction, chunked rendering ([ADR-0011](adr/0011-scan-cache-deferred-exif.md)) |
| NAS navigation latency | Viewer prefetches the next two images; HEIF responses are cached in an in-memory LRU cache and served with `max-age=3600` ([ADR-0022](adr/0022-read-ahead-prefetch.md)) |
| Client-side settings | UI-only preferences (theme, thumbnail quality) persisted in `localStorage` ([ADR-0012](adr/0012-client-side-settings.md)); library-level settings (sort mode, sort order) persisted server-side in `settings.json` and per-library `library_props` |

## 5. Building Block View

### 5.1 Level 1 — System Overview

```
┌─────────────────────────────────────────────────────┐
│                     Unterlumen                       │
│                                                     │
│  ┌──────────────┐          ┌──────────────────────┐ │
│  │   web/       │  static  │   Go HTTP Server     │ │
│  │  (frontend)  │ ◄─────── │                      │ │
│  │              │          │  ┌─────────────────┐  │ │
│  │  index.html  │  JSON/   │  │   internal/api  │  │ │
│  │  js/*.js     │  binary  │  │   (handlers)    │  │ │
│  │  css/*.css   │ ◄──────► │  └────────┬────────┘  │ │
│  └──────────────┘          │           │           │ │
│                            │  ┌────────▼────────┐  │ │
│                            │  │ internal/media  │  │ │
│                            │  │ (scan, exif,    │  │ │
│                            │  │  formats)       │  │ │
│                            │  └─────────────────┘  │ │
│                            └──────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

### 5.2 Level 2 — Backend Packages

| Package | Responsibility |
|---------|---------------|
| `main` | CLI flag parsing, HTTP server startup; `server.go` swaps the whole app when the setup saves a new configuration ([ADR-0042](adr/0042-installation-one-line-setup-in-the-browser.md)) |
| `internal/installation` | `config.json` of the installed app (data folder, shared folder, port), reading an older launcher's flags, migrating the photo folder of before ADR-0047 away, the `.unterlumen-shared` convention, the tools folder put in front of `PATH` ([ADR-0042](adr/0042-installation-one-line-setup-in-the-browser.md), [ADR-0047](adr/0047-independent-libraries-shared-per-library.md)) |
| `internal/toolinstall` | Installs the missing helper programs (ffmpeg, exiftool, cwebp) from the app: Homebrew or winget, otherwise the makers' downloads into the tools folder; the Linux command to run by hand ([ADR-0042](adr/0042-installation-one-line-setup-in-the-browser.md)) |
| `internal/api/folderdialog` | `/api/folder-dialog` — the system's own folder dialog for the installed app, for requests from the same computer only ([ADR-0042](adr/0042-installation-one-line-setup-in-the-browser.md)) |
| `internal/api/setup` | `/api/setup`, `/api/setup/shared`, `/api/setup/dirs` — the setup place, registered only when the app is configured by config.json; `/api/setup/sharing` and `/api/setup/share` — sharing on a server (server mode) |
| `internal/api` | HTTP route registration; delegates to domain subpackages |
| `internal/api/browse` | `/api/browse`, `/api/browse/dates`, `/api/browse/meta`, `/api/browse/folder-stats`, `/api/thumbnail`, `/api/image`, `/api/info` handlers |
| `internal/api/export` | `/api/export/*` handlers; ZIP token store |
| `internal/api/fileops` | Copy, move, delete, mkdir, rename, recursive-list handlers |
| `internal/api/location` | Set/remove GPS location handlers |
| `internal/api/batchrename` | Batch-rename preview and execute handlers; pattern resolution, filename sanitising, conflict suffixing |
| `internal/api/download` | Serves a photo as the file it is, as an attachment under its own name; `?download=1` on `/api/image` and `/api/library/{id}/photo/{photoID}` |
| `internal/api/heifjpeg` | Serves a HEIF as the JPEG a browser can show, for `/api/image` and `/api/library/{id}/photo/{photoID}`: memory cache, disk cache, conversion; a request with `X-Prefetch: 1` gets `204` rather than a conversion ([ADR-0022](adr/0022-read-ahead-prefetch.md)) |
| `internal/api/library` | `/api/library/*` handlers: libraries, indexing (SSE), photo queries and filters, thumbnails and photos, photo info, metadata, located photos for the Map (`/api/library/geo`), statistics, timeline, colour (`/api/library/colour`) the points of the 3D views (`/api/library/colour-space`, `/api/library/exposure-space`, `/api/library/space-time`, [ADR-0046](adr/0046-colour-space-webgpu-glow-stage.md)); the search filters by appearance (`mono`, `hue_bin`, `warmth`, `month`) and by one photo (`photo_id`). The library database's indexes cover these queries ([ADR-0045](adr/0045-indexes-cover-statistics-and-search.md)) |
| `internal/timeline` | Every dated photo of every library as one stream, oldest first, each photo once; display aspect ratios, versioned by the libraries' content stamps and cached ([ADR-0040](adr/0040-timeline-place.md)) |
| `internal/api/timeline` | `/api/timeline` (skeleton: a day and an aspect ratio per photo) and `/api/timeline/photos` (details by index range, 409 when the stream changed) |
| `internal/api/publish` | Publishing: drafts, generating galleries and sites (SSE), rebuilding them, the published-galleries overview, reachability, deploy stamps; taking a photo off a destination |
| `internal/jobs` | Register of long-running work (scans, exports, publishing, rebuilds, deploys) with merging subscriptions ([ADR-0036](adr/0036-activity-and-progress.md)) |
| `internal/api/jobs` | `/api/jobs/stream` (SSE) and `Track`, which reports request-long work to the register |
| `internal/api/sse` | Opens a server-sent event stream and writes JSON data events; used by every streaming handler |
| `internal/site` | Static output of a destination: single-gallery pages and multi-album sites (templates, assets, nav, sitemap), their state files, slugs, and the shared album register ([ADR-0035](adr/0035-shared-album-register.md)); no HTTP |
| `internal/pathguard` | `SafePath` — shared security primitive; symlink-aware root-boundary check |
| `internal/appearance` | What a photo looks like, measured from an image: mono/tinted/colour, average colour and palette in OKLab, tone, texture, dHash; pure, no storage ([ADR-0044](adr/0044-photo-appearance-from-thumbnails.md)) |
| `internal/media` | Filesystem scanning, EXIF extraction (exif.go), orientation (orientation.go), thumbnail generation (thumbnail.go), export/conversion (export.go), Fujifilm simulations (fujifilm.go), aspect-ratio labels (aspectratio.go), recursive folder stats (folder_stats.go) |

### 5.3 Level 2 — Frontend Modules

| File | Responsibility |
|------|---------------|
| `app.js` | `App` — orchestration: init, mode switching, modal wiring, viewer |
| `app-theme.js` | `ThemeManager` — theme preference and thumbnail-quality settings |
| `app-wastebin.js` | `Wastebin` — mark/restore/delete queue and review UI |
| `app-keyboard.js` | `GlobalKeyboard` — global keydown handler |
| `browse.js` | `BrowsePane` — orchestration: load, render, delegation to renderer/selection/keyboard sub-objects |
| `browse-grid.js` | `GridRenderer` — grid-view DOM rendering |
| `browse-list.js` | `ListRenderer` — list-view DOM rendering |
| `browse-justified.js` | `JustifiedRenderer` — justified-view DOM rendering and row-packing layout |
| `browse-selection.js` | `SelectionManager` — toggle, range-select, select-all, class updates |
| `browse-keyboard.js` | `BrowseKeyboard` — focus movement, keyboard activation, column detection |
| `organize.js` | `OrganizePane` class — source browse pane, remembered targets, move/copy with undo |
| `activity.js` | `Activity` — the one way to say something is happening: busy, counted, ended ([ADR-0036](adr/0036-activity-and-progress.md)) |
| `status-line.js` | `StatusLine` — the foot of the sidebar: jobs from `/api/jobs/stream` that outlive their page |
| `dialog.js` | `Dialog` class — the frame and behaviour of every dialog ([ADR-0033](adr/0033-dialogs-and-places.md)) |
| `fullscreen.js` | `Fullscreen` — the browser's own full screen for the slideshow and the viewer; a caller leaves only the full screen it entered |
| `menu.js` | `Menu` class — the ⋯ menu of a place: actions and Toggle switches that need not be on screen, keyboard-owning while open |
| `viewer.js` | `Viewer` class — full-image display, prev/next navigation; `readOnly` leaves out crop and marking for deletion |
| `viewer-menu.js` | `ViewerMenu` — the full view's ⋯ menu: an overview's actions for the photo on screen, from a `libraryRef` (`{ lib, id }`) or a Folders path; Show in library |
| `range-slider.js` | `RangeSlider` — two handles on one track, for pointer and keyboard; the library filter's ranges and the Map's time |
| `map-place.js` | `MapPane` — the Map place: loads `/api/library/geo`, says why nothing is shown, the grey map in the theme's shade, opens photos read-only ([ADR-0039](adr/0039-map-place.md)) |
| `map-markers.js` | `MapMarkers` — clustered GeoJSON source and the round HTML markers; zooms into a group or opens it |
| `photo-column.js` | `PhotoColumn` — photos beside a place as square tiles, rendered in chunks and read in pages: the Map's in view, the ones picked in Statistics; `openLibraryPhotos` opens them read-only |
| `map-time-range.js` | `MapTimeRange` — a month range over the photos' dates, the time part of the shared filter |
| `multi-select.js` | `MultiSelect` — a button naming a choice that opens a list of checkboxes, all on at first ([ADR-0050](adr/0050-shared-scope-filter.md)) |
| `scope-filter.js` | `ScopeState` and `ScopeFilter` — the one filter of the Map, the Statistics and the Timeline: libraries, months, cameras, lenses, shared and kept in the browser ([ADR-0050](adr/0050-shared-scope-filter.md)) |
| `timeline-place.js` | `TimelinePane` — the Timeline place: desk (band and time bar) or phone (list and scrubber), info panel, read-only viewer, reload when the stream changed ([ADR-0040](adr/0040-timeline-place.md)) |
| `place-lede.js` | `placeLede`, `placeLink` — the sentence under a place's title and links to places in running text ([ADR-0041](adr/0041-explaining-the-model-in-the-app.md)) |
| `setup-place.js` | `SetupPane` — the setup (`#setup`): shared folder, data folder, helper programs; opened from Settings ([ADR-0042](adr/0042-installation-one-line-setup-in-the-browser.md), [ADR-0047](adr/0047-independent-libraries-shared-per-library.md)) |
| `about-place.js` | `AboutPane` — About (`#about`), the overview of its topics; `ABOUT_PLACES` decides when the sidebar lists them |
| `privacy-place.js` | `PrivacyPane` — Your data (`#privacy`): every network call the app makes, and when |
| `warranty-place.js` | `WarrantyPane` — No warranty (`#warranty`); `WarrantyNotice`, shown once per browser at the foot of the sidebar |
| `credits-place.js` | `CreditsPane` — Licenses and thanks (`#licenses`), read from `web/licenses/credits.json`, whose license texts are embedded in the binary; `credits_test.go` keeps it complete |
| `guide-place.js` | `GuidePane` — "How Unterlumen works": the model as a diagram and a paragraph per term ([ADR-0041](adr/0041-explaining-the-model-in-the-app.md)) |
| `timeline-stream.js` | `TimelineStream` — the skeleton in typed arrays and the details in pages of 500, at most 40 kept |
| `timeline-calendar.js` | `TimelineCalendar` — days and months in UTC from the first photo's day, date labels |
| `timeline-chart.js` | `TimelineChart` — the step graph and the calendar labels on a canvas; redraws on theme change |
| `timeline-axis.js` | `TimelineAxis` — the desk's axis with the frame for what the band shows |
| `timeline-range.js` | `TimelineRange` — the desk's overview with bracket handles that limit the range to whole months |
| `timeline-band.js` | `TimelineBand` — the desk's photos in 2–4 rows, time left to right, a column per month |
| `timeline-list.js` | `TimelineList` — the phone's photos, newest first, a heading per month |
| `timeline-scrubber.js` | `TimelineScrubber` — the phone's vertical time axis with a month bubble |
| `timeline-tiles.js` | `TimelineTiles` — tiles only near the screen; a tile that leaves releases its image |
| `infopanel.js` | `InfoPanel` class — collapsible side panel showing file metadata and EXIF data; a folder's dashboard comes from `FolderDashboard` |
| `folder-dashboard.js` | `FolderDashboard` — the info panel's view of a folder: contents, size map (treemap), nesting depth, file types, library EXIF stats |
| `stats-place.js` | `StatsPane` — the Statistics place (`#statistics[/<topic>]`): scope in the head, overview cards, a topic's charts, the photos of a picked value in a `PhotoColumn`, and the Full view of a stage chart ([ADR-0043](adr/0043-statistics-place-with-topics.md)) |
| `stats-topics.js` | `STATS_TOPICS` — the topics and their charts, the criterion each click turns into, and the place's address |
| `gpu-stage.js` | WebGPU basics for 3D views: the page's device, `GpuStage` (a canvas that follows its size and draws only while something moves) and `OrbitCamera` ([ADR-0046](adr/0046-colour-space-webgpu-glow-stage.md)) |
| `point-scene.js` | `PointScene` — the WGSL and pipelines of a 3D stage: glowing lights, the trail, faint lines, tone mapping; colour helpers |
| `point-stage.js` | `renderPointStage` / `PointStage` — a 3D stage of photos as lights: labels, legend, turning and zooming, hover and click |
| `colour-space-view.js` | The Colour space topic: photos at their main colour in OKLab |
| `exposure-space-view.js` | The Exposure space topic: photos by focal length, aperture and ISO, coloured by camera |
| `daylight-view.js` | The Daylight topic: photos by day of the year, hour and brightness |
| `character-view.js` | The Character topic: photos by brightness, contrast and colourfulness |
| `space-time-view.js` | The Space and time topic: photos by where (log distance around the middle of the photos) and when, over the world outline |
| `stats-charts.js` | Snapshot charts (formats, film simulations, lenses, exposure, shooting clock, calendar) and the shared chart helpers ([ADR-0034](adr/0034-colour-in-charts.md)) |
| `stats-timeline-charts.js` | Timeline charts: cameras, focal lengths, ISO, apertures, aspect ratios and resolution over time; `renderSeriesLines` for several series as lines |
| `stats-colour-charts.js` | Colour charts from `/api/library/colour`: black and white against colour, a swatch strip per period, a hue wheel, warm and cool per month; marks in the photos' own colour (ADR-0030, Deviations) |
| `api.js` | `API` object — fetch wrappers for all backend endpoints |
| `js/vendor/maplibre-6.11.2/` | Vendored MapLibre GL JS 6.11.2 as ES modules, loaded as the global `maplibregl` ([ADR-0038](adr/0038-maplibre-6-as-es-module.md)) for location maps ([ADR-0013](adr/0013-maplibre-location-maps.md), vendored per [ADR-0031](adr/0031-vendor-maplibre.md)); tiles come from OpenFreeMap over the network |
| `data/natural-earth-lines.json` | Natural Earth 1:50m coastlines and land borders (public domain), simplified and delta-encoded, for the floor of Space and time ([ADR-0046](adr/0046-colour-space-webgpu-glow-stage.md)) |
| `fonts/` | Self-hosted IBM Plex Sans (400/500/600) and IBM Plex Mono (400/500), latin and latin-ext WOFF2 subsets, declared in `fonts/fonts.css` ([ADR-0030](adr/0030-rams-design-tokens.md)) |

## 6. Runtime View

### 6.1 Browse a Directory

```
Browser                     Server                    Filesystem
  │                           │                           │
  │  GET /api/browse?path=x   │                           │
  │ ─────────────────────────►│                           │
  │                           │  ReadDir(root/x)          │
  │                           │ ─────────────────────────►│
  │                           │ ◄─────────────────────────│
  │                           │  EXIF date extraction     │
  │  JSON [{name,type,date}]  │  (per JPEG file)          │
  │ ◄─────────────────────────│                           │
  │                           │                           │
  │  GET /api/thumbnail?...   │                           │
  │ ─────────────────────────►│  Read EXIF thumbnail      │
  │  (per image, parallel)    │ ─────────────────────────►│
  │ ◄─────────────────────────│ ◄─────────────────────────│
```

### 6.2 Copy Files (Organize)

```
Browser                     Server                    Filesystem
  │                           │                           │
  │  POST /api/copy           │                           │
  │  {files:[...], dest:...}  │                           │
  │ ─────────────────────────►│                           │
  │                           │  Validate paths           │
  │                           │  Copy file1 → dest/file1  │
  │                           │ ─────────────────────────►│
  │                           │  Copy file2 → dest/file2  │
  │                           │ ─────────────────────────►│
  │  JSON {results:[...]}     │                           │
  │ ◄─────────────────────────│                           │
  │                           │                           │
  │  GET /api/browse          │  Refresh the source       │
  │ ─────────────────────────►│ ─────────────────────────►│
```

## 7. Deployment View

```
┌─────────────────────────────────────────────┐
│              Host Machine                    │
│                                             │
│  ┌─────────────────┐    ┌────────────────┐  │
│  │ unterlumen      │    │  /photos/      │  │
│  │ (binary)        │───►│  (root dir)    │  │
│  │                 │    └────────────────┘  │
│  │ web/            │                        │
│  │ (static files)  │    ┌────────────────┐  │
│  └────────┬────────┘    │  ffmpeg        │  │
│           │             │  (optional)    │  │
│           │             └────────────────┘  │
│     localhost:8080                           │
│           │                                 │
└───────────┼─────────────────────────────────┘
            │
     ┌──────▼──────┐
     │   Browser   │
     └─────────────┘
```

The binary and `web/` directory must be co-located (the server serves static files from `./web/` relative to the working directory). The start directory is determined by CLI argument, `UNTERLUMEN_ROOT_PATH` environment variable, or user home directory (in that priority order). See [ADR-0010](adr/0010-root-path-resolution.md).

**Multiple installations against the same library.** A common variant runs two independent installations against the same photo folders — e.g. Docker on a NAS that also serves the files, plus a native install on a Mac mounting them over the network. `-lib-dir` (SQLite database, thumbnails, search index) is intentionally per-machine so each installation stays fast and usable offline. `-channels-dir` can optionally point both installations at the same directory to share channel definitions (but not library data or export output) between them. See [ADR-0023](adr/0023-shared-channel-config-directory.md). Without `-channels-dir`, a server or a folder-argument run uses a `.unterlumen-shared` folder inside its folder as that directory, so two installations share by convention ([ADR-0042](adr/0042-installation-one-line-setup-in-the-browser.md)); the installed app is given its shared folder in the setup. Libraries are shared one by one: a shared library carries `.unterlumen-library.json` (ID, name, description) in its folder, so every installation that adds the folder has the same library under the same ID, each with its own index ([ADR-0047](adr/0047-independent-libraries-shared-per-library.md)). What a person writes about a photo — title and fields — lives in its XMP sidecar, which both read; the index only copies it ([ADR-0048](adr/0048-notes-live-in-the-sidecar.md)).

**Installed app.** The one-line installers (`install/install.sh`, `install/install.ps1`, attached to every release and forwarded from huepattl.de) download a release, check it against `checksums.txt`, install ffmpeg, exiftool, cwebp and heif-convert from Homebrew, apt/dnf/pacman or winget, or from their makers into a `tools` folder beside `config.json`, and run `-desktop-install`. The launcher passes only `-desktop`; the data folder and the shared folder are in `config.json` and are chosen in the app (`#setup`). The installed app has no photo folder: it browses the whole disk, and each library names its own folder ([ADR-0047](adr/0047-independent-libraries-shared-per-library.md)). See [ADR-0042](adr/0042-installation-one-line-setup-in-the-browser.md).

**Container.** The image keeps libraries in `/data` (`UNTERLUMEN_LIB_DIR`) and its cache in `/cache`, both writable for any user so `user:` can be the photos' owner; huepattl.de/products/unterlumen-nas writes a `compose.yml` per NAS. A server shares through Settings → Sharing.

**Download packages.** Every release also carries `Unterlumen.dmg` (one universal app bundle, made by `-macos-bundle` and `packaging/macos/build-dmg.sh`) and `Unterlumen-Setup.exe` (Inno Setup, `packaging/windows/unterlumen.iss`), built by `.github/workflows/packages.yml`. Both are unsigned; the helper programs are installed from the app afterwards (`internal/toolinstall`).

## 8. Crosscutting Concepts

### 8.1 Path Security

All API endpoints that accept file paths validate them through `safePath()`:

1. The relative path is cleaned (`filepath.Clean`) to remove `..` and `.` components
2. Absolute paths in the input are rejected
3. The cleaned path is joined with the root and resolved via `filepath.EvalSymlinks`
4. The resolved path must have the root as a prefix

This prevents directory traversal attacks regardless of encoding tricks or symlinks.

### 8.2 Error Handling

- API errors return appropriate HTTP status codes with plain text error messages
- File operation endpoints (copy/move/delete) return per-file success/failure in the JSON response, allowing partial success
- The frontend displays errors inline and uses `alert()` for operation failures

### 8.3 Caching

- **Browser caching** — Thumbnail responses use `Cache-Control: no-cache` with ETag/Last-Modified for revalidation. Full-size JPEG/PNG images are served via `http.ServeFile` which sets ETag and Last-Modified automatically. Full-size HEIF conversions use `Cache-Control: private, max-age=3600` with an ETag derived from the file path and modification time, enabling browser-side caching for the duration of a session. In-place edits (crop) append a `?t=<timestamp>` cache-buster to force a fresh fetch.
- **In-memory scan cache** — Directory listings are cached in a `sync.Map` keyed by directory path. Entries are invalidated when the directory modification time changes or when a copy/move/delete operation touches the directory. Consistent with [ADR-0002](adr/0002-no-persistence.md) — the cache is purely in-memory and lost on restart. See [ADR-0011](adr/0011-scan-cache-deferred-exif.md).
- **In-memory image cache** — Full-size HEIF conversions are cached in a thread-safe LRU cache (`ImageCache`, 20 entries) shared by the browse and library handlers. Cache keys are `absPath:mtime_ns` so entries are automatically stale when the source file changes. Avoids re-reading from disk and re-serving large JPEG payloads on repeated access. See [ADR-0022](adr/0022-read-ahead-prefetch.md).
- **HEIF disk cache** — Converted JPEG data from HEIF/HEIC/HIF files is cached in `$TMPDIR/unterlumen-cache/`. Cache keys include file path, modification time, and purpose (full/preview). Survives restarts but not OS temp cleanup. See [ADR-0004](adr/0004-heif-via-ffmpeg.md).

### 8.4 Activity and Progress

- Every wait goes through `Activity` (`activity.js`): a sentence while busy, a bar and "x of y" when the total is known, a sentence at the end. Nothing shows for the first 400 ms; no looping animation; never an invented count. Work that outlives its page (library jobs, ZIP export, publishing, site rebuilds, deploys) is also registered in `internal/jobs` and shown in the sidebar's status line. See [ADR-0036](adr/0036-activity-and-progress.md).

## 9. Architecture Decisions

See the [ADR directory](adr/) for all recorded decisions:

- [ADR-0001](adr/0001-go-http-server-with-browser-ui.md) — Go HTTP server with browser UI
- [ADR-0002](adr/0002-no-persistence.md) — No persistence, in-memory state only
- [ADR-0003](adr/0003-exif-thumbnails.md) — EXIF embedded thumbnails
- [ADR-0004](adr/0004-heif-via-ffmpeg.md) — HEIF support via ffmpeg
- [ADR-0005](adr/0005-commander-style-culling.md) — Commander-style dual-pane culling (superseded by ADR-0032)
- [ADR-0006](adr/0006-no-authentication.md) — No authentication
- [ADR-0007](adr/0007-vanilla-frontend.md) — Vanilla HTML/JS/CSS frontend
- [ADR-0008](adr/0008-dieter-rams-design-principles.md) — Dieter Rams' ten principles of good design
- [ADR-0009](adr/0009-soft-delete-waste-bin.md) — Soft delete with frontend-only waste bin
- [ADR-0010](adr/0010-root-path-resolution.md) — Root path resolution and navigation boundary
- [ADR-0011](adr/0011-scan-cache-deferred-exif.md) — In-memory scan cache and deferred EXIF extraction
- [ADR-0012](adr/0012-client-side-settings.md) — Client-side settings via localStorage
- [ADR-0013](adr/0013-maplibre-location-maps.md) — MapLibre GL JS for location maps
- [ADR-0014](adr/0014-thumbnail-quality-tiers.md) — Thumbnail quality tiers
- [ADR-0015](adr/0015-coding-standards.md) — Coding standards and quality guidelines
- [ADR-0016](adr/0016-global-channel-output.md) — Global channel output directory
- [ADR-0017](adr/0017-d3-vendored-bundle.md) — Vendor D3.js for statistics visualisations
- [ADR-0018](adr/0018-design-system-tokens.md) — Adopt Hüpattl! Design System token vocabulary (superseded by ADR-0030)
- [ADR-0019](adr/0019-toggle-three-label-rule.md) — Toggle sliders must carry three visible labels
- [ADR-0020](adr/0020-heic-crop-pipeline.md) — HEIC in-place crop via JPEG intermediary
- [ADR-0021](adr/0021-database-schema-migrations.md) — Database schema migration strategy
- [ADR-0022](adr/0022-read-ahead-prefetch.md) — Read-ahead prefetch and in-memory image cache
- [ADR-0023](adr/0023-shared-channel-config-directory.md) — Shared channel config directory for multi-installation setups
- [ADR-0024](adr/0024-published-galleries-overview.md) — Published galleries overview and live reachability check
- [ADR-0025](adr/0025-gallery-edit-and-remote-delete.md) — Gallery title edit and scoped remote delete over SSH
- [ADR-0026](adr/0026-deferred-publish-drafts.md) — Deferred publish drafts replace immediate build
- [ADR-0027](adr/0027-per-album-publication-meta-keys.md) — Album identity within a channel (per-album publication meta keys)
- [ADR-0028](adr/0028-places-not-workflow-steps.md) — Places, not workflow steps (sidebar navigation)
- [ADR-0029](adr/0029-destinations-galleries-one-publish-action.md) — Destinations and galleries; publish as one action
- [ADR-0030](adr/0030-rams-design-tokens.md) — Adopt rams-design tokens (supersedes ADR-0018)
- [ADR-0031](adr/0031-vendor-maplibre.md) — Vendor MapLibre GL JS (supersedes ADR-0013's CDN delivery)
- [ADR-0032](adr/0032-organize-one-source-many-targets.md) — Organize is one source and many targets (supersedes ADR-0005)
- [ADR-0033](adr/0033-dialogs-and-places.md) — Dialogs and places, and one dialog to build them with
- [ADR-0034](adr/0034-colour-in-charts.md) — Colour in charts
- [ADR-0035](adr/0035-shared-album-register.md) — The album list of a website lives in the shared channel directory
- [ADR-0036](adr/0036-activity-and-progress.md) — One way to show activity, and a status line for work that outlives its page
- [ADR-0037](adr/0037-backend-packages-by-domain.md) — Backend packages by domain: library, publish, site
- [ADR-0038](adr/0038-maplibre-6-as-es-module.md) — MapLibre 6 as an ES module, without a bundler (supersedes ADR-0031's version pin)
- [ADR-0039](adr/0039-map-place.md) — The Map is a place, clustered in the browser
- [ADR-0040](adr/0040-timeline-place.md) — The Timeline is a place, laid out in the browser from a skeleton
- [ADR-0041](adr/0041-explaining-the-model-in-the-app.md) — The app explains its model where it is used
- [ADR-0042](adr/0042-installation-one-line-setup-in-the-browser.md) — Installation: one line, setup in the browser, sharing by convention
- [ADR-0043](adr/0043-statistics-place-with-topics.md) — Statistics is a place with topics
- [ADR-0044](adr/0044-photo-appearance-from-thumbnails.md) — What a photo looks like is measured from its thumbnail
- [ADR-0045](adr/0045-indexes-cover-statistics-and-search.md) — Indexes cover what the statistics and the search read
- [ADR-0046](adr/0046-colour-space-webgpu-glow-stage.md) — The 3D views draw with WebGPU, on a glowing stage
- [ADR-0047](adr/0047-independent-libraries-shared-per-library.md) — Libraries are independent folders; sharing is per library
- [ADR-0048](adr/0048-notes-live-in-the-sidecar.md) — A photo's title and fields live in its sidecar; the index copies them
- [ADR-0049](adr/0049-colour-combinations.md) — Colour combinations are counted from sets of hues
- [ADR-0050](adr/0050-shared-scope-filter.md) — One filter for the Map, the Statistics and the Timeline

## 10. Quality Requirements

### 10.1 Quality Tree

```
Quality
├── Simplicity
│   ├── Single binary, no database
│   ├── No build toolchain for frontend
│   ├── No configuration files
│   └── localStorage settings (client-side preferences)
├── Performance
│   ├── EXIF thumbnails (no generation)
│   ├── Scan cache (instant repeat visits)
│   ├── Chunked rendering (batched DOM updates)
│   ├── Browser-side caching
│   └── Read-ahead prefetch + in-memory image cache
├── Security
│   ├── Path traversal prevention
│   ├── Localhost binding by default
│   └── No shell interpolation for ffmpeg
└── Portability
    ├── Pure Go (no CGo)
    └── Browser-based UI
```

### 10.2 Quality Scenarios

| Scenario | Quality | Expected Behavior |
|----------|---------|-------------------|
| User opens a directory with 500 JPEGs | Performance | Directory listing returns in < 2s; thumbnails load progressively |
| User navigates to `../../etc/passwd` | Security | API returns 400 Bad Request; file is not served |
| User starts the binary with no arguments | Simplicity | Server starts, serving the user's home directory on localhost:8080 |
| User runs on a headless server | Portability | Binds to 0.0.0.0 with `-bind` flag; browser on another machine connects |

## 11. Risks and Technical Debt

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| EXIF thumbnails too small for high-DPI displays | Medium | Low | Partially mitigated: High quality thumbnail setting decodes full images at DPR-aware sizes ([ADR-0014](adr/0014-thumbnail-quality-tiers.md)) |
| ffmpeg not installed on target system | Medium | Low | HEIF files fail gracefully; all other formats work. Error message guides user. |
| Large directories (10k+ files) slow to list | Low | Medium | Mitigated: in-memory scan cache and deferred EXIF extraction ([ADR-0011](adr/0011-scan-cache-deferred-exif.md)) |
| `innerHTML` re-rendering causes flicker | Low | Low | Could switch to incremental DOM updates if UX suffers |

## 12. Glossary

| Term | Definition |
|------|------------|
| Culling | The process of selecting the best photos from a set and discarding or separating the rest |
| Target | A folder Organize can send the selection to, remembered between sessions and reachable with a number key |
| EXIF | Exchangeable Image File Format — metadata standard embedded in JPEG and other image files |
| HEIF/HEIC | High Efficiency Image Format — container format used by Apple devices for photos |
| Path traversal | An attack where crafted file paths (e.g. `../../etc/passwd`) escape the intended root directory |
