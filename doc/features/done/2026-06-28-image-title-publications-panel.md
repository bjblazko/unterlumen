# Image Title Field + Publications Panel

*Last modified: 2026-06-28*

## Summary

Two improvements to the info panel in library mode:
1. A short per-photo title field stored as `dc:title` in XMP sidecars
2. A dedicated Publications section replacing raw `published:*` key-value rows

## Details

### Image Title

- Editable title field displayed at the very top of the info panel (above File section) in library mode only
- Stored as `dc:title` in a `.xmp` sidecar file using standard Dublin Core namespace — readable by Lightroom, Capture One, and other tools
- Mirrored to the library database (`photo_meta` key `"title"`) for fast access
- Saving a title creates or updates the sidecar; clearing it removes the `dc:title` block (preserving any other XMP content such as `ul:` publication records)
- Re-indexing a library picks up `dc:title` values from existing sidecars
- Available as `{title}` token in batch rename — automatically slugified (lowercased, non-alphanumeric characters collapsed to `-`, empty value becomes `unknown`)

### Publications Panel

- `published:*` entries no longer appear as raw rows in the Meta section
- A dedicated "Publications" collapsible section renders above Meta showing compact cards: channel name (humanised from slug) + publication date
- Deleting a card removes the primary key and all sub-keys (`postid`, `title`, `account`) via the existing backend cascade
- Meta section filters out all `published:*` keys so they never appear as generic rows

### Bug Fix: Batch Rename in Library Mode

Batch rename now works from all library contexts:
- Library folder view (`LibraryPane`): files are library-source-relative; `sourcePath` is prepended
- Filter/search results (`SearchResultPane`): files are absolute `pathHint` values; leading `/` is stripped
- Cross-library list search: same as search results

## Acceptance Criteria

- [x] Title field appears at top of info panel in library mode
- [x] Editing and blurring saves title to XMP sidecar and library DB
- [x] Clearing title removes `dc:title` from sidecar without touching `ul:` block
- [x] Re-indexing picks up existing `dc:title` from sidecars
- [x] `{title}` token in batch rename slugifies the title
- [x] Publications section shows compact cards, not raw key-value rows
- [x] Deleting a publication card removes all related keys
- [x] Batch rename works from library folder view
- [x] Batch rename works from library filter/search results
