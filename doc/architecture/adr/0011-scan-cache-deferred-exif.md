# ADR-0011: In-Memory Scan Cache and Deferred EXIF Extraction

*Last modified: 2026-03-04*

## Status

Accepted

## Context

Browsing directories with hundreds or thousands of images was slow because `GET /api/browse` performed synchronous EXIF date extraction for every file before returning. A directory of 1,000 JPEGs could take several seconds to list, with the browser waiting on a blank screen.

Three interrelated performance problems needed solving:

1. **Slow initial response** — EXIF extraction blocked the directory listing response.
2. **Redundant rescans** — Navigating back to a previously visited directory repeated all I/O.
3. **DOM pressure** — Rendering thousands of thumbnails at once caused jank and high memory use.

## Decision

Introduce three complementary subsystems:

**In-memory scan cache.** Directory listings are cached in a `sync.Map` keyed by directory path. Cache entries are invalidated when the directory's modification time changes or when a copy/move/delete operation touches the directory. This is consistent with [ADR-0002](0002-no-persistence.md) — the cache is purely in-memory and lost on restart.

**Deferred EXIF extraction.** `GET /api/browse` returns immediately using file modification times as a stand-in for EXIF dates. A background goroutine extracts actual EXIF dates and stores them in the cache. The frontend polls `GET /api/browse/dates` to retrieve EXIF dates once they are ready, then re-sorts the grid if the user is sorting by date.

**Chunked DOM rendering.** The frontend renders grid and list views in batches of 50 items. Additional batches are appended on scroll via `IntersectionObserver`. Keyboard navigation past the rendered range triggers on-demand rendering of the next chunk.

## Consequences

- **Instant repeat visits** — Cached directories load in under 10 ms regardless of file count.
- **Fast first visit** — The directory listing returns immediately; EXIF dates arrive asynchronously.
- **New polling endpoint** — `GET /api/browse/dates` is a new API surface that the frontend must poll until extraction completes.
- **Cache invalidation complexity** — Copy, move, and delete handlers must invalidate both the source and destination directory caches.
- **Memory use** — The cache grows with the number of visited directories and is never evicted (acceptable for a single-user local tool).
- **Chunked rendering trade-off** — Not all items are in the DOM at once, so browser Find (Ctrl+F) only searches rendered items.
