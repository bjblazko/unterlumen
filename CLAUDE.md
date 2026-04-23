# CLAUDE.md

*Last modified: 2026-04-23*

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

## Architecture

- `src/main.go` — entry point, CLI flags, HTTP server
- `src/internal/api/` — HTTP handlers (browse, thumbnail, image, copy/move) and routing with path traversal protection
- `src/internal/media/` — directory scanning, EXIF extraction, format detection, HEIF conversion
- `src/web/` — static frontend (vanilla HTML/JS/CSS, no build step)

## Documentation

- `README.md` — user-facing usage documentation
- `CHANGELOG.md` — tracks all notable changes
- `doc/architecture/arc42.md` — arc42 architecture documentation
- `doc/architecture/adr/` — Architecture Decision Records (ADR-0001 through ADR-0015)
- `doc/features/open/` — feature documents for planned/in-progress work
- `doc/features/done/` — feature documents for completed work

## Design Philosophy

The UI follows Dieter Rams' ten principles of good design ([ADR-0008](doc/architecture/adr/0008-dieter-rams-design-principles.md)), inspired by Braun products (1961–1995). Key rules:

- **Palette**: Off-white (#f5f2ed), warm grays, functional orange (#d35400) for accents
- **Typography**: Helvetica/system sans-serif, restrained sizes, medium weight
- **Controls**: Labeled, minimal, no gradients or heavy shadows, 2px border-radius max
- **Layout**: 8px grid, generous whitespace, photos without ornament
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
- **Changelog** — update `CHANGELOG.md` for every user-visible change (new features, bug fixes, format support, API changes). Add entries under `## [Unreleased]`. When the README Documentation section lists ADRs, keep it in sync when new ADRs are added.
- **Date modified** — all documentation files (except README.md) must include a `*Last modified: YYYY-MM-DD*` line below the title. Update this date whenever the document is changed.
