# ADR-0018: Adopt Hüpattl! Design System token vocabulary

*Last modified: 2026-05-19*

## Status

Accepted

## Context

Unterlumen's CSS accumulated ad-hoc hex colour literals and inconsistent variable names over time (`--surface`, `--text`, `--accent-light`, etc.). The Hüpattl! Design System v1 defines a coherent token set — OKLCH colour space, spacing scale, radius scale, motion tokens, and IBM Plex Mono as the UI typeface — shared across products in the same family. Migrating to this vocabulary makes the UI easier to maintain and lays the groundwork for future product variants.

## Decision

Adopt the Hüpattl! Design System v1 token vocabulary for all CSS custom properties in `style.css`:

- **Colour space** — OKLCH for all neutral and status tokens; `#d35400` (exact logo orange) retained as `--accent` to preserve pixel-exact brand colour fidelity.
- **Variable rename** — Full rename, no compatibility aliases: `--surface` → `--bg`, `--surface-alt` → `--bg-2`, `--text` → `--fg`, `--text-sec` → `--fg-2`, `--accent-light` → `--accentsoft`, `--warning-*` → `--warn-*`.
- **Typography** — IBM Plex Mono as `--font-mono` (UI voice), loaded via Google Fonts CDN. Display and body stacks remain system sans-serif.
- **Border radius** — Replaced CLAUDE.md's "2px max" rule with design system scale (`--radius-sm` 6px to `--radius-pill` 999px). ADR-0008 Dieter Rams principles remain in effect for layout and ornamentation.
- **Button groups** — Standardised pattern: `background: var(--border); gap: 1px; overflow: hidden` on container; `border: none; border-radius: 0` on inner buttons.

## Consequences

- All existing CSS custom property references updated in one pass; no legacy aliases remain.
- Light/dark theme toggle mechanism is unchanged (still `data-theme` on `<html>`).
- `data-accent="orange"` added to `<html>` for product variant identification.
- The CLAUDE.md "2px border-radius max" rule is superseded by the design system radius scale.
- Google Fonts CDN dependency introduced for IBM Plex Mono (graceful degradation to `ui-monospace` if offline).
