# Remove Geolocation

*Last modified: 2026-03-16*

## Summary

Add a "Remove" button to the Tools menu alongside the existing "Set" button for geolocation. Restyle the section as a label + toggle-button row matching the View menu's Layout row. The label shows the count of actionable images when the menu is opened.

## Details

- **Tools menu restyled** — The geolocation section now renders as `<label>Geolocation (N images)</label>` + `[Set] [Remove]` toggle buttons, consistent with the View menu's "Layout" row.
- **Remove Geolocation modal** — A lightweight confirmation modal (no map) warns the user that GPS data will be stripped and cannot be undone, then calls `POST /api/remove-location`.
- **Backend** — `media.RemoveGPSLocation()` runs `exiftool` with blank GPS tag assignments and `-overwrite_original`. `handleRemoveLocation` in `internal/api/location.go` mirrors `handleSetLocation`, registered at `/api/remove-location`.
- **Count label** — `_updateToolsGeoLabel()` in `BrowsePane` recomputes `getActionableFiles().length` each time the Tools menu opens and updates the label text.

## Acceptance Criteria

- [x] `go vet ./...` passes
- [x] Tools menu shows "Geolocation" label with "Set" / "Remove" buttons in a toggle row
- [x] Label updates to "Geolocation (N images)" when actionable files exist
- [x] "Set" opens existing map modal
- [x] "Remove" opens confirmation modal; confirm removes GPS; cancel dismisses
- [x] After remove, info panel shows no GPS data for affected files
- [x] CHANGELOG updated; feature doc moved to `done/`
