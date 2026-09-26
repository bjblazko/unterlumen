# One way to show activity, and a status line

*Last modified: 2026-09-26*

## Summary

Unterlumen showed that something was happening in about 37 places, with seven different means. About 30 other waits showed nothing, and batch rename invented its progress. Now one component says what is happening:
- busy: in words, with the elapsed time after 3 s;
- counted: a bar and "x of y" when the total is known;
- ended: a sentence.

Nothing shows for quick answers. Long work also appears in a status line at the foot of the sidebar, which stays in view when you move to another place.

The owner chose this on 2026-09-26 as option B out of three: "inline where it was started, plus a status line". The other two were "only inline" (option A) and "every write in a progress dialog" (option C). See [ADR-0036](../../architecture/adr/0036-activity-and-progress.md).

## Details

**Frontend**
- `src/web/js/activity.js` — `Activity` provides:
  - `in(container, text, {area})`, `busy`, `count(done, total, {noun, current})`, `done`, `fail` and `clear`;
  - `Activity.button(btn, verb)` and `Activity.errorList(lines)`.

  It waits 400 ms before showing anything and adds the elapsed time from 3 s. It has `role="status"`; the live region stays quiet while counting, and the bar is a `role="progressbar"`.
- `src/web/js/status-line.js` — `StatusLine` reads `/api/jobs/stream` through `EventSource`, which reconnects on its own.
  - Each job is a link to its place; at most three are shown, then "and N more".
  - A finished job stays 8 s; a failed one stays until it is clicked.
  - The collapsed rail shows a ring per running job and a disc per failed one.
- Moved onto `Activity`:
  - the folder view, filter panel and first results, info panel and statistics with its timeline;
  - the folder picker, galleries, the collected photos, publish, destinations and the collect dialog;
  - library cards, library maintenance, creating a library, the export dialog, the progress dialog and batch rename (preview and run).
- New feedback:
  - filter results that are being replaced fade (`is-stale`), and a failed search says so and keeps them faded;
  - changing the filter's library;
  - a slow or failed photo in the viewer, and the next page of results;
  - collecting folders for a slideshow or a gallery (`App.whileSlow`);
  - Organize's mkdir and undo;
  - saving and deleting a destination, deleting a library, renaming a gallery and clearing the cache, all without double submit.
- Removed:
  - `.browse-spinner`, `.lib-results-spinner(-wrap)`, `.progress-bar-*`, `.progress-status/-detail/-errors` and `.export-progress-*`;
  - `.library-loading`, `.channel-loading`, `.stats-loading`, `.gal-loading`, `.lib-filter-loading` and `.tools-menu-loading`;
  - `@keyframes spin` and `export-progress-slide`.
- The folder tools' progress moved from the 3-second hint into the status line.
- `imageCountLabel()` is the one place that writes "N image(s) · M selected".

**Backend**
- `internal/jobs`:
  - `Registry`, `Job` and `Handle` (`Progress`, `Step`, `Finish`; a nil handle is safe);
  - `Subscription`, which merges changes per job and never loses the end;
  - finished jobs stay for 60 s.
- `internal/api/jobs`: `GET /api/jobs/stream` (SSE, at most one batch per 150 ms), and `Track`, which fails a job on an error status or on `"ok": false`.
- How each kind of work reports:
  - Library jobs report through `library.Broadcaster.Send`, and `Manager.StartScan(id, verb)` names the job.
  - ZIP export and publishing mirror their SSE events.
  - Rebuilding the site, galleries and album list, renaming or unpublishing a gallery, and deploying are wrapped with `Track`.
- A library scan closed without its last event ends as failed.

## Acceptance Criteria

- [x] No spinner or looping animation left; one bar component, one busy style.
- [x] Nothing shows for answers under 400 ms; slow ones say what they are doing and for how long.
- [x] Counts appear only where the server reports them; batch rename shows none.
- [x] A finished activity says how it ended and hides the bar; a failure says why and what to do.
- [x] Buttons that start work are disabled with a verb until it ends.
- [x] Filter results being replaced fade instead of emptying; a failed search keeps them faded and says why.
- [x] Library scans, exports, publishing, rebuilds and deploys appear in the status line and stay there on another page.
- [x] A background scan after a move appears in the status line.
- [x] A failed job stays until clicked; the collapsed rail shows a ring per running job and a disc per failed one.
- [x] Go tests for the register, the subscription, `Track` and the scan bridge; e2e spec `e2e/specs/activity.spec.js`.

## Not included

- Cancelling work on the server (library jobs still run on `context.Background()`).
- Moving the four existing SSE writers onto one helper.
- A library card that starts showing a job started elsewhere while the overview is already open; it picks the job up the next time the list loads.
- The status line on a phone, where the sidebar is hidden and only browsing is possible.
