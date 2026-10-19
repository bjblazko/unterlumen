# The index follows the folder

*Last modified: 2026-10-19*

## Summary

A library's index shows what the photo folder holds — after an edit in place,
after a change on the other installation, after a deletion — and a scan in any
order never undoes a change. Shared libraries are checked on their own. See
[ADR-0052](../../architecture/adr/0052-the-index-follows-the-folder.md).

## Details

- Found on 2026-10-19: a photo taken in Hagen whose location was fixed still
  showed in Austin on the Map after a rescan. The edited file had a new
  content hash, so the scan added it again and kept the old row.
- A file changed in place replaces its old row; drafts and the album register
  naming the old ID find the photo through `photo_replaced`.
- Publications follow the sidecar, removals included, as notes already did.
- Photos are removed only after a complete walk of the folder; an unreachable
  share or an unreadable folder removes nothing.
- Shared libraries are checked at start and every 30 minutes.
- A scan uses the walk's list of sidecars instead of asking the disk for each
  photo's: 53 s → 19 s for 3 218 unchanged photos over SMB.

## Acceptance Criteria

- [x] A photo whose file changed (location set, crop) is listed once, with the new file's data
- [x] A changed file whose old content is still at another path leaves that copy alone
- [x] A draft naming the old ID publishes the edited photo
- [x] Rows left by earlier versions and paths no file answers to go at the next full index or cleanup
- [x] A library folder that is not reachable, or a folder that cannot be read, takes no photo out and says so
- [x] A gallery removed from a photo's sidecar is no longer claimed by the index
- [x] Two installations changing the same photo lose nothing, whichever scans first (test)
- [x] Shared libraries are checked at start and every 30 minutes; libraries that are not shared are not
- [x] Changelog, README, arc42, CLAUDE.md and ADR-0052
