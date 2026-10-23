# Mark a face by hand

*Last modified: 2026-10-23*

## Summary

A face the detector missed — in profile, half hidden — can be marked on the
photo and named, so the person is in that photo for People, filters and
statistics. Backlog item from the People work (ADR-0051), decided with the
owner on 2026-10-23: a frame first, then the name.

## Details

- **Where:** the viewer. The info panel's People section has Mark a face,
  shown also on a photo without any face found. The toolbar says what to
  do and has Cancel and Save; known faces show as quiet frames with their
  names. Drag the frame (resize and move it with the handles), type the
  name — the names of all libraries are offered — and press Return. Escape
  leaves.
- **What it is:** a row of `faces` with `manual = 1`, named and confirmed,
  with no embedding. It counts as a named face everywhere; groups,
  suggestions and the Face space leave it out. × in the info panel, Is not
  or hiding removes it: there is no face marked by hand without a name.
  A frame over a known face is refused with "There is a face there
  already; name it in the list."
- **Other installation:** the name goes into the sidecar as an MWG region
  first (ADR-0052). A scan takes regions that no found face covers as faces
  marked by hand — added, renamed and removed as the sidecar says; with
  another program's regions in the sidecar the face stays in this index
  and nothing is removed. Names Lightroom or digiKam gave to faces
  Unterlumen did not find become such faces too.
- **A new pass** keeps them; where it now finds the face itself, that face
  takes the name and the frame drawn by hand goes.
- **Not in it:** moving or resizing a saved frame (remove and draw again),
  marking on a phone, pets.

## Acceptance Criteria

- [x] Mark a face in the viewer's People section, also when no face was found
- [x] A frame drawn on the photo and a name save a named face; Escape cancels
- [x] The face shows in the info panel and counts as the person's
- [x] × removes it from the index and the sidecar
- [x] A frame over a known face is refused with a sentence beside the frame
- [x] The other installation adds, renames and removes it from the sidecar
- [x] A pass keeps it and takes it over when it finds the face itself
- [x] Go tests: marking, sync between installations, passes; e2e: marking in the viewer
