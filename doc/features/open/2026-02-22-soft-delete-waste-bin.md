# Soft Delete & Waste Bin

*Last modified: 2026-02-22*

## Summary

Non-destructive photo culling workflow: mark unwanted files for deletion, review them in a dedicated Waste Bin view, then either restore or permanently delete. Files remain on disk until the user explicitly confirms permanent deletion.

## Details

- **Waste bin state is frontend-only (in-memory).** Consistent with ADR-0002 (no persistence). State is lost on page refresh.
- **Waste Bin is a third mode** alongside Browse and Commander, accessed via the header mode switcher. A count badge appears when files are marked.
- **Marked files show reduced opacity** (0.45) in browse and commander grids.
- **Delete key** marks selected files in Browse mode and in the Viewer.
- **Commander mode** has a Delete button alongside Copy/Move.
- **Viewer** has a Delete button in the toolbar; marking advances to the next image.
- **Waste Bin view** shows marked files in a grid with Restore and "Delete permanently" buttons. Permanent deletion calls `POST /api/delete` which removes files from disk.
- **Confirmation dialog** before permanent deletion is the safety net.

## Acceptance Criteria

- [x] `POST /api/delete` endpoint validates paths, rejects directories, removes files
- [x] Waste Bin mode button appears in header with count badge
- [x] Badge shows count when files are marked, hidden when empty
- [x] Delete key in Browse mode marks selected files
- [x] Commander Delete button marks selected files from active pane
- [x] Viewer Delete button marks current image and advances
- [x] Marked files show reduced opacity in browse/commander grids
- [x] Waste Bin view shows marked files with selection support
- [x] Restore removes files from waste bin
- [x] "Delete permanently" with confirmation removes files from disk
- [x] Already-deleted files (e.g., removed externally) treated as success
- [x] State resets on page refresh (no persistence)
