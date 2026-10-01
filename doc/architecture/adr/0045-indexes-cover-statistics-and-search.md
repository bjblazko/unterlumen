# ADR-0045: Indexes cover what the statistics and the search read

*Last modified: 2026-10-01*

## Status

Accepted.

## Context

On 2026-10-01 the installed app stopped answering while the owner clicked through the new Colour topic. A search by hue had run for minutes. Each library database has one connection (`SetMaxOpenConns(1)`), so that one query blocked every other request to the library, and the libraries list with it.

Measured on a copy of the owner's largest library (36,000 photos, 1.7 million EXIF rows, 680 MB):

- The statistics took 7.8 s, and 9.3 s scoped to a folder. A search with no criterion took 0.6 s.
- Almost all of it was reading table rows the query did not need. A photo's row carries its whole EXIF as JSON, 1.7 KB on average. Every query that checked a photo's `status` by its id, or read `date_taken` or `id` beside an index, fetched that row.
- `exif_index` had a rowid, so its `(photo_id, field)` key was a second structure as large as the table (186 MB), and its `(field, value)` and `(field, numeric_value)` indexes had no `photo_id`.
- A correlated `EXISTS` on `photo_palette` made SQLite search the swatches of a hue once for every photo.
- No library has `sqlite_stat1`; the planner chooses without statistics.

## Decision

- **`exif_index` is `WITHOUT ROWID`.** The table is its `(photo_id, field)` key, so a lookup by photo finds the value at once, and every secondary index carries `photo_id` without storing it a second time. Existing libraries are rebuilt once when they are opened. That takes about 11 s for the largest library, and the write-ahead log is checkpointed right after, or every read searches 500 MB of log until the next write.
- **Two covering indexes on `photos`:** `(status, date_taken, id)` and `(status, path_hint, date_taken, id)`. They replace `(status, path_hint)`. Counts by date, hour, month and folder, and the "photos that are ok" set, never read a row.
- **A set, not a join or a correlated subquery,** where a query asks "is this photo among those": `c.photo_id IN (SELECT id FROM photos WHERE status='ok' …)` for camera and lens, and `p.id IN (SELECT photo_id FROM photo_palette WHERE hue_bin=? AND share>=?)` for a hue. SQLite builds the set once from a covering index.
- **The search counts with `COUNT(*)`.** `COUNT(p.id)` read every matching row for the id.
- **No `ANALYZE` for now.** Covering indexes make the plans right without statistics; collected statistics would change plans across the app in ways nobody measured.
- A regression test checks the hue filter's query plan (`TestHueFilterIsNotCorrelated`). `probe_timing_test.go` (build tag `probe`) times every statistics and search query on a copy of a real library.

## Consequences

- On the largest library, the statistics take 0.4–1 s instead of 7.8–9.3 s. Every search except "aspect" answers within 0.03–0.4 s, and the camera-and-lens step within 0.1–0.4 s instead of 2–3 s.
- The file grows by about 66 MB for that library (EXIF index and table 536 MB instead of 470 MB). The space the rebuild frees inside the file is reused; nothing runs `VACUUM`.
- The first start after the update rebuilds the EXIF index of each library, one after the other. For the owner's six libraries that kept the app from answering for 86 s.
- The timeline (1.5–2 s) and the search by frame shape (1 s) still read every photo's JSON for its width and height. Columns of their own would fix that; this ADR leaves them.
- One connection per library remains. A slow query still blocks its library, so a new query must be measured on a real library before it ships.
