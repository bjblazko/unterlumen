# Organize: one source, many targets

*Last modified: 2026-09-24*

## Summary

Organize becomes one place with one job: put the photos of the folder you are
looking at into the folders they belong in. The screen shows the source folder
large — as large as Folders shows it — and the targets as a narrow list on the
right, each with a number key. Selecting photos and pressing that number moves
them; the line below takes the move back.

This replaces the dual-pane file manager (ADR-0005). The decision and its
reasons are in [ADR-0032](../../architecture/adr/0032-organize-one-source-many-targets.md);
the three designs it was chosen from are in the Organize clickdummy
(https://claude.ai/artifact/ERsGf24yvm5ifXLXZJ7VpT).

## Details

### The screen

- **Head**: "Organize", the source folder's path in mono, "Change folder…",
  and the key hints. Once something has been sorted, the head also says how
  much: "34 in this session".
- **Source (left, wide)**: a full browse pane — breadcrumb, folder chips,
  layout and sort, the photos at Folders size. What acts on a folder
  (Slideshow, Make library…, Clear cache) is hidden here, as it is in the
  current Organize: this place moves photos.
- **Targets (right, narrow)**: one row per target folder, each with its name,
  its path in mono, a session counter once it has received something, and its
  number key. "Mark for deletion" is the last target and always present.
- **One accent**: the current target, the one Enter sends to. The number keys
  pick another and make it current.

### Interaction

| Action | Mouse | Keyboard |
| --- | --- | --- |
| Select photos | Click, ⌘/⇧-click | Arrow keys, Space |
| Move the selection to a target | Click the target | `1`…`9`, or `Enter` for the current one (while something is selected) |
| Copy instead of move | ⌥-click the target | `⌥`+number |
| Take the last move back | "Undo" in the bar | `U` |
| Clear the selection | "Clear selection" | `Esc` |
| Add a target | "Add target…" (folder picker) | — |
| Create a target folder | "New target…" | — |

The number keys are the place shortcuts everywhere else, so they only aim at
a target while something is selected; with nothing selected they still switch
places, and `Enter` still opens the focused folder. Selecting first and sending
second is how the rest of the app works too.

Moves run immediately, as they do today (ADR-0005's one virtue): the undo bar
sits beside the result instead of a question in front of it. A copy has no
undo — it would mean deleting files the user might already have touched — and
says so by offering no button.

### Targets

- One list for everything, not per source folder: the same "Auswahl",
  "Familie", "Reisen" are the targets whichever import you open.
- Stored client-side (ADR-0012), like the other UI settings.
- A target that has gone missing says so in its row and offers to be removed
  from the list; it is never silently dropped.
- No thumbnail and no total count per target: both need a full scan of a
  folder that may hold 30 000 files, on a NAS, every time the screen opens.
  The path identifies the folder, and the session counter says what this
  sorting run has put there.

### Folders as targets of "Mark for deletion"

Deleting a folder with its contents exists today only in Organize's middle
column, behind a browser-style question. It moves to the waste bin like
everything else: a selected folder can go to the "Mark for deletion" target,
appears in *Marked for deletion* with a folder tile, and is deleted there —
where the count and the two-step confirmation already live. The confirmation
says what it is deleting ("3 photos and 1 folder with its contents").

### What goes away

- `commander.js`, the dual-pane screen, its resizer and its middle column.
- "Jump to library…" — the source folder is chosen with the folder picker,
  which can reach a library's folder like any other.
- The `commander` mode name: the place is `organize` (`#mode-organize`,
  hash `#organize`, shortcut `3` as before).

## Acceptance Criteria

- [x] Organize shows one source pane and a target list; no second browse pane.
- [x] Selecting photos and pressing a number key moves them to that target;
      ⌥ copies. Enter uses the current target.
- [x] The current target is the only accent-coloured element on the screen.
- [x] A move can be taken back with `U` or the button in the bar, and the
      photos are where they were.
- [x] Targets survive a reload; a missing target says so instead of failing.
- [x] "Mark for deletion" works from Organize for photos and for folders;
      *Marked for deletion* shows a folder tile and names folders in its
      delete confirmation.
- [x] "Show in Organize" from a library still opens that folder with the
      photos preselected.
- [x] `commander.js` and its spec are gone; nothing references `mode-commander`.
- [x] e2e spec for the new screen; CHANGELOG entry; README and arc42 updated.
