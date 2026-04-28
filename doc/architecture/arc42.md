# arc42 Architecture Documentation — Unterlumen

*Last modified: 2026-04-28*

## 1. Introduction and Goals

### 1.1 Requirements Overview

Unterlumen is a photo browser and culler. It allows users to:

- Browse directories of photos in grid or list view
- View individual photos full-screen with prev/next navigation
- Organize photos by copying or moving them between directories using a dual-pane Commander interface
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
| `/` (static) | HTTP GET | HTML/CSS/JS files |

## 4. Solution Strategy

| Goal | Approach |
|------|----------|
| Fast thumbnails | Extract embedded EXIF thumbnails rather than decoding full images ([ADR-0003](adr/0003-exif-thumbnails.md)) |
| Simple culling | Dual-pane Commander interface with copy/move ([ADR-0005](adr/0005-commander-style-culling.md)) |
| Easy deployment | Single Go binary, static files served from `web/` directory ([ADR-0001](adr/0001-go-http-server-with-browser-ui.md)) |
| No state management | Filesystem is the only store; no database ([ADR-0002](adr/0002-no-persistence.md)) |
| HEIF support | Shell out to ffmpeg ([ADR-0004](adr/0004-heif-via-ffmpeg.md)) |
| Large-folder performance | In-memory scan cache, deferred EXIF extraction, chunked rendering ([ADR-0011](adr/0011-scan-cache-deferred-exif.md)) |
| Client-side settings | User preferences persisted in `localStorage` ([ADR-0012](adr/0012-client-side-settings.md)) |

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
| `main` | CLI flag parsing, HTTP server startup |
| `internal/api` | HTTP route registration; delegates to domain subpackages |
| `internal/api/browse` | `/api/browse`, `/api/browse/dates`, `/api/browse/meta`, `/api/thumbnail`, `/api/image`, `/api/info` handlers |
| `internal/api/export` | `/api/export/*` handlers; ZIP token store |
| `internal/api/fileops` | Copy, move, delete, mkdir, rename, recursive-list handlers |
| `internal/api/location` | Set/remove GPS location handlers |
| `internal/api/batchrename` | Batch-rename preview and execute handlers; pattern resolution, filename sanitising, conflict suffixing |
| `internal/pathguard` | `SafePath` — shared security primitive; symlink-aware root-boundary check |
| `internal/media` | Filesystem scanning, EXIF extraction (exif.go), orientation (orientation.go), thumbnail generation (thumbnail.go), export/conversion (export.go), Fujifilm simulations (fujifilm.go), aspect-ratio labels (aspectratio.go) |

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
| `commander.js` | `Commander` class — dual-pane layout, copy/move orchestration |
| `viewer.js` | `Viewer` class — full-image display, prev/next navigation |
| `infopanel.js` | `InfoPanel` class — collapsible side panel showing file metadata and EXIF data |
| `api.js` | `API` object — fetch wrappers for all backend endpoints |
| `maplibre-gl.js` | External dependency (CDN) — MapLibre GL JS for location maps ([ADR-0013](adr/0013-maplibre-location-maps.md)) |

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

### 6.2 Copy Files (Commander Mode)

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
  │  GET /api/browse (×2)     │  Refresh both panes       │
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

- **Browser caching** — Thumbnail and image responses include `Cache-Control: no-cache` to prevent stale images after server updates while still allowing conditional requests.
- **In-memory scan cache** — Directory listings are cached in a `sync.Map` keyed by directory path. Entries are invalidated when the directory modification time changes or when a copy/move/delete operation touches the directory. Consistent with [ADR-0002](adr/0002-no-persistence.md) — the cache is purely in-memory and lost on restart. See [ADR-0011](adr/0011-scan-cache-deferred-exif.md).
- **HEIF disk cache** — Converted JPEG data from HEIF/HEIC/HIF files is cached in `$TMPDIR/unterlumen-cache/`. Cache keys include file path, modification time, and purpose (full/preview). Survives restarts but not OS temp cleanup. See [ADR-0004](adr/0004-heif-via-ffmpeg.md).

## 9. Architecture Decisions

See the [ADR directory](adr/) for all recorded decisions:

- [ADR-0001](adr/0001-go-http-server-with-browser-ui.md) — Go HTTP server with browser UI
- [ADR-0002](adr/0002-no-persistence.md) — No persistence, in-memory state only
- [ADR-0003](adr/0003-exif-thumbnails.md) — EXIF embedded thumbnails
- [ADR-0004](adr/0004-heif-via-ffmpeg.md) — HEIF support via ffmpeg
- [ADR-0005](adr/0005-commander-style-culling.md) — Commander-style dual-pane culling
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
│   └── Browser-side caching
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
| Commander mode | Dual-pane file browser layout inspired by Norton Commander (1986) |
| EXIF | Exchangeable Image File Format — metadata standard embedded in JPEG and other image files |
| HEIF/HEIC | High Efficiency Image Format — container format used by Apple devices for photos |
| Path traversal | An attack where crafted file paths (e.g. `../../etc/passwd`) escape the intended root directory |
