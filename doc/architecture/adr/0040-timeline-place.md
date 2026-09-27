# ADR-0040: The Timeline is a place, laid out in the browser from a skeleton

*Last modified: 2026-09-27*

## Status

Proposed.

## Context

The owner asked on 2026-09-27 for a place, next to the Map, that shows every library photo on one time axis and ignores folders and libraries. On a desk, the axis sits along the bottom: oldest on the left, newest on the right, labelled with years and months, with a graph of how many photos were taken behind it. A window marker slides along the axis, the axis can be limited to a range, and a multi-row justified photo lane runs above it. It has to load lazily so the browser's memory stays bounded. On a phone it may be portrait and simpler.

A clickdummy (https://claude.ai/artifact/N2RjBab31exPDAy4A2Xxh5) offered three desktop designs (A Band: vertical lane; B Horizontal: horizontal lane; C Dense: compressed axis, a point marker) and two phone designs (1 Scrubber, 2 Strip). The owner chose **B** and **Mobile 1**. Asked about the details, they chose:

- window and lane coupled both ways;
- undated photos left out, with a count;
- the same photo shown once;
- a click shows the info panel and a double-click opens the viewer, with no multi-selection;
- the number of rows (2/3/4) in the ⋯ menu;
- no range limit on the phone.

What was there already:

- every library's index has `date_taken` (indexed) and `width`/`height` in `exif_json`;
- `Manager.GeoPoints` shows the "every library, skip a broken one" loop;
- `RangeSlider`, the `--chart-seq-*` ramp (ADR-0034), `Activity` (ADR-0036), `Menu`;
- the read-only viewer that the Map opens with its own list (ADR-0039).

## Decision

- **A skeleton for the layout, details for the view.** The horizontal band needs every photo's position before it can size its scroll width, place the frame or jump to a date. Only the day and the aspect ratio feed that. `GET /api/timeline` returns the skeleton for all dated photos, sorted by date taken:
  - `version`;
  - `days` (days since the first photo);
  - `ratios` (width ÷ height, display orientation, 3:2 when unknown);
  - `undated`.

  `GET /api/timeline/photos?v=&from=&count=` returns the details for an index range, at most 500 per request: photo ID, library ID, filename, date as ISO text. A stale `v` answers `409`. The browser keeps the skeleton in typed arrays (6 bytes a photo) and lays out, virtualises and draws the graph from it. The alternative, cursor paging by date, cannot know the scroll width or where a date lies without loading everything in between.
- **One photo, once.** The skeleton is merged across libraries; a photo ID (SHA-256) seen in an earlier library is skipped. Which library's copy wins follows the sidebar's library order.
- **Undated photos are left out and counted.** Placing them by file date would put them at an invented time.
- **Version from the libraries themselves.** The version is a hash of each library's photo count and latest `indexed_at`. It is cheap to read on every request and needs no hook from `library` into the new package. The skeleton is cached per version.
- **Own packages, imports in one direction** (in the spirit of ADR-0037). `internal/timeline` builds and caches the stream from `library.Manager`. `internal/api/timeline` holds the two handlers. `library` gains one store query for the dated photos and does not import either. The CSS lives in its own `css/timeline.css`: `style.css` is past 7 000 lines.
- **Desktop: horizontal band, two-layer time bar.** Tiles of equal height in 2–4 rows. Each photo goes into the row that ends furthest left, and a month starts a new column with its label. Below, an overview with bracket handles limits the range, and the axis shows that range with a step graph (photos per day within each bin) and the frame. Frame and band drive each other. While the frame is dragged, the band's reports do not move it.
- **Phone: vertical list, newest first, a scrubber on the right edge.** There is no limit: the scrubber shows the whole span.
- **Viewer: read-only, over a window of the stream.** The viewer takes a list of keys. The place passes the 1 000 photos around the one opened, loading their details first, rather than 100 000 keys.
- **Navigation.** "Timeline" in Explore below Map, `#mode-timeline`, `#timeline`, number key 8. The existing keys stay (ADR-0028).

## Consequences

- A photo appears only once its library is indexed, as on the Map.
- The skeleton grows with the library, by about 6 bytes a photo in memory and a few bytes on the wire. A million photos would be the point to page the skeleton by year.
- The graph's scale is linear and relative to the range shown, so a burst day can flatten quiet years. That is honest, and the absence of numbers keeps it a shape.
- Two copies of a photo in different libraries show once. The library named in the info panel is the first in sidebar order.
