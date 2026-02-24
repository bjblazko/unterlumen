# Hide Filenames by Default

*Last modified: 2026-02-21*

## Summary

In grid view, hide filenames beneath thumbnails by default. The photo is the content — the filename is metadata that adds visual noise. A session toggle lets users reveal filenames when needed.

## Details

### Design rationale (Rams principles)

- **Unobtrusive** — The UI should recede; filenames under every thumbnail create a visual rhythm that competes with the images themselves.
- **As little design as possible** — If the user is browsing photos visually, the filename adds nothing. Remove it until the user asks for it.
- **Useful** — Some workflows need filenames (e.g. matching files across tools). A toggle makes them available without making them the default.

### Behavior

- **Grid view**: Filenames hidden by default. When hidden, the grid item is just the image — no text label, no extra padding below.
- **List view**: Filenames always visible. The list view's purpose is to show metadata; hiding names would make it useless.
- **Directories**: Directory names always visible in both views. Without a name, a folder icon is meaningless.
- **Toggle**: A "Names" button in the controls bar, same style as the Grid/List view toggle. Active state (orange) means names are shown.
- **Scope**: Per-pane state. Each pane in commander mode can toggle independently. State is session-only (not persisted).

## Acceptance Criteria

- [x] Grid view hides filenames by default
- [x] "Names" toggle in controls bar reveals/hides filenames in grid view
- [x] Directory names always visible regardless of toggle
- [x] List view always shows filenames regardless of toggle
- [x] Toggle is per-pane in commander mode
- [x] Toggle follows the existing button style (Rams-consistent)
