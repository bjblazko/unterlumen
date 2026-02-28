# ADR-0010: Root Path Resolution and Navigation Boundary

*Last modified: 2026-02-28*

## Status

Accepted

## Context

The original design used a single path — the command-line argument, defaulting to `.` — for two distinct purposes:

1. **Start directory**: where the browser initially opens
2. **Navigation boundary**: the filesystem boundary that `safePath()` enforces, preventing users from navigating above it

This conflation made certain deployment configurations impossible:

- Starting at a specific directory while still allowing navigation up the tree (e.g., start in `~/Pictures` but allow navigation to `~` or `/`)
- Providing a starting directory via environment variable for containerized or self-hosted deployments without requiring a CLI argument

## Decision

Separate `startPath` from `boundary` as two independent concepts:

- **`boundary`** — the directory that `safePath()` enforces as the navigation ceiling. All API path validation uses this value.
- **`startPath`** — the path (relative to `boundary`) where the frontend begins. Delivered to the browser via a new `GET /api/config` endpoint.

Path resolution priority chain in `main.go`:

| Priority | Source | Start dir | Boundary |
|----------|--------|-----------|----------|
| 1 | CLI argument | argument | `/` (unrestricted) |
| 2 | `UNTERLUMEN_ROOT_PATH` env var | env var | env var (restricted) |
| 3 | Default | user home dir | `/` (unrestricted) |

A new `GET /api/config` endpoint exposes `{ "startPath": "..." }` to the frontend. The `App.init()` function fetches this before the first `browse.load()` call, ensuring the browser opens at the correct directory regardless of how the server was started.

## Consequences

**Positive:**
- Users starting with a CLI arg can now navigate freely around the filesystem, matching typical CLI tool expectations
- Self-hosted / NAS deployments can use `UNTERLUMEN_ROOT_PATH` to confine browsing without needing the CLI arg
- The default (no args, no env) is now more user-friendly: starts in the home directory rather than whichever directory the binary was launched from
- `safePath()` itself is unchanged — it correctly handles `boundary = "/"` (always passes) and `boundary = /some/path` (restricts navigation)

**Negative:**
- The frontend now depends on an async config fetch at startup before it can render the initial view; errors in the config fetch fall back to `path = ""` (which resolves to the boundary root)
- Users who previously relied on `.` (current directory) as the default start and boundary will now start in their home directory instead

## Alternatives Considered

- **Keep conflated behavior, add separate `--boundary` flag** — more flags, more complexity, still doesn't support the environment variable use case cleanly.
- **Always use `UNTERLUMEN_ROOT_PATH` as boundary regardless of CLI arg** — surprising: a CLI arg would be ignored for navigation purposes.
- **Embed startPath in the HTML at serve time** — requires template rendering, adds complexity, conflicts with the embedded static file approach.
