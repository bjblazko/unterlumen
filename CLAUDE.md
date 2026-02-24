# CLAUDE.md

*Last modified: 2026-02-24*

## Project

Unterlumen — a photo browser and culler with a Go backend and vanilla HTML/JS/CSS frontend. Runs as a local HTTP server accessed via the browser.

## Build & Run

```
go build -o unterlumen .
./unterlumen /path/to/photos
```

## Test

```
go vet ./...
```

## Architecture

- `main.go` — entry point, CLI flags, HTTP server
- `internal/api/` — HTTP handlers (browse, thumbnail, image, copy/move) and routing with path traversal protection
- `internal/media/` — directory scanning, EXIF extraction, format detection, HEIF conversion
- `web/` — static frontend (vanilla HTML/JS/CSS, no build step)

## Documentation

- `README.md` — user-facing usage documentation
- `CHANGELOG.md` — tracks all notable changes
- `doc/architecture/arc42.md` — arc42 architecture documentation
- `doc/architecture/adr/` — Architecture Decision Records (ADR-0001 through ADR-0008)
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
