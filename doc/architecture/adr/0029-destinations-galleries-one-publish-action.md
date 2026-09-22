# ADR-0029: Destinations and galleries; publish as one action

*Last modified: 2026-09-23*

## Status

Accepted

## Context

"Channel" carries five meanings at once: export format, web address, website configuration, upload method, and — since one gallery per channel was the only thing that worked before [ADR-0027](0027-per-album-publication-meta-keys.md) — album. The practical result is five channels pointing at the same rsync host, one per birthday.

Three further problems come from the same overload:

- **States and actions are mixed.** The "Published" tab contains drafts. "Publish" sits on every live row even when nothing is pending — and pressing it re-dates the gallery, because the publish dialog defaults "Updated date" to today.
- **One intent, four words.** Generate, Deploy, Publish and Build all describe parts of "make the online version equal to what I see". The split into Generate → review → Deploy exists because [ADR-0026](0026-deferred-publish-drafts.md) deferred the build, but that decision was about *collecting* photos lazily, not about making the user drive the build pipeline by hand.
- **Reachability reads as a state.** "Unreachable" appears in the same slot as Live and Draft, with no time and no cause, although it is the result of a check that can be true of a gallery in any state.

`LastDeployedAt`/`LastDeployOK` live on the channel, so a gallery that was built but whose upload failed cannot be told apart from one that was never built.

## Decision

**1. Split the concept in the UI. Backend names stay.**

| UI term | Meaning | Backend |
| --- | --- | --- |
| Destination | Where and how: type, address, upload, image format | `channel` (`galleryExport`, `siteExport`, plain export) |
| Gallery | What: a titled set of photos at one destination | album / draft / `gallery.json` entry |

`channel`, `galleryExport`, `siteExport` and `drafts.json` keep their names in Go, in the API and on disk. Only human-readable strings change: "Channel" → "Destination", the "Published" tab → "Galleries" (grouped by destination), "Add to channel…" → "Add to gallery…".

**2. States are adjectives, actions are verbs.**

| State | Meaning |
| --- | --- |
| Not online yet | Collected, never published |
| Online | Uploaded, and the link answered at the last check |
| Changes not online | Online, with collected additions or removals still missing |
| Built, not uploaded | Generate succeeded, upload missing or failed |
| Exported to folder | Files destination — the files are ready; posting them is the user's job |

"Not reachable · &lt;time&gt;" is a **check result shown on top of the state**, never a state of its own: a gallery can be "Changes not online" *and* "Not reachable". It names the time and, where known, the cause, and offers "Check again". A guessed address is labelled as guessed. The sweep runs once when the Galleries screen opens (cached with its timestamp) and after each publish for the affected URL only, not on every tab switch.

A row shows an action only when there is something to do ("Publish 3 changes", "Retry upload", "Show in Finder"). An Online gallery with nothing pending shows no publish button.

**3. Publish is one action.** One press runs: export photos → build HTML → upload (when an upload is configured) → check the link. Progress is determinate and driven by the existing `buildStream` SSE events; errors appear inline in the sheet instead of `alert()`. If the upload fails, the build is kept and the gallery becomes "Built, not uploaded", whose action is "Retry upload" (upload only). "Preview locally" remains an optional secondary action and is never a required step. "Rebuild" (generate with nothing pending, needed only after a theme or format change) moves to the destination under Advanced.

**4. Per-gallery `generatedAt` and `deployedAt`.** Both are persisted per gallery in `gallery.json` / `site.json`, alongside the existing channel-level `LastDeployedAt`/`LastDeployOK`. "Built, not uploaded" is `generatedAt > deployedAt` (or no `deployedAt` at all).

**5. The publish date is a property of the gallery.** It defaults to the gallery's existing date and is sent only when the user changes it. Publishing an unchanged gallery no longer re-dates it.

## Consequences

- The five same-host channels remain valid and keep their URLs; the Galleries screen groups them by destination so the workaround is at least legible. Merging them is a separate feature, [Move gallery to…](../../features/open/2026-09-23-move-gallery-to.md), because published links must not break.
- `gallery.json` and `site.json` gain two timestamp fields. Readers must treat both as optional: entries written before this change have neither, and are shown as Online (when a deploy is recorded on the channel) or Built, not uploaded (when it is not).
- The date fix is a behavior change with no schema change: the dialog stops defaulting to `new Date()`.
- Deploy is no longer separately triggerable from the UI for the normal path. The endpoint stays, since "Retry upload" uses it.
- Server-initiated egress for reachability drops sharply compared to [ADR-0024](0024-published-galleries-overview.md)'s sweep-on-every-open, at the cost of showing a cached result with its timestamp rather than a live one.
- [ADR-0026](0026-deferred-publish-drafts.md)'s collect-then-publish model is kept in full. What changes is only that the second phase is one action instead of three.
