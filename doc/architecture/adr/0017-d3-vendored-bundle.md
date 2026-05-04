# ADR-0017: Vendor D3.js for statistics visualisations

*Last modified: 2026-05-02*

## Status

Accepted

## Context

The statistics modal requires data visualisation (histograms, donut charts, treemaps, radial charts, calendar heatmaps). The existing frontend has no charting library. MapLibre GL — used for location maps — is loaded from a CDN, but that dependency was inherited and not a deliberate choice.

Unterlumen is a local-first app with no assumed network connectivity. A statistics modal that silently shows blank charts when the CDN is unreachable would be worse than no modal at all.

D3.js v7 is the de-facto standard for bespoke, data-driven SVG visualisations in the browser. It has no transitive dependencies and ships a single-file minified bundle (~280 KB).

## Decision

Bundle D3.js v7 (`d3.v7.min.js`) inside `src/web/js/vendor/`. The file is committed to the repository and served by Unterlumen's static file handler. No CDN request is made at runtime.

The vendored file must be updated manually when upgrading D3. The chosen version (7.x) has a stable API, making infrequent updates acceptable.

## Consequences

- Statistics charts work without network access — consistent with Unterlumen's local-first design.
- Repository size increases by ~280 KB.
- D3 is a global (`window.d3`), which is consistent with how all other JS in the project is loaded (no module bundler).
- Future upgrades require re-downloading and committing a new `d3.v7.min.js`.
