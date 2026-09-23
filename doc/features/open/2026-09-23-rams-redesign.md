# Rams redesign: places, galleries and one publish action

*Last modified: 2026-09-23*

## Summary

A full UX and visual redesign of Unterlumen, based on the global
`rams-design` skill (Dieter Rams' ten principles plus a neutral visual
system). The biggest change is publishing. "Channel" is split into
**Destination** and **Gallery**. States (adjectives) are kept apart from
actions (verbs). Publishing becomes a single action. Navigation becomes a
sidebar of places instead of a pseudo-workflow stepper. On a phone the UI is
read-only browsing.

The target design is a clickable prototype with per-element design notes, a
review and a phased rebuild plan (private link, owner only):
https://claude.ai/artifact/NfDYzmU9qvsQPwNkGi68LE

Each note in the prototype names the files, functions and APIs to change.
Read the note for a screen before building it.

## Details

### Review findings (current UI)

- Understandable and honest are rated weak:
  - "Publish" sits on every live row even when nothing is pending.
  - The "Published" tab also contains drafts.
  - Generate, Deploy, Publish and Build are four words for one intent.
  - Chevron steps left of the active one look "done" although nothing was
    done.
  - "Unreachable" gives no time or cause.
- As little design as possible is rated weak. A channel is at once export
  format, address, website config, upload method and, as a workaround, an
  album. Five channels point at the same share host.
- Kept on purpose:
  - local-first, XMP sidecars
  - Commander culling (ADR-0005)
  - keyboard-first culling
  - the three-label toggle rule (ADR-0019)
  - collect-then-publish drafts (ADR-0026)
  - the reachability check
  - all thumbnail overlays (GPS, format, film simulation, aspect ratio)
  - the full info panel including the map

### Decisions (2026-09-22/23)

