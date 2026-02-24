# Rename Project to Unterlumen

*Last modified: 2026-02-24*

## Summary

Rename the project from "iseefourlights" to "Unterlumen" across all code, build artifacts, and documentation.

## Details

- **Go module** — Module path changed from `huepattl.de/iseefourlights` to `huepattl.de/unterlumen`. All internal imports updated.
- **Cache directory** — Temp directory name changed from `iseefourlights-cache` to `unterlumen-cache`. Temp file prefix changed from `iseefourlights-sips-` to `unterlumen-sips-`.
- **Binary** — Build output changed from `iseefourlights` to `unterlumen`.
- **Frontend** — Browser tab title and header updated to "Unterlumen". CSS comment updated.
- **Documentation** — README, CHANGELOG, CLAUDE.md, arc42, and feature docs updated with the new name.

## Acceptance Criteria

- [x] `go.mod` module path is `huepattl.de/unterlumen`
- [x] All Go imports use the new module path
- [x] Cache directory uses `unterlumen-cache`
- [x] Binary builds as `unterlumen`
- [x] Browser tab title shows "Unterlumen"
- [x] Header shows "Unterlumen"
- [x] All documentation references updated
- [x] `go vet ./...` passes
- [x] `go build -o unterlumen .` succeeds
