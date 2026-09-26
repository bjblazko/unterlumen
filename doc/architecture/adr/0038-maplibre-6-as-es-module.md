# ADR-0038: MapLibre 6 as an ES module, without a bundler

*Last modified: 2026-09-26*

## Status

Accepted. Supersedes the version pin of [ADR-0031](0031-vendor-maplibre.md); vendoring stays.

## Context

[ADR-0031](0031-vendor-maplibre.md) vendored MapLibre GL JS 5.24.0, the last release with a UMD bundle, and pinned it there: MapLibre 6 ships only ES modules — an entry module, a shared chunk and a worker — which looked like a job for a bundler that a no-build frontend ([ADR-0007](0007-vanilla-frontend.md)) does not have.

A dependency review on 2026-09-26 found 6.11.2 current and 5.24.0 a major version behind. Reading the 6.x changes showed the chunks reference each other by relative URL (`./maplibre-gl-shared.mjs`; the worker is found next to the entry module through `import.meta.url`), so a browser loads them without any build step.

The app itself uses four stable parts of the API — `Map`, `Marker`, `NavigationControl`, `AttributionControl` — none of which the 6.0 breaking changes touch. WebGL2 is now required, which every browser Unterlumen supports has.

## Decision

- Vendor the ES build of MapLibre GL JS into a directory named after its version, `src/web/js/vendor/maplibre-6.11.2/`: `maplibre-gl.mjs`, `maplibre-gl-shared.mjs`, `maplibre-gl-worker.mjs`, `maplibre-gl.css` and the license. The version in the path is the cache buster for all of them, including the chunks the entry module loads itself.
- `index.html` loads it with one module script that makes it the global the app's classic scripts use:

  ```html
  <script type="module">
      import * as maplibregl from '/js/vendor/maplibre-6.11.2/maplibre-gl.mjs';
      window.maplibregl = maplibregl;
  </script>
  ```

  Module scripts run before `DOMContentLoaded`, so `maplibregl` exists before the app starts; `initMap` keeps saying so in words when it does not.
- The Go server already serves `.mjs` as `text/javascript`, which module scripts require.
- Source maps are left out; the files are otherwise unchanged from the npm package.

## Consequences

- Upgrading MapLibre is again a commit: a new versioned directory and one path in `index.html`.
- The UMD file and its stylesheet are gone; the three modules are about 1.1 MB together, similar to before.
- A browser without module-script support would get no map. Every browser that has WebGL2 supports module scripts.
- The e2e test from ADR-0031 still asserts that `maplibregl` is defined and that a map canvas appears for a photo with coordinates.