- **Navigation.** A sidebar of places, grouped in working order:
  - Cull: Folders, Marked for deletion, Organize
  - Catalog: Libraries
  - Publish: Galleries, Destinations
  - Settings is pinned to the bottom.

  Other rules:
  - Entries are links (`<a href>`, `aria-current`), not buttons, so back
    and deep links work.
  - The sidebar and the info panel collapse (`\` and `I`). The state is
    remembered.
  - `#mode-*` IDs and the number shortcuts stay.
- **Vocabulary.** UI wording only; backend names stay:

  | Before | After |
  | --- | --- |
  | Channel | Destination |
  | Published tab | Galleries |
  | Add to channel… | Add to gallery… |
  | Generate + Deploy | Publish / Publish N changes |

- **Gallery states:**
  - Not online yet
  - Online
  - Changes not online
  - Built, not uploaded
  - Exported to folder (files destinations)

  "Not reachable · <time>" is a check result shown on top of the state, not
  a state of its own. Row actions appear only when there is something to do.
- **Publish is one action:** export, build, upload (rsync), then check the
  link. Progress is determinate and errors show inline (no `alert()`).
  "Preview locally" is optional and never forced.
- **Date bug.** Publishing keeps the gallery's existing date. Today the
  dialog defaults to the current date and silently re-dates a gallery.
- **Selection bar.** All actions on a selection live in one bar at the
  bottom: Add to gallery (primary), Export, Rename, Set location, Mark for
  deletion, Clear selection (Esc).
- **Buttons.**
  - Every action has a frame. Orange marks only the one primary action per
    screen.
  - Frameless are only navigation entries and links in running text.
  - Controls in one row share one height: 36 px by default, 30 px for the
    small variant.
- **Overlays.** Badges are uniformly dark with a mono font instead of one
  color per format and film simulation. "Show details" and "Show names"
  become visible toolbar switches.
- **Info panel.**
  - The short list comes first.
  - The map appears when the photo has GPS, with a 2D/3D switch.
  - Everything else sits in "All metadata", grouped by Camera, Exposure,
    Fujifilm, File and XMP. Its open/closed state is remembered.
- **Destinations.** Destinations become their own place instead of a modal
  reachable only from a library. The form asks for the type first (Share
  links / Website / Files) and then shows only the relevant fields. The slug
  is derived and read-only.
- **Phone (≤ 700 px).**
  - Read-only browsing: libraries, folders, statistics, metadata and
    gallery status.
  - A tab bar (Libraries, Folders, Galleries) replaces the sidebar.
  - A full-screen viewer with swipe and an info bottom sheet.
  - Statistics as a full page with responsive charts.
  - No actions, no slideshow, no Organize/Destinations/Settings.
- **Tokens.** Replace the Hüpattl! tokens (ADR-0018) with the rams-design
  tokens (`~/.claude/skills/rams-design/references/tokens.css`). IBM Plex
  Sans is the UI voice, IBM Plex Mono only for data.
- **Existing albums.** They and their metadata stay untouched. A website
  showing "Not reachable" needs no action as part of this redesign.

### ADRs

Written on 2026-09-23:

- [ADR-0028](../../architecture/adr/0028-places-not-workflow-steps.md) —
  places, not workflow steps (navigation; references
  `2026-03-09-workflow-oriented-ui.md`).
- [ADR-0029](../../architecture/adr/0029-destinations-galleries-one-publish-action.md)
  — destinations and galleries; publish as one action (vocabulary, state
  model, and `generatedAt`/`deployedAt` per gallery to detect "Built, not
  uploaded").
- [ADR-0030](../../architecture/adr/0030-rams-design-tokens.md) — adopt
  rams-design tokens (supersedes ADR-0018).

### Follow-up feature (separate doc)

[Move gallery to…](2026-09-23-move-gallery-to.md) merges the per-album
channels on the same host into one destination without breaking links. It
rewrites `gallery.json`, the output folder and the `built:<slug>:<postID>`
XMP keys (ADR-0027). The `2026-09-20-many-galleries-per-channel` doc
excludes migration, so this needs its own design.

### Out of scope

- Backend renames (channel, galleryExport, siteExport, drafts.json stay).
- Any change to published sites' URLs.
- A redesign of the viewer, slideshow, crop, batch rename or export dialogs
  beyond tokens and button rules.

## Acceptance Criteria

Phases are independent enough to ship one at a time. Phase 8 can come at any
point.

- [x] Phase 1: tokens and type. The `style.css` tokens are replaced with
      rams-design. Plex Sans is the UI font, Mono is used only for data.
      There is a new ADR superseding 0018.
- [x] Phase 2: navigation. Sidebar with groups; entries are `<a href>` with
      `aria-current`. The sidebar and info panel collapse. Chevrons and the
      slide animation are removed. e2e selectors `#mode-*` keep working.
- [x] Phase 3: Galleries.
  - The Published tab becomes Galleries, grouped by destination.
  - It uses the new status labels and contextual row actions.
  - A gallery detail view covers pending photos, title, date, visibility
    toggle (ADR-0019) and unpublish.
  - The date bug is fixed.
  - "Built, not uploaded" and "Exported to folder" wait for phase 4: both
    need per-gallery `generatedAt`/`deployedAt` in the statefiles. Until
    then a built gallery with no address says exactly that.
- [x] Phase 4: publish as one action.
  - Generate, then (rsync) deploy, then the reachability check for that URL.
  - Progress comes from `buildStream` events.
  - Errors show inline.
  - The state "Built, not uploaded" is persisted per gallery.
  - A plain-export destination still has no row of its own after its draft
    is consumed — it writes no statefile, so there is nothing to list.
    "Exported to folder" covers the destinations that do write one.
- [x] Phase 5: selection bar and "Add to gallery".
  - One shared SelectionBar for folders, libraries and search results.
  - The collect dialog becomes a gallery list with search and "New
    gallery".
  - "Add to gallery" appears only where there is a library to collect
    from: a plain folder holds files, not library photos, and the collect
    API needs photo IDs. The prototype shows the action everywhere
    because everything in it is a library.
- [x] Phase 6: Destinations as a place.
  - A type-first form.
  - The slug is derived.
  - An upload section with "Test connection".
  - Accounts and key/value settings under Advanced, together with
    "Rebuild"'s neighbours: reveal the output folder and delete the
    destination.
- [x] Phase 7: overlays and info panel.
  - Uniform dark badges.
  - Details and Names as visible switches.
  - The info panel ordered as short list, then map, then "All metadata".
  - The map turned out to be broken for everyone: MapLibre came from an
    unversioned CDN URL that MapLibre 6 emptied out. It is vendored now
    ([ADR-0031](../../architecture/adr/0031-vendor-maplibre.md)).
- [ ] Phase 8: phone.
  - Tab bar; `.desk-only` hides all actions.
  - A full-screen viewer with swipe and an info sheet.
  - Statistics as a full page with responsive d3 charts.
  - Desktop-only screens show a notice instead.
- [ ] Each phase is compared against the prototype and covered by e2e
      tests. New or changed behavior is documented in the CHANGELOG.
