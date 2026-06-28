# ADR-0022: Read-Ahead Prefetch and In-Memory Image Cache

*Last modified: 2026-06-28*

## Status

Accepted

## Context

Photos served from a NAS over SMB cause noticeable latency on forward navigation in
the viewer. Each new image requires a round-trip to the file server at the moment the
user presses the arrow key. Backward navigation is already fast because the browser
caches previously fetched images via ETag/Last-Modified (set automatically by
`http.ServeFile` for JPEG/PNG). HEIF files were worse: they used `Cache-Control: no-cache`
with no ETag, so the browser never cached them and every navigation hit the NAS.

## Decision

### Frontend: viewer prefetch

The `Viewer` class (`src/web/js/viewer.js`) calls `_prefetch(2)` after every
`navigate()` and after `open()`. This creates two bare `Image` objects pointing at
`images[currentIndex+1]` and `images[currentIndex+2]`, triggering browser downloads
before the user navigates. The objects are stored in `this._prefetchCache` to prevent
garbage collection. This follows the existing pattern in `slideshow-player.js`.

### Backend: in-memory image cache

A new `ImageCache` struct (`src/internal/media/imagecache.go`) is a thread-safe,
slice-based LRU cache (20 entries) for `[]byte` image data, keyed by
`absPath + ":" + mtime.UnixNano()`. The mtime component in the key means entries are
automatically stale when the source file changes on disk. The cache is instantiated
once in `NewRouter` and shared between the browse and library image handlers.

Only HEIF images are stored in this cache. Non-HEIF images are served via
`http.ServeFile` which delegates to the OS page cache and handles conditional requests
natively.

### Backend: HEIF HTTP caching headers

HEIF responses now use `Cache-Control: private, max-age=3600` with an ETag derived
from `sha256(absPath)[:4]` and `mtime.Unix()`. This allows the browser to cache
converted HEIF images for the duration of a browsing session. `If-None-Match` is
handled to return 304 when the ETag matches.

The existing HEIF disk cache (`$TMPDIR/unterlumen-cache/`) remains in place as the
persistence layer between the NAS and the in-memory cache. The caching hierarchy is:

```
NAS → HEIF disk cache (~/Library/Caches/unterlumen/) → in-memory ImageCache → browser cache
```

## Consequences

- Forward navigation through JPEG images is near-instant once the prefetch has
  settled (typically one RTT to the NAS ahead of the user).
- Forward navigation through HEIF images benefits from prefetch, in-memory cache,
  and browser caching — repeated navigation to an already-seen HEIF image is served
  from browser memory with no network round-trip.
- The `ImageCache` uses at most ~100–300 MB of server RAM at peak (20 entries × avg
  5–15 MB per converted HEIF JPEG). Acceptable for a single-user local app.
- In-place crop edits already append `?t=<timestamp>` to viewer URLs, which produces
  a cache-miss in both the in-memory cache (different key) and the browser cache.
- The 20-entry LRU cap was chosen to cover a typical forward-browsing window
  (current + 2 prefetched + ~17 recently seen) without unbounded memory growth.
