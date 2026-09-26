# ADR-0037: Backend packages by domain — library, publish, site

*Last modified: 2026-09-26*

## Status

Accepted. Applies [ADR-0015](0015-coding-standards.md) (single responsibility,
domain grouping, a directory past ~8–10 files wants a split).

## Context

`internal/api/library` had grown to 34 files and about 10 000 lines. Besides the
library handlers it held everything about publishing — drafts, generating
galleries and sites, rebuilding them, the published-galleries overview, deploy
stamps — and the generation of the static output itself: templates, assets,
navigation, sitemap, slugs, the state files and the shared album register
([ADR-0035](0035-shared-album-register.md)). One file, `handler.go`, was 2 865
lines long. A change to the site's HTML and a change to how photos are listed
landed in the same package, and nothing kept publishing code from reaching into
library internals or the other way round.

Five handlers in three packages each set up server-sent events their own way.

## Decision

Split by domain into three packages, with imports in one direction only:

| Package | Holds | Imports |
|---|---|---|
| `internal/api/library` | Libraries, indexing, photo queries and filters, thumbnails and photos, photo info, metadata | `publish` |
| `internal/api/publish` | Drafts, generating galleries and sites, rebuilding them, the published-galleries overview, reachability, deploy stamps, taking a photo off a destination | `site` |
| `internal/site` | The static output: single-gallery pages and multi-album sites, their assets, navigation, sitemap, slugs, state files and the album register. No HTTP | — |

- `apilibrary.Handle` registers `publish.Handle`, so `routes.go` and every URL
  stay as they were.
- Removing a `pending:` or `built:` meta key is publishing work; the library's
  metadata handler hands those keys to `publish.DeletePendingMeta` and
  `publish.DeleteBuiltMeta`.
- `internal/api/sse` opens every event stream (`Start`) and writes every JSON
  data event (`Send`).
- Inside `site`, names drop the package's own word: `site.Album`, `site.Photo`,
  `site.Store`, not `site.SiteAlbum`.

## Consequences

- Each package can be read on its own; the largest file in the three is under
  750 lines.
- `library` depends on `publish` for the two meta-key operations. If publishing
  ever needs library handlers, that is a cycle and a sign the code sits in the
  wrong package.
- Test helpers that both API packages need (`seedLibraryPhoto`,
  `writeTestJPEG`, `photoMeta`) exist once per package; test code cannot be
  shared across packages without a package of its own, which three small
  helpers do not justify.
- `writeJSON` has one copy in `library` and one in `publish`.
