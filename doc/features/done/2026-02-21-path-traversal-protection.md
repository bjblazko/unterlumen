# Path Traversal Protection

*Last modified: 2026-02-21*

## Summary

All API endpoints validate file paths to prevent access to files outside the configured root directory.

## Details

### Validation logic (`safePath`)

Every API handler that accepts a file path passes it through `safePath(root, relative)` before any filesystem access:

1. Empty paths resolve to the root directory itself
2. `filepath.Clean` removes `.`, `..`, and redundant separators
3. Absolute paths in the input are rejected outright
4. The cleaned path is joined with the root
5. `filepath.EvalSymlinks` resolves symlinks to their real location
6. The resolved path must have the root directory as a prefix
7. For destinations that don't exist yet (copy/move targets), the parent directory is validated instead

### Endpoints protected

- `GET /api/browse?path=...`
- `GET /api/thumbnail?path=...`
- `GET /api/image?path=...`
- `POST /api/copy` — both source files and destination directory
- `POST /api/move` — both source files and destination directory

### ffmpeg path safety

HEIF conversion passes file paths as discrete command arguments to `exec.Command`, not via shell interpolation. This prevents injection attacks through crafted filenames.

## Acceptance Criteria

- [x] `../` sequences in paths are rejected
- [x] Absolute paths are rejected
- [x] Symlinks pointing outside root are rejected
- [x] Copy/move destination paths are validated
- [x] ffmpeg invocations don't use shell interpolation
- [x] Invalid paths return HTTP 400
