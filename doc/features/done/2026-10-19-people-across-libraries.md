# People across libraries

*Last modified: 2026-10-19*

## Summary

Each library keeps its own people. The same name links them: Anna in one
library is Anna in every other. People's library picker gets *All
libraries*, a read-only view of everyone named anywhere, with suggestions
for libraries where someone looks like them under another name or still
waits for a name. Names are given and changed in each library.

## Details

- `#people/all` lists everyone named in any library, put together by name
  without regard to case, with their photos and the libraries that have
  them (`GET /api/people`).
- `#people/all/person/<name>` (`GET /api/people/someone?name=`) shows the
  person's largest faces per library, each opening its photo and showing it
  in its photo on rest, Show photos across all libraries, and a hint linking
  to the person's page in each library, where renaming happens.
- *Not named in* lists the other libraries. The person's faces are compared
  on this computer with the people and the groups waiting for a name there;
  the closest above `faces.SuggestThreshold` is suggested: a person named
  otherwise (link to their page, to rename) or a group (link to name it).
- A person's page in one library says where else the name is given and
  links to the page across libraries; Rename and Take the name away say they
  change this library only and name the others.
- Nothing new is stored and nothing new is shared: the link is the name the
  sidecars already carry. Explicit links were rejected (ADR-0051, rule 8).

## Acceptance Criteria

- [x] All libraries appears in the picker once there are two libraries
- [x] Everyone named in any library is listed once per name, case aside
- [x] A person's page shows their faces per library and Show photos across libraries
- [x] Libraries without the name suggest a differently named look-alike, else a waiting group, else say nobody looks like them
- [x] The page has no way to rename; it links to each library's person page
- [x] Rename and Take the name away in one library name the other libraries that keep the name
- [x] Go test across five libraries; e2e spec `people-everyone.spec.js`
