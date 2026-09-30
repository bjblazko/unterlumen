# ADR-0043: Statistics is a place with topics

*Last modified: 2026-09-30*

## Status

Accepted. Supersedes the Statistics verdict in [ADR-0033](0033-dialogs-and-places.md).

## Context

ADR-0033 kept Statistics a dialog "by decision", against its own rule 3: you read and compare in it, and it always belonged to the library it was opened from. It said Statistics would become a place once it got a second entry point. Map (ADR-0039) and Timeline (ADR-0040) have since become places in the sidebar, although they too show only library photos. On 2026-09-30 the owner asked for Statistics to become a place as well, and for three more things:

- **Room to grow.** More is planned: statistics over time (such as how the average colour develops), statistics from other metadata, possibly something in three dimensions. Two tabs, Snapshot and Timeline, will not hold that.
- **Click through.** Clicking a value in a chart should show the photos it counts, as the Map shows the photos of a group.
- **Readable developments.** "Camera usage" (stacked bars) and "Aspect ratio mix" (stacked areas) hid one series behind another; they should be lines.

The owner chose among the options offered:

- topics as sidebar sub-entries, like the libraries under Libraries. With the sidebar collapsed the sub-entries go, and the overview's cards lead to the topics, just as the library overview's filmstrips lead to the libraries;
- grouping by topic, with time as a dimension inside each topic, rather than "Snapshot" and "Over time" side by side;
- the photos in a column inside Statistics, rather than a jump to the Libraries filter;
- only the two stacked charts redrawn.

## Decision

- **A place, `#statistics`, in Explore below Timeline, number key 9.** Its address carries a topic and a scope: `#statistics[/<topic>][?library=<id>&path=<folder>]`. The folder is relative to the library, never absolute, so the same address works on the NAS and on the Mac (CLAUDE.md, "two views of the same photos"). The place makes the absolute `pathPrefix` the API takes from the library's `sourcePath`, as the dialog did.
- **One registry, `STATS_TOPICS` (`stats-topics.js`).** Each topic has an id, a name, a sentence, a preview chart and its charts. Each chart names its source (`snap`: `/api/library/statistics`, `tl`: `/api/library/timeline`) and turns the value a click hit into a criterion `{ subject, params }`. The place, the sidebar sub-entries and the overview cards are all built from it, so a new topic is one entry and its charts. Today's 14 charts are sorted into four topics: Equipment (camera and lens, camera usage, film simulation, resolution, format), Exposure (focal length, aperture, ISO, and each over time), Time (time of day, calendar) and Frame (aspect ratio). Colour, when it comes, is a fifth.
- **The scope in the head.** A library select (all libraries, or one), a folder with "Whole library" when the place was opened from a folder, and "Periods" (auto, months, years) on topics with a development over time. A library's Statistics button, and on a phone its ⋯ entry, open the place scoped to that library and the folder shown. The Libraries overview's button is gone: the sidebar covers it.
- **The photo column is shared with the Map.** `MapPhotos` becomes `PhotoColumn` (`photo-column.js`). It gains a subject line above the count and paging: the Map passes all its photos at once, while Statistics reads 200 at a time from `/api/library/search` as the column scrolls. Both open photos read-only through `openLibraryPhotos`. A new topic or scope closes the column; Escape closes it too.
- **Search learns the statistics' own filters.** `pathPrefix` (the folder scope), `hour` (0–23) and `aspect` (a frame shape). Frame shapes are defined once, `aspectClassSQL`, and the timeline counts by that definition while the search filters by it, so a clicked shape finds the photos it counted. Everything else a chart counts was already a search filter: camera, lens, film simulation, format, the numeric ranges and the date. A camera cell "(no lens)" cannot be searched as such, so it shows all of that camera's photos and says only the camera's name.
- **Lines for developments.** `renderSeriesLines` draws several series over the same periods: 2 px lines, a legend always, the name at each line's end when there are four or fewer, and a rule, dots and a tooltip with every value of the period under the pointer. Arrow keys move it and Enter picks the period. Camera usage counts photos; aspect ratio shows each shape's share of the period, with a gap where a period has no photos.
- **Periods without photos are on the axis.** The server lists only periods that have photos; the place fills in the ones between (`continuousTimeline`), so three empty years no longer look as long as one.
- **Cameras are cut to five after merging, not before.** Each library used to cut its camera list to five plus "Other", and merging libraries then ranked that "Other" as a camera, so it could appear twice. A library now hands over every camera, and `topCameras` cuts the answer as a whole.
- **Charts grow by at most a third.** They are drawn at a fixed size; scaled to a wide card their 9–11 px labels grew past the page's text. `svgBase` caps them at 1.3× and lets them shrink.
- **Every clickable mark is keyboard-reachable.** `pickable` gives a mark `role="button"`, a tab stop, a name ("X-T50, 412 photos") and Enter/Space. The focus ring is the app's.
- **Own stylesheet.** `css/statistics.css`, as the Timeline has `css/timeline.css`.

## Consequences

- ADR-0033's table now lists Statistics as a place. Its "strains the rule" paragraph is history.
- A calendar year has one tab stop per day with photos. That is many for a busy year, but each is a real way into the photos; a roving tab index can come later if it bothers someone.
- Search counts a photo in two libraries twice, as the statistics do. The Map and Timeline show it once. The column's count agrees with the chart it came from, which matters more here.
- The overview reads the timeline as well as the snapshot, for the Frame card. The server caches both.
- On a phone Statistics has no tab of its own; it is reached from a library's ⋯ menu, and the library select there reaches all libraries.
- The charts still draw at fixed sizes. Drawing each to its card's width is the next step if the fixed sizes start to hurt, especially on a phone.
- Ideas for further statistics, 3D views with WebGPU among them, are collected in [doc/ideas/statistics.md](../../ideas/statistics.md).
