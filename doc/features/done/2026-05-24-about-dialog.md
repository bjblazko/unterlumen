# About Dialog

*Last modified: 2026-05-24*

## Summary

An About dialog reachable by clicking the Unterlumen logo/title in the header. Shows the GitHub repository link, author contact, and a legal disclaimer.

## Details

- Clicking the `<h1>Unterlumen</h1>` header title (logo + text) opens the About dialog.
- The h1 shows a pointer cursor on hover with a subtle opacity fade.
- The dialog follows the standard `.modal-overlay` pattern used by all other modals, so the keyboard guard in `app-keyboard.js` blocks global hotkeys while it is open.
- Escape and backdrop click both close the dialog.
- Content: app name and tagline, GitHub link, author name and email, disclaimer.

## Acceptance Criteria

- [x] Clicking the header title opens the About dialog
- [x] Keyboard (Enter / Space) on the focusable h1 also opens it
- [x] Escape closes the dialog
- [x] Clicking the backdrop closes the dialog
- [x] Dialog shows GitHub link, author name, email, and disclaimer
- [x] Global keyboard shortcuts are blocked while the dialog is open
