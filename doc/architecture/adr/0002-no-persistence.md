# ADR-0002: No Persistence — In-Memory State Only

*Last modified: 2026-02-21*

## Status

Accepted

## Context

Photo management apps often maintain databases for metadata, tags, ratings, and thumbnail caches. This adds complexity: schema migrations, data corruption recovery, storage location choices, and synchronization between the database and the filesystem.

This application is designed as a lightweight tool for browsing and culling, not a photo library manager.

## Decision

All state is in-memory and discarded when the process exits. No database, no config files, no thumbnail cache written to disk.

## Consequences

- **Zero setup** — Point the binary at a directory and go. No initialization step, no database file left behind.
- **Filesystem is the source of truth** — The directory structure and file metadata are read on each request. Changes made outside the app (e.g. another tool moving files) are reflected immediately.
- **No ratings, tags, or flags** — Culling is done exclusively through copy/move operations (see ADR-0005). There is no concept of "flagging" a photo.
- **Repeated EXIF parsing** — Metadata is re-extracted on each directory listing. For typical directory sizes (hundreds of files) this is fast enough. If performance becomes an issue, an in-memory cache (still discarded on exit) could be added.
- **No resume** — If the app is restarted mid-session, there is no state to recover. Acceptable because the only durable action (copy/move) writes directly to the filesystem.
