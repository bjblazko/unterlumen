# One system for every place

*Last modified: 2026-10-22*

## Summary

The UX review of 2026-10-21 ([doc/ideas/2026-10-21-ux-review.md](../../ideas/2026-10-21-ux-review.md))
found five page-header patterns, counts repeated up to four times, three
date formats, two filter interfaces for one filter, "Done" on things that
are not done, and tools for rare jobs in plain view. This change puts every
place on one system and removes what the system makes unnecessary, without
losing a function. Decisions by the owner, one per finding.

## Details

### Structure

1. **One page header** (`PlaceHeader`): title and count on the left, the
   place's actions and ⋯ on the right, the lede below. Every place uses it —
   Folders, Marked for deletion, Organize, Libraries, a library, Map,
   Timeline, Statistics, People, Galleries, Destinations, Settings, About.
   The grey library toolbar goes.
2. **A library in one row**: ‹ Libraries, then the library's name as the
   first breadcrumb (no more "Root"), Filter and Slideshow on the right;
   Statistics, Edit library… and Open in Folders move into ⋯. The filter's
   results are titled "Results".
3. **A count once, in "photos"**: the total in the title's meta; beside
   Select all only "N selected"; the selection bar loses its count and
   Clear selection (Select none does that). "files" only where folders are
   counted too.
4. **Panels hide, places don't close**: a panel (Info, Filter, the photos of
   Map and Statistics) has one framed icon button, the sidebar's panel icon
   mirrored, named "Hide info" / "Hide filter" / "Hide photos", in the same
   place. Settings loses Done; the filter's results lose ×.
5. **Filter and Info share the right column**, one open at a time, each
   opened from a tool strip at the right edge (as in IntelliJ, after the
   owner's review: buttons in the head among Slideshow and ⋯ read as actions,
   and tabs in the column were a second way in). Counts beside a word are
   one pill everywhere (`countHTML`).
6. **One filter interface**: Map, Timeline and Statistics open the same
   Filter panel; their headers show the conditions on as chips.

### Same thing, one look

7. **Folder tiles**: a folder a library covers shows the library's four
   photos, count and years in Folders too; others keep the folder symbol.
8. **Dates**: ISO dates, times as `14:05`, no fractions, no zone (the zone
   stays in All metadata); the filter's date fields are ISO text fields.
9. **Publishing states**: "Not published" or "Published 2026-10-21"
   (`--confirm`), with "built, upload missing" in `--warning-ink` where it
   applies; no `--time`. Group headers show the type only when it differs
   from the name. Destinations is a list in the grid of Galleries.
10. **Mark for deletion in the viewer**: the same words as everywhere, its own
    button before ⋯, apart from Crop. (No "Unmark": a marked photo leaves the
    viewer's list at once.)

### Less at once

11. **Library maintenance: two buttons** — Scan for new photos (new, gone,
    missing previews) and Read everything again… (metadata, previews,
    analysis, confirmed, with its duration).
12. **Viewer toolbar in three groups**: Back, name, n of m · zoom out, Fit,
    zoom in · Crop, Mark for deletion, ⋯. Film strip, full screen, download
    and rotate go into ⋯; their keys stay.
13. **Folders' ⋯ is about the view**: Names, Details, Hide interface (H).
    Clear cache only in Settings › Storage; Hide interface leaves Settings.
14. **Export in segments**: Format, Size, Metadata and To as labelled
    segments on one grid; the `~ / •` switch gets words; the default keeps
    EXIF without GPS.
15. **Rename**: neutral tokens grouped under Date, Camera, Exposure, File;
    the last pattern used is the default; the preview warns "date twice".
16. **Organize**: one Add target… (the folder picker has New folder); the
    key hints move under the targets.

### Navigation

17. **Sidebar**: About has no sub-entries; Settings stays at the bottom; only
    the most specific entry is filled as current.
18. **Timeline: one band**, the shown span framed in it and dragged there.
    The span it covers is the shared filter's Taken (dropping its chip shows
    all), so the Timeline's own range control goes.
19. **Phone**: viewing only, by design; the unlabelled "i" gets a word or goes.

### Defects

- Exposure shows `0/1` for an unknown aperture.
- A folder's Modified shows nanoseconds and a zone.
- The list view truncates names with room to spare and drops "In library …".
- Help text in one size.

## Acceptance Criteria

- [x] 1 PlaceHeader on every place
- [x] 2 A library in one row, its name instead of Root
- [x] 3 One count, "photos"
- [x] 4 Hide button with panel icon; no Done on places; no × on results
- [x] 5 Filter and Info in one right column, opened from the tool strip
- [x] 6 Filter panel on Map, Timeline, Statistics; chips in their headers
- [x] 7 Library mosaic for covered folders in Folders
- [x] 8 ISO dates and times everywhere, no zone, no fractions
- [x] 9 Two publishing states; Destinations as a list
- [x] 10 Mark for deletion in the viewer, apart from Crop
- [x] 11 Two maintenance buttons
- [x] 12 Viewer toolbar in three groups
- [x] 13 Folders' ⋯ holds only the view
- [x] 14 Export in segments, keeps EXIF without GPS
- [x] 15 Rename: neutral grouped tokens, last pattern, date-twice warning
- [x] 16 Organize: one Add target, hints under the targets
- [x] 17 Sidebar without About entries, one filled entry
- [x] 18 Timeline with one band
- [x] 19 Phone: no unlabelled "i"
- [x] Defects fixed
- [x] E2E suite green
- [x] README screenshots retaken
