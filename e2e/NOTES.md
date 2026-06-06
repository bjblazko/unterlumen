# E2E Test Notes

Non-obvious patterns and traps discovered during test development.

## Server configuration

Use `UNTERLUMEN_ROOT_PATH=e2e/fixtures` (env var), **not** a CLI positional argument. The CLI arg sets boundary=`/` (whole filesystem), which makes `path=gps-jpeg.jpg` resolve to `/gps-jpeg.jpg` (not found) and prevents path-traversal blocking. The env var restricts boundary to the fixtures dir so `path=` browses the fixtures root correctly. Set in `playwright.config.js`.

## App initialisation race

`setMode('browse')` fires asynchronously after `API.config` + `toolsCheck`. Clicking `#mode-library` before that completes gets overridden. Guard with `waitForAppReady(page)` (waits for `.browse-layout` in DOM) before clicking any mode button. Use in ALL specs that navigate modes.

`const App = {}` in a plain `<script>` does NOT become `window.App`. The DOM signal (`.browse-layout`) is the only reliable readiness check.

## Shared SQLite DB

The test server (port 8082) shares `~/.unterlumen/` with any production libraries. Card, filter, and search assertions must scope to the named test library (e.g. `{ hasText: 'E2E Library UI' }` on locators, or `selectOption(libID)` in the search panel) to avoid pollution from production data.

## Library search panel async build

The panel becomes `.visible` immediately on click but controls only appear after `Promise.all([list(), exifRanges(), ...])` resolves. Use `waitForSelector('.lib-search-select', { timeout: 20_000 })` before interacting with the panel. Under full-suite load the global exif-ranges query (~1 s) can be slower.

After `selectOption(libID)` the change handler rebuilds sliders + status async. Wait for the status or content to reflect the new library before asserting. Use `waitForFunction` on `.lib-search-status` text change — capture `prevStatus` first and wait for it to differ.

Three `.lib-filter-groups` elements exist: first wraps the date filter, second wraps sliders, third wraps text filters. Use `{ hasText: '…' }` to target a specific group — `.first()` no longer reliably points to the sliders wrapper.

## Statistics API latency

`GET /api/library/statistics` (no ids) takes ~4 s with large photo sets. Use `{ timeout: 15_000 }` for the `.stats-grid` selector.

## `reindexLibrary` helper

`e2e/helpers/library.js` — POSTs to `/api/library/{id}/reindex` and checks for `"finished":true` in the buffered SSE response. Fixtures (3 photos) reindex in < 1 s.

## Selectors

- **View mode buttons** live inside `.view-menu` (hidden by default). Click `.view-menu-btn` first, then `button[data-view="grid|list|justified"]`.
- **Justified layout** (default) uses `.justified-item.image-item`, NOT `.grid-item.image-item`. Grid mode uses `.grid-item.image-item`.

## Multi-select modifier key

Use `{ modifiers: ['Meta'] }` (Cmd/Meta), not `['Control']`. On macOS headless Chrome, `Ctrl+click` fires `contextmenu`, not `click`.

`devices['Desktop Chrome']` sets `navigator.platform = 'Win32'` → `isMac = false` → `modKey = e.ctrlKey`. Ctrl+A in commander/browse tests must use `Control+a`, not `Meta+a`. Multi-select clicks still work with `{ modifiers: ['Meta'] }` since the click handler checks `e.ctrlKey || e.metaKey`.

`page.keyboard.press('Control+a')` only fires if a non-button element had focus. Always click a relevant item (e.g. an image) before pressing Ctrl+A, otherwise the keydown event may not reach the app handler.

## Browse pane stale DOM

The browse pane does not re-render on viewer close. `marked-for-deletion` persists until the next `load()` call. To assert class removal, navigate to a subdirectory and back first.

## Commander pane-specific waits

`waitForThumbnailsLoaded` may return when items appear in either pane. For tests targeting the left pane, use:
```js
page.waitForFunction(() =>
  document.querySelectorAll('#left-pane [data-type="image"]').length >= 1
)
```

## Fixtures

Downloaded via `e2e/fixtures/setup.sh`, gitignored. Sources: ianare/exif-samples (MIT) for JPEGs, strukturag/libheif (Apache 2.0) for HEIC.
