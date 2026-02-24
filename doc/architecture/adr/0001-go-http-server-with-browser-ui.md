# ADR-0001: Go HTTP Server with Browser UI

*Last modified: 2026-02-21*

## Status

Accepted

## Context

The application needs a user interface for browsing and organizing photos. Common approaches include:

- Native desktop application (Qt, GTK, Cocoa)
- Electron/Tauri wrapper around a web UI
- Go HTTP server with browser-based UI
- Pure CLI tool

The target audience uses the app both locally and on remote machines (e.g. a NAS or headless server with photos).

## Decision

Run as a Go HTTP server that serves a browser-based frontend. No desktop wrapper (Electron, Tauri) is used.

## Consequences

- **Single binary deployment** — `go build` produces one executable with no runtime dependencies beyond the OS and optionally ffmpeg.
- **Remote use** — Works naturally over SSH tunnels or LAN by binding to `0.0.0.0`. No X11 forwarding or remote desktop needed.
- **Cross-platform** — Go compiles for all major OS/arch combinations; the browser handles rendering differences.
- **No native OS integration** — Cannot register as a default image viewer or use native file dialogs. Acceptable for the use case.
- **Static files embedded or served from disk** — Currently served from the `web/` directory alongside the binary. Could be embedded via `go:embed` in the future.
