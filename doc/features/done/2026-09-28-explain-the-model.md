# Explain the model inside the app

*Last modified: 2026-09-28*

## Summary

Unterlumen's model — one photo folder; Folders shows it as it is on disk;
a library catalogs a folder and every folder inside it without moving anything; Map and Timeline read
across all libraries; a gallery is a set of library photos published together
to a destination — was written down only in the README and the ADRs. Inside
the app a newcomer met four sidebar groups and some terse empty states, one of
them still in the old vocabulary ("Add to channel…").

The app now explains itself where it is used, in three steps: a sentence under
each place's title, a place that explains the whole model, and the relations
between places shown where they hold. No tour, no coach marks, no tooltip as
the only explanation (ADR-0008, ADR-0033).

## Details

### Phase 1 — one sentence per place, honest empty states

- `placeLede(text)` (`src/web/js/place-lede.js`) builds the sentence; it is the
  last child of each place's head and takes a line of its own there. The
  library overview shows it at the top of its list.
- Places with a sentence: Folders (top folder only), Organize, Libraries, Map,
  Timeline, Galleries, Destinations. Marked for deletion already explains
  itself; Settings needs nothing.
- `placeLink(mode, hash, label)` makes a place named in running text a link;
  `App.initNav` routes every `.place-link` from the document.
- Empty states say what is missing and what to do next, with a link where a
  place is named. "Add to channel…" became "Add to gallery…".

### Phase 2 — the place "How Unterlumen works" (`#guide`)

A place with an address, reached from each sentence, from Settings and from
About. It shows the model as a diagram and explains each term in a paragraph,
shows with an example tree (Photos holding the libraries Projects, with one subfolder per
project, and Travel) that a library holds subfolders and that every library lies
inside the one photo folder, says what Unterlumen never does, and maps the UI words to the names in
`channels.json` (destination = channel, gallery = album).

### Phase 3 — relations shown where they hold

- Folders already asked the server which library catalogs the folder shown
  (`API.detectLibrary`) and showed its name as a mute chip. It now reads
  "In library …", and the name is a `libraryLink` to the library. The server
  returned only the first library that covered the folder; libraries may
  overlap, so `Manager.LibrariesForPath` returns them all and the endpoint
  answers `{"libraries": [...]}`.
- A library's head adds Open in Folders beside its path (`folderLink`,
  `App.openFolder`). Its path goes through `absPathRelativeToBoundary`; a
  folder outside the photo folder is said to be so instead of linking nowhere.
- A gallery's detail names its destination and its kind; the name opens it.
- A destination already linked its galleries (the count in its row), and
  Galleries already grouped them by destination with Destination settings.
- Left out: a hint in the Folders selection bar about Add to gallery. The
  "In library" link already leads to where it is.

## Acceptance Criteria

- [x] Every place listed above shows its sentence; a subfolder in Folders does not repeat it
- [x] A place named in a sentence is a link; it goes there, and Back returns
- [x] Empty states of Libraries, Galleries, Destinations and Timeline name the next step
- [x] No "channel" left in UI copy of the Galleries empty state
- [x] `#guide` opens directly, from every sentence, from Settings and from About; readable on a phone
- [x] The diagram reads in both themes and in grayscale
- [x] A cataloged folder names its library; a library opens its folder in Folders
- [x] A gallery names its destination; a destination lists its galleries
- [x] e2e: `e2e/specs/explain-the-model.spec.js`
