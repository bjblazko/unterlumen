# ADR-0015: Coding Standards and Quality Guidelines

*Last modified: 2026-04-23*

## Status

Accepted

## Context

The codebase has grown organically and shows signs of drift that will compound over time:

- `src/web/js/browse.js` — 1,174 lines; one class handles rendering, three view-mode layouts, selection, keyboard navigation, EXIF polling, and tool checking
- `src/web/js/app.js` — 767 lines; a single object mixes mode switching, theming, wastebin state, modal management, and global keyboard routing
- `src/internal/media/exif.go` — 735 lines; multiple date-parsing strategies and format-specific branches in one file
- Zero unit tests in Go or JavaScript; only end-to-end tests exist

Without explicit standards, new code tends to follow the existing pattern and the problems compound. The goal is to establish lightweight, language-agnostic rules that apply to all new code and guide opportunistic cleanup when existing files are touched.

## Decision

Adopt the following coding standards across all languages used in the project (Go, JavaScript, CSS, HTML). These are enforced by convention, referenced from `CLAUDE.md` so they are applied automatically during development, and described here for rationale.

### 1. Single Responsibility

Each file, class, or Go package has one reason to change. If the natural description of a file requires "and also", it is a candidate to split. This applies at every level: a function, a class, a file, a package.

### 2. Function Size

Functions longer than roughly 40 lines are a signal to extract named helpers. The name of the helper should make its purpose obvious without a comment. If a name cannot be found, the extraction boundary is probably wrong.

### 3. YAGNI (You Aren't Gonna Need It)

Never add parameters, abstractions, generics, or features for hypothetical future use. Three concrete, existing uses justify an abstraction; one does not. When in doubt, leave it flat and duplicate.

### 4. Domain Grouping

Group code by business domain, not technical layer. Prefer `export/`, `location/`, `wastebin/` over `handlers/`, `utils/`, `helpers/`. When a package or directory exceeds roughly 8–10 files, look for a domain-based split rather than adding more files to the same bucket.

### 5. Testing

- **Go:** New packages or non-trivial functions get a `_test.go` file alongside the source.
- **JavaScript:** Non-trivial modules get a spec when the logic is complex enough to be worth isolating.
- **E2E:** Every new user-visible feature gets at least one spec in `e2e/specs/`.
- **Bug fixes:** When fixing a bug, look for the test that would have caught it and add it.

### 6. CSS and HTML

CSS rules are grouped by component with a `/* --- Component Name --- */` section comment. No speculative utility classes are added. HTML structure reflects the logical component tree, not visual presentation order.

## Acknowledged Baseline Violations

The following pre-existing files violate these standards. They are not required to be fixed immediately, but should be improved opportunistically when touched:

| File | Violation |
|------|-----------|
| `src/web/js/browse.js` (1,174 lines) | Multiple responsibilities: rendering, layout, selection, EXIF polling |
| `src/web/js/app.js` (767 lines) | Multiple responsibilities: modes, theming, wastebin, modals, keyboard routing |
| `src/internal/media/exif.go` (735 lines) | Multiple parsing strategies in one file |
| All Go and JS source | Zero unit tests |

## Consequences

- New code is written to these standards without exception.
- Pre-existing violations are not a blocker but are reduced when a file is already being changed.
- No tooling enforcement is added at this time; the rules are applied by convention and code review.
