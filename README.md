# Unterlumen

[![Website](https://img.shields.io/badge/Website-huepattl.de-d35400)](https://huepattl.de/products/unterlumen.html)
[![E2E Tests](https://github.com/bjblazko/unterlumen/actions/workflows/e2e.yml/badge.svg)](https://github.com/bjblazko/unterlumen/actions/workflows/e2e.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)

Unterlumen is a local image browser and culler. It runs as a lightweight web server — open it in any browser, no cloud, no subscription.

**What it does:**
- Browse photos in justified, grid, or list view with breadcrumb navigation
- Cull: review shots, mark rejects, restore or delete in bulk
- Organize: one folder in front of you, your target folders one key away (move, copy, undo)
- Batch rename using EXIF fields (date, camera, film simulation, …)
- Export & convert: resize, change format (JPEG/PNG/WebP), strip GPS, download as ZIP
- Geolocation: set or remove GPS coordinates via interactive map
- **Publish your own website:** turn collected photos into a complete, multi-album website with navigation and a sitemap, or into a single gallery behind a private share link for family and friends — no photo platform in between. You need your own webspace reachable over SSH; Unterlumen uploads with rsync. Without it, the result is a folder you can put online any way you like.
- Map: every photo with a location on one map, grouped by place
- Timeline: every dated library photo on one time axis, with a graph of when they were taken
- **Optional — Digital Asset Management (DAM):** build a persistent, searchable catalog of your photos. Search by aperture, focal length, camera, lens, or Fujifilm film simulation across multiple libraries. Track where and when photos were published, with platform presets (Instagram 1080 px, Mastodon 1920 px, …).

**What it is not:**
- Not an image editor — no color grading, no retouching, no compositing
- No RAW file support — JPEG, PNG, WebP, and HEIF/HEIC (via ffmpeg) only

## Screenshots

The pictures below are made by `npm run screenshots` in `e2e/` from the example photos in `src/examples`, so they always show the current interface. Click one for the full size.

#### Folders

Your photo folders as a justified grid, with the metadata and the place a photo was taken beside it. The bar at the bottom holds everything you can do with a selection.

![Folders with the info panel](doc/screenshots/folders.webp)

#### Light and dark

Unterlumen follows the system's light or dark setting; Settings can fix one.

| Light | Dark |
|---|---|
| ![Light theme](doc/screenshots/folders.webp) | ![Dark theme](doc/screenshots/folders-dark.webp) |

#### Viewer

One photo at a time, with the film strip below and its metadata and location beside it. The arrow keys step through the folder; Crop and Delete are one click away.

![Viewer with film strip and info panel](doc/screenshots/viewer.webp)

#### Organize

The folder you are sorting on the left, your target folders on the right with a number key each: select, press the key, done. ⌥ copies instead, `U` takes the last move back, and "Mark for deletion" is a target like any other.

![Organize](doc/screenshots/organize.webp)

#### Marked for deletion

Marked photos disappear from Folders and libraries but stay on disk until you delete them here, or restore them.

![Marked for deletion](doc/screenshots/marked.webp)

#### Libraries

A library indexes a folder tree once. Its folders then show what they hold: the four newest photos, how many there are and the years they span.

![A library with its folders as tiles of four photos](doc/screenshots/library.webp)

#### Libraries and the filter

Because it is indexed, a library can be filtered across all of it: date, shutter speed, aperture, focal length, ISO, camera, lens, film simulation. The results replace the folders beside the filter.

![Library filter](doc/screenshots/library-filter.webp)

#### Sharing a library

Each library is its own folder, anywhere: on this computer, a disk or a NAS. Shared from Edit library, it is the same library on a second installation, which adds the folder and gets its name; each keeps its own index.

![Edit library with Share with other installations switched on](doc/screenshots/library-sharing.webp)

#### Statistics

What you shoot with, when, where and in what colours, by topic: Equipment (cameras, lenses, formats, film simulations), Exposure (focal length, aperture, ISO), Time (time of day, calendar), Frame (aspect ratio), Colour (black and white, main colours, warm and cool) and Places, each with how it changed over the years. The overview has a card per topic showing a few of its charts, and the topics are in the sidebar while Statistics is open. Click a bar, a slice or a point and the photos it counts appear beside the charts.

| Overview | A topic |
|---|---|
| ![Statistics overview with a card per topic, each showing a few of its charts small](doc/screenshots/statistics.webp) | ![Equipment, with the photos of one camera and lens beside the charts](doc/screenshots/statistics-photos.webp) |

Exposure, Time, Colour and Places end with a view in three dimensions, drawn with WebGPU: every photo is a point of light, and where many photos meet, it glows. Drag to turn a view, scroll to zoom, point at a light to read it and click it to see the photo. Full view shows a view and its photos over the whole window.

| Colour space | Exposure space |
|---|---|
| ![Colour space: every photo at its main colour, lightness up and hue around, with a trail through each year's average colour](doc/screenshots/statistics-colour-space.webp) | ![Exposure space: every photo by focal length, aperture and ISO, coloured by camera, with a trail through each year's median settings](doc/screenshots/statistics-exposure-space.webp) |
| **Daylight** | **Space and time** |
| ![Daylight: every photo by day of the year around the ring, hour outward and brightness up, with a trail round the months](doc/screenshots/statistics-daylight.webp) | ![Space and time: every located photo where it was taken, on a floor of the world around home with distance on a log scale, and when, upward](doc/screenshots/statistics-space-time.webp) |

#### Map

Every photo with a location on one map. Photos close together form a group showing the newest one; zooming in splits the groups. The Photos column lists what is in view, the slider narrows it to a period, and Style switches between grey and colour tiles.

![Map with the photos column](doc/screenshots/map.webp)

#### Timeline

Every dated photo of every library on one time axis, oldest on the left. The graph below shows when the photos were taken; the frame marks what the rows above show, and the overview limits both to a span of months.

![Timeline with the time bar and a photo in the info panel](doc/screenshots/timeline.webp)

#### Rename, export, location

Tools for a selection: rename from EXIF tokens with a live preview, export to JPEG, PNG or WebP with size and metadata options, and set or remove the location of several photos at once.

| Rename | Export | Location |
|---|---|---|
| ![Batch rename](doc/screenshots/rename.webp) | ![Export](doc/screenshots/export.webp) | ![Set location](doc/screenshots/location.webp) |

#### Galleries

**Your photos, on your own website.** Photos you collect for a destination become galleries, and Publish turns them into static HTML on your own webspace:

- **A complete website** — many albums with an index, navigation and a sitemap, growing with every gallery you publish, and found by search engines.
- **Share links** — one gallery per link under an unguessable address, not indexed, for sending to family and friends.
- **Files** — the photos sized for a platform such as Instagram, in a folder, to post by hand.

Publish is the one action that exports, builds, uploads and checks the link; each gallery says in words where it stands. Uploading needs your own webspace reachable over SSH and uses rsync. Without an upload target the build stays in a local folder you can copy anywhere.

![Galleries](doc/screenshots/galleries.webp)

What Publish builds — a website with its albums, one album of it, and a gallery behind a share link:

| Website | Album | Share link |
|---|---|---|
| ![A generated website with two albums](doc/screenshots/website.webp) | ![An album of the generated website](doc/screenshots/website-album.webp) | ![A gallery behind a share link](doc/screenshots/share-link.webp) |

#### On a phone

On a phone Unterlumen is for looking: libraries, folders, photos, the map, the timeline and your galleries, with a tab bar at the bottom.

| Folders | Map | Timeline |
|---|---|---|
| <img src="doc/screenshots/phone-folders.webp" alt="Folders on a phone" width="320"> | <img src="doc/screenshots/phone-map.webp" alt="Map on a phone" width="320"> | <img src="doc/screenshots/phone-timeline.webp" alt="Timeline on a phone, newest first, with the scrubber" width="320"> |

## Contents

- [Features](#features)
- [Install](#install)
  - [One line (recommended)](#one-line-recommended)
  - [Download and double-click](#download-and-double-click)
  - [macOS](#macos)
  - [Windows](#windows)
  - [Linux](#linux)
  - [Docker / Podman](#docker--podman)
  - [Build from source](#build-from-source)
- [Usage](#usage)
  - [Advanced usage (command line)](#advanced-usage-command-line)
- [Docker / Podman](#docker--podman-1)
  - [Sharing channel config across installations](#sharing-channel-config-across-installations)
- [Keyboard Shortcuts](#keyboard-shortcuts)
- [Documentation](#documentation)
- [Development & Testing](#development--testing)
- [Notes](#notes)

## Features

- **Browse & Cull mode** — Justified, grid, or list view of photos in a directory with breadcrumb navigation
- **Organize** — One source folder shown large, your target folders as a list with a number key each: select, press the key, done. Moves run immediately with an undo beside the result; ⌥ copies instead. Targets are remembered between sessions, and "Mark for deletion" is a target like any other (folders included)
- **Waste bin** — Mark photos for deletion, review in a dedicated view, restore or permanently delete
- **Libraries (DAM)** — Index a folder into a SQLite library (no CGo). Photos are identified by SHA-256 so metadata survives renames. Full-text EXIF search (the filter points to Statistics for finding photos by colour, warmth or hour), key/value annotations, HQ thumbnails, and re-index progress via Server-Sent Events. Library data stored in `~/.unterlumen/libraries/<id>/` (overridable with `--lib-dir` / `UNTERLUMEN_LIB_DIR`)
- **Publish to your own website** — Needs your own webspace reachable over SSH; the upload uses rsync (without it, the build stays in a local folder). A two-phase collect-then-publish workflow. From library mode, select photos (from the folder tree or EXIF filter results, within a single library or across libraries) and use the selection bar's "Add to gallery…" to add them to a gallery's pending draft — a new or existing gallery/album. Nothing is exported yet, so a gallery can be built up incrementally across sessions. When ready, open Galleries and press Publish: one action exports the photos, builds the HTML (writing an XMP sidecar per photo using a custom `xmlns:ul` namespace — non-destructive and portable), uploads the result where an upload is configured (rsync channels), and checks the link, showing each step as it runs. A failed upload keeps the build, so the gallery reads "Built, not uploaded" and the retry only uploads. Afterwards you can open the gallery in a browser, copy the local path, reveal it in Finder/Explorer, or preview it locally. Supports named accounts (e.g. two Mastodon logins), optional grouped post IDs for carousels, and platform-optimised export (channel presets: Instagram 1080px, Mastodon 1920px, Website 2400px). Two publishing modes: a **multi-album site** (a real, growing website whose albums are navigable and indexable) and **single gallery** (one host holding many unrelated albums, each under its own unguessable 24-hex URL — for sharing one album by link with a specific group, marked Unlisted by default so it carries a `noindex` tag). A single-gallery channel holds as many albums as you like; each is published, renamed and deleted on its own. Channel settings managed via a dedicated UI; stored globally in `~/.unterlumen/channels.json` (overridable with `-channels-dir` / `UNTERLUMEN_CHANNELS_DIR`, e.g. to share channel config between multiple installations — see [Sharing channel config across installations](#sharing-channel-config-across-installations))
- **On a phone** — Below 700 px the sidebar becomes a tab bar (Libraries, Folders, Map, Timeline, Galleries) and Unterlumen becomes read-only: browse libraries and folders, open a photo full screen and swipe through the set, read its metadata in a sheet from the bottom, look at statistics, and see how your galleries are doing. Everything that changes files or settings stays on the desktop; a desktop-only place says so instead of showing controls that cannot work there. Reach it by binding the server to your network (`UNTERLUMEN_BIND=0.0.0.0`) or through the Docker deployment
- **Map** — Every photo with a location, from every library, on one large map. Photos taken close together form a group, shown as a round thumbnail of the newest one with the number of photos beside it; zooming in splits the groups until each photo stands at its own place. Clicking a group zooms in; a group whose photos share one spot, or a single photo, opens in the viewer (read-only: no crop, no marking for deletion). A time slider narrows the map to the months the photos were taken in. A column beside the map shows the photos in the part of the map on screen, newest first; it is open when you arrive on the desk, and the Photos button hides or shows it (on a phone it waits for the button). The column follows the map as you move and zoom; Done or Escape closes it, and it stays closed in that browser. The map is grey by default, light or dark with the theme; the Style switch shows it in colour. Its tiles come from OpenFreeMap and need an internet connection. Only photos in a library appear. See [ADR-0039](doc/architecture/adr/0039-map-place.md)
- **Explains itself** — Under the title of each place one sentence says what it is and what it does to your files, and links the places it names. "How Unterlumen works" (`#guide`, linked from each sentence, Settings and About) shows the whole model in a diagram — your folders, Folders, libraries, Map and Timeline, galleries, destinations — with a paragraph for each. See [ADR-0041](doc/architecture/adr/0041-explaining-the-model-in-the-app.md)
- **Statistics** — A place under Explore (`#statistics`, key 9) with a topic per sidebar sub-entry, shown while Statistics is open: Equipment, Exposure, Time, Frame, Colour and Places. The overview shows a card for each, which is how to reach them with the sidebar collapsed. Each card previews up to four of its topic's charts. Exposure, Time, Colour and Places end with a 3D view. A library select limits it to one library; a library's Statistics button opens it for that library and the folder shown. Developments over time are lines, one per camera or frame shape, with a legend and every value of a period on hover. Colour shows black and white against colour, the colour of each period as a strip of swatches, the main colours on a hue wheel, and warm against cool through the year (all years together or one year), from the measurements of ADR-0044. In Colour, Colour space shows every photo as a glowing point at its main colour in 3D (lightness up, hue around, chroma outward), with a trail through each period's mean colour. You can turn and zoom it, hover over a point to read it and click it to see the photo. In Exposure, Exposure space shows every photo by focal length, aperture and ISO in stops, coloured by camera, so your shooting habits show as glowing clusters. In Time, Daylight shows every photo by day of the year (around), hour (outward) and brightness (up). In Colour, Character shows every photo by brightness, contrast and colourfulness. In Places, Space and time shows every located photo by where it was taken, on a floor of the world seen from the middle of your photos with distance on a log scale, and when, upward. A trail of days climbs the home column and reaches out on journeys. Coastlines and borders are Natural Earth data (public domain), included in the app. Full view shows only the 3D view and its photos, dark, over the whole window. They need a browser with WebGPU ([ADR-0046](doc/architecture/adr/0046-colour-space-webgpu-glow-stage.md)). The statistics and the search read from indexes that cover them, so they stay fast on libraries of tens of thousands of photos ([ADR-0045](doc/architecture/adr/0045-indexes-cover-statistics-and-search.md)). A column beside the charts shows every photo of the scope when you arrive on the desk; clicking a value — a lens, an f-stop, an hour, a day, a camera in one year, a hue — shows its photos there instead; they open read-only. See [ADR-0043](doc/architecture/adr/0043-statistics-place-with-topics.md)
- **Photo appearance** — After each scan a library measures each photo's thumbnail: black and white, toned or colour, its average and main colours, brightness and contrast, sharpness and detail, and a hash for look-alikes. Photos indexed before are measured at the next start; Edit library → Maintenance → "Analyse photos again" measures a library anew. The values are the ground for statistics and search by colour to come. See [ADR-0044](doc/architecture/adr/0044-photo-appearance-from-thumbnails.md)
- **Timeline** — Every dated photo of every library on one time axis, each photo once even when it sits in several libraries. On a desk the photos run left (oldest) to right (newest) in two to four rows (⋯ menu) above a time bar: an overview with bracket handles that limits the view to a span of months, and an axis with a graph of how many photos were taken when, years and month names, and a frame marking what the rows show. Drag the frame or click the axis to go somewhere; scroll the photos and the frame follows. A click selects a photo and the info panel (I) shows it, a double click or Enter opens it read-only. On a phone the newest come first, grouped by month, with a scrubber down the right edge. Only the photos near the screen are loaded. Photos without a date taken are left out and counted. See [ADR-0040](doc/architecture/adr/0040-timeline-place.md)
- **Galleries** — A dedicated "Galleries" place lists every gallery across every destination (the UI name for a channel), grouped by destination and showing its state in words: Not online yet, Changes not online, Online, or Built. An action appears only where there is something to do — "Publish", "Publish N changes", "Check again". Link reachability is checked once when the screen opens and after each publish, and is reported next to the state with the time of the check rather than as a state of its own. Opening a gallery shows the photos waiting to go online (removable individually), its title, date, visibility toggle (single-gallery destinations), address, and Unpublish — with a scoped remote delete over SSH for rsync destinations. The channels list links to a destination's public site and to its galleries
- **Image viewer** — Full-screen image view with keyboard navigation, and a button that downloads the original file (a HEIF stays HEIF)
- **Download originals** — Download in the selection bar saves the selected photos as they are: one as its file, several as a ZIP of the originals
- **Crop tool** — Interactive crop in the fullscreen viewer. Draw a rectangle, pick an aspect ratio (free, standard, or cinema formats), and save in-place. All metadata including Fujifilm film simulation is preserved via exiftool
- **Info panel** — Collapsible sidebar showing file metadata, EXIF data, and location map for GPS-tagged photos. In library mode: editable title (stored as `dc:title` in the XMP sidecar, interoperable with Lightroom/Capture One) and free fields (stored as `ul:Fields` in the same sidecar, so a second installation reads them too; the library's index only copies them — see [ADR-0048](doc/architecture/adr/0048-notes-live-in-the-sidecar.md)), and a Publications section showing compact cards for each channel a photo was published to. Clicking a folder shows a folder dashboard: total size, file count, nesting depth, a squarified treemap of subfolder sizes (click to navigate), and a file-type breakdown. In library mode the folder dashboard also shows EXIF-based photo statistics (shooting date range, format breakdown, camera × lens usage, hourly activity chart). Available in browse, library, and fullscreen viewer
- **Convert & Export** — Export selected images to JPEG, PNG, or WebP with quality control, flexible scaling (original, percentage, max dimension), and EXIF metadata options (strip, keep, or keep without GPS). Shows per-file estimated output size and pixel dimensions. Saves to a local folder or downloads as a ZIP; server mode (`UNTERLUMEN_ROOT_PATH`) is ZIP-only
- **Batch rename** — Rename multiple photos using EXIF-based patterns (date, camera, film simulation, image title, etc.) with color-coded draggable token pills, live preview and conflict resolution. While it runs, the dialog says how many files it is renaming; the server reports no count, so none is shown. The `{title}` token inserts the photo's slugified title. Works in browse mode and all library views. Also includes a simple single-file rename option
- **Geolocation editing** — Set or remove GPS coordinates on one or more images via an interactive map picker (requires exiftool)
- **Thumbnail quality** — Standard (fast EXIF thumbnails) or High (full-image decode with bicubic resampling for retina displays), selectable in Settings
- **Sorting** — By name, date, or size, ascending or descending
- **Multi-select** — Click, Shift+click, Ctrl/Cmd+click for bulk operations
- **Status bar** — Live image count and selection count in every pane
- **Activity and the status line** — Everything that takes a moment says what it is doing in words ("Reading the folder…"), and after 3 seconds for how long. Where the total is known it shows a bar and "412 of 1 280 photos". Nothing shows for quick answers. Long work — library scans, ZIP exports, publishing, site rebuilds, deploys — also appears at the foot of the sidebar and stays there when you move to another place; a failure stays until you click it. See [ADR-0036](doc/architecture/adr/0036-activity-and-progress.md)
- **EXIF/HEIF orientation** — Portrait and rotated images display correctly
- **HEIF support** — Automatic conversion via ffmpeg (requires ffmpeg installed)
- **Read-ahead prefetch** — The viewer prefetches the next two images on each navigation for near-instant forward navigation, even over a NAS
- **Fujifilm film simulation** — Film simulation name (e.g. Classic Chrome, Velvia, Acros) shown in the info panel and as a grid overlay badge for Fujifilm images
- **Formats** — JPEG, PNG, GIF, WebP natively; HEIF/HEIC/HIF via ffmpeg

## Install

### One line (recommended)

Paste one line into a terminal. It downloads the newest release, checks it, installs the helper programs Unterlumen uses (ffmpeg, exiftool, cwebp, heif-convert), sets Unterlumen up as an app with an icon and opens it. Running the same line again updates it.

**macOS and Linux** — open Terminal and paste:

```
curl -fsSL https://huepattl.de/unterlumen/install | sh
```

**Windows** — open PowerShell and paste:

```
irm https://huepattl.de/unterlumen/install.ps1 | iex
```

Unterlumen then opens on its setup page: choose the folder that holds your photos, and press *Set up Unterlumen*. If another Unterlumen installation already works in that folder — on a NAS, say — the page offers to share its destinations and galleries (see [Sharing channel config across installations](#sharing-channel-config-across-installations)). Settings links back to the setup to change the folder later. Wherever the installed app asks for a folder, it opens your system's own folder dialog, which shows your NAS and external disks.

Where the helper programs come from: Homebrew when it is installed, otherwise, on a Mac, the makers' own downloads (ffmpeg by martin-riedl.de, exiftool from exiftool.org's SourceForge, cwebp from Google) into Unterlumen's own `tools` folder; apt, dnf or pacman on Linux (sudo asks for your password); winget on Windows. The scripts are [install/install.sh](install/install.sh) and [install/install.ps1](install/install.ps1) — read them before you run them if you like. `UNTERLUMEN_SKIP_TOOLS=1` leaves the helper programs alone, `UNTERLUMEN_VERSION=0.13.0` installs that version.

### Download and double-click

- **macOS:** [Unterlumen.dmg](https://github.com/bjblazko/unterlumen/releases/latest/download/Unterlumen.dmg) — for Apple Silicon and Intel. Open it and drag Unterlumen onto Applications. The first time you open it macOS says it cannot check the app, because it is not signed by an Apple developer account: open **System Settings → Privacy & Security**, scroll down to the note about Unterlumen and click **Open Anyway**.
- **Windows:** [Unterlumen-Setup.exe](https://github.com/bjblazko/unterlumen/releases/latest/download/Unterlumen-Setup.exe) — installs for your user without administrator rights. If Windows shows "Windows protected your PC", click **More info**, then **Run anyway**.

A package cannot install ffmpeg and exiftool for you; the setup page (and Settings) offers **Install the missing ones** instead.

### Install by hand with the terminal

Download the latest release for your platform from the [Releases page](https://github.com/bjblazko/unterlumen/releases) and extract the archive — you will get a single file called `unterlumen` (or `unterlumen.exe` on Windows).

The installer sets Unterlumen up as a proper desktop application with an icon, so you can open it from Spotlight, Launchpad, or the Start Menu just like any other app — no terminal needed afterwards.

#### macOS

1. **Open Terminal** — press **Cmd + Space**, type `Terminal`, and press **Enter**. A window with a text prompt appears.

2. **Go to your Downloads folder** — type the following and press **Enter**:
   ```
   cd ~/Downloads
   ```

3. **Allow the file to run** — type the following two commands, pressing **Enter** after each:
   ```
   xattr -d com.apple.quarantine unterlumen
   chmod +x unterlumen
   ```
   The first command removes macOS's download restriction — macOS blocks programs downloaded from the internet by default, and this tells it the file is safe to run. The second makes the file executable.

4. **Run the installer** — type the following and press **Enter**:
   ```
   ./unterlumen -desktop-install
   ```

5. **Done.** Open Unterlumen: Folders browses any folder on your Mac, and Libraries → New library… catalogs one. Unterlumen now appears in **Spotlight** (press **Cmd + Space** and type "Unterlumen") and in **Launchpad**. You can close the Terminal window.

> **If macOS blocks the app when you first open it** ("cannot be opened because the developer cannot be verified"), open **System Settings → Privacy & Security**, scroll down, and click **Open Anyway**. If that button does not appear (common on macOS Sonoma and later), open Terminal and run:
> ```
> xattr -d com.apple.quarantine ~/Applications/Unterlumen.app
> ```
> Then try opening Unterlumen again from Spotlight or Launchpad.

#### Windows

1. **Open PowerShell** — press the **Windows key**, type `PowerShell`, and press **Enter**. A blue window with a text prompt appears.

2. **Go to your Downloads folder** — type the following and press **Enter**:
   ```
   cd $HOME\Downloads
   ```

3. **Run the installer** — type the following and press **Enter**:
   ```
   .\unterlumen.exe -desktop-install
   ```

4. **Done.** Unterlumen now appears in the **Start Menu**. You can close the PowerShell window. Open it: Folders browses any folder, and Libraries → New library… catalogs one.

#### Linux

1. **Open a terminal** — on GNOME, press the **Super key** (the Windows key on most keyboards), type `Terminal`, and press **Enter**. On KDE, right-click the desktop and choose **Open Terminal**. The terminal name varies by distribution (GNOME Terminal, Konsole, xterm, etc.) but any of them will work.

2. **Go to your Downloads folder** — type the following and press **Enter**:
   ```
   cd ~/Downloads
   ```

3. **Allow the file to run** — type the following and press **Enter**:
   ```
   chmod +x unterlumen
   ```

4. **Run the installer** — type the following and press **Enter**:
   ```
   ./unterlumen -desktop-install
   ```

5. **Done.** Unterlumen now appears in your application launcher (GNOME Activities, KDE application menu, etc.). You can close the terminal window. Open it: Folders browses any folder, and Libraries → New library… catalogs one.

#### Re-installing or updating

To update Unterlumen, run the one line again, or download the new binary and run `-desktop-install` again — it replaces the app and keeps your settings. They are in `config.json` in Unterlumen's configuration folder (`~/Library/Application Support/Unterlumen` on macOS, `%APPDATA%\Unterlumen` on Windows, `~/.config/Unterlumen` on Linux). Installing over an older version copies the settings its launcher carried into that file first.

### Docker / Podman

Pre-built images for `linux/amd64` and `linux/arm64` are on the GitHub Container Registry and include ffmpeg, heif-convert, cwebp, and exiftool — no separate install needed. Jump to the [Docker / Podman](#docker--podman) section below.

### Build from source

Requires Go 1.27+. Binaries built with it run on macOS 13 Ventura or later.

```
cd src && go build -o ../unterlumen .
```

### Optional dependencies

- **ffmpeg** — required for HEIF/HEIC/HIF support (embedded preview extraction and HEVC decode fallback) and WebP export (when built with `libwebp`)
- **cwebp** (from `libwebp` / `brew install webp`) — required for WebP export when ffmpeg is built without `libwebp` (e.g. the default Homebrew ffmpeg on macOS). If ffmpeg already has WebP support, cwebp is not needed.
- **heif-convert** (from `libheif-examples` / `libheif`) — recommended alongside ffmpeg; handles HEIF files that ffmpeg cannot parse, such as standard Fujifilm HEIC files that carry no embedded JPEG preview stream. Without it those files show a placeholder instead of a thumbnail.
- **exiftool** — required for Set/Remove Geolocation, Batch Rename, and Export EXIF copy/GPS-strip

## Usage

Once installed with `-desktop-install`, just open Unterlumen from your OS launcher (Spotlight / Launchpad on macOS, Start Menu on Windows, application grid on Linux). The app opens in its own window and closes cleanly when you are done.

### Advanced usage (command line)

You can also run Unterlumen directly from the terminal without installing it. This is useful for scripting, server deployments, or trying it out before installing.

```
./unterlumen [flags] [directory]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `directory` | Directory to start in (default: home directory). Navigation is unrestricted — users can navigate to any directory on the filesystem. |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | HTTP server port (env: `UNTERLUMEN_PORT`) |
| `-bind` | `localhost` | Bind address (`0.0.0.0` for remote access) (env: `UNTERLUMEN_BIND`) |
| `-lib-dir` | `~/.unterlumen` | Root directory for library data (env: `UNTERLUMEN_LIB_DIR`) |
| `-channels-dir` | (same as `-lib-dir`) | Directory for `channels.json`; override to share channel config across installations (env: `UNTERLUMEN_CHANNELS_DIR`) |
| `-desktop` | off | Open in a Chrome/Chromium app window (no URL bar). Server exits when the window is closed. Falls back to the default browser if Chrome is not found. |
| `-desktop-install` | — | Installer: sets up a native app launcher with icon (macOS `.app`, Linux `.desktop`, Windows Start Menu shortcut). It asks nothing; the app is set up on its first start. |

**Environment variables:**

| Variable | Description |
|----------|-------------|
| `UNTERLUMEN_PORT` | HTTP server port. Overridden by `-port` flag. |
| `UNTERLUMEN_BIND` | Bind address. Overridden by `-bind` flag. |
| `UNTERLUMEN_ROOT_PATH` | Restrict navigation to this directory. The server starts here and users cannot navigate above it. Takes effect only when no `directory` argument is provided. |
| `UNTERLUMEN_LIB_DIR` | Root directory for library data (SQLite databases, thumbnails, channel exports). Default: `~/.unterlumen`. Overridden by `-lib-dir` flag. |
| `UNTERLUMEN_CHANNELS_DIR` | Directory for `channels.json`. Default: same as `-lib-dir`. Overridden by `-channels-dir` flag. See [Sharing channel config across installations](#sharing-channel-config-across-installations). |

**The installed app's settings:** started without a `directory` and without `UNTERLUMEN_ROOT_PATH` — the way the installed app's launcher starts it — Unterlumen reads `-lib-dir`, its shared folder (`-channels-dir`) and port from `config.json` in its configuration folder; Settings → Shared folder changes them. It has no photo folder: Folders browses the whole disk and each library names its own folder. A `photosDir` from an older version is removed at start, and its `.unterlumen-shared` becomes the shared folder. A flag or an environment variable still wins over the file.

**Path resolution priority:**

1. **Command-line argument** — starts in the given directory; navigation unrestricted (up to filesystem root)
2. **`UNTERLUMEN_ROOT_PATH` env var** — starts there and restricts navigation to that directory
3. **Default** — starts in the user's home directory; navigation unrestricted

**Examples:**

```
# Browse photos in ~/Pictures and open in the browser manually
./unterlumen ~/Pictures

# Open as a desktop app window (Chrome required; falls back to default browser)
./unterlumen -desktop ~/Pictures

# Use a different port
./unterlumen -port 3000 ~/Pictures

# Allow access from other machines on the network
./unterlumen -bind 0.0.0.0 ~/Pictures

# Restrict navigation to /mnt/photos (useful for self-hosted setups)
UNTERLUMEN_ROOT_PATH=/mnt/photos ./unterlumen
```

Then open `http://localhost:8080` in your browser (or whichever port you configured).

## Docker / Podman

Pre-built images for `linux/amd64` and `linux/arm64` are published to the GitHub Container Registry and include ffmpeg, heif-convert, cwebp, and exiftool — no separate install needed.

```
docker run -p 8080:8080 -v /path/to/photos:/photos ghcr.io/bjblazko/unterlumen:latest
```

**Podman:** The container runs as UID 1000, but on macOS the mounted directory is owned by your host user (typically UID 501 or 502). Pass `--user $(id -u):$(id -g)` to run as your own UID:

```
podman run --user $(id -u):$(id -g) -p 8080:8080 -v /path/to/photos:/photos:ro ghcr.io/bjblazko/unterlumen:latest
```

Then open `http://localhost:8080`.

By default the container runs in **server mode** — navigation is locked to `/photos`. Override environment variables to change behaviour:

| Variable | Default (container) | Description |
|----------|---------------------|-------------|
| `UNTERLUMEN_PORT` | `8080` | HTTP port |
| `UNTERLUMEN_BIND` | `0.0.0.0` | Bind address |
| `UNTERLUMEN_ROOT_PATH` | `/photos` | Root directory (navigation locked here) |
| `UNTERLUMEN_LIB_DIR` | `/data` | Libraries, thumbnails and generated galleries; mount a volume here |
| `UNTERLUMEN_CACHE_DIR` | `/cache` | Preview cache |

**Example with Docker Compose** — or let [huepattl.de/products/unterlumen-nas.html](https://huepattl.de/products/unterlumen-nas.html) write it for your NAS, with the steps for Synology, QNAP, TrueNAS, Unraid or Portainer:

```yaml
services:
  unterlumen:
    image: ghcr.io/bjblazko/unterlumen:latest
    user: "1026:100"            # the owner of the photos; Unterlumen writes there
    ports:
      - "8080:8080"
    volumes:
      - "/volume1/photo:/photos"
      - unterlumen-data:/data   # libraries, thumbnails, generated galleries
      - unterlumen-cache:/cache
    restart: unless-stopped

volumes:
  unterlumen-data:
  unterlumen-cache:
```

**Portainer:** enter `https://huepattl.de/unterlumen/portainer.json` under *Settings → App Templates*; Unterlumen then appears under *App Templates* and asks for the photo folder, its owner and the port. (That URL replaces the templates Portainer lists by default.)

The image keeps its libraries in `/data` (`UNTERLUMEN_LIB_DIR`) and its cache in `/cache`; both are writable for any user, so `user:` can be the owner of the photos.

### Sharing channel config across installations

If you run Unterlumen on more than one machine against the *same* photo folders (e.g. a Docker instance on a NAS that also serves the photos, plus a native install on a Mac that mounts them over the network), each installation normally has its own independent `channels.json` under its own `-lib-dir` — so a channel published from one install won't be recognized by name on the other, even though the underlying "published" record in the photo's `.xmp` sidecar is already shared and portable.

Point `-channels-dir` (or `UNTERLUMEN_CHANNELS_DIR`) on **both** installations at the same directory on a filesystem they can both reach (e.g. a folder alongside the photo library on the NAS) to share channel definitions between them, while `-lib-dir` (SQLite database, thumbnails, search index) stays independent and local to each machine as usual:

```
# NAS (Docker), photos already mounted at /photos:
docker run -p 8080:8080 \
  -v /path/to/photos:/photos \
  -e UNTERLUMEN_CHANNELS_DIR=/photos/.unterlumen-shared \
  ghcr.io/bjblazko/unterlumen:latest

# Mac (native), same folder reachable over the network mount:
./unterlumen -channels-dir "/Volumes/<share>/.unterlumen-shared" ~/Pictures
```

**By convention:** on a server, a folder named `.unterlumen-shared` directly inside the folder it serves is used this way without any flag. On the NAS, **Settings → Sharing → Share with another installation** makes it — so with the NAS example above, the Mac only has to choose that folder (as it mounts it) under **Settings → Shared folder**. Choosing a folder there also makes `.unterlumen-shared` when none is there yet, and copies this installation's destinations into it; what is already shared there is never overwritten.

**Libraries are shared one by one.** In **Edit library**, *Share with other installations* writes a small `.unterlumen-library.json` (the library's ID, name and description, no path) into the library's folder. The other installation adds that folder with New library… and gets the same library — same ID, same name — with its own index. A library made of that folder before it was shared offers *Join* instead, which keeps its index and thumbnails. Libraries you do not share stay on one installation, so two installations can have some libraries in common and not others. See [ADR-0047](doc/architecture/adr/0047-independent-libraries-shared-per-library.md).

Notes:
- The mount used for `UNTERLUMEN_CHANNELS_DIR` must be writable (the read-only `:ro` mount shown above works for browsing but not for a channels directory located on it).
- A **website** channel also keeps its album list in the shared directory, one file per album under `albums/<channel>/`, so publishing album A from one installation and album B from the other leaves a site index and sitemap with both. **Shared:** `channels.json`, the album register (one `<postID>.json` per album, and a `<postID>.deleted` tombstone for a deleted one), and the XMP sidecars next to the photos, which record each album's address. **Per machine:** `-lib-dir` (database, thumbnails, search index), each destination's output folder (`output-paths.json` in `-lib-dir`), and the generated output, including its `site.json` cache. The album files contain no paths, so the two installations may disagree about the photo folder's location. When upgrading an existing setup, update and open the installation that publishes (and holds the real `site.json`) first: it copies its albums into the register once, and only then should the other installation start. *Rebuild album list* (Destinations → Advanced) then writes each album's address into its photos' sidecars, and restores the list from them if it is ever lost; see [ADR-0035](doc/architecture/adr/0035-shared-album-register.md).
- `channels.json` can include credentials (e.g. publish tokens) for some channel handlers — only point installations at a shared directory you trust equally.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Arrow keys | Navigate grid/list in browse view; prev/next in image viewer |
| Enter | Open focused folder or image |
| Space | Toggle selection of focused item |
| Escape | Close viewer / go up a directory / close the Map's photos column |
| `I` | Collapse or expand the info panel |
| `\` | Collapse or expand the sidebar |
| Backspace / Delete / Cmd+D | Mark selected files for deletion |
| Cmd/Ctrl+A | Select all files in current pane |
| 1 / 2 / 3 / 4 / 5 / 6 / 7 / 8 / 9 | Go to Folders / Marked for deletion / Organize / Libraries / Galleries / Destinations / Map / Timeline / Statistics |
| Tab | Switch panes in File Manager mode |
| F5 | Copy selected files (File Manager) |
| F6 | Move selected files (File Manager) |
| Ctrl/Cmd + Click | Toggle selection |
| Shift + Click | Range selection |

## Documentation

- [Changelog](CHANGELOG.md)
- [Architecture (arc42)](doc/architecture/arc42.md) — system overview, building blocks, decisions, and ADR index

## Development & Testing

### E2E tests

Requires the binary to be built first.

```
cd src && go build -o ../unterlumen .
cd e2e && npm ci
npm run setup        # download test fixtures once
npm test             # run all tests headlessly (CI mode)
npm run test:headed  # run with browser visible
```

To use the **Playwright interactive UI** — watch tests run step-by-step, inspect DOM snapshots, and re-run individual specs:

```
cd e2e && npx playwright test --ui
```

This opens a browser-based test runner at a local port. Select any spec or individual test in the sidebar and click the play button to run it with a live preview pane.

Test reports and failure screenshots/videos are saved to `e2e/playwright-report/` and `e2e/test-results/`.

### Screenshots

The pictures in [Screenshots](#screenshots) come from a script, not by hand:

```
cd src && go build -o ../unterlumen .
cd e2e && npm run screenshots            # all of them
npm run screenshots -- map viewer        # only these
```

It starts a throwaway server on port 8097 with a copy of `src/examples` and its own library directory, sets up a library, destinations and galleries through the API, and writes WebP files to `doc/screenshots/` (light theme; `folders-dark` is the dark twin). It needs `cwebp` (`brew install webp`) and a network connection for the map tiles. The shots themselves are in `e2e/screenshots/shots.mjs`.

## Notes

- Browse/cull/file-manager state is in-memory and discarded on exit. Library mode writes SQLite databases and thumbnails to `~/.unterlumen/` (or the configured `lib-dir`)
- By default the server binds to `localhost` only; use `-bind 0.0.0.0` if you need remote access (no authentication is provided)
- HEIF/HEIC/HIF conversion shells out to ffmpeg; file paths are passed as arguments (not interpolated into a shell string)
- `UNTERLUMEN_ROOT_PATH` is ignored when a directory argument is also provided on the command line
