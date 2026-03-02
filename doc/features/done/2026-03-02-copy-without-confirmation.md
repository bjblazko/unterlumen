# Copy/Move Without Confirmation

*Last modified: 2026-03-02*

## Summary

Remove confirmation dialogs from copy (F5) and move (F6) operations in File Manager (commander) mode to reduce friction during photo culling workflows.

## Details

Previously, every copy or move action triggered a `confirm()` dialog asking the user to approve the operation. For a photo culling workflow where the user is rapidly sorting files between directories, this added unnecessary friction.

The operations are already safe: the backend returns an error if a file already exists at the destination (no silent overwrite), and results are shown to the user if any errors occur.

## Acceptance Criteria

- [x] Pressing F5 in File Manager copies selected files immediately without a dialog
- [x] Pressing F6 in File Manager moves selected files immediately without a dialog
- [x] Clicking the Copy/Move buttons works the same way (no dialog)
- [x] Error feedback still appears if any files fail to copy/move
