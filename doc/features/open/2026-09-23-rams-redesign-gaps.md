# Rams redesign: remaining screens

*Last modified: 2026-09-23*

## Summary

Phases 1–8 of [the Rams redesign](2026-09-23-rams-redesign.md) cover
navigation, galleries, publish, the selection bar, destinations, overlays and
the phone layout. The prototype redesigns more than that. These screens and
controls exist in the prototype (and its notes) but had no phase in the
feature doc, so they still look as before:

- the Folders toolbar (View and Tools dropdowns)
- the Libraries overview and the library detail header
- Marked for deletion
- Organize
- Settings

This doc lists them as phase 9. It ends with a full comparison against the
prototype, so nothing else slips through the same way.

Prototype (private): https://claude.ai/artifact/NfDYzmU9qvsQPwNkGi68LE. Read
it with the Artifact tool (`action: "read"`). Each screen is a `scr*`
function in its script, and the `NOTES` object holds the rationale and
implementation hints per element.

## Details

### Folders (`scrFolders`, notes `folders-start`, `toolbar`, `selbar`)

- **Start folder.** Start in the last visited folder, falling back to the
  configured photo folder. Never start in the filesystem root with
  "0 images".
- **Page head.**
  - Breadcrumb.
  - Layout as a visible segment group (Justified · Grid · List) instead of
    the View dropdown.
  - Sort as a button-styled select.
  - The Details / Names switches (done in phase 7).
  - Slideshow as a framed quiet button.
- **Tools dropdown goes away.** Inventory every entry and move it:
  - Needs a selection (rename, batch rename, set/remove location, export,
    crop…): goes to the selection bar, which already exists since phase 5.
  - Acts on the current folder (for example scanning it into a library):
    goes to the page head as a framed quiet button with a clear label.
  - Rare maintenance (cache): goes to Settings.

  Write down where each entry went in this doc before building. Ask the
  owner if an entry fits none of the three.
- **Folder title line.** The folder name as a heading, followed by
  `N photos · N folders · size` in the secondary text colour.
- **Subfolders.** Shown as a row of framed buttons with a folder icon above
  the photos, instead of large empty folder tiles inside the photo grid.

### Tools inventory (written 2026-09-23, before building)

Every entry of the two dropdowns, and where it goes. "Selection" means the
selection bar from phase 5, "folder" the Folders page head, "library" the
library's own screen, "settings" the Settings place.

| Entry | What it does | Goes to | Why |
| --- | --- | --- | --- |
| View › Layout (Grid / Justified / List) | Switches how photos are laid out | folder — segment group in the page head | A view you switch while looking; two clicks deep is one too many |
| View › Sort field (Name, File modified, Photo taken, Size) | Orders the folder | folder — select styled as a button, with the direction in the same control | Same reason; the direction arrow becomes part of the select's label rather than a separate 30 px button |
| View › Show names, View › Show details | Toggles the name and the chips on a thumbnail | done in phase 7 — switches in the page head | — |
| Tools › Make library | Turns the focused folder into a library | folder — quiet framed button "Make library…", shown when a folder is focused | Acts on the folder, not on a selection |
| Tools › Scan for new photos | Indexes new files of the open library | library — the row's own "Scan for new photos" on Libraries, and "Edit library…" in the detail | Acts on the library; the Libraries overview already shows when it was last indexed |
| Tools › Rebuild metadata & previews | Re-reads EXIF, rebuilds previews | library — "Edit library…" | Rare maintenance on one library |
| Tools › Generate missing previews | Fills gaps in the preview cache | library — "Edit library…" | Rare maintenance on one library |
| Tools › Rebuild all previews | Rebuilds every preview | library — "Edit library…" | Rare maintenance on one library |
| Tools › Remove deleted photos | Drops rows whose files are gone | library — "Edit library…" | Rare maintenance on one library |
| Tools › Clear cache for selection | Clears cached previews for the selected files, or the focused folder | settings — folded into the existing "Clear cache" | See the open question below |
| Selection: rename, batch rename, export, set location, mark for deletion, add to gallery, show in Organize | — | done in phase 5 — selection bar | — |
| Crop | Crops one photo | stays in the viewer | It needs the photo on screen to draw a rectangle on |

#### Decided 2026-09-23

