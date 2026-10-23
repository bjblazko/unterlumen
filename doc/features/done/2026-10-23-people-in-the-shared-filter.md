# People in the shared filter

*Last modified: 2026-10-23*

## Summary

The filter of the Map, the Timeline and Statistics (ADR-0050) narrows to
people, as a library's filter does. Wherever there is a filter, a person can
be added to it and taken out of it again.

## Details

- The filter's column has a People group, shown when names were given in
  People. A select adds a person ("Add a person…", then "Add someone
  else…"); each chosen person stands below it as a chip with ×, and in the
  place's head as "Person: Anna".
- Several people must all be in the photo, the meaning a library's filter
  gives them.
- The server takes repeated `person` parameters; `library.Filter.People`
  narrows to one set of a person's photos per name (confirmed faces only),
  never a correlated subquery (ADR-0045). Caches key on them.
- A chart's photos in Statistics search with the filter's people and the
  chart's own person together.
- Planned on 2026-10-19 in `open/2026-10-19-person-in-shared-filter.md`,
  which this replaces. Left out of that plan: photo counts beside the names,
  and links from a person's page to the Map and the Timeline with the
  filter set.

## Acceptance Criteria

- [x] Map, Timeline and Statistics offer People in the filter's column when names exist
- [x] A person is added from the column and dropped there or from the head
- [x] Several people narrow to photos with all of them
- [x] The person condition is a set, checked by a query plan test
- [x] Undated photos on the Timeline are narrowed by people too
- [x] e2e: the Map's filter adds a person, asks the server with it, and drops them
