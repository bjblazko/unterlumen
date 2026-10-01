# Titles and fields live in the sidecar

*Last modified: 2026-10-01*

## Summary

What a person writes about a photo — its title and free fields — lives in the
photo's XMP sidecar, so every installation that sees the photo reads it; the
library's index only copies it. New library says, in plain words, what a library
does to the folder. See
[ADR-0048](../../architecture/adr/0048-notes-live-in-the-sidecar.md).

## Details

- Fields are written as `ul:Fields` beside `ul:Publications`; the title stays in
  `dc:title`.
- Writing goes to the sidecar first; a failure saves nothing and is shown.
- Scans and opening a photo's metadata make the index follow the sidecar,
  removals included, once a library's notes have been moved.
- At start, notes only an index holds are written into the sidecars, once per
  library.
- New library lists what happens to the folder.

## Acceptance Criteria

- [x] A field set in the info panel is in the sidecar and the index.
- [x] A sidecar that cannot be written changes nothing and the panel says so.
- [x] Publications and other programs' XMP are kept when fields are written.
- [x] A field written on one installation is read on the other without a scan; removing it travels too.
- [x] Notes only the index held are written to sidecars once; a sidecar's different value is kept; vanished photos get no sidecar.
- [x] New library shows what it does to the folder.
- [x] Go tests (`media`, `library`) and e2e (`library-sharing`).
