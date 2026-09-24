# ADR-0032: Organize is one source and many targets

*Last modified: 2026-09-24*

## Status

Accepted. Supersedes [ADR-0005](0005-commander-style-culling.md).

## Context

ADR-0005 built culling as a Norton-Commander dual pane: two equal browse
panes, select on one side, copy or move to the other. It was the right
decision at the time — no metadata, a familiar paradigm, filesystem
operations only — and those virtues are kept here.

What the redesign made visible is the cost of the symmetry. Both panes are
full browsers, so the Folders toolbar appears twice on one screen; the middle
column collected actions from three different responsibilities (selection,
folder, navigation); and the photos, the thing you are actually judging, end
up in two narrow columns. Sorting keepers out of an import means selecting,
then aiming at a pane, then pressing a button — for each destination in turn.

Three designs were drawn and compared in a clickdummy
(https://claude.ai/artifact/ERsGf24yvm5ifXLXZJ7VpT): the dual pane tidied up,
a source with target baskets, and a one-photo-at-a-time session whose
assignments are applied at the end. The owner chose the second.

## Decision

Organize shows **one source folder, large, and a list of target folders,
narrow**. The selection is sent to a target by clicking it or pressing its
number key; ⌥ copies instead of moving. The targets are a single remembered
list, stored client-side (ADR-0012), not per source folder.

Moves still run immediately without a confirmation dialog — that part of
ADR-0005 survives — but the result line offers to take the last move back.
Copies are not undoable and offer no button.

"Mark for deletion" is a target like the others. Folders can be sent to it
too, which moves folder deletion out of Organize's middle column into the
waste bin, where the count and the two-step confirmation already live.

## Consequences

- **The dual pane is gone.** `commander.js`, its resizer, its middle column
  and its spec are removed; the mode is called `organize`.
- **Still no metadata.** Sorting remains copy, move and the waste bin —
  ADR-0002 is untouched. The only stored state is the list of target paths.
- **One primary action per screen** (ADR-0030): the current target is the
  single accent element, even though there are several targets.
- **Keyboard first, different keys.** `Tab`/`F5`/`F6` had meaning only with
  two panes. Number keys, `Enter`, `U` and `Esc` replace them; the rest of
  the browse keyboard is unchanged.
- **Targets can go missing.** A remembered path can be renamed or unmounted
  outside Unterlumen. The row says so and offers to forget it; nothing is
  dropped silently.
- **No per-target thumbnail or total count.** Both would need a full scan of
  a folder that may hold tens of thousands of files on a NAS, every time the
  screen opens (principle 9). The path and a session counter say enough.
- **Deleting a folder is now two steps**, in a different place than before:
  mark it, then delete it in *Marked for deletion*. That is slower on purpose
  — it deletes a folder with its contents and cannot be undone.
