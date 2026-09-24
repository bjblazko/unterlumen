# Rams redesign: remaining screens

*Last modified: 2026-09-24*

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
| View › Sort field (Name, File modified, Photo taken, Size) | Orders the folder | folder — select styled as a button, plus a small button for the direction | Same reason. Built with two controls rather than the prototype's single select: four fields × two directions is eight options to read through, where a field and a direction is two things to point at. Both sit in one row at one height |
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
  Playwright screenshot and list the differences in a table. A difference is
  fine only when it has a reason, for example a real data or API constraint as
  in phase 5's "Add to gallery only in libraries". The result is the section
  below.

## Screenshot comparison (2026-09-23)

How it was made: the prototype was rendered from its saved source at 1440×950
(desktop) and 480×950 (phone) with the notes layer off; the app was driven with
Playwright against a dev build on port 8080 holding the real libraries,
galleries and destinations, at 1440×950 and at 390×844 (2× phone). Every
prototype screen has a counterpart: Folders, Marked for deletion, Organize,
Libraries, library detail, Galleries, gallery detail, Destinations, destination
detail, Settings; on the phone Folders, Libraries, Galleries and the
desktop-only notice.

### Fixed during the comparison

| Screen | Element | Prototype | App (before) | Fix |
| --- | --- | --- | --- | --- |
| All with a path | Breadcrumb | Grey path, current segment plain `--fg` | Separators and current segment in orange, bold | Crumbs are links (underlined, `--fg-2`), current segment `--fg` 500. Orange is the primary action, and a path is not an action |
| Folders, library | Selected / focused photo | 3 px ring in `--fg`, image dimmed | 3 px orange ring plus orange wash | Ring and wash in `--fg`. Selecting is not the screen's primary action, and the selection bar's "Add to gallery…" is |
| Folders, Organize | List rows | `--bg-2` with a `--fg` bar | Orange tint with an orange bar | Same, in `--bg-2` / `--fg` |
| Organize | Between the panes | Three framed buttons | Same buttons over a large orange arrow, the whole column tinted | Arrow removed; the direction is in the labels, which now flip (`Copy →` / `← Copy`) with the active pane |
| Organize | Pane toolbars | Plain file lists | Each pane repeated Slideshow, Make library…, Clear cache | Folder tools hidden inside Organize; they belong to Folders |
| Organize | Jump to library | — | `dropdown-btn` with a custom menu | A `<select>`: a library is a place to navigate to, not an action. Libraries outside the server root say so instead of being offered |
| Libraries | Row | Hairline between rows | Bordered card on `--bg-2` | Hairline, page background, filmstrip aligned with the text |
| Libraries | Maintenance | Not shown | Chevron menu on the row with four rare runs | The four runs moved into "Edit library…", as the tools inventory says; the row keeps only "Scan for new photos" |
| Library detail | Head | Details · Statistics · Edit library… | Also a chevron menu with the same four runs | Removed; "Scan for new photos" stays, because it scans the folders you have open |
| Galleries, gallery detail | State | One state per row; a dead link reads "Not reachable · time" | Row and detail showed a green "Online" next to a red "Not reachable · 03:33 AM", and the detail's sentence contradicted its own pill | "Online" means uploaded *and* answering, so a failed check now replaces it rather than sitting beside it. In every other state ("Changes not online", "Built, not uploaded") the check is still shown beside the state, because both can be true at once. "Check again" is no longer the accent button either — it publishes nothing |
| Library detail | Filter sliders | — | Four orange range sliders in one panel | The chosen range is state, not the screen's one primary action |
| Settings | Cache size | `2.4 GB` | `5759.7 MB` | Gigabytes above 1024 MB |
| Phone · Folders | Page head | Breadcrumb + Details switch | Also layout segment, sort, sort direction, image count | Those are desktop controls (`desk-only` in the prototype); the count repeats the title line |
| Phone · Libraries | Page head | Title + nothing that writes | Sort select and Scan buttons, head overlapping its own title | Sort, Scan and the reorder arrows hidden; the head wraps instead of overlapping |
| Phone · desktop-only | Way out | Secondary button | Accent button | Secondary |

