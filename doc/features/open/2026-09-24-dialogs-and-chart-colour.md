# One dialog, one rule — and colour in charts

*Last modified: 2026-09-24*

## Summary

Two things that grew wild while the places were being tidied: the dialogs and
the chart colours.

Twelve dialogs existed in three different builds with three keyboard
behaviours, which is why CLAUDE.md had to warn about which one to copy. They
are now one component with one behaviour, and the rule that decides between a
dialog and a place is written down ([ADR-0033](../../architecture/adr/0033-dialogs-and-places.md)).

The statistics drew with fixed hex values that ignored dark mode, and one of
them had been dead since ADR-0030 renamed a token — the axes were drawing with
an empty colour. Charts now take their colours from a validated ramp of their
own ([ADR-0034](../../architecture/adr/0034-colour-in-charts.md)).

## Details

### The dialog

`src/web/js/dialog.js` owns the frame and the behaviour, never the content:

- scrim, header (title + subtitle), scrolling body, footer;
- Escape and a click on the scrim cancel; focus goes in on open, stays inside
  while open, and returns to the button that opened it;
- at most one accent action, always last; a sentence can sit on the left of
  the footer (what is selected, where it goes, what failed);
- three sizes (`sm` 26 rem, `md` 35 rem, `lg` 56 rem);
- on a phone every dialog is a sheet at the bottom edge — that was true only
  for the statistics before;
- dialogs stack: a folder picker opened from the export dialog sits above it,
  and Escape closes the topmost.

No dialog has a closing cross any more. Cancel does that job, and an
icon-only button beside a labelled one is one button too many.

### What each dialog lost or gained

| Dialog | What changed |
| --- | --- |
| About, Helper programs | Frame only; "Dependencies" is now "Helper programs", with a line saying what the list is for |
| Progress | Not dismissible while files are moving; when the run ends, the one action is "Close" |
| Slideshow | `.seg` with `aria-pressed` instead of a private button group with `.active`; "Start slideshow" instead of "Start" |
| Choose folder | Frame only (it had just been rebuilt); the path note moved into the footer |
| Location | Frame; **"Remove location…" works again** — the button called a method that never existed, so it threw since phase 9. It now asks in place, naming how many photos it strips |
| Add to gallery, Publish | Frame; the footers are the dialog's, so the publish steps no longer carry their own |
| Export | Frame; the status line became the footer sentence, and its messages say what to do ("Name a folder to export into") |
| Batch rename | Frame; the token chips take chart colours; the failure message is a sentence, not "Rename failed." |
| Statistics | Frame; stays a dialog by decision, see ADR-0033 |
| New library, Edit library | Frame; "Delete library…" is its own width and reads as destructive at rest, not only on hover |

### Colour outside the charts

The audit that followed found orange standing in for things it does not mean:
error messages in five dialogs, progress bars and spinners, active tabs,
checked boxes and radios, hover states, the "new photos" dot, a drag target,
the viewer's active tool, a glow around the autocomplete panel. Orange is now
left in four places — the one primary action, the focus ring, a value being
adjusted right now, and nothing else — and the list is in CLAUDE.md so it can
be checked.

The same pass removed every hand-written colour from the stylesheet: sixteen
different alphas of white in the photo chrome (viewer, slideshow, crop tool,
badges, treemap labels) became eight named `--overlay-*` steps, which do not
follow the theme because a photo is the surface underneath them.

### Chart colour

- `--chart-1 … --chart-8` (categorical, fixed order, both themes),
  `--chart-seq-1 … 5` (magnitude), `--chart-grid`, `--chart-axis`.
- `--film-*` for the fourteen film simulations, as a documented exception.
- Every step validated with the dataviz palette checker in both modes: all
  six checks pass.
- `chartColors()` reads tokens only; the dead `--border` lookup is gone.
- Magnitude is a step on the sequential ramp, not an opacity guess.

## Acceptance Criteria

- [x] One dialog component; `modal-overlay`, `modal-backdrop` and
      `library-dialog-backdrop` no longer exist.
- [x] `app-keyboard.js` has a single guard, and CLAUDE.md's two-pattern
      gotcha is replaced by the rule.
- [x] Every dialog: Escape closes, focus is trapped and returned, one accent
      action at most, no closing cross.
- [x] Every dialog is a bottom sheet below 700 px.
- [x] Chart tokens for both themes, validated; no hard-coded colours left in
      `stats-modal.js`.
- [x] "Remove location…" asks and then removes, instead of throwing.
- [x] e2e spec for the shared dialog behaviour; the full suite green (282).
- [x] ADR-0033 and ADR-0034 in the arc42 index; CHANGELOG entry.
