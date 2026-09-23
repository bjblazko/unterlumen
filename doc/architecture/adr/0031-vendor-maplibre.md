# ADR-0031: Vendor MapLibre GL JS

*Last modified: 2026-09-23*

## Status

Accepted. Supersedes the delivery decision in [ADR-0013](0013-maplibre-location-maps.md); MapLibre and OpenFreeMap remain the choice, only the way the library reaches the browser changes.

## Context

[ADR-0013](0013-maplibre-location-maps.md) loaded MapLibre GL JS from `unpkg.com` with an unversioned URL:

```html
<script src="https://unpkg.com/maplibre-gl/dist/maplibre-gl.js"></script>
```

MapLibre 6 dropped the UMD build. `dist/maplibre-gl.js` no longer exists in the package, so the unversioned URL — which always resolves to the newest release — started returning a 404 page. Chrome refuses that response for a `<script>` tag (`net::ERR_BLOCKED_BY_ORB`), `maplibregl` stayed undefined, and `initMap` returned early. The Info Panel then rendered an empty box where the map should be: the failure looked exactly like a photo without a location, in an application whose whole point is to be honest about what it knows.

This is the same class of problem [ADR-0017](0017-d3-vendored-bundle.md) already solved for D3, and it contradicts [ADR-0030](0030-rams-design-tokens.md), which self-hosts the fonts so the interface renders identically offline — a NAS install has no reason to depend on a third-party CDN being reachable, unblocked and API-compatible.

## Decision

Vendor MapLibre GL JS and its stylesheet into the repository, alongside the D3 bundle:

- `src/web/js/vendor/maplibre-gl.js` — MapLibre GL JS 5.24.0, the last release shipping a UMD bundle, which is what a no-build frontend ([ADR-0007](0007-vanilla-frontend.md)) can load with a plain `<script>` tag.
- `src/web/css/vendor/maplibre-gl.css` — the matching stylesheet.

Both are served from the binary's embedded `web/` directory, like every other asset.

Tiles keep coming from OpenFreeMap over the network: they are content, not code, and a map without tiles is not a map. What changes is that a missing map library, or missing tiles, is now stated rather than shown as an empty rectangle — `initMap` writes a sentence into the map container when `maplibregl` is undefined.

Moving to MapLibre 6 would mean loading ESM modules plus their shared and worker chunks, which is a bundler-shaped problem; the version is pinned instead, and upgrading is a deliberate step rather than something a CDN does to us overnight.

## Consequences

- The map works offline, in air-gapped or network-restricted deployments, and in browsers that block third-party script hosts.
- Roughly 1 MB of JavaScript and 68 KB of CSS enter the repository and the binary. That is the cost of the map feature being reliable; it is loaded on every page, as before, and is the largest single asset in `web/`.
- Upgrading MapLibre is now a commit — which is the point: an unversioned CDN URL silently upgraded the library across a major version and broke the feature with no code change on our side.
- MapLibre is BSD-3-Clause licensed; the license header stays in the vendored file.
- An e2e test asserts that `maplibregl` is defined and that a map canvas appears for a photo with coordinates, so the same regression cannot return unnoticed.
