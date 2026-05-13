# Global Channel Output & Path Buttons

*Last modified: 2026-04-28*

## Summary

Channel export output is now stored globally at `~/.unterlumen/channels/<slug>/` instead of per-library at `~/.unterlumen/libraries/<id>/channels/<slug>/`. This makes channels truly cross-library: albums from any library publish into the same channel directory, and site-export channels accumulate a unified site regardless of which library the photos came from. Three path-navigation buttons are added to each channel row in the Channels dialog.

## Details

### Global output path

Previously, publishing photos to a channel wrote output under the library's own directory. With this change, all channel output lands in a single global location:

- Before: `~/.unterlumen/libraries/<libID>/channels/<slug>/`
- After: `~/.unterlumen/channels/<slug>/`

Publishing still requires a library context (photos are read from a specific library's index and files), but the output directory is no longer tied to any library. Existing per-library channel output is left in place and not migrated.

The `channels.Store.OutputDir(slug)` method computes this path from the store's base directory (`filepath.Dir(channels.json)`), keeping the logic in one place.

### Unified site export

For site-export channels, albums from multiple libraries now accumulate in the same `site/` directory and `site.json` statefile. Publishing from library A and then library B to the same site channel produces one root `index.html` listing all albums.

### Rebuild site (now global)

The rebuild-site endpoint moves from `POST /api/library/{id}/channels/{channel}/rebuild-site` to `POST /api/channels/{slug}/rebuild-site`. No library ID is required since the site directory is globally addressed.

### Channel path buttons

Each channel row in the Channels dialog gains three buttons placed before Edit/Delete:

| Button | Action |
|--------|--------|
| Copy path | Fetches `GET /api/channels/{slug}/path` and writes the path to the clipboard. Button briefly shows "Copied!" as feedback. |
| Show in Files | Calls `POST /api/channels/{slug}/reveal`; the server ensures the directory exists and opens it with `open` (macOS), `explorer` (Windows), or `xdg-open` (Linux). |
| Open in Commander | Fetches the path, closes the modal, switches to Commander mode, and loads the path into the left pane. |

### Channels button in library list header

A "Channels" button now appears in the library list header alongside "New library". Channel config (and all three path buttons) is accessible without entering a specific library.

## Acceptance Criteria

- [x] Publishing photos exports to `~/.unterlumen/channels/<slug>/` regardless of which library is open
- [x] Site-export channels accumulate albums from multiple libraries in one unified site
- [x] Rebuild site endpoint is `POST /api/channels/{slug}/rebuild-site` with no library ID required
- [x] "Copy path" copies the global channel directory to the clipboard
- [x] "Show in Files" opens the directory in the OS file manager (creates dir if absent)
- [x] "Open in Commander" navigates the Commander left pane to the channel directory
- [x] All three buttons work from both the library list view and the library detail view
- [x] `go vet ./...` passes cleanly
