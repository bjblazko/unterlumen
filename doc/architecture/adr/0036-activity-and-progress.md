# ADR-0036: One way to show activity, and a status line for work that outlives its page

*Last modified: 2026-09-26*

## Status

Accepted. Applies [ADR-0030](0030-rams-design-tokens.md) (tokens, `--time`) and
[ADR-0033](0033-dialogs-and-places.md) (what blocks, what stays inline).

## Context

About 37 places showed that something was happening, with seven different means:
two identical spinners of different sizes, two progress bars (4 px and 3 px),
the publish step list, a toast that hid itself after 3 s while a scan went on,
eight "Loading…" classes in three sizes and two greys, and disabled buttons with
or without a verb. About 30 other waits showed nothing, including saves that
could be sent twice. Batch rename showed "N of M files" and a bar driven by a
timer while the server reported nothing — invented progress.

The server reported real progress for library jobs, ZIP export, publishing and
the link check, each over its own SSE stream with its own event shape. Work that
runs for minutes (a scan, a publish, a site rebuild, a deploy) was visible only
on the page that started it; leaving the page hid it, and a scan started in the
background after a move was visible nowhere.

## Decision

**One component, three levels chosen by what is known, and one status line for
work that outlives the page that started it.**

### The component (`src/web/js/activity.js`)

- **Busy** — a sentence that says what is happening ("Reading the folder…",
  "Counting the photos…"). After 3 s the elapsed seconds follow in
  `--time-ink` ("· 12 s"). No spinner, no looping animation.
- **Counted** — when the total is known: a 4 px bar (`--time` on `--line`),
  "412 of 1 280 photos" with tabular numerals, and the current file in mono.
- **Ended** — a sentence that says how it ended. The bar goes: yellow means time
  passing, and it has passed. A failure is `--warning-ink`, says what happened
  and what to do next, and lists at most five failed items.
- Nothing shows for the first **400 ms**, so a quick answer never flickers.
- `role="status"`; the live region is quiet while counting (the bar carries the
  count as `role="progressbar"`) and speaks the end. The host gets `aria-busy`.
- `Activity.button(btn, verb)` disables the button that started the work and puts
  the verb on it ("Saving…"), so one click is one request.
- Results that a newer filter is replacing stay in view and fade after the same
  delay (`is-stale`), instead of emptying the area; they stay faded when the new
  search fails.
- Never invented: where the server reports nothing (batch rename), the component
  says what it is doing and not how far it is.

### The status line (`src/web/js/status-line.js`)

- At the foot of the sidebar, below Settings, above a hairline, without a surface
  of its own. Nothing is reserved while nothing runs.
- Shows work that outlives its page: library scans (including background scans
  after a move), ZIP exports, publishing, site/gallery/album-list rebuilds,
  gallery renames and unpublishing (they rebuild the site), and deploys. Reads
  are never shown there.
- Each job is a link to its place (`#libraries`, `#galleries`, `#destinations`),
  titled with what and which ("Scanning \"Archive\""), with the component below.
  At most three are shown, then "and N more".
- A job appears after 400 ms; a finished one stays 8 s; a failed one stays until
  it is clicked. A job that finished before anyone saw it is not announced.
- The collapsed rail shows a mark per running or failed job: a `--time` ring
  while it runs, a `--warning` disc when it failed — shape as well as colour —
  with the job's text as its tooltip and accessible name. A finished job leaves
  the rail.
- The place that started the work still shows it (the library card, the publish
  steps, the export dialog); the status line mirrors it.

### The server (`internal/jobs`, `internal/api/jobs`)

- `jobs.Registry` holds `Job{id, kind, title, place, step, done, total, current,
  started, finished, ended, error}`. Producers report through a `*jobs.Handle`
  (`Progress`, `Step`, `Finish`); a nil handle does nothing.
- A subscriber gets every job, then each change. Changes to the same job are
  merged while the subscriber is busy, so a slow reader sees fewer steps but
  never misses how a job ended. `GET /api/jobs/stream` writes at most one batch
  per 150 ms. Finished jobs stay 60 s for a page that reconnects.
- Library jobs report through `library.Broadcaster.Send`, the one path all scan
  progress already took; export and publish mirror their own SSE events;
  request-long work is wrapped with `apijobs.Track`, which fails the job on an
  error status or an `"ok": false` answer (deploy).
- The existing SSE endpoints are unchanged. Their four hand-written SSE writers
  are left alone until they are touched for another reason.

## Consequences

- One vocabulary for waiting across the app; eight loading classes, two spinners,
  two bars and their keyframes are gone.
- Long work stays visible when you move on, and a background scan is no longer
  invisible.
- A failed library scan no longer reads "Done — 0 photos": an error arrives with
  `finished`, and it is now checked first.
- Creating a library whose first index fails no longer invites a second library:
  the dialog offers to open the one that was created.
- Nothing can be cancelled on the server yet; library jobs still run on
  `context.Background()`. Cancel is a separate decision.
- **Phone:** the sidebar is hidden on a phone and the phone only browses, so the
  status line is not shown there.