### Kept differences

| Screen | Element | Prototype | App | Reason |
| --- | --- | --- | --- | --- |
| Folders | Page head | Breadcrumb, layout, sort, switches, Slideshow | Also Home and Up buttons, "Make library…", "Clear cache · N files", an image count, wrapping to a second row | Home/Up are real navigation in a filesystem the prototype only sketches; the two folder tools are where the inventory (and the owner's decision) put them. Five more controls do not fit one row at 1440 px |
| Folders | Sort | One select with eight combinations | A field select plus a direction button | Written down in the tools inventory: a field and a direction are two things to point at, not eight to read |
| Folders, library | Switch labels | `Details On/Off` | `DETAILS SHOWN/HIDDEN` | ADR-0019's three-label rule and phase 7's wording; the prototype predates it |
| Folders | Subfolder row | Framed buttons, always secondary | Same, with the focused folder filled | Keyboard focus has to be visible; the prototype has no keyboard |
| Library detail | Filter panel | Search, chips, film simulation, folders | Date taken, shutter, aperture, focal length, ISO, camera, lens, film simulation, more filters | The real filter is the feature; the prototype only sketches one |
| Library detail | Head | No Filter button | "Filter" toggles the panel | The panel is permanent, but on a narrow window it has to be closable; the width is remembered |
| Libraries | Head | Sort, New library… | Also Filter and Statistics | Cross-library search and statistics have no other entry point; both only read |
| Libraries | Custom order | Arrows only in custom order | Same | — |
| Galleries, gallery, Destinations | Everything | — | — | Phases 3, 4 and 6; no new differences found |
| Marked for deletion | Empty state | "press Delete" | "press Backspace" | Both keys mark; Backspace is the one that works everywhere, including laptops without a Delete key |
| Organize | Page head | "Organize" plus `Tab` / `Space` hints | No head | The sidebar already names the place, and the two file lists need the height. The key hints live in the pane headers' own row |
| Organize | Pane content | Plain lists | Full browse panes with layout, sort and switches | The panes are real browse panes (ADR-0005); choosing list or grid while moving files is the point of them |
| Settings | Section labels | Sentence case | Small caps | The app's form-label style, used on every other form |
| Settings | Sections | Theme, quality, cache, helpers | Also "Interface" (hide the UI) and "What these are for" | Real settings that existed before and have nowhere else to live |
| Phone · Libraries | Row | Name, count, path, strip | Also "Indexed <date>" | It is information, not an action, and it is the one thing that says whether the list is current |

## Acceptance Criteria

- [x] Tools inventory written down here, one line per entry. Tools dropdown
      and View dropdown removed; layout segment, sort select and Slideshow
      in the page head.
- [x] Folders starts in the last or configured folder; title line;
      subfolders as a button row.
- [x] Libraries overview: row clickable, no accent buttons, Scan with
      indexed date, sort select, Edit and Delete only in the detail.
- [x] Library detail: permanent filter panel with chips; head as specified;
      "Channels ›" and "Organise: jump to folder" gone.
- [x] Marked for deletion: sentence, inline two-step delete, empty state.
- [x] Organize: pane headers and labelled Copy/Move buttons.
- [x] Settings as a place.
- [x] Code search above returns no leftovers: no `confirm(`, `alert(` or
      `prompt(` left in `src/web/js`, no `dropdown-btn` (`dropdown.js` and its
      CSS are gone, since nothing used them any more), at most one
      `btn-accent` per screen or dialog state, every action framed.
- [x] Comparison table for every prototype screen, desktop and phone. Every
      kept difference has a written reason.
- [x] e2e specs updated; CHANGELOG entry. The suite is green at 283/283
      (2026-09-24). The specs that broke were reading the old DOM (folder
      tiles instead of chips, `.wastebin-header`) or the old behaviour (a
      Filter button that opens a panel which is now open, buttons that were
      greyed out without a reason); one of them, the library's blank pane,
      was a real defect and not a stale selector.
