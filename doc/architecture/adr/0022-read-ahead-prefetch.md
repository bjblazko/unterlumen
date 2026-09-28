# ADR-0022: Read-Ahead Prefetch and In-Memory Image Cache

*Last modified: 2026-09-28*

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

## Amendment 2026-09-28: a prefetch never converts, and full decodes run one at a time

On the NAS (Raspberry Pi 5, 8 GB, shared with other services) the viewer stalled
the machine for about three minutes. Each step in the viewer asked for the next
two photos, and for a HEIF not seen before each such request started a full
decode with `heif-convert` — about 640 MB and five seconds apiece. The thumbnail
work limit counts cores, so two ran at once, and the machine went into swap;
everything else, stored thumbnails on the Map and Timeline included, waited.

- **A prefetch takes only what is ready.** The viewer sends its read-ahead with
  the header `X-Prefetch: 1` (a header, not a parameter, so the URL is the one it
  later shows and the browser reuses the answer). `heifjpeg.Serve`
  (`internal/api/heifjpeg`, now the one place Folders and the libraries serve a
  HEIF as JPEG) answers such a request from memory or the disk cache, and
  otherwise with an empty `204` and `Cache-Control: no-store`.
- **A library photo opens on its stored preview** (the 1200 px thumbnail made at
  scan time) and the full-size photo replaces it when it arrives. Moving on
  cancels that request. Crop waits for the full-size photo.
- **Full decodes run one at a time** (`media.oneFullDecode`), whatever the number
  of cores: `heif-convert` and ffmpeg's HEVC decode, for the viewer, exports and
  the thumbnail fallback alike.

