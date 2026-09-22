# ADR-0028: Places, not workflow steps

*Last modified: 2026-09-23*

## Status

Accepted

## Context

[ADR-0008](0008-dieter-rams-design-principles.md) makes Dieter Rams' ten principles the governing design philosophy. The navigation added by the [workflow-oriented UI redesign](../../features/done/2026-03-09-workflow-oriented-ui.md) violates two of them:

- **Honest (6).** The chevron stepper greys out every step left of the active one, which reads as "done". Nothing was done — the user simply hasn't visited those screens. Opening Organize marks Folders and Marked for deletion as completed although no photo was culled.
- **Understandable (4).** Only three of the six top-level screens are steps of anything. Libraries, Published and Settings were bolted onto a control whose shape promises a sequence, so the shape lies about four of its entries.

The stepper also forces a mode switch to be a `<button>` press: there is no URL for a screen, the browser back button does nothing, and "open in a new tab" is impossible. The 24 px slide animation on every mode change conveys nothing beyond the fact that a click landed.

What the workflow redesign got right is the *order*: Folders → Marked for deletion → Organize is the natural culling sequence, and grouping matters. That ordering survives; only the false state model and the stepper shape go.

## Decision

Replace `nav.workflow` with a sidebar of **places**, grouped in working order, with no notion of a step being done, current or future.

| Group | Entries |
| --- | --- |
| Cull | Folders (1), Marked for deletion (2), Organize (3) |
| Catalog | Libraries (4), plus one sub-entry per library |
| Publish | Galleries (5), Destinations (6) |
| — | Settings, pinned to the bottom |

Rules:

- **Entries are links.** Each is an `<a href="#<place>">` with `aria-current="page"` on the current one, not a `<button>`. Navigation changes where you are and is always reversible; an action changes something. Routing uses the hash plus `pushState`/`popstate`, so back, forward and deep links work.
- **Counts only where something is waiting.** A count appears next to Marked for deletion (photos marked) and next to Galleries (galleries with unpublished changes). Totals that ask nothing of the user are not shown.
- **The sidebar collapses** to an icon rail with `\`, the info panel with `I`. Both states are remembered client-side ([ADR-0012](0012-client-side-settings.md)).
- **`#mode-*` IDs and the number shortcuts 1–6 stay**, so existing e2e selectors and muscle memory keep working.
- The `clip-path` chevrons, the completed/future step styling and the directional slide animation are removed.
- On phones the same places are reachable from a bottom tab bar with the same `href`s; the phone scope itself is specified in the [redesign feature doc](../../features/open/2026-09-23-rams-redesign.md).

## Consequences

- The screens that were never steps (Libraries, Galleries, Destinations, Settings) stop pretending to be, and the Publish group gives Destinations a home of its own instead of a modal reachable only from an open library.
- Deep links and the back button work for the first time; a gallery or library can be opened in a new tab.
- e2e specs keep addressing `#mode-*`; specs that assert chevron classes or the slide transition must be updated.
- The count next to Galleries needs the aggregate endpoint (`GET /api/channels/galleries`, [ADR-0024](0024-published-galleries-overview.md)) at startup and after collect/publish, rather than on every render.
- Sidebar and info-panel collapse state joins the existing `localStorage` settings; nothing moves to the server.
- Supersedes the chevron stepper decided in `2026-03-09-workflow-oriented-ui.md`. That document's ordering and keyboard mapping remain in force.
