# People on a phone

*Last modified: 2026-10-19*

## Summary

On a phone, People replaces Galleries in the tab bar. It finds people and
shows their photos; managing them stays on the computer.

## Details

- Owner's decisions (2026-10-19): Galleries leaves the phone entirely
  (publishing is desktop work); only named people show.
- `#people` on a phone: everyone named in any library, as cards; a search
  field narrows them as you type, without regard to case.
- `#people/all/person/<name>`: cover, name, born and age when known, photo
  count; Often with (`SomeoneEverywhere.With`) as switches that narrow the
  photos to all chosen people together; photos newest first under year
  headings, more as the end comes into view; a photo opens the viewer.
- A library's person address (`#people/<library>/person/<n>`) leads there by name.
- No groups, naming, hiding, cover, birthday or download on a phone; the info
  panel shows only named faces, with ages.
- On the map follows with the person filter (2026-10-19-person-in-shared-filter).

## Acceptance Criteria

- [x] The tab bar reads Libraries, Folders, Map, Timeline, People
- [x] Typing a name narrows the people; nobody found says so
- [x] A person's photos come by year; Often with narrows them to both
- [x] Nothing to manage on the phone; Galleries shows the desktop notice
- [x] Go test for companions; e2e `people-phone.spec.js`, `phone.spec.js` updated