1. **"Clear cache for selection" stays**, as a quiet framed button in the
   Folders page head (owner: "einbauen, clear cache for selection als ruhiger
   knopf").
2. **Both location actions stay** (owner: "beides benoetigen wir, du
   entscheidest, wohin die aufrufe hinkommen"). "Set location…" keeps its
   place in the selection bar; the dialog it opens gains "Remove location" as
   a destructive secondary action, so everything about a photo's location is
   in one dialog instead of an eighth button in the bar.

#### The open questions those decisions answered

1. **"Clear cache for selection" disappears.** Settings already has a global
   "Clear cache"; a per-selection variant is a maintenance action hidden in a
   menu, used to work around a stale preview. Dropping it removes a capability
   — say so if you use it, and it becomes a quiet button in the page head
   instead.
2. **"Remove location" has no entry point at all right now.** Phase 5 moved
   the selection actions into the bar and took the Tools dropdown's
   Geolocation section (Set / Remove) with it, but only "Set location…" was
   put back. The handler and `POST /api/remove-location` are untouched, so
   this is a UI regression introduced by that phase, not a lost feature.
   Proposal: everything about a photo's location belongs in one dialog, so
   the Set location dialog gets "Remove location" as a destructive secondary
   action, rather than an eighth button in the bar. The alternative is a
   separate "Remove location" entry in the selection bar.

### Libraries overview (`scrLibs`, note `libs`)

- **One row per library, the whole row clickable.** No orange "Open" button;
  five accent buttons on one screen break "one primary action per screen".
- **Row content.**
  - Name and photo count.
  - The path in mono.
  - A thumbnail strip.
  - Right side: "Scan for new photos" as a small framed quiet button, with
    "Indexed <date>" below it.
- **Edit and Delete move into the library detail.** Delete is never next to
  Edit.
- **Sort.** The "Sort by recent additions / Custom" toggle becomes a select
  (Recently added · Name · Custom order). The reorder arrows appear only in
  Custom order.
- **New library.** "New library…" is a small secondary button in the page
  head.

### Library detail (`scrLib`, note `lib-detail`)

- **Page head.**
  - Breadcrumb `Libraries / <name>`.
  - The Details switch.
  - "Statistics".
  - "Edit library…", which holds rename, path, rescan and a two-step
    delete.
- **Filter panel.** Permanently on the left instead of a Filter toggle
  button. Active filters show as removable chips. The panel width is
  remembered. On the phone it stays the sheet from phase 8.
- **Removed from the head.** "Organise: jump to folder" becomes the
  selection-bar action "Show in Organize". "Channels ›" disappears, because
  destinations have their own place now.

### Marked for deletion (`scrMarked`, note `marked`)

- **Explanatory sentence.** "These photos are hidden from Folders and
  Libraries. Nothing is removed from disk until you delete them here."
- **Buttons.** "Restore all" is secondary. "Delete N permanently…" is
  destructive (framed, red label, not orange). It confirms inline with the
  count ("Delete 12 files from disk? This can't be undone." Cancel /
  Delete 12 files), not with `confirm()`.
- **Empty state.** One sentence and a "Go to Folders" button.

### Organize (`scrPanes`, note `panes`)

- **Active pane.** Its header is dark (`--fg` fill); the inactive one uses
  `--bg-2`.
- **Pane headers.** Each shows its path in mono and "N of M selected".
- **Buttons.** Labelled framed buttons "Copy →" and "Move →", each with its
  key hint (F5 / F6), plus "New folder". The logic is unchanged (ADR-0005).

### Settings (`scrSettings`)

A place in the sidebar (pinned to the bottom since phase 2) instead of a
header dropdown:

- Theme as a segment group (Light · System · Dark).
- Thumbnail quality as a three-label switch.
- Cache size and path with "Clear cache…".
- Helper tools found or missing.

### Across all screens

- **Search the code** for leftovers and fix every hit:
  - `confirm(`, `alert(`
  - `btn-accent` (at most one per screen)
  - `dropdown-btn`
  - frameless action buttons
  - controls in one row with different heights
- **Compare the finished work against the prototype.** For every prototype
  screen (desktop and phone), put the app into the same state, take a
  Playwright screenshot and list the differences in a table:

  | Screen | Element | Prototype | App | Fix / reason |
  | --- | --- | --- | --- | --- |

  A difference is fine only when it has a reason, for example a real data
  or API constraint as in phase 5's "Add to gallery only in libraries".
  Record the kept differences in this doc.

## Acceptance Criteria

- [ ] Tools inventory written down here, one line per entry. Tools dropdown
      and View dropdown removed; layout segment, sort select and Slideshow
      in the page head.
- [ ] Folders starts in the last or configured folder; title line;
      subfolders as a button row.
- [ ] Libraries overview: row clickable, no accent buttons, Scan with
      indexed date, sort select, Edit and Delete only in the detail.
- [ ] Library detail: permanent filter panel with chips; head as specified;
      "Channels ›" and "Organise: jump to folder" gone.
- [ ] Marked for deletion: sentence, inline two-step delete, empty state.
- [ ] Organize: pane headers and labelled Copy/Move buttons.
- [ ] Settings as a place.
- [ ] Code search above returns no leftovers.
- [ ] Comparison table for every prototype screen, desktop and phone. Every
      kept difference has a written reason.
- [ ] e2e specs updated; CHANGELOG entry.
