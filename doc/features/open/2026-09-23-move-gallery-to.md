# Move gallery to…

*Last modified: 2026-09-23*

## Summary

Move a published gallery from one destination (channel) to another without
breaking the links already shared for it. This merges the per-album channels
that all point at the same host — the workaround from before
[ADR-0027](../../architecture/adr/0027-per-album-publication-meta-keys.md) —
into a single destination holding many galleries, which is what
[ADR-0029](../../architecture/adr/0029-destinations-galleries-one-publish-action.md)
calls a Destination.

Follow-up to the [Rams redesign](2026-09-23-rams-redesign.md). The
[many-galleries-per-channel](../done/2026-09-20-many-galleries-per-channel.md)
doc explicitly excludes migration, so the move needs its own design.

## Details

### The situation

Five channels — "Pauls 2. Geburtstag", "Julias 66. Geburtstag", "AIDA UK &
Schottland 2026", "Andys 60. Geburtstag" and "Fotoshare" — are configured
against the same rsync target on `fotos.boewingnoebel.de`. Each holds exactly
one album, because one album per channel was the only thing that worked at
the time. Since ADR-0027 a single channel can hold many albums, so the five
should be one destination with five galleries.

The links that were shared with family are
`https://fotos.boewingnoebel.de/<postID>/`. The `postID` is the album's
24-hex folder and is independent of the channel slug, so **the public URL does
not change when the gallery changes channel — as long as both channels write
to the same `remotePath`.** That is the whole reason this is feasible.

### What a move touches

For a gallery with album `postID` moving from channel slug `A` to slug `B`:

| Thing | Change |
| --- | --- |
| `gallery.json` | The album entry moves from A's to B's statefile, including `title`, `publishedAt`, `unlisted`, and the `generatedAt`/`deployedAt` added by ADR-0029. |
| Output folder | `<output>/A/<postID>/` → `<output>/B/<postID>/`, a local directory rename ([ADR-0016](../../architecture/adr/0016-global-channel-output.md)). |
| Library meta keys | `built:A:<postID>`, `built:A:<postID>:title` → the same keys under `B`. The channel marker `built:A` is dropped only when A has no albums left; `built:B` is added. The unqualified convenience keys `built:A:title` / `built:A:postid` are recomputed for both channels from their remaining albums. |
| XMP sidecars | The same rewrite in each affected photo's `Publication` entries, so a re-index does not resurrect the old slug. |
| Drafts | A pending draft for that album in A's `drafts.json` moves to B's, keeping its `Target.PostID`. |
| Remote | Nothing. Same host, same `remotePath`, same folder name. |

### Preconditions

A move is offered only when both channels have the same export mode
(`galleryExport`), the same `remotePath` and the same handler. Anything else
would change the public URL, and this feature exists precisely to not do that.
Site albums (`siteExport`) are out of scope: their slug encodes the album path,
so moving one *does* change its URL.

### Flow

1. Galleries screen → gallery detail → "Move to…" (under Advanced).
2. A list of eligible destinations, each with the reason ineligible ones are
   greyed out ("different remote path", "different type").
3. A confirmation naming what happens in plain words, including that the
   public link stays the same and that the source destination will be left
   empty (with an offer to delete it when it is).
4. Progress inline, per step. Errors inline, no `alert()`.
5. Afterwards: a reachability check for the moved gallery's URL, so a mistake
   is visible immediately.

### Safety

- The move is metadata-only and local. Nothing is uploaded and nothing is
  deleted remotely.
- Steps run in an order that is safe to interrupt: write B's statefile, then
  rename the folder, then rewrite meta and sidecars, then remove A's entry. A
  crash between steps leaves the album reachable under its URL throughout.
- A dry-run summary (counts of photos, meta keys and sidecars to rewrite) is
  shown before anything is written.

## Acceptance Criteria

- [ ] "Move to…" appears in gallery detail for `galleryExport` albums only,
      and lists only destinations with the same `remotePath` and handler,
      with a reason shown for the ineligible ones.
- [ ] After a move the public URL is unchanged and the gallery still resolves
      (verified by the post-move reachability check).
- [ ] `gallery.json` of both channels, the output folder, the
      `built:<slug>:<postID>` meta keys and the XMP sidecars all name the new
      slug; nothing still names the old one.
- [ ] A pending draft for the moved album follows it, keeping its
      `Target.PostID`.
- [ ] A source destination left with no galleries offers deletion but is
      never deleted automatically.
- [ ] Re-indexing ("Rebuild metadata") after a move does not reintroduce the
      old slug.
- [ ] The five `fotos.boewingnoebel.de` channels can be merged into one
      destination, and every previously shared link still works.
- [ ] Go tests cover the statefile and meta-key rewrite, including the
      "last album leaves the channel" case; an e2e spec covers the move flow.
- [ ] CHANGELOG entry under Unreleased.
