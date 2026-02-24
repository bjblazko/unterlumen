# ADR-0009: Soft Delete with Frontend-Only Waste Bin

*Last modified: 2026-02-22*

## Status

Accepted

## Context

The app supports culling photos via copy/move in Commander mode, but has no way to mark files for deletion. Users need a non-destructive workflow: mark unwanted photos, review them, then either restore or permanently delete. This follows the same deliberate, two-step pattern as the existing copy/move operations.

The key design question is where to store the "marked for deletion" state: on disk (e.g., moving files to a trash directory), in a server-side data structure, or purely in the frontend.

## Decision

**Waste bin state is frontend-only (in-memory JavaScript Map).** Files remain untouched on disk until the user explicitly confirms permanent deletion through a confirmation dialog.

- The waste bin is an in-memory `Map` in `app.js`, keyed by relative file path.
- State is lost on page refresh — consistent with [ADR-0002](0002-no-persistence.md) (no persistence).
- The confirmation dialog before permanent delete is the safety net.
- The backend provides a simple `POST /api/delete` endpoint that removes files from disk, following the same pattern as copy/move.

The Waste Bin is exposed as a third mode alongside Browse and Commander in the header mode switcher, with a count badge showing when files are marked. This is the most discoverable approach (Rams principle 4: understandable).

## Consequences

- **Non-destructive by default** — Marking a file does nothing to the filesystem. The user must take two deliberate actions (mark, then confirm permanent delete) to remove a file.
- **Ephemeral state** — Refreshing the page clears the waste bin. This is acceptable because the app is designed for session-based workflows.
- **No undo after permanent delete** — Once confirmed, `os.Remove()` is called. There is no OS trash integration.
- **Simple implementation** — No new server-side state, no trash directories, no filesystem metadata.
