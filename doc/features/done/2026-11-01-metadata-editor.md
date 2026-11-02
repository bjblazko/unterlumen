# Metadata editor and date correction

*Last modified: 2026-11-01*

## Summary

A place of its own, **Metadata** (`#metadata`), changes what photos and
videos say about themselves: date taken, title, description, keywords,
creator, copyright and rating. Where it is written is always in view and
chosen for each change: into the file (EXIF/IPTC/XMP through exiftool), or
beside it (the XMP sidecar of a photo, the Kodi NFO of a video). The date
of many files is corrected at once without losing each file's time of day.
The same work fixes the Libraries entry, which now always opens the
overview. See ADR-0060.

## Details

- Reached from the selection bar's **Edit metadata…** in Folders, Organize and
  a library, and from the viewer's ⋯. Videos come too.
- **Write to: Sidecar (XMP · NFO) · File (EXIF)**:
  - The sentence below names what happens to how many photos and videos.
  - File is disabled without exiftool.
  - The sentence warns when the sidecar's own value will still be the one shown.
- Date: **Keep · Set the day · Shift**.
  - Set the day keeps each file's time and offset. A time can be given for files without a date; without one they are skipped.
  - Shift adds days, hours and minutes, which may be negative.
- Fields with different values show "Several values" and are written only
  when changed. Keywords: chips with "n of m" for those only some files
  have; × removes from all, Add adds to all.
- The preview table shows each file's date before → after, the other fields
  that change, and why a file is left as it is.
- Backend:
  - `internal/metadata` reads, plans and writes; `internal/api/metadata` serves it.
  - `media/xmp_props.go` writes the sidecar.
  - `Manager.MetadataWritten` brings the change into each library that covers the file.
  - The index lays the sidecar's or NFO's values over the file's (`Overlay`, `refreshOverlay`).
- The Libraries fix: `App.enterPlace` resets a place to its start for a click on its sidebar
  entry, a tab, a place link or its key; for Libraries it leaves the open
  library (`LibraryTab.leaveLibrary`).

- Stars: keys 1–5 and 0 (the place keys are gone except `,`), clickable
  stars in the info panel and the viewer, always written to the sidecar;
  shown on tiles, in the list's Rating column, in the info panel and the
  viewer; the library filter's Rating choice. Metadata's list shows a
  thumbnail beside each name.

## Acceptance Criteria

- [x] The target is visible and chosen per change; the default is the sidecar
- [x] Sidecar writes leave the photo byte-identical and its library ID unchanged
- [x] File writes change the photo, and the library knows it by its new ID
- [x] Videos are never written; their metadata goes to a Kodi/Jellyfin NFO beside them, which moves and is deleted with them
- [x] Set the day keeps each file's time of day; Shift keeps the time between files; files without a date are named and skipped
- [x] Fields with different values say "Several values" and stay unless changed; keywords are added and removed, not replaced
- [x] Sidecar/NFO values win over the file in the editor, the index (Timeline, Map, Statistics) and the info panel, with their source shown
- [x] A sidecar changed alone (by another program or installation) updates the index on the next scan
- [x] Clicking Libraries always shows the overview (sidebar, phone tab, place links)
- [x] 1–5 rate and 0 removes the rating, in Folders, a library and the viewer; stars clickable in the info panel and viewer; written to the sidecar
- [x] Stars on tiles, in the list, in the info panel and the viewer; a rating in the file itself is read
- [x] Library filter: Rating — Doesn’t matter · at least 1–4 stars · 5 stars · Not rated; not correlated
- [x] Metadata's list shows a thumbnail per file
- [x] Go tests: `internal/metadata`, `media/xmp_props_test.go`, `library/index_overlay_test.go`, `api/metadata`; e2e: `metadata-editor.spec.js`, `library-nav-overview.spec.js`
