# Birthdays and ages

*Last modified: 2026-10-19*

## Summary

A person can have a birthday. With it Unterlumen tells how old they were in
each photo — exact, from the photo's date, without a model guessing ages
from faces.

## Details

- Kept by name in `people.json` beside `channels.json`, the folder the
  installations share; so in every library and on the other installation.
  Written whole and re-checked like a sidecar (`internal/atomicfile`). Not in
  the sidecars.
- `GET /api/people/birthdays`, `PUT /api/people/birthdays {name, birthday}`
  (empty takes it away; a date to come is refused).
- A person's ⋯ menu (a library's person page and All libraries): Birthday…
  The page shows "Born 12 May 2015, 11 today".
- The info panel shows a named face's age in that photo ("Anna · 4").
- Statistics: Photos by age, lines per person with a birthday; a click shows
  that person's photos of that year of life. Life lines tooltips give the age.

## Acceptance Criteria

- [x] A birthday is given, changed and taken away from a person's page
- [x] It is kept by name, in any case, in the shared folder
- [x] The info panel shows ages; photos before the birthday show none
- [x] Photos by age counts per year of life and opens the photos
- [x] Go tests (birthdays, ages through the API); e2e for the dialog and the chart
