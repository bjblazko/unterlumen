# Cache Management

*Last modified: 2026-05-12*

## Summary

Add a "Cache" section to the Settings dropdown that shows the thumbnail cache location and current disk usage, with a button to clear the cache manually.

## Details

The disk thumbnail cache (`~/Library/Caches/unterlumen/` on macOS) has no automatic eviction — it grows as thumbnails are generated and is only reclaimed by the OS under disk pressure. Users had no way to see how large it was or free space manually.

**New API endpoints:**
- `GET /api/cache/info` — returns `{ path, bytes }` for the cache directory
- `POST /api/cache/clear` — removes all files in the cache directory (directory itself is preserved)

**Settings UI addition:**
- A new "Cache" section appears in the Settings dropdown between "Interface" and "Check dependencies"
- Shows the size in MB (refreshed each time the menu opens)
- Shows the full cache path in muted, truncated text
- A "Clear cache" button empties the cache; the size display refreshes automatically after clearing

**LRU / automatic eviction:** Not implemented. macOS handles cache pressure for `~/Library/Caches/` automatically; manual clear is the appropriate control for a single-user local tool.

## Acceptance Criteria

- [x] Settings dropdown shows a "Cache" section with current size in MB and cache path
- [x] Size is loaded when the settings menu opens (not on page load)
- [x] "Clear cache" button removes all files from the cache directory and refreshes the size display
- [x] Cache directory is not deleted — only its contents
- [x] Button is disabled during the clear operation to prevent double-click
- [x] `go vet ./...` passes
