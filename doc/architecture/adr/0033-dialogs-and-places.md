# ADR-0033: Dialogs and places, and one dialog to build them with

*Last modified: 2026-09-26*

## Status

Accepted.

## Context

The redesign (ADR-0028 to ADR-0032) sorted out the places. The dialogs grew
on their own: twelve of them in three different builds — `modal-overlay`,
`modal-backdrop` and `library-dialog-backdrop` — each with its own keyboard
behaviour. CLAUDE.md carried a warning about which build to copy, and every
new dialog had to read it first. Two `.modal` rules even overrode each other,
which is why the dialogs looked subtly different from one another.

Underneath the styling sat a question nobody had answered: when is something
a dialog at all, and when is it a place with an address?

## Decision

**A dialog is for one decision about what is already on the screen.** It fits
on one screen, it needs the context behind it, and it ends with an action or
a cancel.

**A place is for what you read, compare or work in for a while.** It has an
address, a way back, and it survives a reload.

The questions, in order:

1. Does it act on a selection or an object that has to stay visible behind
   it? → dialog.
2. Would you link it, share it, or leave it with the back button? → place.
3. Does it take longer than a decision — editing a table, comparing values,
   paging through? → place.
4. Does it block the work behind it without needing it? → neither: put it
   inline on the screen that raised it, the way the delete confirmations do.
5. Do you keep it in view while you work, watching the screen answer it?
   → neither: a **panel**, a part of the screen you open and close.

**The fifth answer, added 2026-09-25, is the filter.** A filter is not a
decision (it changes continuously), and it is not a place (as a screen of
its own you cannot see what it does). It is a panel, in the same sense as
the sidebar: a column you open and close from a panel button that sits
directly above it, carrying the same glyph as the sidebar's own. Open, it
takes its width from the photos instead of lying over them — nothing is
hidden behind it, and the thumbnails beside it answer for every criterion as
it is set. It has no scrim and never traps the keyboard, because the photos
stay live. On a phone there is no width to share, so there it covers the
library for as long as it is open and gives the screen back on *Done*.

**One filter, in both places (2026-09-26).** The Libraries overview had a
"Search…" button of its own that replaced the list with results at once. It
used the same panel and endpoint but behaved differently. Now the overview has
the same filter as a library:
- the same button at the left end of the head, and the same column;
- nothing changes until a criterion is set;
- *Done* closes the column and keeps the results;
- the × on the results drops the criteria and gives back what they replaced.

The only difference is the scope it starts with: every library in the
overview, the open library inside one. The panel's library select changes
the scope in both places. Searching across libraries is filtering with a
wider scope, not a different tool.

All dialogs are built with one component, `src/web/js/dialog.js`, which owns
the frame and the behaviour but never the content: scrim, header, scrolling
body, footer; Escape and a scrim click cancel; focus moves in on open, stays
inside, and returns where it came from; at most one accent action, always
last; on a phone it is a sheet at the bottom edge.

There is no closing cross. Cancel already does that job, and an icon-only
button beside a labelled one is one button too many.

## The twelve, measured against the rule

| Dialog | Verdict |
| --- | --- |
| Choose folder, Publish, Progress, Helper programs, About, Location, Add to gallery, New library, Edit library | Dialog. One decision, one screen, the context behind matters. |
| Export, Batch rename | Dialog. They are forms, but they belong to a selection that stays visible behind them; leaving them is a cancel, not a navigation. |
| Slideshow | Dialog. One decision (how it should play) before one action. |
| Statistics | Dialog, against rule 3 — see below. |

**Statistics strains the rule.** It is 1 200 lines, fourteen charts and a
library selector; you read and compare in it, which by rule 3 makes it a
place. It stays a dialog by decision (2026-09-24): it always belongs to
exactly one library, it is opened from that library's own screen, and the way
back is one press. If it grows a second entry point — from the sidebar, or
across libraries — it becomes a place.

## Consequences

- `modal-overlay`, `modal-backdrop` and `library-dialog-backdrop` are gone,
  with their duplicate `.modal` rules; the gotcha section in CLAUDE.md is
  three lines now.
- `app-keyboard.js` has one guard left: a dialog owns the keyboard while it
  is open. The crop tool, which is not a dialog, says the same thing with
  `.keyboard-owner`.
- Focus is trapped and returned for every dialog, which none of them did
  before.
- Dialogs stack (a folder picker on top of an export dialog): each sits above
  the one that opened it, and Escape closes the topmost.
- A new dialog is a `new Dialog({...})` with a body and actions. Nothing to
  copy, nothing to look up.
