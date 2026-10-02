# Licenses and thanks

*Last modified: 2026-10-02*

## Summary

Every license Unterlumen is built on asks for its text to travel with the
program. An audit found the release binaries, the IBM Plex fonts and D3
without theirs, and the map in Set location without OpenStreetMap's credit.
The texts now come with the binary and the release archives, and a place
reached from About lists every project with its license, its website and,
where it takes support, a link to support it.

## Details

- `src/web/licenses/credits.json` lists three groups: built in (Go modules,
  MapLibre, D3, IBM Plex), programs Unterlumen calls (FFmpeg, ExifTool,
  libheif, libde265, libwebp) and maps and data (OpenStreetMap, OpenFreeMap,
  OpenMapTiles, Natural Earth). Support links are only those that were
  checked to exist.
- The license texts lie beside it in `src/web/licenses/*.txt`, embedded in the
  binary and served under `/licenses/`; goreleaser puts them into a
  `licenses/` folder of every archive.
- `src/credits_test.go` fails when a module is built in but not listed, or a
  built-in entry has no license text.
- `#licenses` is a place (ADR-0033), reached from About next to How
  Unterlumen works.
- The Docker image carries `org.opencontainers.image.licenses`; the README's
  License section says where the image's GPL/LGPL tools have their sources.
- `src/examples/LICENSE`: the example photos are © Timo Böwing, all rights
  reserved, not under Apache-2.0.

## Acceptance Criteria

- [x] License texts of every built-in part are inside the binary and the release archives.
- [x] About links to Licenses and thanks; each entry has license, website, and support where it exists.
- [x] The Set location map shows the OpenStreetMap credit.
- [x] A Go test catches a module that is built in without being listed.
- [x] The example photos have their own license note.
- [x] e2e (`licenses`).
