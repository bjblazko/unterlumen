# CLAUDE.md

*Last modified: 2026-05-24*

## Project

Unterlumen — a photo browser and culler with a Go backend and vanilla HTML/JS/CSS frontend. Runs as a local HTTP server accessed via the browser.

## Build & Run

```
cd src && go build -o ../unterlumen .
./unterlumen /path/to/photos
```

## Test

```
cd src && go vet ./...
```

## E2E Tests

Requires the binary to be built first (`cd src && go build -o ../unterlumen .`).

```
cd e2e && npm ci
npm run setup      # download fixtures once
npm test           # run all tests headlessly
npm run test:headed  # run with browser visible
```

Test specs live in `e2e/specs/`. Fixtures download to `e2e/fixtures/` (gitignored).

See `e2e/NOTES.md` for non-obvious patterns: app-init race, selector quirks, library search async build, modifier key differences between platforms.

## Architecture

- `src/main.go` — entry point, CLI flags, HTTP server
- `src/internal/api/` — HTTP handlers (browse, thumbnail, image, copy/move) and routing with path traversal protection
- `src/internal/media/` — directory scanning, EXIF extraction, format detection, HEIF conversion
- `src/web/` — static frontend (vanilla HTML/JS/CSS, no build step)

## Documentation

- `README.md` — user-facing usage documentation
- `CHANGELOG.md` — tracks all notable changes
- `doc/architecture/arc42.md` — arc42 architecture documentation
- `doc/architecture/adr/` — Architecture Decision Records (ADR-0001 through ADR-0020)
- `doc/features/open/` — feature documents for planned/in-progress work
- `doc/features/done/` — feature documents for completed work

## Design Philosophy

The UI follows Dieter Rams' ten principles of good design ([ADR-0008](doc/architecture/adr/0008-dieter-rams-design-principles.md)), inspired by Braun products (1961–1995). Key rules:

