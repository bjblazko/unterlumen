# ADR-0013: MapLibre GL JS for Location Maps

*Last modified: 2026-03-04*

## Status

Accepted

## Context

Photos with GPS EXIF data should display their location visually. The Info panel already shows raw latitude/longitude text, but a map provides immediate spatial context.

The frontend is vanilla HTML/JS/CSS with no build step ([ADR-0007](0007-vanilla-frontend.md)). Adding a map library is the first third-party JavaScript dependency. The main options considered:

1. **Static map image** — Fetch a tile image from a tile server. Simple but no interactivity, and composing a map from tiles is non-trivial.
2. **Leaflet** — Mature, lightweight. Uses DOM-based rendering (slower for vector tiles).
3. **MapLibre GL JS** — Open-source fork of Mapbox GL JS. GPU-accelerated vector tile rendering. Larger bundle but richer experience.

## Decision

Use **MapLibre GL JS** loaded from a CDN (`unpkg.com`), with **OpenFreeMap** as the tile source. OpenFreeMap serves OpenStreetMap-based vector tiles with no API key required and no usage limits.

Key implementation details:

- MapLibre GL JS and its CSS are loaded via `<script>` and `<link>` tags (no bundler).
- The map is created inside the Info panel's Location section when GPS data is present.
- Map instances are explicitly destroyed (`map.remove()`) when the Info panel updates to prevent WebGL context leaks.
- A toggle switches between 2D and 3D (pitched) views.
- A link opens the location on OpenFreeMap in a new tab.

## Consequences

- **First external JS dependency** — The vanilla frontend now loads a third-party library. This is a deliberate exception to [ADR-0007](0007-vanilla-frontend.md)'s spirit of minimal dependencies, justified by the complexity of map rendering.
- **CDN dependency** — MapLibre GL JS is loaded from `unpkg.com`. The app degrades gracefully if the CDN is unreachable (no map shown, no errors).
- **No API key** — OpenFreeMap requires no registration or authentication, keeping deployment simple.
- **WebGL requirement** — MapLibre GL JS requires WebGL. Browsers without WebGL support will not render the map.
- **Memory management** — Map instances must be explicitly destroyed to release WebGL contexts. The Info panel handles this on every update cycle.
