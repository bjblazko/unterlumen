*Last modified: 2026-06-05*

# Output Parity: In-App Folder Picker & Channel ZIP Download

## Summary

Full output parity across desktop and server/container mode for both export and channel publish.

Previously:
- Export dialog hid the "Save to folder" option in server mode (ZIP-only).
- Channel publish always wrote to `~/.unterlumen/channels/<slug>/` with no download option in any mode.

## Details

### In-app folder browser (`FolderPicker`)

New `src/web/js/folder-picker.js` — a modal-overlay directory browser component. Uses a new `GET /api/browse/dirs?path=<relpath>` endpoint that returns only subdirectories at a given path (validated via pathguard). Returns a relative path on confirm, or `null` on cancel.

The `FolderPicker` replaces the OS-native folder picker (`osascript`/`zenity`) in the export dialog. The OS picker endpoint (`/api/export/folder-picker`) is kept registered on desktop but is no longer called by the frontend.

### Export dialog — server mode parity

The "Output" section is now always visible (was hidden in server mode). In server mode the placeholder text guides the user to use a relative path (e.g. `exports/batch`). The backend `POST /api/export/save` is now registered in all modes and resolves the destination path via `pathguard.SafePath` when it is relative, rejecting absolute paths in server mode.

### Channel publish — Download ZIP

The publish dialog gains a **Download ZIP** button alongside the existing **Publish** button. Clicking it calls `POST /api/library/{id}/publish-download` which:
- Exports each selected photo with channel settings into a temporary ZIP.
- Streams the ZIP back to the browser as an attachment.
- Does **not** write to the channel output directory, XMP sidecars, or the library database.

This is available in all modes (desktop and server).

## Acceptance Criteria

- [x] Export dialog shows "Save to folder" and "Download as ZIP" in both desktop and server mode
- [x] "…" button opens in-app folder browser in all modes
- [x] Navigating the picker and confirming fills the destination input with a relative path
- [x] Export save works with relative paths (resolved against browse root via pathguard)
- [x] Absolute paths still work in desktop mode; server mode rejects them
- [x] Publish modal has a "Download ZIP" button that downloads channel-exported photos without recording in XMP/DB
- [x] Backend: `GET /api/browse/dirs` returns subdirectory list for a relative path
