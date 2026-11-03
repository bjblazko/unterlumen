# ADR-0063: Readers beside the writer, and Statistics read ahead

*Last modified: 2026-11-03*

## Status

Accepted. Amends [ADR-0045](0045-indexes-cover-statistics-and-search.md) (one connection per library) and [ADR-0062](0062-statistics-tallies-read-by-their-topic.md) (the tallies' queries).

## Context

On 2026-11-03 Statistics were measured on copies of the owner's six libraries (49,500 photos, the largest 33,000). A build with a stopwatch on every request was clicked through every topic, cold and warm. Warm, every topic took 1.5 s or less. Cold, the first visit took this long:

- The overview took 9 s. Its ten requests ran one after another on each library's only connection. `statistics` and `timeline` each took about 2 s on their own and 8 s in the queue.
- Equipment took 8.2 s, Things 13.0 s, Colour 5.6 s and Places 4.3 s.

Four causes:

1. **One connection per library** (ADR-0045). Requests that only read waited for each other and for a scan.
2. **Tallies that tested `p.id IN (ok photos)`** on a join with `photos p`. SQLite then fetched every photo's row, which carries 1.7 KB of EXIF JSON. Filtering `p.status` itself walks the covering index `(status, date_taken, id)`. For one tally that is 0.08 s instead of 3 s.
3. **Width, height and place read from the JSON.** Frame, Places, the map, the timeline's sizes and the search by shape parsed every photo's JSON, 1–2 s per view.
4. **The meaning cloud's principal axes.** Power iteration over all 33,000 vectors of 512 dimensions took 3 s, and reading the vectors took another 1–1.6 s.

There were also two smaller costs. People and Things statistics were never cached, because naming a face changes them outside a scan. The shared filter asked for its choices again on every topic change.

## Decision

- **Readers beside the writer.** Each library gets a pool of three read-only connections next to its writer (`read_pool.go`).
  - The pool is opened after the writer, so the database is already migrated. It sets `query_only`, `busy_timeout`, a 16 MB page cache each and `mmap_size` 2 GB.
  - `OpenFiltered` (Statistics, Map, Timeline), the list of libraries (`readLibrary`) and the filter's choices go through `OpenReader`.
  - Everything that writes keeps `OpenStore`, and a write through a reader is an error.
  - WAL lets readers run beside each other and beside the writer. A scan no longer holds up a page that only reads.
  - With mmap the readers share the operating system's pages. A fresh reader read `exif_index` for 1.4 s before its own cache held it; mapped, it takes 0.07 s.
  - The index lives in `-lib-dir` on a local disk, never on a share, which is where a mapped file is safe. Idle readers close after 10 minutes.
- **Tallies filter `photos p` directly.** `okIn(sc)` is `p.status = 'ok'` plus the scope fragment `sc.p`, the folder and the shared filter qualified with `p.`.
  - `inScope` remains for queries that start from another table.
  - `TestInsightTalliesFindPhotosByStatus` checks every tally's plan: no correlated subquery, and without a filter, `photos` is never looked up by id.
  - Cropped's largest-frame-per-camera subquery uses `CROSS JOIN`, so it walks the size index. The planner chose the lookup by id on the real library but not on the fixtures.
- **Width, height, lat and lon are columns of `photos`** (`store_columns.go`).
  - Triggers fill them from `exif_json` on every insert and every change of `exif_json`, so no writer has to remember to.
  - On the first open, one transaction fills existing rows. It takes the place of photos indexed before coordinates were parsed from their GPS tags, so the map no longer needs its fallback to the tags.
  - Two covering indexes: `(status, width, height, date_taken, id)` and `(status, lat, lon, date_taken, id)`.
  - `library_props.photo_columns` records that the fill is done.
- **Principal axes from a sample.** Orthogonal iteration refines the three axes together on 4,096 vectors taken evenly, and every vector is then placed along them.
  - On the real library, the places correlate with the previous result by 0.998–0.9995, at 0.39 s instead of 2.56 s.
  - Each axis is turned so that its largest component is positive, so the view does not flip when a photo is added.
- **A change counter for faces and things.** Triggers on `faces`, `people` and `photo_things` count every change in `index_changes`.
  - The People and Things statistics and the `people` and `things` tally groups are cached with the counter beside the value, and read again when it moved.
  - A naming, a hiding, a pass that found faces or a decision from the other installation shows at once, whichever code path made it.
  - This fixes a stale cache in 0.24.0: the `people` tallies were cached but not emptied by a naming.
- **The filter's choices are cached** (`scopeValuesCache`, emptied with the statistics). Statistics asks for them again only when the place is entered from elsewhere or the libraries or cameras change (`ScopeFilter.sync({ reload })`, `App.previousMode`).
- **A group's tallies run side by side** on the library's readers (`tallyAll`).
- **Statistics are read ahead.**
  - When a scan's follow-up work ends (appearance, faces, things), the overview, every topic and both point clouds are worked out for all libraries and for that library (`prewarm.go`).
  - The same happens once after start-up work (`PrewarmAllStatistics`).
  - One read-ahead runs at a time. Libraries that finish together are read once.

## Consequences

- Measured on the copies, all libraries:
  - After a restart and its read-ahead, the overview opens in 1.1 s instead of 9 s, and every topic within 0.4 s, cold or warm.
  - Right after the very first start, while the columns are filled and the read-ahead runs, the overview took 7.6 s. Equipment then took 1.6 s and Colour 3.1 s.
- Per library (Chronologisch, store level):
  - Frame takes 0.45 s instead of 2.9 s, Places 0.22 s instead of 0.86 s, and the map's points 0.16 s instead of 1.6 s.
  - The search by shape takes 0.04 s instead of 1.2 s, and the timeline 0.56 s instead of 1.7 s.
- After the same click-through, the process held 161 MB instead of 409 MB: smaller page caches, and mapped pages are shared.
- The first open after the update fills the new columns of each library once, in about 2.4 s for the largest library with the sqlite3 CLI. Under the Go driver and on the NAS's disk it takes several times that.
- The read-ahead costs CPU after every scan that changed something. It loads the photo vectors of every library for the meaning cloud, about 64 MB for the largest.
- A query that writes must not go through `OpenFiltered` or `OpenReader`.
- Still open: the writer keeps its 64 MB page cache though most reading has left it.
