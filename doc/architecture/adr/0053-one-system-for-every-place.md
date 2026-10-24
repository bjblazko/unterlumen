# ADR-0053: One system for every place

*Last modified: 2026-10-24*

## Status

Accepted.

## Context

A review of every place on 2026-10-21 ([doc/ideas/2026-10-21-ux-review.md](../../ideas/2026-10-21-ux-review.md))
found that the places had grown apart. There were five kinds of page head,
from none at all (Folders) to a grey toolbar (a library) to a title row with
filters (Map). A library stacked three toolbars and called its top folder
"Root". One count stood up to four times, in three words. "Done" closed
panels and a place. One shared filter (ADR-0050) had two interfaces, and
the Timeline drew two bands that looked the same. Dates came in three
formats, a gallery's state in five words and the time colour, and a
library had six maintenance buttons that overlapped.

Each was a small decision taken locally. Together they broke principle 4
(the same thing is always in the same place) and principle 3 (one system,
consistently applied).

## Decision

**One head for every place** (`placeHeaderHTML`, `src/web/js/place-header.js`):
title and count on the left, the place's actions and ⋯ on the right, what
the place is below. A detail page's title is its way back and its name
(*Galleries / Rome*). Places that browse photos (`BrowsePane`) draw it
themselves, followed by one row with where you are (*Libraries / Pictures /
2017*), how the photos are shown and Select all. A library has no toolbar
of its own; Statistics, Edit library… and Open in Folders are in its ⋯.

**A count once.** The total is in the title's meta, always "photos" ("files"
only where folders are counted with them); beside Select all only "N
selected". The selection bar carries actions only.

**A tool strip at the right edge, as in IntelliJ.** Every place with panels
has a narrow strip at its right edge (`toolStrip`, `panelToggleButton`,
`src/web/js/panel-icon.js`): one toggle per panel — Filter, Info, Photos —
with its glyph, its word and a count where it helps. The panel opens beside
the strip, and strip and panels run the place's whole height. One panel is
open at a time. Heads hold actions only; toggles sat there first among
Slideshow and ⋯, which read as one more action, and tabs inside the column
made the column's own head a second way in.

**Panels hide, places do not close.** A panel's head names it and has one
framed button with the sidebar's panel glyph, mirrored (`panelHideButtonHTML`).
A place has no Done; you leave it by going elsewhere.

**A count looks the same everywhere** (`countHTML`, `src/web/js/activity.js`):
the number in a pill, then the word; thousands grouped with a narrow
no-break space (`formatCount`). The pill marks the count of a place or a
control — the head, a toggle, a link to what waits — so there are few on a
screen. In a collection (a folder's tiles, the libraries' cards, the people
cards) a count is data, plain in mono (`countText`): a pill in every tile
made eleven equal signals of one grid, and indented the line it opened.
Decided with the owner on 2026-10-23.

**One filter interface.** Map, Timeline and Statistics use the library's kind
of filter: the conditions as chips in the head, each dropping its own, the
Filter toggle in the strip, and the column to set them. The
Timeline takes its span from the filter's Taken and draws one band.

**Dates are ISO**, to the second, without fractions or zone (`formatDateTime`,
`formatDay`, `src/web/js/dates.js`); the zone stays in All metadata.

**A gallery is published or not.** "Not published" or "Published
2026-10-21", with a note where something is left: "built, upload missing"
in the warning colour. The time colour means time passing and nothing else.

**Rare tools are few.** A library's maintenance is Scan for new photos and
Read everything again…; the cache is cleared in Settings only; the viewer
keeps Back, the zoom, Crop, Mark for deletion and ⋯ in view, and the film
strip, full screen and the download in ⋯.

## Consequences

- A new place starts from `placeHeaderHTML`; a new panel from
  `panelHideButtonHTML`; a new date from `dates.js`. CLAUDE.md says so.
- The Timeline's own range control (`TimelineRange`) is gone; its job is
  the shared filter's Taken. On a phone the Timeline still shows every day.
- Folders shows a library's four photos for the folders a library covers;
  `/api/library/detect` says where in the library a folder is.
- `/api/cache/evict` is gone with the menu entry that used it.
- e2e specs that looked for the old heads, counts and words were rewritten.

## Addendum 2026-10-24: the head on a phone, and where the app starts

On a phone the head is one row: title and count, the place's one setting,
the round "i" that opens its sentence, its actions and ⋯. The setting is
still `segmentField`, drawn there as a small select named by the setting —
the segment and its label took a row of their own, and with the search bar
and the tab bar the photos had half the screen. The count gives way before
the name. An opened sentence takes its own row.

A bare address starts in Libraries, on the desk as on a phone, and the
phone's tab bar carries Statistics where it carried Folders: photos are
found in libraries, and Folders is reached through its links.
