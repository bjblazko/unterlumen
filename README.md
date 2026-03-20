# Unterlumen

A photo browser and culler that runs as a local web server. Browse your photo library in the browser, view images full-screen, and organize files using a dual-pane File Manager interface.

## Features

- **Browse & Cull mode** — Justified, grid, or list view of photos in a directory with breadcrumb navigation
- **File Manager mode** — Dual-pane Norton Commander-style layout for copying/moving files between directories
- **Waste bin** — Mark photos for deletion, review in a dedicated view, restore or permanently delete
- **Image viewer** — Full-screen image view with keyboard navigation
- **Info panel** — Collapsible sidebar showing file metadata, EXIF data, and location map for GPS-tagged photos. Available in browse and fullscreen viewer
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

## Requirements

- Go 1.21+
- ffmpeg (optional, only needed for HEIF/HEIC/HIF files)
- exiftool (optional, needed for Set/Remove Geolocation and Batch Rename)

## Install

```
go build -o unterlumen .
```

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

## Notes

- All state is in-memory and discarded on exit — no database, no config files written
- By default the server binds to `localhost` only; use `-bind 0.0.0.0` if you need remote access (no authentication is provided)
- HEIF/HEIC/HIF conversion shells out to ffmpeg; file paths are passed as arguments (not interpolated into a shell string)
- `UNTERLUMEN_ROOT_PATH` is ignored when a directory argument is also provided on the command line
