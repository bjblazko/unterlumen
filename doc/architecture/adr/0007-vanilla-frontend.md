# ADR-0007: Vanilla HTML/JS/CSS Frontend

*Last modified: 2026-02-21*

## Status

Accepted

## Context

The frontend needs to render thumbnail grids, list views, a dual-pane layout, and an image viewer. Options:

1. **Framework (React, Vue, Svelte)** — Component model, reactivity, large ecosystem. Requires a build step (Node.js, npm, bundler).
2. **Vanilla HTML/JS/CSS** — No build tools, no dependencies, served as static files directly.

## Decision

Use vanilla HTML, JavaScript, and CSS with no framework and no build step.

## Consequences

- **No build toolchain** — The `web/` directory is served as-is. No Node.js, npm, or bundler required. The entire project builds with `go build` alone.
- **Class-based structure** — UI components are organized as ES5-compatible classes (`BrowsePane`, `Viewer`, `Commander`) in separate script files. State is managed explicitly within each class.
- **Manual DOM updates** — Without a virtual DOM or reactivity system, the UI re-renders by replacing `innerHTML`. This is sufficient for the application's complexity level but would not scale to a much larger UI.
- **No module system** — Scripts are loaded via `<script>` tags in order. Globals (`API`, `BrowsePane`, `Viewer`, `Commander`, `App`) are used for cross-file communication. Acceptable for the small number of files involved.
- **Smaller payload** — No framework code shipped to the browser. The entire frontend is a few KB.
