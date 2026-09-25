# ADR-0030: Adopt rams-design tokens

*Last modified: 2026-09-26*

## Status

Accepted. Supersedes [ADR-0018](0018-design-system-tokens.md).

## Context

[ADR-0018](0018-design-system-tokens.md) adopted the Hüpattl! Design System v1: OKLCH neutrals, a 4 px spacing scale, a 6–14 px radius scale and IBM Plex Mono as the UI voice. It did what it set out to do — the ad-hoc hex literals are gone and the vocabulary is consistent — but two of its choices work against [ADR-0008](0008-dieter-rams-design-principles.md):

- **Mono as the UI voice makes everything equally loud.** With every label, button and heading in IBM Plex Mono, hierarchy can only come from size. Mono also costs readability in running text and in the long German labels this UI is full of.
- **The radius scale (6 px … 14 px) reads as soft product styling**, not as the Braun-derived vocabulary ADR-0008 describes.

Meanwhile the `rams-design` skill (`~/.claude/skills/rams-design/`) is now the project's normative UI guidance per the global instructions, and it ships a complete, contrast-verified token file (`references/tokens.css`) built from the same principles ADR-0008 adopted. Maintaining a second, diverging token vocabulary for the same philosophy is duplicated work with no benefit: there is no second product in the "Hüpattl! family" that the shared vocabulary was meant to serve.

## Decision

Replace the `:root` token block in `src/web/css/style.css` with the rams-design tokens, value for value, keeping the canonical token names so the project stays comparable with the skill's reference.

- **Color** — the neutral ramp `bg`, `bg-2`, `bg-3`, `line`, `line-strong`, `fg-3`, `fg-2`, `fg` in both themes, plus the signal colors `accent`/`accent-fg`/`accent-ink`, `confirm`, `time`, `warning` and their `*-ink` variants. Each signal color has exactly one meaning: orange = the one primary action, green = active/confirmed, yellow = time passing, red = needs attention. The budget is ~90 % neutral, ~9 % structure, ~1 % signal, and every screen must still read correctly in grayscale.
- **Accent** — `--accent` becomes `#E85D04` (light) / `#F07A2A` (dark) with `--accent-fg: #1E1F21`. The logo orange `#d35400` disappears from the UI: the logo mark's triangle is painted with `var(--accent)` so it follows the theme, and the favicon carries the same value as a literal. `data-accent="orange"` on `<html>` is dropped.
- **Typography** — `--font-sans` (IBM Plex Sans) is the UI voice for all labels, buttons, headings and body text. `--font-mono` (IBM Plex Mono) is used **only for data**: file names, paths, EXIF values, coordinates, counters, IDs and keyboard keys, with `font-variant-numeric: tabular-nums`. Weights 400/500/600 only. The size scale is `--text-xs` 11 px … `--text-xl` 26 px. **No small caps (2026-09-26):** rams-design allows uppercase with tracking for `--text-xs` meta labels, but it never requires it, so the app does not use it. Every label, section title, table header, sidebar section and switch state is in sentence case, and hierarchy comes from size, weight and colour. Field labels are `--text-sm` 500 in `--fg-2`. Section titles are `--text-sm` 600. Table headers, sidebar sections and switch states are `--text-xs`.
- **Shape and depth** — radii `--radius-sm` 2 px, `--radius-md` 4 px, `--radius-lg` 8 px, `--radius-full` for round controls and badges only. Flat surfaces: areas are separated by a surface step *or* a hairline, never both. `--shadow-float` applies only to layers that float above content (menu, popover, modal, drag preview); nothing at rest has a shadow.
- **Control height** — new tokens `--control-h` (36 px) and `--control-h-sm` (30 px). Everything in one row shares one height — buttons, selects, inputs, segment groups, switches — set with `height` and `box-sizing: border-box`, so a border counts inside the height rather than on top of it. Default and small controls are never mixed in one row.
- **Buttons** — every action has a frame; rank comes from color and order, not from a missing frame. Primary (accent fill) appears at most once per screen. Only two things are frameless: navigation entries and links in running text.
- **Motion** — `--dur-quick` 120 ms, `--dur` 160 ms, `--dur-slow` 240 ms with `--ease: cubic-bezier(.2,0,0,1)`; all durations collapse to 0 under `prefers-reduced-motion: reduce`. Motion explains a state change or does not happen.
- **Fonts are self-hosted**, subsetted WOFF2 for Plex Sans and Plex Mono, replacing the Google Fonts CDN link introduced by ADR-0018 (principle 9, and the app is meant to work offline on a NAS).

### Deviations from rams-design

Recorded here as the skill requires; anything not listed follows the skill.

- **Thumbnail overlay badges** are dark chips over the photo (`rgb(18 19 20 / .72)` with light mono text) rather than the skill's soft-tint badges. A tinted badge is unreadable over arbitrary photo content. The chips are uniform: the per-format and per-film-simulation colors of the old design are dropped, because eight decorative colors on top of a photograph compete with the photograph and dilute the signal colors' meaning.
- **The viewer** keeps its near-black background (`#111213`) and light-on-dark controls rather than following the theme, for the same reason it always has: it is a presentation surface, not chrome.

## Consequences

- Every CSS rule referring to `--radius-sm`/`--radius-lg`, to the old neutral names or to `--font-mono` as the default face needs review in one pass; radii shrink (6 → 2/4, 14 → 8) and `body` switches to Plex Sans.
- The Google Fonts CDN dependency from ADR-0018 is removed; two subsetted WOFF2 files are added under `src/web/`, which makes the UI render identically offline.
- `#d35400` disappears from the UI. The app looks slightly different in its most recognizable detail; this is deliberate — the accent is verified against `--accent-fg` at ≥ 4.5:1, which the logo orange with white text was not. The raster logos (`logo.png`, `logo-96.png`, `apple-touch-icon.png`) still carry the old orange and are replaced separately.
- ADR-0008 remains in force and unchanged; this ADR is its token-level implementation. The "2 px radius max" rule that ADR-0018 superseded is effectively restored for inputs and tags, with 4 px for buttons and 8 px for panels.
- Screenshot-based e2e assertions and any test matching on color literals must be updated.
