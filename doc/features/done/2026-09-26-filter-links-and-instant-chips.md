# Filter: instant chip removal, and a way from galleries and destinations to their photos

*Last modified: 2026-09-26*

## Summary

Two follow-ups to the unified library filter ([one library filter](../done/2026-09-26-one-library-filter.md)), both asked for by the owner on 2026-09-26:

1. **Dropping a filter chip feels slow.** The chip stayed on screen until the new search had come back from the server, which can take a moment across large libraries. A dropped chip now disappears at once, and the search runs behind it.
2. **Galleries and destinations have no way to their photos.** A gallery's detail and a destination's detail now have a "Show photos" link. It opens Libraries with every library in scope and the filter already set:
   - a gallery sets that one gallery;
   - a destination sets the destination.

## Details

**Instant chips**
- `LibraryFilterPanel` renders its active chips and the count on the button from its own state as soon as a criterion changes, before the query starts. It no longer waits for `_renderResults`.
- A query that runs straight away cancels a pending debounced one. Each query carries a generation number, so a slow, older answer never overwrites a newer filter.

**Show photos**
- `ChipInput.add(nsInfo, value)` adds a chip without going through the autocomplete. The existing `_selectValue` uses it too.
- `LibraryFilterPanel.openWith(criteria)` works as follows:
  1. it opens the panel;
  2. it resets the criteria without running a query;
  3. it sets the scope to all libraries;
  4. it adds one chip per criterion;
  5. it runs one query.
- `LibraryTab.showFiltered(criteria)` goes back from a library's detail to the overview if needed, then calls `openWith`.
- `App.showPhotos(criteria)` switches to the Libraries place and calls `showFiltered`. The links are `<a class="btn btn-sm" href="#libraries">`. They are navigation, framed like the existing "Open in browser" link, and a modified click still opens the plain place.
- Criteria:
  - **Destination:** `channel` is the destination slug. It matches photos with a `built:<slug>` key.
  - **Gallery:** the new parameter `album=<slug>:<postID>`, read by `parseListPhotosOpts`, matches the gallery's membership key `built:<slug>:<postID>`, with the usual `published:` fallback.
- The gallery is not filtered by its title. Renaming a gallery changes only the gallery's own record, not the titles its photos recorded, so a title filter would find nothing after a rename. The chip shows the title and carries the ID (`GALLERY_CHIP_NS`).
- The fixed chip namespaces now use the UI vocabulary of ADR-0029: "Destination" (formerly "Channel") and "Gallery title" (formerly "Album"). The active-filter chips show a criterion's label and display value instead of its internal namespace.
- Drafts have no "Show photos" link. Their photos are not built yet, and the draft's own pending list already shows them. A new destination has none either.

## Acceptance Criteria

- [x] Dropping a chip removes it and updates the count immediately, before the search returns.
- [x] A published gallery's detail has "Show photos". It opens Libraries with "All libraries" and the chip "Gallery: <title>", and shows exactly that gallery's photos, even after a rename.
- [x] A saved destination's detail has "Show photos", which opens Libraries filtered by "Destination: <slug>".
- [x] The link always lands in the overview, whichever library was open before.
- [x] e2e specs cover both links and the instant chip removal (with the search delayed by 2 s). A Go test covers the `album` parameter.