- **Palette**: Warm neutrals (OKLCH-based), functional orange (#d35400) for accents — see [ADR-0018](doc/architecture/adr/0018-design-system-tokens.md)
- **Typography**: IBM Plex Mono as UI voice (`--font-mono`); system sans-serif for display/body; restrained sizes, medium weight
- **Controls**: Labeled, minimal, no gradients or heavy shadows; border-radius uses design system tokens (`--radius-sm` 6px → `--radius-lg` 14px)
- **Layout**: 8px grid (`--unit`), 4px base spacing scale (`--space-1` … `--space-10`), generous whitespace, photos without ornament
- **Principle**: "Remove until it breaks." Every element must justify its existence.
- Apply these principles to all future UI changes.

## Coding Standards

These rules apply automatically on every bug fix, refactor, or new feature — no need to ask. See [ADR-0015](doc/architecture/adr/0015-coding-standards.md) for rationale.

- **Single responsibility** — each file, class, or Go package has one reason to change. If a description needs "and also", split it.
- **Function size** — functions over ~40 lines are a split signal. Extract named helpers whose names make comments unnecessary.
- **YAGNI** — never add parameters, abstractions, or features for hypothetical future use. Three concrete uses justify an abstraction; one does not.
- **Domain grouping** — group by business domain (`export`, `location`, `wastebin`), not technical layer. When a directory exceeds ~8–10 files, look for a domain split. Names like `utils`, `helpers`, or `tools` are a warning sign — try harder to find a name that describes what the code actually does.
- **Testing** — new Go packages or complex functions get a `_test.go`. New user-visible features get an e2e spec in `e2e/specs/`. When fixing a bug, add a test that would have caught it.
- **CSS** — group rules by component with a `/* --- Component --- */` section comment. No speculative utility classes.
- **Dialogs & keyboard guard** — `app-keyboard.js` blocks global hotkeys when a dialog is open. Two patterns exist; every new dialog must use one:
  - **`.modal-overlay`** (used by `deps-modal`, `slideshow-modal`, `location-modal`, `export-modal`, `progress-dialog`, `batch-rename-modal`): register your own `document.addEventListener('keydown', ...)` that calls `this.close()` on Escape — the global guard returns early and defers to your listener.
  - **`.modal-backdrop` / `.library-dialog-backdrop`** (used by channel settings, publish dialog, new-library dialog): the global guard handles Escape centrally by clicking the first `.modal-close` or `[id$="-cancel"]` button it finds — make sure your dialog has one. Do **not** also add a separate keydown listener.
  - If you introduce a third root class, add it to the `querySelector` selector in `_handle()` in `app-keyboard.js`.
- **Input focus guard** — `GlobalKeyboard._isInputFocused(e)` in `app-keyboard.js` is the single place that blocks all non-Escape shortcuts when any `INPUT`, `TEXTAREA`, `SELECT`, or `contenteditable` is active. It uses three complementary mechanisms: a `_inputActive` flag maintained by `focusin`/`focusout` listeners, plus `e.composedPath()` to detect events that bubble from inside shadow DOMs (e.g. `input[type="date"]`'s year segment in Safari bubbles keydown to the document while month/day don't). **Do not add per-shortcut `e.target.tagName` checks** — they don't survive shadow DOM retargeting and will silently fail. New form fields anywhere in the app are automatically covered; no extra work is needed.

## Gotchas

Non-obvious bugs that have already occurred and are easy to repeat:

- **Library keyboard shortcuts — two search paths**: `LibraryTab` (`src/web/js/library.js`) has two separate code paths: `_searchPane` (single-library filter view) and `_listSearchPanel._searchPane` (cross-library list-view search). Any code routing keyboard events, info-panel updates, or selection state must handle both. Always go through `getActivePaneForKeyboard()`; never assume `_searchPane` or `_infoPanel` is non-null in list-view mode. Info panel must fall back to `_listInfoPanel` when `_infoPanel` is null.

- **Library pane interface**: `LibraryPane` (`src/web/js/library-pane.js`) satisfies the same pane interface as browse panes. `entry.name = photo.id` (SHA-256), `entry.label = photo.filename`. Info panel uses `loadInfoData(photoInfo)`, not `loadInfo(path)` — pathguard rejects absolute paths. `PhotoInfo` struct needs camelCase JSON tags (`json:"filename"` etc.).

- **HEIC/sips orientation**: `sips -s format jpeg` preserves EXIF orientation as a tag (does not bake it into pixels) for cameras like Fujifilm that store rotation in the embedded JPEG's EXIF rather than the HEIC irot box. Go's `jpeg.Decode` ignores EXIF orientation. Any pipeline using `sipsConvert` output must call `extractJPEGOrientation(data)` + `applyOrientation(img, ori)` before pixel-space operations. Do not use `sips --cropOffset` — its coordinate space is ambiguous across camera manufacturers. See ADR-0020.

- **HEIF orientation — two sources, one canonical function**: HEIF rotation can live in (a) the ISOBMFF `irot` box (Apple/standard devices, read by `ExtractHEIFOrientation`) or (b) the HEIF's embedded EXIF block (Fujifilm and similar, no `irot` set). **Always use `heifOrientation(path)` when you need the display orientation of a HEIF file** — it checks `irot` first and falls back to the embedded EXIF via `heifExifOrientation`. Never call `ExtractHEIFOrientation` directly in a conversion or thumbnail pipeline; it silently returns 1 for Fujifilm-style files. Additionally, each decoder has its own orientation behavior: `sipsConvert` preserves EXIF but does not bake pixels; `heif-convert` bakes `irot` but may or may not copy EXIF; `ffmpegRun` bakes nothing. The correct pattern for every path: read `extractJPEGOrientation(outputJPEG)` first (covers sips and embedded JPEG streams), and fall back to `heifOrientation(path)` when the JPEG result is 1.

## Reminders

- **Keep README.md up to date** when making important changes (new features, changed CLI flags, new dependencies, changed requirements).
- **Keep architecture docs up to date** when making architectural changes:
  - Add a new ADR in `doc/architecture/adr/` for significant design decisions (format: `NNNN-short-title.md`).
  - Update `doc/architecture/arc42.md` when the system structure, interfaces, deployment, or quality requirements change.
  - Update the ADR index in arc42 section 9 when adding new ADRs.
- **Feature documents** — every feature gets a dedicated markdown file:
  - New/planned features go in `doc/features/open/` with filename `YYYY-MM-DD-short-title.md`.
  - When a feature is completed, move its file from `open/` to `done/`.
  - Each doc should include: Summary, Details, and Acceptance Criteria (checkboxes).
  - When the user prompts for a new feature, create the feature doc as part of the work.
- **Changelog** — update `CHANGELOG.md` for every user-visible change (new features, bug fixes, format support, API changes). Add entries under `## [Unreleased]`. Each subsection heading (`### Added`, `### Changed`, `### Fixed`, etc.) must appear **at most once per version block** — merge new entries into the existing subsection rather than adding a duplicate heading. When the README Documentation section lists ADRs, keep it in sync when new ADRs are added.
- **Date modified** — all documentation files (except README.md) must include a `*Last modified: YYYY-MM-DD*` line below the title. Update this date whenever the document is changed.
