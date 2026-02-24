# ADR-0004: HEIF/HEIC Support via ffmpeg Shell-Out

*Last modified: 2026-02-21*

## Status

Accepted

## Context

HEIF/HEIC is the default photo format on modern iPhones and iPads. Supporting it is important for the target use case. Options:

1. **Pure Go HEIF decoder** — No mature, well-maintained library exists in the Go ecosystem.
2. **CGo bindings to libheif** — Adds a C dependency, complicating cross-compilation and deployment.
3. **Shell out to ffmpeg** — ffmpeg has robust HEIF decoding. It is widely available on systems that handle media files.

## Decision

Convert HEIF/HEIC files to JPEG on-the-fly by invoking ffmpeg as a subprocess. The converted output is piped to stdout (no temporary files).

## Consequences

- **External dependency** — ffmpeg must be installed for HEIF support. Without it, HEIF files will fail to display but all other formats work normally.
- **No CGo** — The binary remains a pure Go build with `CGO_ENABLED=0` cross-compilation support.
- **Performance** — Each HEIF request spawns an ffmpeg process. For browsing (thumbnails), this means one process per HEIF thumbnail on initial load. Acceptable for moderate directory sizes; an in-memory cache could mitigate this if needed.
- **Security** — File paths are passed as command arguments to ffmpeg (not interpolated into a shell string), preventing shell injection. Path traversal is handled at the API layer (see ADR-0006 scope).
