# People in Statistics

*Last modified: 2026-10-19*

## Summary

A People topic in Statistics: who is in the photos, when and with whom, how
many faces a photo holds, and three 3D views of the people and their faces.

## Details

- Data: `GET /api/library/people-stats` (libraries, folder, periods and the
  shared filter like every topic). Named people only, merged by name across
  libraries; faces per photo count every face found, only in photos looked at.
- 2D: Who appears most (top 20), People over time (top 8, chart ramp), Who
  with whom (pairs among the top 12), Alone or in a group (none, 1, 2, 3–5,
  6+), Faces per photo over time (share per period).
- 3D: Face space (each face placed by its print on the principal axes of the
  named people's average faces), Circle of people (layout by shared photos),
  Life lines (lanes per person through time). Views may now set the camera's
  starting angle (`yaw`, `pitch` in the stage spec).
- Clicks: one person (`person=`), two (`person=` twice), a number of faces
  (`faces_min`, `faces_max`), a person in a period, one photo.

## Acceptance Criteria

- [x] The topic appears in the sidebar and the overview
- [x] Each 2D chart opens the matching photos
- [x] The 3D views draw with WebGPU and say so without it
- [x] Folder, libraries and the shared filter narrow the topic
- [x] Go tests for the builder and the projection; e2e `statistics-people.spec.js`
