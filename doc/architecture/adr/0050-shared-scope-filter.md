# ADR-0050: One filter for the Map, the Statistics and the Timeline

*Last modified: 2026-10-02*

## Status

Accepted. Builds on [ADR-0039](0039-map-place.md) (the Map, whose span of
months it takes over), [ADR-0040](0040-timeline-place.md) and
[ADR-0043](0043-statistics-place-with-topics.md).

## Context

The Map had its own span of months, the Statistics a library select, the
Timeline nothing. The owner asked on 2026-10-02 for one filter component in
the head of each of these places that narrows the whole place: a span of
time, cameras and lenses, all optional, with a reset — and on a phone either
not there or folded away.

Decided with the owner: the Map, the Statistics and the Timeline; one state
shared by the three; the lenses offered follow the cameras chosen; the
library select joins the filter as its fourth part. Every part is optional
and nothing is filtered at first: every library, the whole time, every
camera and lens.

## Decision

1. **One component, `ScopeFilter`** (`scope-filter.js`), mounted in each
   place's head: Libraries, Taken (a span of months, the Map's former
   `MapTimeRange`), Cameras and Lenses, and Reset, shown only when something
   is narrowed. Libraries, cameras and lenses are a `MultiSelect`
   (`multi-select.js`): a button naming the choice ("Cameras: all", "Cameras:
   X-T50", "Cameras: 3 of 12") that opens a list of checkboxes, every one on
   at first. Unchecking narrows; "All" turns all on again; all off is "none",
   which matches no photo rather than silently all.

2. **One state, `ScopeState`**, shared by the three places and kept in the
   browser (`localStorage`), so a place shows what the others show, also after
   a reload. A choice of all is `null`, so a camera that appears later is in
   it too.

3. **The Timeline takes no months.** Time is its axis, and its own span
   chooses within it; libraries, cameras and lenses narrow it.

4. **On a phone** the filter folds behind a Filter button in the head, which
   counts what is narrowed; open, it takes a line of its own.

5. **The server filters.** The statistics, colour, 3D-view, timeline and
   map endpoints take `ids`, `month_from` and `month_until` (`YYYY-MM`,
   inclusive) and repeated `model` and `lens` values, as `exif_index` stores
   them. `library.Filter` turns them into one condition per narrowed part —
   "photo ID in a set", each set collected once through an index, never per
   row. A store opened with a filter (`Manager.OpenFiltered`) adds it in the
   three places every statistics query takes its photos from (`statsScope`,
   `photoCond`, `aliasCond`), so the queries themselves are unchanged and run
   exactly as before without a filter. Caches take the filter into their key;
   the Timeline's stream takes it into its version.

6. **The photo column searches within the filter.** `/api/library/search`
   takes the same parameters, so a value clicked in a chart shows its photos
   within the filter.

7. **`/api/library/scope-values`** answers what there is to choose from in the
   chosen libraries: cameras and lenses with their photo counts (lenses of the
   chosen cameras only) and the first and last month of the photos.

## Consequences

- A library's Statistics button sets the filter's libraries to that library,
  which the Map and the Timeline then show too; Reset widens all three.
- The Statistics' folder scope stays where it was, beside the filter; choosing
  other libraries leaves the folder.
- Measured on a copy of a 36,000-photo library: every filtered query is as
  fast as or faster than the same query without a filter (the Timeline 0.9 s
  instead of 3.9 s); the map's points come filtered from the server.
- A new statistics query must take its photos through `statsScope`,
  `photoCond` or `aliasCond`, or the filter does not reach it.
- Undated photos are outside any span of months; without one, cameras and
  lenses still narrow them.
