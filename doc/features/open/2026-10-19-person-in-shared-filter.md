# A person in the shared filter

*Last modified: 2026-10-19*

## Summary

The shared filter of the Map, Statistics and the Timeline (ADR-0050) gets
People: choose one or more named people, and every place shows only the
photos with all of them — where Anna was, when Anna and Ben were together,
what the camera did while photographing them.

## Details

- **Meaning:** all chosen people in the photo (AND), as the library filter's
  Person chips; decided by the owner on 2026-10-19. Names match without
  regard to case, across libraries, as everywhere in People (ADR-0051).
- **Backend:** `library.Filter` gains `People []string`; `Filter.Cond` adds
  one `AND col IN (SELECT f.photo_id FROM faces f JOIN people pe … WHERE
  f.confirmed = 1 AND pe.name = ? COLLATE NOCASE)` per person, a set
  collected once through `faces_photo`, never per row (ADR-0045). `Empty`
  and `Key` include it, so every cache keyed by `Filter.Key()` (statistics,
  exposure space, space and time, timeline) separates it. Query parameter
  `scope_person` (repeated), read by `filterFromQuery`, so it cannot be
  mistaken for the search's own `person`.
- **Options:** `GET /api/library/scope-values` adds `people: [{name, count}]`
  — named people in the chosen libraries with their photo counts, most
  first, merged by name.
- **Frontend:** `ScopeState.value.people` (null for all), a People
  multi-select in `ScopeFilter` beside Cameras and Lenses, labelled
  "People: Anna and Ben"; the reset clears it; `params()` adds
  `scope_person`. Shown only when at least one person is named.
- **Places:** nothing per place — Map, Timeline and Statistics take their
  photos through `statsScope`, `photoCond` and `aliasCond`, which take the
  filter's `Cond`. The People topic under a person filter shows who else is
  in their photos. A photo search from a chart (`pick`) passes the scope
  along, so its photos match too.
- **Links:** a person's page in People gets "On the map" and "In the
  timeline" beside Show photos, opening those places with the filter set
  to that person.
- **Not in it:** OR between people; unnamed faces or groups as a filter.

## Acceptance Criteria

- [ ] Choosing a person narrows the Map, the Timeline and every Statistics topic to their photos
- [ ] Two people narrow to the photos with both
- [ ] The choice holds when changing between the three places, and Reset clears it
- [ ] Caches keep filtered and unfiltered answers apart (`Filter.Key`)
- [ ] The People option lists named people of the chosen libraries with counts; hidden when nobody is named
- [ ] A person's page links to the Map and the Timeline with the filter set
- [ ] Measured on a copy of a large real library: the filtered statistics stay within the times of ADR-0045
- [ ] Go tests for `Filter.Cond` with people; e2e: set a person, change place, reset
- [ ] ADR-0050 amended, CLAUDE.md gotcha (parameter `scope_person`), CHANGELOG, README
