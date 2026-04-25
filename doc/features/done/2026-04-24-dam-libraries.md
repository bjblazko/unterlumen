*Last modified: 2026-04-24*

# Feature: DAM Libraries

## Summary

An optional Digital Asset Management layer that allows users to designate a photo folder as a **library** — a named, indexed collection whose metadata, thumbnails, and user-defined tags live in `~/.unterlumen/libraries/<uuid>/`. Libraries are browsed via a new **Libraries** tab in the UI. Unlike the browser mode, libraries support full-text and faceted metadata search and extensible per-photo key/value annotations.

---

## Motivation

Browse mode is ephemeral: no persistent metadata, no search, no annotations. Libraries fill this gap for users who want to manage a curated set of folders long-term — rating, tagging, tracking publication status — without touching the originals and without depending on an external database.

---

## Storage Architecture

### Why SQLite (`modernc.org/sqlite`)

| Requirement | SQLite fit |
|---|---|
| No external process | ✅ embedded |
| Pure Go, no CGo | ✅ `modernc.org/sqlite` (Wasm-compiled, works macOS/Linux/Windows) |
| Queryable metadata search | ✅ SQL |
| Extensible schema | ✅ EAV table + JSON columns |
| Filesystem-resident | ✅ single `.db` file |
| Migration path | ✅ SQL dialect close to Postgres; `database/sql` driver swap |
| No concurrency concern now | ✅ WAL mode, single writer |

Each library gets `~/.unterlumen/libraries/<uuid>/library.db`. The directory is self-contained: copy or delete the directory to clone or drop a library. No central registry beyond a `MEMORY.md`-style `libraries.json` index at `~/.unterlumen/libraries.json`.

### Schema (initial)

```sql
-- Core photo record
CREATE TABLE photos (
    id          TEXT PRIMARY KEY,   -- SHA-256 of file content (hex)
    path_hint   TEXT NOT NULL,      -- last known absolute path (informational)
    filename    TEXT NOT NULL,      -- last known filename
    file_size   INTEGER NOT NULL,
    indexed_at  DATETIME NOT NULL,
    exif_json   TEXT,               -- full ExifData serialised as JSON blob (for display)
    thumb_path  TEXT,               -- relative path within library dir
    status      TEXT NOT NULL DEFAULT 'ok'  -- 'ok' | 'missing'
);

-- Inode fast-lookup cache (avoids rehashing on every scan)
CREATE TABLE path_cache (
    abs_path    TEXT PRIMARY KEY,
    photo_id    TEXT NOT NULL REFERENCES photos(id),
    inode       INTEGER,
    mtime       INTEGER,            -- Unix nano
    file_size   INTEGER
);

-- EXIF fields indexed for search (EAV, same pattern as photo_meta)
CREATE TABLE exif_index (
    photo_id    TEXT NOT NULL REFERENCES photos(id),
    field       TEXT NOT NULL,
    value       TEXT NOT NULL,
    PRIMARY KEY (photo_id, field)
);
CREATE INDEX exif_index_field_value ON exif_index(field, value);

-- Extensible user-defined metadata (EAV)
CREATE TABLE photo_meta (
    photo_id    TEXT NOT NULL REFERENCES photos(id),
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    updated_at  DATETIME NOT NULL,
    PRIMARY KEY (photo_id, key)
);

-- Library-level key/value properties
CREATE TABLE library_props (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL
);
-- populated at creation: name, description, source_path, created_at, schema_version
```

### Photo Identity and Rename Resilience

Problem: if a user renames `/photos/IMG_1234.jpg` to `/photos/paris.jpg`, we must keep associating existing metadata with that file.

**Two-level identity**:

1. **Fast path** — `path_cache` stores `(abs_path, inode, mtime, file_size)`. On re-scan, if all four match, the record is considered unchanged; no rehash.
2. **Canonical identity** — SHA-256 of the full file content (`id` in `photos`). If the fast path misses (file moved or mtime changed), compute the hash and look up `photos.id`. If found, update `path_hint` and `path_cache`; metadata survives the rename.

For large RAW files, full SHA-256 is the correct choice despite the cost: a library scan is a background operation, and correctness matters more than speed here. A progress indicator in the UI covers the UX.

### Thumbnail Storage

```
~/.unterlumen/libraries/<uuid>/
  library.db
  meta.json              ← name, description, source_path, schema_version
  thumbs/
    ab/
      abcdef1234…jpg     ← keyed by SHA-256 prefix shard (first 2 chars)
    …
```

HQ thumbnails (1200px long edge, JPEG 85) are generated once and cached by content hash. Sharding by first two hex chars keeps directory entry count manageable for large libraries (~256 subdirs, ~390 files each for 100 k photos).

### Base Directory Parameterisation

For testing and CI, the library root (`~/.unterlumen`) must be overridable:

- CLI flag: `--lib-dir <path>`  
- Env var: `UNTERLUMEN_LIB_DIR`  
- Default: `os.UserHomeDir() + "/.unterlumen"`

E2E tests spin up with a `UNTERLUMEN_LIB_DIR` pointing to a temp dir, same pattern as `UNTERLUMEN_ROOT_PATH`.

