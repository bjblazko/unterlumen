# Unterlumen

A photo browser and culler that runs as a local web server. Browse your photo library in the browser, view images full-screen, and organize files using a dual-pane File Manager interface.

## Features

- **Browse & Cull mode** — Justified, grid, or list view of photos in a directory with breadcrumb navigation
- **File Manager mode** — Dual-pane Norton Commander-style layout for copying/moving files between directories
- **Waste bin** — Mark photos for deletion, review in a dedicated view, restore or permanently delete
- **Image viewer** — Full-screen image view with keyboard navigation
- **Info panel** — Collapsible sidebar showing file metadata, EXIF data, and location map for GPS-tagged photos. Available in browse and fullscreen viewer
- **Convert & Export** — Export selected images to JPEG, PNG, or WebP with quality control, flexible scaling (original, percentage, max dimension), and EXIF metadata options (strip, keep, or keep without GPS). Shows per-file estimated output size and pixel dimensions. Saves to a local folder or downloads as a ZIP; server mode (`UNTERLUMEN_ROOT_PATH`) is ZIP-only
- **Batch rename** — Rename multiple photos using EXIF-based patterns (date, camera, film simulation, etc.) with color-coded draggable token pills, live preview, conflict resolution, and progress indication. Also includes a simple single-file rename option
- **Geolocation editing** — Set or remove GPS coordinates on one or more images via an interactive map picker (requires exiftool)
- **Thumbnail quality** — Standard (fast EXIF thumbnails) or High (full-image decode with bicubic resampling for retina displays), selectable in Settings
- **Sorting** — By name, date, or size, ascending or descending
- **Multi-select** — Click, Shift+click, Ctrl/Cmd+click for bulk operations
- **Status bar** — Live image count and selection count in every pane
- **EXIF/HEIF orientation** — Portrait and rotated images display correctly
- **HEIF support** — Automatic conversion via ffmpeg (requires ffmpeg installed)
- **Fujifilm film simulation** — Film simulation name (e.g. Classic Chrome, Velvia, Acros) shown in the info panel and as a grid overlay badge for Fujifilm images
- **Formats** — JPEG, PNG, GIF, WebP natively; HEIF/HEIC/HIF via ffmpeg

### Screenshots

Browse mode:
![Browse mode](doc/screenshot-1-overview.png)

Fullscreen (here with optional file and EXIF data):
![File Manager mode](doc/screenshot-2-fullscreen-and-exif.png)

File manager  ("Commander style") mode:
![File Manager mode](doc/screenshot-3-filemanager.png)

Waste bin (marked for deletion during culling) mode:
![Marked for deletion](doc/screenshot-4-wastebin.png)

Add (or remove) geolocation to one or more files:
![Marked for deletion](doc/screenshot-5-addgeolocation.png)

Export to ZIP (download) or destination folder and convert to JPEG, PNG or WebP with optional scaling:
![Export and resize](doc/screenshot-6-export.png)

## Install

Three options — pick the one that fits your setup:

### Pre-built binary (recommended)

Download the latest release for your platform from the [Releases page](https://github.com/bjblazko/unterlumen/releases). Extract the archive and run the binary directly — no runtime dependencies required (ffmpeg and exiftool are optional extras, see below).

### Docker / Podman

Pre-built images for `linux/amd64` and `linux/arm64` are on the GitHub Container Registry and include ffmpeg and exiftool — no separate install needed. Jump to the [Docker / Podman](#docker--podman) section below.

### Build from source

Requires Go 1.21+.

```
cd src && go build -o ../unterlumen .
```

### Optional dependencies

- **ffmpeg** — required for HEIF/HEIC/HIF support
- **exiftool** — required for Set/Remove Geolocation, Batch Rename, and Export EXIF copy/GPS-strip

## Usage

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

**Environment variables:**

| Variable | Description |
|----------|-------------|
| `UNTERLUMEN_PORT` | HTTP server port. Overridden by `-port` flag. |
| `UNTERLUMEN_BIND` | Bind address. Overridden by `-bind` flag. |
| `UNTERLUMEN_ROOT_PATH` | Restrict navigation to this directory. The server starts here and users cannot navigate above it. Takes effect only when no `directory` argument is provided. |

**Path resolution priority:**

1. **Command-line argument** — starts in the given directory; navigation unrestricted (up to filesystem root)
2. **`UNTERLUMEN_ROOT_PATH` env var** — starts there and restricts navigation to that directory
3. **Default** — starts in the user's home directory; navigation unrestricted

**Examples:**

```
# Browse photos in ~/Pictures; navigate freely around the filesystem
./unterlumen ~/Pictures

# Use a different port
./unterlumen -port 3000 ~/Pictures

# Allow access from other machines on the network
./unterlumen -bind 0.0.0.0 ~/Pictures

# Restrict navigation to /mnt/photos (useful for self-hosted setups)
UNTERLUMEN_ROOT_PATH=/mnt/photos ./unterlumen

# Run on port 3000 via environment variable
UNTERLUMEN_PORT=3000 ./unterlumen ~/Pictures
```

Then open `http://localhost:8080` in your browser.

## Docker / Podman

Pre-built images for `linux/amd64` and `linux/arm64` are published to the GitHub Container Registry and include ffmpeg and exiftool.

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

**Example with Docker Compose:**

```yaml
services:
  unterlumen:
    image: ghcr.io/bjblazko/unterlumen:latest
    ports:
      - "8080:8080"
    volumes:
      - /mnt/photos:/photos:ro
    restart: unless-stopped
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Arrow keys | Navigate grid/list in browse view; prev/next in image viewer |
| Enter | Open focused folder or image |
| Space | Toggle selection of focused item |
| Escape | Close viewer / go up a directory |
| `I` | Toggle info panel |
| Backspace / Delete / Cmd+D | Mark selected files for deletion |
| Cmd/Ctrl+A | Select all files in current pane |
| Cmd/Ctrl+1/2/3 | Switch to Browse & Cull / File Manager / Marked for Deletion |
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

## Notes

- All state is in-memory and discarded on exit — no database, no config files written
- By default the server binds to `localhost` only; use `-bind 0.0.0.0` if you need remote access (no authentication is provided)
- HEIF/HEIC/HIF conversion shells out to ffmpeg; file paths are passed as arguments (not interpolated into a shell string)
- `UNTERLUMEN_ROOT_PATH` is ignored when a directory argument is also provided on the command line
