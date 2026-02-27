# ADR-0004: HEIF/HEIC Support via ffmpeg Shell-Out

*Last modified: 2026-02-27*

## Status

Accepted

## Context

HEIF/HEIC is the default photo format on modern iPhones and iPads. Supporting it is important for the target use case. Options:

1. **Pure Go HEIF decoder** — No mature, well-maintained library exists in the Go ecosystem.
2. **CGo bindings to libheif** — Adds a C dependency, complicating cross-compilation and deployment.
3. **Shell out to ffmpeg** — ffmpeg has robust HEIF decoding. It is widely available on systems that handle media files.

## Decision

Convert HEIF/HEIC files to JPEG on-the-fly by invoking ffmpeg as a subprocess. The converted output is piped to stdout. Converted results are cached to disk in `$TMPDIR/unterlumen-cache/` to avoid re-running ffmpeg on repeated requests for the same file.

## Consequences

- **External dependency** — ffmpeg must be installed for HEIF support. Without it, HEIF files will fail to display but all other formats work normally. On macOS, `sips` is also tried as an intermediate fallback before the HEVC decode path.
- **No CGo** — The binary remains a pure Go build with `CGO_ENABLED=0` cross-compilation support.
- **Performance** — On first access, each HEIF file spawns one or two ffmpeg subprocesses (a probe + an extraction/decode). Subsequent requests are served from the disk cache with no subprocess invocation.
- **Disk cache** — Converted JPEGs are stored in `$TMPDIR/unterlumen-cache/` keyed by file path + mtime. Cache entries survive process restarts. No explicit cleanup; the OS temp-directory policy handles eviction. This does not conflict with ADR-0002 because nothing is written to the photo directory.
- **Security** — File paths are passed as command arguments to ffmpeg (not interpolated into a shell string), preventing shell injection. Path traversal is handled at the API layer (see ADR-0006 scope).
- **Details** — See `doc/architecture/image-processing-flow.md` for the complete decision tree, all ffmpeg invocations, and cache key design.