---

## User-Defined Metadata

### EAV Design

`photo_meta(photo_id, key, value)` stores arbitrary string key/value pairs per photo. No fixed columns.

Example rows:
```
("sha256abc…", "published_website", "2026-03-15")
("sha256abc…", "posted_instagram", "true")
("sha256abc…", "print_order", "A3")
("sha256abc…", "rating", "5")
```

Keys are user-defined strings. The UI presents them as a tag/annotation panel alongside EXIF. Future: a library-level key registry (`meta_keys` table) can store label, type hint, and display order — not needed in v1.

### EXIF Search

EXIF fields are stored in a dedicated `exif_index(photo_id, field, value)` EAV table — the same pattern as `photo_meta`. This keeps the schema uniform, works on any SQLite version, and makes both EXIF and user meta searchable with identical query patterns. The full EXIF blob is also kept in `photos.exif_json` for the info panel display.

---

## UI

### Libraries Tab (4th tab)

- Lists all known libraries (name, source path, photo count, last indexed).  
- **Create library**: name + folder picker → triggers background indexing.  
- **Open library**: loads the library grid (same justified layout as browse).  
- **Library header**: name, description, re-index button, status (indexed N of M).

### Library Grid

- Same grid component as the browse tab.
- Photos are served via a new `/api/library/<uuid>/photo/<id>` and `/api/library/<uuid>/thumb/<id>` route.
- Clicking a photo opens the existing single-image viewer.

### Search & Filter Panel

- A collapsible panel above the grid (or a drawer).
- **Free-text**: searches across all EXIF fields and user meta values.
- **Faceted filters**: lens, focal length range, aperture, ISO range, camera, date range, film simulation, custom keys.
- Filters compose with AND; results update the grid live.

### Annotation Panel

- Extension of the existing info panel.
- Shows all `photo_meta` rows for the selected photo.
- Inline editing: click a value to edit, press Enter to save, "+" to add a new key.

---

## API Routes (new, under `/api/library/`)

```
GET    /api/library/                        — list all libraries
POST   /api/library/                        — create library (name, source_path)
GET    /api/library/:id                     — library info + stats
DELETE /api/library/:id                     — delete library (data only, not photos)
POST   /api/library/:id/index               — trigger re-index (async, SSE progress)
GET    /api/library/:id/photos              — paginated photo list (+ search params)
GET    /api/library/:id/thumb/:photoID      — serve HQ thumbnail
GET    /api/library/:id/photo/:photoID      — serve original photo
GET    /api/library/:id/photo/:photoID/meta — get all user meta for a photo
PUT    /api/library/:id/photo/:photoID/meta — upsert a key/value pair
DELETE /api/library/:id/photo/:photoID/meta?key=:key
```

### Server Mode

Library routes are available in both local and server modes. In server mode, the `--lib-dir` path should be passed explicitly. Future multi-user support would require per-user library isolation — the current single-library-dir model is designed to make that refactor obvious.

---

## Indexing Process

1. Walk source directory recursively; collect all supported media files.
2. For each file: check `path_cache`. If fast-path hit → skip rehash.
3. If miss: compute SHA-256. Look up `photos`. If found → update path_hint. If not → insert new photo record.
4. Extract EXIF (reuse `media.ExtractAllEXIF`), store full blob in `photos.exif_json`, and upsert all fields into `exif_index`.
5. Generate HQ thumbnail if not already present in `thumbs/`.
6. After full walk: mark any `photos` records whose `path_hint` no longer exists as `status = 'missing'` (do not delete).
7. Emit SSE progress events (files processed / total).

## Design Decisions

| # | Question | Decision |
|---|---|---|
| 1 | Re-index trigger | Manual only — user clicks re-index button |
| 2 | EXIF search storage | EAV table (`exif_index`) — same pattern as user meta |
| 3 | Custom key scope | Per-library only |
| 4 | Metadata value types | Freeform strings |
| 5 | Source folder scope | Single root folder, indexed recursively |
| 6 | Missing photos | Mark `status='missing'`, preserve metadata and thumbnail |

---

## Acceptance Criteria

- [x] `--lib-dir` / `UNTERLUMEN_LIB_DIR` controls the base directory; defaults to `~/.unterlumen`
- [x] `POST /api/library/` creates a library directory and `library.db` with correct schema
- [x] Indexing walks the source folder, computes SHA-256 per file, stores photo records and EXIF JSON
- [x] HQ thumbnails are generated and stored in `thumbs/` keyed by content hash
- [x] Re-scanning a folder where files were renamed re-associates existing metadata via hash match
- [x] `GET /api/library/:id/photos?q=...` returns filtered photo list by EXIF or user meta fields
- [x] `PUT /api/library/:id/photo/:photoID/meta` persists a key/value pair; survives server restart
- [x] Libraries tab lists all libraries; opening one shows the photo grid
- [x] Annotation panel shows and allows editing of user-defined key/value pairs
- [ ] E2E spec covers: create library, index, search by EXIF field, add annotation, verify persistence
- [x] No changes to the existing browse/server mode behaviour
