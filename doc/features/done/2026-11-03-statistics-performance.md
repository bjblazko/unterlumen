# Statistics: quick on the first visit

*Last modified: 2026-11-03*

## Summary

Warm, Statistics already answered quickly. Their first visit after a start or a scan did not: on the owner's six libraries (49,500 photos) the overview took 9 s, Equipment 8 s and Things 13 s. The measurement, the causes and the decisions are in [ADR-0063](../../architecture/adr/0063-readers-beside-the-writer-and-statistics-read-ahead.md). In short:
- Each library got read-only connections beside its writer.
- The counts read their photos from indexes.
- Width, height and place became columns.
- The meaning cloud's axes come from a sample.
- Statistics are worked out in the background before anybody asks.

## Details

- **Measured on copies of the real libraries.**
  - `sqlite3 .backup` copies of every library, with `source_path` pointing nowhere, so no scan and no sidecar write.
  - A build with a stopwatch on every request.
  - A Playwright click-through of the overview and every topic: cold and warm, for all libraries and for the largest alone.
  - `probe_insights_test.go` (build tag `probe`) times each tally on one copy.
  - The routine is to be run again whenever statistics, index or search queries change.
- **Readers** (`read_pool.go`).
  - Three per library: `query_only`, a 16 MB page cache each, the file mapped into memory, closed after 10 idle minutes.
  - `OpenFiltered` (Statistics, Map, Timeline), the list of libraries and the filter's choices use them.
- **Tallies on `photos p`** filter `p.status` and the scope's `p.`-qualified fragment (`okIn`), not `p.id IN (ok photos)`. A group's tallies run side by side.
- **Columns** `width`, `height`, `lat` and `lon` on `photos`.
  - Triggers fill them from `exif_json`. They were filled once for existing photos, from the GPS tags where an old index had no parsed coordinates.
  - Two covering indexes go with them.
- **Principal axes** come from 4,096 vectors taken evenly, by orthogonal iteration. Each axis is turned so that its largest component is positive.
- **Caches.**
  - People and Things statistics and their tallies are cached with a change counter that triggers on faces, people and things keep.
  - The filter's choices are cached, and are not asked for again when only the topic changes.
- **Read ahead** after a scan's follow-up work and once after start-up work, one run at a time.

## Acceptance Criteria

- [x] On the copies, after a restart and its read-ahead, the overview of all libraries opens within about 1 s (1.1 s, before 9 s) and every topic within 0.5 s (0.1–0.4 s, before up to 13 s).
- [x] Right after the first start, while the columns are filled and the read-ahead runs, the overview opens faster than before (7.6 s).
- [x] A page that only reads does not wait for a write in progress (`TestReadersReadBesideAWriteAndCannotWrite`), and a reader cannot write.
- [x] No tally reads photos by id without a filter, and none is correlated (`TestInsightTalliesFindPhotosByStatus`; it fails 28 times with the old form).
- [x] The columns follow the JSON on insert and on change, and an old index is filled once (`TestPhotoColumnsFollowTheJSON`, `TestGeoPointsKeepsOnlyUsableLocations`).
- [x] A renamed person or a changed thing shows in the cached People and Things statistics at once (`TestStatisticsSeeANamingAtOnce`; it fails without the counter).
- [x] The axes from a sample place the real library's photos as all of them did (correlation 0.998–0.9995), in 0.39 s instead of 2.56 s (`TestPrincipalAxesFromASample`).
- [x] After a scan's work the overview, the topics' tallies and the filter's choices are cached (`TestPrewarmLeavesTheOverviewCached`).
- [x] Another topic does not ask for the filter's choices again; entering Statistics from another place does (e2e `statistics.spec.js`).
- [x] Memory after the same click-through: 161 MB instead of 409 MB.
