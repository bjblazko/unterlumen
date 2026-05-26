# Library–Organizer Thumbnail Bridge

*Last modified: 2026-05-25*

## Summary

When the user browses a folder in organizer mode that happens to be inside a library's source path, the thumbnail system currently regenerates everything from scratch — even for photos already indexed (and thumbnailed at 1200 px) by the library. This feature bridges the two systems so the library's pre-generated thumbnails are used directly, and newly copied files are indexed automatically.

## Details

### 1. Library thumbnail fast path

`GET /api/thumbnail` now checks `library.Manager.FindThumbnailForPath(absPath)` before falling back to normal generation. If the file is in a library's `path_cache` and its pre-generated JPEG exists under `~/.unterlumen/libraries/{id}/thumbs/{id[:2]}/{id}.jpg`, the library JPEG is decoded and resized via the same Catmull-Rom pipeline, then cached to the browse disk cache. Subsequent requests for the same file hit the browse cache without touching the library DB.

This is most impactful for HEIF/HEIC files, where normal generation requires sips or ffmpeg. Serving the library JPEG instead reduces per-thumbnail cost from ~100–300 ms (HEIF decode) to ~5–15 ms (JPEG decode + resize).

### 2. Auto incremental scan after copy/move

After a successful `POST /api/copy` or `POST /api/move` where the destination is under a library's source path, `manager.TriggerScanNewBackground(libID)` is called. This runs `Indexer.RunScanNew` in a background goroutine using the existing broadcaster/SSE infrastructure. The scan only adds new/changed files without marking existing photos as missing.

### 3. Library indicator in browse mode

`GET /api/library/detect?path=X` returns `{"id":"…","name":"…"}` or `{}`. The browse frontend calls this after each directory load and shows a small `.browse-library-badge` label in the breadcrumb row (e.g. `Reisen`) when the folder is tracked by a library. The badge is hidden when outside any library folder.

## Acceptance Criteria

- [ ] Browsing a folder inside a library source path shows thumbnails without HEIF/sips processing for already-indexed files (verify via server logs: no `sips` invocations for indexed files on second-session browse)
- [ ] Copying photos into a library folder triggers a background `scan-new` — newly copied files appear in the library view within seconds, without a manual index
- [ ] Library name badge appears in breadcrumb row when inside a library folder; disappears immediately on navigation outside the library's source path
- [ ] `go vet ./...` passes
- [ ] Existing e2e tests continue to pass
