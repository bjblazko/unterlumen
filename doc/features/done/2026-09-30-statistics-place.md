# Statistics as a place, by topic

*Last modified: 2026-09-30*

## Summary

Statistics leaves its dialog and becomes a place in the sidebar under
Explore, with a sub-entry per topic, like the libraries under Libraries.
Clicking a value in a chart shows the photos it counts in a column beside the
charts, and the two stacked charts are lines. See
[ADR-0043](../../architecture/adr/0043-statistics-place-with-topics.md).

## Details

- `#statistics[/<topic>][?library=<id>&path=<folder>]`, key 9. Topics:
  Equipment, Exposure, Time, Frame. The overview shows a card per topic with
  a preview chart; with the sidebar collapsed, the cards are the way in.
- One registry (`stats-topics.js`) builds the place, the sidebar entries and
  the cards. A new topic (colour over time, more metadata, something in 3D)
  is one entry and its charts.
- The head holds the scope: library, folder ("Whole library" to widen it) and
  periods (auto, months, years) where a topic shows a development.
- A click on a mark turns into a search (`/api/library/search`), which learned
  `pathPrefix`, `hour` and `aspect`. The Map's photo column is now
  `PhotoColumn`, shared by both places, and reads further pages as it scrolls.
- Camera usage and aspect ratio are line charts with a legend, end labels up
  to four lines, and a hover rule with every value. The time axis includes
  periods without photos.
- Fixed on the way: across libraries, "Other" could show twice in camera
  usage, because each library cut its cameras to five before they were
  merged.

Out of scope, noted for later: drawing charts to their card's width rather
than at a fixed size; a tab of its own in the phone's tab bar.

## Acceptance Criteria

- [x] Statistics is a place in Explore with sub-entries for its topics; the
      sub-entries go with a collapsed sidebar and the overview's cards lead
      to the topics.
- [x] Every topic and scope has an address; back, reload and bookmarks work.
- [x] Today's charts are sorted into topics, distribution and development
      side by side; no Snapshot/Timeline tabs.
- [x] A library's Statistics button opens the place scoped to that library
      and folder.
- [x] Clicking a value shows its photos in a column; Escape and Done close it;
      marks can be reached and picked by keyboard.
- [x] Camera usage and aspect ratio are line charts with a legend and hover.
- [x] "Other" appears at most once in camera usage across libraries (Go test).
- [x] Search filters by folder, hour and frame shape (Go tests).
- [x] e2e: `statistics.spec.js` rewritten for the place; map and phone specs
      updated; full suite green.
- [x] ADR-0043, ADR-0033 updated, arc42, README, CHANGELOG.
