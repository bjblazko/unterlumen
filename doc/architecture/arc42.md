# arc42 Architecture Documentation — Unterlumen

*Last modified: 2026-02-24*

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
| `/api/browse` | HTTP GET | JSON (directory listing) |
| `/api/thumbnail` | HTTP GET | JPEG/PNG binary |
| `/api/image` | HTTP GET | JPEG/PNG/GIF/WebP binary |
| `/api/copy` | HTTP POST | JSON request/response |
| `/api/move` | HTTP POST | JSON request/response |
| `/api/info` | HTTP GET | JSON (file metadata + EXIF) |
| `/api/delete` | HTTP POST | JSON request/response |
| `/` (static) | HTTP GET | HTML/CSS/JS files |

## 4. Solution Strategy

| Goal | Approach |
|------|----------|
| Fast thumbnails | Extract embedded EXIF thumbnails rather than decoding full images ([ADR-0003](adr/0003-exif-thumbnails.md)) |
| Simple culling | Dual-pane Commander interface with copy/move ([ADR-0005](adr/0005-commander-style-culling.md)) |
| Easy deployment | Single Go binary, static files served from `web/` directory ([ADR-0001](adr/0001-go-http-server-with-browser-ui.md)) |
| No state management | Filesystem is the only store; no database ([ADR-0002](adr/0002-no-persistence.md)) |
| HEIF support | Shell out to ffmpeg ([ADR-0004](adr/0004-heif-via-ffmpeg.md)) |

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
| `internal/api` | HTTP route registration, request handling, path validation |
| `internal/media` | Filesystem scanning, EXIF extraction, format detection, ffmpeg invocation |

### 5.3 Level 2 — Frontend Modules

| File | Responsibility |
|------|---------------|
| `app.js` | Application entry point, mode switching, global keyboard shortcuts |
| `browse.js` | `BrowsePane` class — directory listing, grid/list rendering, selection |
| `commander.js` | `Commander` class — dual-pane layout, copy/move orchestration |
| `viewer.js` | `Viewer` class — full-image display, prev/next navigation |
| `infopanel.js` | `InfoPanel` class — collapsible side panel showing file metadata and EXIF data |
| `api.js` | `API` object — fetch wrappers for all backend endpoints |

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

The binary and `web/` directory must be co-located (the server serves static files from `./web/` relative to the working directory). The root photo directory is specified as a CLI argument.

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

- Thumbnail and image responses include `Cache-Control: public, max-age=3600`
- Browser caching reduces repeated requests when navigating back to a previously viewed directory
- No server-side cache exists (consistent with [ADR-0002](adr/0002-no-persistence.md))

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

## 10. Quality Requirements

### 10.1 Quality Tree

```
Quality
├── Simplicity
│   ├── Single binary, no database
│   ├── No build toolchain for frontend
│   └── No configuration files
├── Performance
│   ├── EXIF thumbnails (no generation)
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
| User starts the binary with no arguments | Simplicity | Server starts, serving the current directory on localhost:8080 |
| User runs on a headless server | Portability | Binds to 0.0.0.0 with `-bind` flag; browser on another machine connects |

## 11. Risks and Technical Debt

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| EXIF thumbnails too small for high-DPI displays | Medium | Low | Could add a generated thumbnail fallback with in-memory LRU cache |
| ffmpeg not installed on target system | Medium | Low | HEIF files fail gracefully; all other formats work. Error message guides user. |
| Large directories (10k+ files) slow to list | Low | Medium | EXIF date extraction is per-file; could be parallelized or lazy-loaded |
| `innerHTML` re-rendering causes flicker | Low | Low | Could switch to incremental DOM updates if UX suffers |

## 12. Glossary

| Term | Definition |
|------|------------|
| Culling | The process of selecting the best photos from a set and discarding or separating the rest |
| Commander mode | Dual-pane file browser layout inspired by Norton Commander (1986) |
| EXIF | Exchangeable Image File Format — metadata standard embedded in JPEG and other image files |
| HEIF/HEIC | High Efficiency Image Format — container format used by Apple devices for photos |
| Path traversal | An attack where crafted file paths (e.g. `../../etc/passwd`) escape the intended root directory |
