# One filter for the Map, the Statistics and the Timeline

*Last modified: 2026-10-02*

## Summary

The Map, the Statistics and the Timeline share one filter in their heads:
libraries, a span of months, cameras and lenses, each optional and all on at
first, with a reset; on a phone it folds behind a Filter button. See
[ADR-0050](../../architecture/adr/0050-shared-scope-filter.md).

## Details

- `ScopeFilter` and `MultiSelect` in the browser; `ScopeState` shared by the
  three places and kept in `localStorage`.
- The Timeline leaves out the span of months: time is its axis.
- The server filters every statistics, colour, 3D-view, timeline, map and
  search query through the store's scope (`library.Filter`).
- `/api/library/scope-values`: cameras and lenses with counts, lenses of the
  chosen cameras, first and last month.

## Acceptance Criteria

- [x] One component in the heads of Map, Statistics and Timeline; Reset only when narrowed.
- [x] Nothing filtered at first; all off is "none" (no photos), not all.
- [x] The choice is shared across the three places and survives a reload.
- [x] Lenses follow the chosen cameras; a lens no longer offered falls away.
- [x] Statistics, Map, Timeline and the photo column count within the filter.
- [x] On a phone the filter folds behind a Filter button.
- [x] Go tests for the filter, the scope values and the search; timings on a 36,000-photo copy.
- [ ] e2e: set the filter, change place, reset; phone.
- [ ] README pictures with the filter.
- [ ] Released and deployed.
