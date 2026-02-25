# Dark Mode & Settings Button

*Last modified: 2026-02-25*

## Summary

Add a Settings button to the header, next to the mode-switcher buttons, that opens a dropdown menu. The first setting is a theme selector with Light, Dark, and Auto (OS detection) options.

## Details

- A gear-icon **Settings** button appears to the right of the mode-switcher in the header.
- Clicking it opens a dropdown panel (right-aligned) containing a **Theme** row with a three-way toggle: Light / Auto / Dark.
- **Light** forces light theme (`:root` defaults).
- **Dark** forces dark theme (`:root[data-theme="dark"]` variables).
- **Auto** follows the OS `prefers-color-scheme` media query, and updates in real time if the OS setting changes.
- The selected preference is persisted in `localStorage` under the key `theme`. First-time visitors default to `auto`.
- A small inline `<script>` in `<head>` applies the theme before CSS paints, preventing a flash of the wrong theme (FOIT).
- The fullscreen image viewer retains its hardcoded dark appearance regardless of theme — it is always dark to display photos.

## Acceptance Criteria

- [ ] Settings button appears in the header to the right of mode-switcher buttons
- [ ] Clicking the button opens a dropdown with Light / Auto / Dark toggle
- [ ] Clicking Light switches to light theme; the Light button highlights orange
- [ ] Clicking Dark switches to dark theme; the Dark button highlights orange
- [ ] Clicking Auto follows OS setting; changing OS dark mode preference updates the UI immediately without page reload
- [ ] Theme preference persists across page reloads
- [ ] No flash of wrong theme on initial load
- [ ] Image viewer is always dark regardless of theme
- [ ] `go vet ./...` passes with no errors
