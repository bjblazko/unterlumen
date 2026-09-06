# ADR-0025: Gallery Edit and Remote Delete

*Last modified: 2026-09-05*

## Status

Accepted

## Context

The [Published Galleries overview](0024-published-galleries-overview.md) (ADR-0024) made it possible to see every published gallery in one place, but offered no way to act on a row: renaming a mistitled album, or taking a gallery down, required editing `site.json`/`gallery.json` by hand and manually deleting files. Deletion is also incomplete even for someone comfortable editing state files directly: `deploy.Deploy` deliberately never uses rsync `--delete` (see `rsyncArgs` in `internal/deploy/rsync.go`) — several channels can share one `RemotePath`, and `--delete` would let deploying one channel wipe another's already-deployed content. That correct-but-conservative choice means a gallery removed locally has never been automatically removed from the remote; there was previously no way to remove a single remote gallery at all, scoped or otherwise.

## Decision

**Rename** (`PATCH /api/channels/{slug}/galleries/{postID}`) changes only the stored title, never the slug — the slug (and therefore the URL) stays immutable after creation by the same design already established for the `Unlisted` flag (see the [Unlisted galleries](../features/done/2026-08-02-unlisted-galleries.md) feature doc): changing a slug after the fact would break any link already shared. After updating the title in `site.json`/`gallery.json`, the handler reuses the existing regeneration logic — `rebuildSiteChannel` (extracted from the "Rebuild site" handler) for site-export, `regenerateGalleryFolder` (extracted from "Rebuild galleries") for gallery-export — rather than duplicating the HTML-generation/photo-pruning logic a third time.

**Delete** (`DELETE /api/channels/{slug}/galleries/{postID}`) always removes the statefile entry and the local output folder. It additionally accepts an opt-in `{"deleteRemote": true}` body flag that, for `rsync`-handler channels only, runs a new `deploy.DeleteRemote(target, remoteSubpath)` — the first remote-exec-over-SSH capability in this codebase that removes something (as opposed to `TestConnection`'s harmless `true` and `fixRemotePermissions`'s `chmod`). It is scoped to exactly one album's subfolder (`rm -rf -- <RemotePath>/<subpath>`), never the whole `RemotePath` — matching the sharing concern that ruled out rsync `--delete` in the first place. `remoteDeleteCommand` rejects an empty subpath and any `.`/`..`/empty path segment before the string is ever built, so a malformed or malicious `postID`/slug can't expand into deleting more than the one intended folder. A failed remote delete is reported back to the caller as `remoteDeleteError` but never rolls back the local delete that already succeeded — the two are independent operations by design, since "gone from the local overview" and "gone from a remote host reachable only over SSH" are different guarantees.

## Consequences

**Positive:**
- Renaming a gallery no longer requires manual file editing, and reuses (rather than reimplements) the existing, already-tested regeneration pipeline for both export types.
- Deleting a gallery now has a real "take it fully offline" option, closing the gap left by `Deploy` never using `--delete`.
- The remote-delete path is opt-in per request and scoped to one folder — a user who wants the conservative, local-only behavior from before gets exactly that by leaving `deleteRemote` unset.

**Negative / Trade-offs:**
- This is the first place the server executes a *destructive* remote command over SSH on the user's behalf, rather than a read-only check or a permission fix — worth calling out explicitly for anyone auditing what this server can do to a remote host it has credentials for.
- Local delete and remote delete are not transactional: a local delete always happens first and always succeeds or fails independently of the remote attempt, so a failed remote delete leaves the gallery gone from the local overview but still live on the remote host until the next manual attempt or a full redeploy.
- Rename intentionally cannot change Unlisted or the slug — a title-only edit is a smaller feature than a full "edit gallery" form, but matches the immutability guarantee already established for Unlisted and avoids reopening that design decision here.
