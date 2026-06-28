# Read-Ahead / Prefetch

*Last modified: 2026-06-28*

## Summary

When browsing photos from a NAS, forward navigation is slow because each new image
requires a round-trip to the file server. This feature reduces perceived latency by
prefetching the next two images before the user navigates to them.

## Details

**Frontend prefetch:** When the viewer opens or the user navigates to a photo,
the next two images are prefetched in the background by creating bare `Image` objects.
This triggers browser downloads before the user requests them. Works in both browse
and library modes (both use the same `Viewer` class).

**Server-side HEIF cache:** HEIF/HEIC images are converted to JPEG on-the-fly. The
converted bytes are now cached in a server-side LRU cache (20 entries, keyed by path
+ mtime). Subsequent requests for the same HEIF image are served from RAM instead of
re-reading from disk and re-converting.

**HTTP caching for HEIF:** HEIF responses previously used `Cache-Control: no-cache`
with no ETag, preventing any browser caching. They now use `Cache-Control: private,
max-age=3600` with an ETag derived from the file path and modification time, enabling
the browser to cache converted images for the duration of a browsing session.

## Acceptance Criteria

- [x] Navigating forward through JPEG images shows near-zero latency after the first
      prefetch has completed
- [x] Navigating forward through HEIF images benefits from both prefetch and server-side
      caching
- [x] HEIF images are served with `Cache-Control: private, max-age=3600` and a valid ETag
- [x] `If-None-Match` requests for HEIF images return 304 when ETag matches
- [x] The LRU cache holds at most 20 entries; older entries are evicted when full
- [x] Works in both browse mode and library mode
