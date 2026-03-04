# ADR-0014: Thumbnail Quality Tiers

*Last modified: 2026-03-04*

## Status

Accepted

## Context

[ADR-0003](0003-exif-thumbnails.md) chose EXIF embedded thumbnails for speed — they are pre-generated and tiny to extract. However, EXIF thumbnails are typically 160×120 pixels, which look noticeably soft on high-DPI (retina) displays where a grid cell might be 200–300 CSS pixels rendered at 2× or 3×.

Users on high-DPI displays requested sharper thumbnails. The trade-off is clear: higher quality requires decoding the full image on the server, which is significantly slower than extracting the embedded thumbnail.

## Decision

Offer two thumbnail quality tiers, selectable via a "Thumbnails" setting in the Settings menu:

- **Standard** (default) — Extracts the embedded EXIF thumbnail, as per [ADR-0003](0003-exif-thumbnails.md). Fast, but limited to the embedded resolution.
- **High** — Bypasses EXIF thumbnails entirely. The server decodes the full image, resizes it to the requested display size × device pixel ratio using bicubic (Catmull-Rom) resampling, and encodes to JPEG at quality 85.

The frontend passes a `quality=high` query parameter and `dpr` value to `GET /api/thumbnail` when High mode is active. The setting is persisted in `localStorage` ([ADR-0012](0012-client-side-settings.md)).

## Consequences

- **User choice** — Users on low-end hardware or slow storage can keep Standard mode. Users who prioritize visual quality can opt into High mode.
- **Architectural fork in the thumbnail pipeline** — The `GET /api/thumbnail` handler now has two distinct code paths: EXIF extraction and full-image decode+resize. Both must be maintained.
- **Significantly slower in High mode** — Decoding a 24 MP JPEG and resizing it takes ~100–500 ms per image versus <1 ms for EXIF extraction. Directories with many images will load thumbnails more slowly.
- **DPR-aware sizing** — High mode renders pixel-perfect thumbnails for the user's actual display density, eliminating browser upscaling artifacts.
- **Interaction with scan cache** — The scan cache ([ADR-0011](0011-scan-cache-deferred-exif.md)) helps offset the cost by eliminating redundant directory scans; thumbnail generation is the bottleneck in High mode, not directory listing.
