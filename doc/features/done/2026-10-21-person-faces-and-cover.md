# A person's faces across libraries, and which cover shows them

*Last modified: 2026-10-21*

## Summary

A person's page shows all their faces in one grid, from the newest photo to
the oldest, whichever library they are in. Where a person is in several
libraries, the cover chosen last shows them everywhere. Often with shows
each person's face.

## Details

- `GET /api/people/someone` answers `faces` (every face, each with its
  `library` and the photo's `date`, newest first, undated last) and `cover`
  (`{ library, face }`); `in` lists the libraries without faces.
- `people.cover_chosen` records when a cover was chosen (this installation's,
  like the cover). Across libraries the latest choice wins; with none, the
  library with most of the person's faces. One rule (`showsBetter`) for
  People, the person's page, the phone, Often with and the statistics.
- `FaceGrid` takes faces of several libraries; a selection is in one library
  at a time, the bar names it.
- One face selected: *Open photo* (the viewer, among all their faces'
  photos) and *Show in library* (`showPhotoInLibrary`, shared with the
  viewer's ⋯ menu).
- Often with: a round face (24 px) on each chip, label *Often in photos
  together with · number of photos*.

## Acceptance Criteria

- [x] A person's faces are one grid, newest photo first, across libraries
- [x] A selection stays in one library; the bar says which
- [x] The cover chosen last, in any library, shows the person everywhere
- [x] Without a choice, the library with most faces gives the cover
- [x] Often with chips show the person's face and a longer label
- [x] One face selected opens its photo or shows it in its library's folder
- [x] Go tests (store, manager, statistics, API) and e2e specs
