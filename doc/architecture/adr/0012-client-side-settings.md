# ADR-0012: Client-Side Settings via localStorage

*Last modified: 2026-03-04*

## Status

Accepted

## Context

Several features require persisting user preferences across page reloads: dark mode selection, Commander pane split ratio, interface visibility (H key toggle), and thumbnail quality. [ADR-0002](0002-no-persistence.md) established that the server stores no state, but it did not address whether the frontend could persist state in the browser.

Storing preferences on the server would require a config file or database, violating the "no persistence" principle and adding deployment complexity. Browser `localStorage` is a natural fit: it is per-origin, survives reloads, and requires no server changes.

## Decision

User preferences are stored in `localStorage` and read on page load. Each setting uses a namespaced key (e.g. `theme`, `commanderSplit`, `uiHidden`, `thumbnailQuality`). The frontend reads these values at initialization and applies them before or during first paint to avoid flashes of incorrect state.

This extends [ADR-0002](0002-no-persistence.md): the server remains stateless, but the browser is an acceptable persistence layer for UI preferences.

## Consequences

- **Preferences survive reloads** — Theme, layout ratios, and feature toggles are remembered without any server-side storage.
- **Per-browser, not per-user** — Settings do not roam between browsers or devices. Acceptable for a single-user local tool.
- **No migration path** — If key names change, old values are silently ignored. No versioning mechanism exists.
- **Flash prevention** — Theme preference must be applied in a `<script>` block before CSS paints to prevent a flash of the wrong theme on load.
- **Clear boundary** — Filesystem operations (copy, move, delete) go through the server. UI-only preferences stay in the browser. This keeps the distinction clean.
