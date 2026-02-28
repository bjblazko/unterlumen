# Feature: UNTERLUMEN_ROOT_PATH Environment Variable

*Last modified: 2026-02-28*

## Summary

Separate the concepts of *start directory* (where browsing begins) and *navigation boundary* (how far the user can navigate). Introduce `UNTERLUMEN_ROOT_PATH` as an environment variable for deployment scenarios that need to confine navigation to a specific directory tree.

## Details

Previously, the command-line argument served double duty: it was both the starting directory and the navigation boundary. This made it impossible to start the server in a specific folder while allowing navigation above it.

The new priority chain:

1. **Command-line argument provided** — start there, no navigation restriction (boundary = filesystem root `/`)
2. **`UNTERLUMEN_ROOT_PATH` env var only** — start there, restrict navigation to that directory (boundary = ENV path)
3. **Neither provided** — start in user's home directory, no navigation restriction

Implementation:

- `main.go` resolves `startDir` and `boundary` separately based on the priority chain
- `api.NewRouter` now accepts `boundary` (for `safePath`) and `startPath` (relative path for the frontend) as separate parameters
- A new `GET /api/config` endpoint exposes `startPath` to the frontend
- `App.init()` in `app.js` fetches config before calling `setMode('browse')`, so the browser loads the correct initial directory

## Acceptance Criteria

- [ ] `./unterlumen` (no args, no env) → opens in home directory, can navigate up to `/`
- [ ] `./unterlumen /tmp/photos` → starts at `/tmp/photos`, can navigate up to `/`
- [ ] `UNTERLUMEN_ROOT_PATH=/tmp/photos ./unterlumen` → starts at `/tmp/photos`, cannot navigate above it
- [ ] `UNTERLUMEN_ROOT_PATH=/tmp/photos ./unterlumen /var/images` → cmdline wins, starts at `/var/images`, no nav restriction
- [ ] Invalid path (either source) → prints error message and exits with non-zero code
- [ ] `go vet ./...` passes with no errors
