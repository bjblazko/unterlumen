# 0026. Deferred publish drafts replace immediate build

*Last modified: 2026-09-06*

## Status

Accepted

## Context

The previous "Build" action exported photos and generated gallery/site HTML
in a single, immediate step, with no way to preview the result before an
optional later Deploy. Collecting photos into a channel and publishing them
were the same action, which made it impossible to build up a gallery
incrementally over multiple sessions without repeatedly regenerating and
re-deploying partial content.

## Decision

Introduce a `drafts.json` per channel (`internal/channels.DraftStore`)
holding pending `{libraryID, photoID}` references with no files written.
Collecting photos into a channel only touches this draft. A separate
Generate action (the old build pipeline, refactored to read from a draft)
performs the actual export and HTML generation; deploy remains a further,
optional, separate step for handler-backed channels. A photo's pending vs.
published state is tracked via a `pending:<slug>` library-meta key, written
on collect and replaced by the existing `built:<slug>` key on generate,
reusing the mechanism the Info Panel's Publications card already read.

## Consequences

- Channel output directories are only ever written to by Generate, never by
  collect — makes it safe to add and remove photos from a pending gallery
  freely before committing to a build.
- A draft can span multiple libraries (each collect call is scoped to one
  library, like the old build action was); Generate opens one `*lib.Store`
  per distinct library referenced by the draft.
- The old immediate `POST /api/library/{id}/build` endpoint and its
  `Rebuild`/`Rebuild site`/`Albums` per-channel actions are superseded by
  Generate (with zero pending photos, Generate is the new "Rebuild"). The
  Go handlers backing the old maintenance-only `rebuild-site`/
  `rebuild-galleries` routes are left in place as unreferenced code pending a
  follow-up cleanup, since removing working backend code is out of scope for
  this UI-focused change.
