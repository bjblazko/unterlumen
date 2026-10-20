# People: one place to find, one to name

*Last modified: 2026-10-20*

## Summary

A review of People with the owner's libraries (2026-10-20) found two places
that looked alike and meant different things: a library's People (naming,
managing) and All libraries (finding), switched by a small select. People now
starts with everyone in all libraries, as on a phone; naming is a place of its
own per library; and a person has one page by name, with a section per
library where their faces are managed. Smaller findings — selection that
worked the other way round on two pages, a wrong face that could only be sent
back to the groups, explanations on every page, black bars on faces at the
edge of a photo, a preview covering the work — are fixed along the way.

Decided in an interview with the owner: finding on the phone, finding and
naming on the computer; names stay what links a person across libraries, and
a person with two names is put right by renaming in place, which writes the
sidecars, so Lightroom sees one name too. Two different people of one name
are told apart by their names ("Theo Meier", "Theo (Kita)").

## Details

### Addresses

| Address | Page |
|---|---|
| `#people` | Everyone named in any library; a line *Groups waiting for a name* with each library that has some, and how many |
| `#people/name/<library>` | Waiting for a name in one library: the groups waiting for a name, as a compact list |
| `#people/name/<library>/group/<n>` | One group, all its faces |
| `#people/person/<name>` | A person, by name, with a section per library |

Older addresses lead to their new place: `#people/all` → `#people`,
`#people/all/person/<name>` and `#people/<library>/person/<n>` →
`#people/person/<name>`, `#people/<library>` → `#people/name/<library>`,
`#people/<library>/group/<n>` → `#people/name/<library>/group/<n>`.

### Everyone (`#people`)

- A *Find a person* field on the computer too.
- *Groups waiting for a name:* one link per library with groups waiting, with their number
  (`GET /api/people/to-name`: groups, photos still to be looked at, whether a
  pass runs). Nothing when nothing waits.
- The explanation of People (faces found, names written into sidecars, read
  by Lightroom) shows only while nobody is named in any library.
- Names may take two lines on a card.

### Waiting for a name (`#people/name/<library>`)

- Each group on one row: its faces (a link to the group), *Is this [Anna]?*
  as a button of its own when a person looks like them, the name field, Name
  and Hide — side by side, nothing pushed to the far right.
- The libraries with groups waiting stay one click away at the top.
- On a group's page nothing is selected at first; *Name all 3* and *Hide all
  3* act on the group, and once faces are selected *Name 2* and *Hide 2* on
  those. The same model as a person's page and as Folders.

### A person (`#people/person/<name>`)

- Title, birthday, Show photos, the ⋯ menu (Rename…, Birthday…, Take the
  name away…) and Often with, as before.
- A section per library that has the name: its name, faces and photos, all
  faces newest first; a section can be folded.
- Selecting faces (in one library at a time) brings up a bar: *Use as cover*
  for one face, *Is instead [name] Move* to give the faces to someone else in
  one step, and *Not Anna* to send them back to the groups.
- Rename… lists each library with the name, with its photos, all ticked; an
  untick leaves that library's name as it is (two different Theos). Take the
  name away… does the same.
- *Not named in:* only libraries where someone looks like the person. A
  person named otherwise gets *Rename to Anna in Analogfilm (63 photos)*, in
  place; a group waiting for a name gets *Name them Anna*. Libraries without
  a suggestion are one line: *Not in: Projekte*.

### Faces and their preview

- A face at the edge of its photo is cut from inside the photo, a little off
  centre, instead of padded with black.
- The preview of a face in its photo waits a little longer (450 ms) and
  keeps clear of the bar at the bottom.

### Small fixes

- The viewer opened from People names the photo's file, not its ID.
- Often with says what its numbers count (photos together).
- Show photos is a button, not a link in a frame.
- On a phone, Often with wraps instead of running off the screen.

## Acceptance Criteria

- [x] `#people` shows everyone in all libraries on the computer, with Find a person and the groups waiting for a name per library
- [x] The explanation shows only while nobody is named
- [x] Waiting for a name lists the groups compactly with the suggestion as its own button
- [x] A group's page starts with nothing selected; Name all / Name n
- [x] One person page by name with a foldable section per library, all faces newest first
- [x] Selected faces can be given to someone else in one step (Is instead)
- [x] Rename and Take the name away choose the libraries, all ticked
- [x] Not named in: rename or name in place; libraries without a suggestion in one line
- [x] Faces at a photo's edge have no black bars
- [x] Older addresses lead to their new place
- [x] The viewer from People shows the file name
- [x] Phone People works with the new addresses
- [x] e2e specs cover the new pages; ADR-0051 and the README describe them
