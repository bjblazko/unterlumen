# CLAUDE.md

*Last modified: 2026-09-25*

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

The UI follows Dieter Rams' ten principles of good design ([ADR-0008](doc/architecture/adr/0008-dieter-rams-design-principles.md)), inspired by Braun products (1961–1995). Use the global `rams-design` skill for all UI work; the tokens come from its `references/tokens.css`. Key rules:

- **Tokens**: rams-design tokens — see [ADR-0030](doc/architecture/adr/0030-rams-design-tokens.md), which supersedes ADR-0018. Neutral ramp `--bg`/`--bg-2`/`--bg-3`/`--line`/`--line-strong`/`--fg-3`/`--fg-2`/`--fg`; signal colours `--accent` (#E85D04 light / #F07A2A dark), `--confirm`, `--time`, `--warning` with their `*-ink` variants. One meaning per signal colour, ~90 % neutral / 9 % structure / 1 % signal, and every screen must still read in grayscale.
- **Typography**: IBM Plex Sans (`--font-sans`) is the UI voice for all labels, buttons, headings and body text. IBM Plex Mono (`--font-mono`) **only for data** — file names, paths, EXIF values, coordinates, counters, IDs, keyboard keys — with tabular numerals. Weights 400/500/600.
- **Where orange is allowed** — exactly four places, everywhere in the app: (1) the one primary action of a screen or dialog (`btn-accent`), (2) the focus ring, (3) the current value of something being adjusted right now (a range slider, Organize's current target), (4) nothing else. Errors are `--warning-ink`, progress and spinners `--time`, "on/new/confirmed" `--confirm`, selected and active states `--fg` (or `--bg-2` for a quiet selection), hover a surface step, chart marks `--chart-*`. Orange as an error colour was the pre-ADR-0030 habit; it is a defect now.
- **Controls**: Every action has a frame; rank comes from colour and order. Orange marks the one primary action per screen. Frameless only for navigation entries and links in running text. Everything in one row shares one height: `--control-h` 36px, `--control-h-sm` 30px (set `height` with `box-sizing: border-box`; the border counts inside).
- **Navigation**: A sidebar of places ([ADR-0028](doc/architecture/adr/0028-places-not-workflow-steps.md)) — entries are `<a href>` with `aria-current`, never `<button>`. `#mode-*` IDs and the number shortcuts stay.
- **Shape and depth**: Flat. Radii `--radius-sm` 2px / `--radius-md` 4px / `--radius-lg` 8px, `--radius-full` for round controls and badges. Separate areas with a surface step *or* a hairline, never both. `--shadow-float` only for floating layers (menu, popover, modal); nothing at rest has a shadow. No gradients, glows or backdrop blur.
- **Layout**: 4px base spacing scale (`--space-1` … `--space-12`), no off-scale values, generous whitespace, photos without ornament.
- **Motion**: 120/160/240ms ease-out, state changes only; 0ms under `prefers-reduced-motion`.
- **Copy**: Sentence case, plain declarative sentences, specifics over superlatives, no exclamation marks or emoji. Errors say what happened, why, and what to do next — inline, never `alert()`/`confirm()`.
- **Vocabulary** ([ADR-0029](doc/architecture/adr/0029-destinations-galleries-one-publish-action.md)): in the UI a channel is a **Destination** and an album is a **Gallery**; publishing is one action called **Publish**. Backend names (`channel`, `galleryExport`, `siteExport`, `drafts.json`) are unchanged.
- **Charts** ([ADR-0034](doc/architecture/adr/0034-colour-in-charts.md)): colour in a chart encodes data, so charts have their own ramp — `--chart-1 … --chart-8` (categorical, fixed order, designed per theme), `--chart-seq-1 … 5` (magnitude), `--chart-grid`, `--chart-axis`. The signal colours stay with the interface and appear in a chart only with their own meaning. One series takes slot 1 and no legend; several take slots in order, never cycled. Never colour alone — always a label or a legend. Film simulations keep their own `--film-*` tokens as a recorded exception. Validate any new step with `dataviz/scripts/validate_palette.js` in both modes.
- **Deviations** from rams-design are recorded in ADR-0030 under "Deviations from rams-design". An unrecorded deviation is a defect.
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
- **Dialogs** — one component builds all of them: `new Dialog({ title, subtitle, size, body, actions })` from `src/web/js/dialog.js` ([ADR-0033](doc/architecture/adr/0033-dialogs-and-places.md)). It owns the scrim, the header, the scrolling body, the footer, Escape, the scrim click and the focus trap, so a dialog file only writes its own body and actions. No closing cross — Cancel does that. At most one `btn-accent` action, and it comes last. `app-keyboard.js` defers to anything with `.dialog-scrim` or `.keyboard-owner` (the crop tool), which is the only guard left.
- **Dialog or place?** A dialog is one decision about what is on the screen, fits on one screen, and ends in an action or a cancel. Anything you read, compare or work in for a while is a place with an address. Something that blocks without needing the context goes inline on the screen that raised it. The rule and the verdict for each of today's dialogs are in ADR-0033.
- **Input focus guard** — `GlobalKeyboard._isInputFocused(e)` in `app-keyboard.js` is the single place that blocks all non-Escape shortcuts when any `INPUT`, `TEXTAREA`, `SELECT`, or `contenteditable` is active. It uses three complementary mechanisms: a `_inputActive` flag maintained by `focusin`/`focusout` listeners, plus `e.composedPath()` to detect events that bubble from inside shadow DOMs (e.g. `input[type="date"]`'s year segment in Safari bubbles keydown to the document while month/day don't). **Do not add per-shortcut `e.target.tagName` checks** — they don't survive shadow DOM retargeting and will silently fail. New form fields anywhere in the app are automatically covered; no extra work is needed.

## Gotchas

Non-obvious bugs that have already occurred and are easy to repeat:

- **Paths: two views of the same photos.** The same tree is reached as a container path on the NAS and as a mounted path on the Mac, so a path string that is right in one view is silently wrong in the other. Before using any path, say which kind it is: *browse-root relative* (what the folder picker and API bodies carry) goes through `pathguard.SafePath(root, rel)` on the Go side and never through a `filepath.Join` against the process working directory; *absolute, headed back to the UI* goes through `absPathRelativeToBoundary(abs, boundary)` in `src/web/js/api.js`, whose `null` means "outside the root" and must be reported rather than dropped; *shared configuration* (`channels.json` in `-channels-dir`) must hold no machine-local absolute path — a destination's output folder lives in `output-paths.json` under the installation's own `-lib-dir` ([ADR-0035](doc/architecture/adr/0035-shared-album-register.md)); `channels.Store.Save` never writes it to the shared file, and a legacy value there is read only where that folder exists; *`-lib-dir` content* (library DBs, thumbnails, generated output) may be absolute, it never leaves the machine. Two shipped bugs came from getting this wrong: batch rename stripped a leading `/` to make a path relative (right only when the browse root is `/`, a doubled nonexistent path otherwise), and a destination's output path was handed to the filesystem as picked, resolving against the working directory — every gallery of that destination then vanished from the overview.

- **Library keyboard shortcuts — two search paths**: `LibraryTab` (`src/web/js/library.js`) has two separate code paths: `_searchPane` (single-library filter view) and `_listSearchPanel._searchPane` (cross-library list-view search). Any code routing keyboard events, info-panel updates, or selection state must handle both. Always go through `getActivePaneForKeyboard()`; never assume `_searchPane` or `_infoPanel` is non-null in list-view mode. Info panel must fall back to `_listInfoPanel` when `_infoPanel` is null.

- **Library pane interface**: `LibraryPane` (`src/web/js/library-pane.js`) satisfies the same pane interface as browse panes. `entry.name = photo.id` (SHA-256), `entry.label = photo.filename`. Info panel uses `loadInfoData(photoInfo)`, not `loadInfo(path)` — pathguard rejects absolute paths. `PhotoInfo` struct needs camelCase JSON tags (`json:"filename"` etc.). **`entry.date` must be an ISO string, not a `Date` object** — browse mode's real entries come from the Go API as JSON strings, and shared renderers (`browse-list.js`'s `formatDate`) call `.replace()` on it directly with no type check. `LibraryPane` builds synthetic entries client-side and once passed `new Date(...)` here, which crashed list view for any folder containing a subdirectory (or a photo with no EXIF date-taken) — looked like the View menu was completely unresponsive, since the render throws mid-way and never updates the DOM. e2e coverage for the View menu (list/grid/justified, Show names, Show details) must include library mode, not just browse mode — they are different code paths for data loading even though `LibraryPane extends BrowsePane`.

- **HEIC/sips orientation**: `sips -s format jpeg` preserves EXIF orientation as a tag (does not bake it into pixels) for cameras like Fujifilm that store rotation in the embedded JPEG's EXIF rather than the HEIC irot box. Go's `jpeg.Decode` ignores EXIF orientation. Any pipeline using `sipsConvert` output must call `extractJPEGOrientation(data)` + `applyOrientation(img, ori)` before pixel-space operations. Do not use `sips --cropOffset` — its coordinate space is ambiguous across camera manufacturers. See ADR-0020.

- **HEIF orientation — two sources, one canonical function**: HEIF rotation can live in (a) the ISOBMFF `irot` box (Apple/standard devices, read by `ExtractHEIFOrientation`) or (b) the HEIF's embedded EXIF block (Fujifilm and similar, no `irot` set). **Always use `heifOrientation(path)` when you need the display orientation of a HEIF file** — it checks `irot` first and falls back to the embedded EXIF via `heifExifOrientation`. Never call `ExtractHEIFOrientation` directly in a conversion or thumbnail pipeline; it silently returns 1 for Fujifilm-style files.
- **`heif-convert` bakes rotation but lies about it in its own output's EXIF**: on Linux/Docker, `heif-convert` (libheif) decodes the HEIF's primary image plane already in final display orientation — even for Fujifilm-style files with no `irot` box — but it copies the *source* file's original EXIF orientation tag into its output JPEG unchanged. That tag is stale metadata describing the pre-decode state, not an instruction; treating it as one (either re-applying it ourselves, or serving it to a browser, which auto-rotates on EXIF) rotates an already-correct image a second time, most visibly on portrait photos. **Never call `extractJPEGOrientation`/`heifOrientation` on `heifConvert()`'s output and reapply the result** — `heifConvert()` is self-contained: it strips a stale tag via `stripStaleHeifConvertOrientation` before returning, and every caller must treat its bytes as final. This is architecturally different from `sipsConvert` (preserves the tag correctly, does not bake — still needs the normal detect-and-bake treatment) and `ffmpegRun` (bakes nothing). Each HEIF decoder function must return already-final, correctly-oriented, tag-clean bytes — do not add a "detect orientation from the merged/combined result" step in a wrapper that calls multiple decoders, since a single rule can't be correct across decoders with different baking behavior (this was the root cause of a recurring double-rotation bug across several fix attempts).

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
