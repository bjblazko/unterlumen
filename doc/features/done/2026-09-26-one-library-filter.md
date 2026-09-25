# One library filter: overview and detail

*Last modified: 2026-09-26*

## Summary

The Libraries overview ("Search…") and a library's detail ("Filter") offered the same function in two versions. They used the same panel and the same endpoint, but the button, the opening, closing and loading behaviour, the column and the keyboard routing were all different. Now there is one filter, called **Filter**, with one code path in the frontend and one filter parser in the backend. The only parameter is the scope: all libraries, or one library.

Decided by the owner on 2026-09-26 as proposal B, out of three: "the same column in both places". The other two were "a filter only in the detail" and "the filter as a place of its own with an address".

## Details

**Frontend**
- `LibraryFilterPanel` (`src/web/js/library-filter.js`, formerly `LibrarySearchPanel`) has one path:
  - Opening does not run a query.
  - Results always go to the host (`onResults`).
  - The panel's own results pane and its spinner overlay are gone.
- `LibraryTab._mountFilter(el, underEl, scopeLibID)` (`src/web/js/library.js`) hosts the filter the same way in both views:
  - button (`_filterButtonHTML`) and column (`_filterBodyHTML`)
  - results pane `#lib-results-pane`, which replaces `underEl` while it is shown
  - info panel `#lib-info-panel`, count on the button, relayout
  - × = `clearAndHide` plus `_hideFilterResults`
- Removed: `_listSearchPanel`, `_listInfoPanel`, `_onListSearchFocus`, `_showSearchResults` and `_showLibraryPane`.
- `getActivePaneForKeyboard()` knows one results pane, and returns it only while it is visible.
- CSS: `lib-search-*` becomes `lib-filter-*`, with one block for both views: 280 px, `--bg-2`, full screen on a phone. The dead rules for the old results grid are removed.
- The overview head is `Filter · Libraries · Sort · Statistics · New library…`, all with `btn-sm`.

**Backend**
- `parseListPhotosOpts(url.Values)` in `src/internal/api/library/handler.go` is the only reader of the filter vocabulary. Both handlers use it:
  - `searchLibraries` (`GET /api/library/search`)
  - `listPhotos` (`GET /api/library/{id}/photos`), which now accepts meta, album, ext and channel as well.
- The comment on the merge sort now states the real order: date taken, newest first.

**Defects fixed along the way**
- On log-scale sliders, `sliderToValue(1, …)` returned `exp(log(max))`, which is 51199.99999999997 for 51200. An untouched ISO slider therefore counted as active:
  - its chip could not be dropped;
  - photos at the maximum ISO were missing from every result.
  The ends of the slider are now exactly `min` and `max`. `isNarrowed()` is the one check, used both for the chips and for the query.
- In the overview, the hidden results pane kept taking keys after it was closed.
- After ×, the count on the button stayed at its old value.

**Rams points in the filter column** (follow-up on the same day)
- "Reset filters" was a frameless text button. It is now `btn btn-sm`, because every action has a frame.
- Selects, date fields and the chip field are `--control-h-sm` high, with a `--line-strong` border and the app's focus ring (`0 0 0 2px bg, 0 0 0 4px accent`). They were 28 px high with a `--line` border.
- Field labels are `--text-sm`, sentence case, `--fg-2`. Small caps in 10 px were off-scale and are not allowed for labels above a field. Range values are data: `--font-mono`, tabular.
- The date field had a fixed `color-scheme: dark`, so its calendar icon was barely visible in the light theme. Two rules used an undefined `--text` token; they now use `--fg`.
- **Small caps removed app-wide** (owner's decision on the same day: "if Rams doesn't need small caps, remove them"). The 16 uppercase rules are split into three groups:
  - labels: `--text-sm` 500
  - section titles: `--text-sm` 600
  - table headers, sidebar sections and switch states: `--text-xs`

  The switch defaults are now "On"/"Off", and the rule is recorded in ADR-0030 and CLAUDE.md. The switches keep ADR-0019's three labels; only the case changed.

## Acceptance Criteria

- [x] The overview and the detail have the same filter button at the left end of the head, the same column and the same behaviour (quiet open, *Done*, ×).
- [x] One name everywhere: "Filter".
- [x] The scope select starts at "All libraries" in the overview and at the open library in the detail, and can be switched in both.
- [x] There is one filter parser in the backend, shared by both photo endpoints, with a Go test.
- [x] An untouched slider sets no criterion. e2e regression test added.
- [x] × in the overview resets the criteria, and the keys go back to the list. e2e test added.
- [x] Docs updated: CLAUDE.md, ADR-0033, library-internals §7, CHANGELOG.
- [x] Filter column: every action is framed, every field is the small control height, and labels are in sentence case.
- [x] No `text-transform: uppercase` left in `style.css`.
