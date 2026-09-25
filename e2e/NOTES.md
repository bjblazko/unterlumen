# E2E Test Notes

Non-obvious patterns and traps discovered during test development.

## Server configuration

Use `UNTERLUMEN_ROOT_PATH=e2e/fixtures` (env var), **not** a CLI positional argument. The CLI arg sets boundary=`/` (whole filesystem), which makes `path=gps-jpeg.jpg` resolve to `/gps-jpeg.jpg` (not found) and prevents path-traversal blocking. The env var restricts boundary to the fixtures dir so `path=` browses the fixtures root correctly. Set in `playwright.config.js`.

**Run the suite with `npm test`, never bare `npx playwright test`.** `npm test` is `bash fixtures/setup.sh && playwright test`, and that setup step re-copies the fixtures from `src/examples`. Several specs mutate fixtures in place — `crop.spec.js` crops the centre 50% out of the same Canon JPEG on every run, and `gps-editing.spec.js` rewrites GPS tags — so skipping setup silently degrades the images across runs. After enough bare runs the crop target shrinks to a few KB and `POST /api/crop` starts failing, taking apparently unrelated specs (overlays, library-search) down with it. Failures that move between runs, or a spec that fails in isolation but passed an hour ago, are the symptom.

**Rebuild the binary after every frontend change.** `main.go` has `//go:embed web`, so the HTML/JS/CSS the tests run against is the copy compiled into `../unterlumen` — not the files on disk. Editing `src/web/js/*.js` and running the suite silently tests the *previous* frontend, which looks like a spec that fails for no reason. Always `cd src && go build -o ../unterlumen .` first. ("No build step" in CLAUDE.md means no bundler/transpiler; it does not mean the server reads the files live.)

## App initialisation race

`setMode()` fires asynchronously after `API.config` + `toolsCheck`. Clicking `#mode-library` before that completes gets overridden. Guard with `waitForAppReady(page)` before clicking any mode button. Use in ALL specs that navigate modes.

**The helper no longer waits for `.browse-layout`.** Since ADR-0028 the app routes on the hash, so a `page.reload()` after clicking a nav entry — or a `goto('/#libraries')` — starts in that place and never renders browse. The helper waits for `.nav-item[aria-current="page"]` plus any child of `#app` instead. A spec that reloads mid-test used to pass only because every start was a browse start; if you assert on browse DOM after a reload, navigate back to `#folders` first.

**Places are marked with `aria-current="page"`, not a CSS class.** The chevron stepper's `.active`/`.completed` classes are gone. Assert `toHaveAttribute('aria-current', 'page')`.

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

## Selection actions live in the selection bar

Since ADR-0029 there is no Tools-menu entry for export, rename or location: a spec that acts on a selection clicks `.selection-bar [data-action="export|rename|location|mark|collect|clear"]`. The bar exists only while something is selected, and only one is visible at a time (the library list view and an opened library each own one), so an unscoped `.selection-bar` locator is unambiguous.

`[data-action="rename"]` always opens the batch-rename dialog, including for a single photo.

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

## Two installations in one spec

`site-album-register.spec.js` starts a second server on port 8083 from `beforeAll`, with its own `UNTERLUMEN_LIB_DIR` (a temp dir) and `UNTERLUMEN_CHANNELS_DIR` pointing at the main server's `fixtures/.unterlumen-test`. That is exactly what two real installations share. Create the `APIRequestContext`s yourself in `beforeAll`: the `{ request }` fixture of a hook cannot be reused inside a test. A channel's output directory differs per installation; read it from `GET /api/channels/` (`outputDir`), not from `GET /api/channels/{slug}`. Index the second installation's library only after album A was published, so it reads A's XMP sidecar.
