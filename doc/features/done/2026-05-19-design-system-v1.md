# Design System v1 (Hüpattl!)

*Last modified: 2026-05-20*

## Summary

Apply Hüpattl! Design System v1 to Unterlumen's frontend CSS, establishing a coherent token-based design language with full light/dark theme support and an orange accent.

## Details

- **Color tokens** — OKLCH-based neutral scale (`--bg`, `--bg-2`, `--bg-3`, `--fg`, `--fg-2`, `--fg-3`, `--border`, `--hairline`) with warm undertone. Accent is `#d35400` (exact logo orange) in light mode, `#e06020` in dark mode.
- **Typography** — IBM Plex Mono (`--font-mono`) as the UI voice for all labels, metadata, and numbers. Display/body stacks remain system sans-serif.
- **Spacing** — `--space-1` … `--space-10` (4px base) alongside legacy `--unit: 8px`.
- **Radius** — `--radius-sm` (6px) through `--radius-pill` (999px); modals use `--radius-lg` (14px).
- **Motion** — `--dur-quick` (120ms) / `--dur` (150ms) / `--dur-slow` (240ms) with `--ease` cubic-bezier.
- **Status tokens** — `--success`, `--warn`, `--danger` in OKLCH with `-soft` variants.
- **Button groups** — Consistent pattern: container gets `background: var(--border); gap: 1px; overflow: hidden;`, inner buttons get `border: none; border-radius: 0`. Applied to settings theme toggle, export format tabs, export estimate toggle, info-map controls, View/Tools/Slideshow toolbar, and the viewer zoom/action groups.
- **Toggle switches** — The library Search and Filter buttons now use the design system's pill toggle (`.toggle` with `.toggle-track` / `.toggle-thumb`). `LibrarySearchPanel` manages `data-state="on/off"` and `aria-checked` instead of a CSS `.active` class.
- **SVG iconography** — Replaced all typographic navigation characters with stroke-based SVG icons: viewer Back/Previous/Next chevrons, collapsed info-panel ⓘ icon, and up-directory arrow in breadcrumb rows.
- **Viewer overlay navigation** — Previous/Next buttons are absolutely positioned overlays (`opacity: 0` → `opacity: 1` on `.viewer-body:hover`), so the photo fills the full viewport at all times.
- **Selection ring** — Photo grid items use `box-shadow: inset 0 0 0 3px var(--accent)` instead of an opaque overlay.
- **Themes** — Light/dark toggle via `data-theme` on `<html>` is preserved; `data-accent="orange"` added to `<html>` for product variant identification.
- **E2E coverage** — `e2e/specs/theme.spec.js` added (6 tests): accent attribute, theme buttons, light/dark switching, viewer always-dark.

## Acceptance Criteria

- [x] All CSS custom properties use design system tokens; no bare hex literals except `--accent` and `--accent-hover`
- [x] Light and dark themes render correctly
- [x] IBM Plex Mono loaded via Google Fonts CDN
- [x] Button groups visually connected with consistent outer rounding
- [x] Selected photos show a 3px accent ring, not an opaque overlay
- [x] All e2e theme tests pass (6/6)
- [x] 183 pre-existing tests still pass
- [x] Search/Filter toggle switches reflect open state via `data-state` attribute
- [x] Viewer zoom and action button groups visually connected
- [x] Viewer photo navigation fills full width; arrows are hover overlays
