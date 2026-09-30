# ADR-0041: The app explains its model where it is used

*Last modified: 2026-09-30*

## Status

Accepted.

## Context

Unterlumen has one model that is not obvious from its sidebar: one photo
folder; Folders shows it as it is on disk; a library catalogs a folder and
every folder inside it, however deep, without moving anything; Map and Timeline read all libraries; a gallery is a
set of library photos published to a destination. Why there are both Folders
and Libraries, and what galleries and destinations are, was written only in
the README and the ADRs. The owner asked on 2026-09-28 how the software could
explain itself and chose all three proposals: a sentence per place, a place
that explains the model, and the relations shown where they hold.

What rules out the usual answers:

- A guided tour or coach marks cover the screen, are skipped, and cannot be
  found again (ADR-0008: unobtrusive, as little design as possible).
- A tooltip is not seen on a phone or by keyboard, so it can never be the only
  explanation.
- A help dialog is something you read, so by ADR-0033 it is a place.

## Decision

1. **One sentence per place.** `placeLede(text)` in
   `src/web/js/place-lede.js` builds the sentence under a place's title. It
   says what the place is for and what it does to your files, and ends with a
   link to the guide. It is the last child of the place's head and takes a line
   of its own. Folders shows it at the top folder only; Marked for deletion and
   Settings have none, the first because it already explains itself.
2. **Places named in text are links.** `placeLink(mode, hash, label)` makes
   them; `App.initNav` routes every `a.place-link[data-mode]` from the
   document, so a link built later needs no listener of its own.
3. **A place `#guide`, "How Unterlumen works".** `GuidePane`
   (`guide-place.js`): a diagram of the model and a paragraph per term. The
   boxes that are places link there. It has no sidebar entry and no number key;
   it is reached from every sentence, from Settings and from About, and it is
   readable on a phone. It also maps the UI words to the names in the files
   (destination = channel, gallery = album), for people who read
   `channels.json`.
4. **Relations are shown where they hold.** A folder in Folders says which
   library catalogs it (the server's `detectLibrary`, which already knew) and
   links there; a library links its folder in Folders; a gallery names its
   destination. `App.initNav` routes `a.library-link` and `a.folder-link` too.
5. **The diagram is inline SVG in the interface's colours.** Places are framed
   by a hairline (`--line-strong`), what is not a place sits on a surface step
   (`--bg-2`), connectors are `--fg-3`. No signal colour except the focus ring;
   it reads in grayscale.

## Consequences

- A newcomer learns the model at the place where the question comes up, and
  finds the whole picture one click away.
- Every place's head is one line taller. The sentence is kept to two lines at
  64 characters.
- A new place gets a sentence as part of being built; the guide's paragraphs
  and diagram are updated when the model changes.
- The copy is in the app, so it is tested like the app
  (`e2e/specs/explain-the-model.spec.js`).
- On a phone (from 2026-09-30) the sentence costs the room the photos and the
  map need, so it folds behind a round "i" button beside the title
  (`aria-expanded`); tapping it shows the sentence, with the button left of
  the text. The Map puts its sentence into its folded Options instead, and
  the Libraries overview moves it from the top of the list into its head. The
  desk shows it as before.
