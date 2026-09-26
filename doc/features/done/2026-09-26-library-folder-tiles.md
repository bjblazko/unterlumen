# Library folders as tiles that show what they hold

*Last modified: 2026-09-26*

## Summary

A library's subfolders were bare chips with a folder glyph and a name, a row of buttons that said nothing about what lay behind them. In a library, the index already knows what every folder holds. So a folder is now a tile with:
- a 2×2 mosaic of its four newest photos;
- its name;
- "N photos · years".

The owner asked for this on 2026-09-26 ("a peep into the folder") and chose the mosaic tile and "newest by date taken". Folders in the filesystem place keep their chips, because showing their contents would mean reading the disk.

## Details

- **Backend:**
  - `Store.FolderPreviews(folderAbs)` in `src/internal/library/folder_previews.go` counts each direct subfolder's whole branch and returns:
    - the photo count;
    - the first and last date taken, where an empty `date_taken` counts as missing;
    - the four newest photo IDs, ordered by date taken with undated photos last by indexed time.
  - A folder with only subfolders shows the photos further down.
  - Endpoint: `GET /api/library/{id}/folder-previews?path=`. It resolves the path with `SafePathLogical`, so the volume need not be mounted.
- **Frontend:**
  - Both the justified and grid renderers ask the pane for a folder's markup (`BrowsePane._folderItemHTML`), so the chip lives in one place. `LibraryPane` overrides it with the tile.
  - The tiles appear with the folder names at once and are filled in place when the previews arrive. Cells without a photo stay a flat `--bg-2` surface. A folder with a single photo shows it across the whole square.
  - The list view keeps its rows.
- **Design:** The tile is a hairline box like a photo's, 176 px wide, two per row on a phone.
  - The name is `--text-sm` 500. The count and years are data (`--font-mono`, `--text-xs`, `--fg-3`).
  - Focused or selected is the strong `--fg` fill, like a chip. One click opens the folder, as with a chip, and a modified click selects it.

## Acceptance Criteria

- [x] In a library, every subfolder is a tile with up to four of its newest photos, its name and "N photos · years".
- [x] A folder that holds only more folders shows photos from further down.
- [x] Folder names appear before the previews; the previews fill the tiles in place.
- [x] The Folders place keeps its chips.
- [x] Go test for `FolderPreviews` (depth, order, undated, sibling-root isolation, empty). e2e test for the tile.
