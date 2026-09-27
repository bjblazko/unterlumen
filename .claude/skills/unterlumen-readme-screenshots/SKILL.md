---
name: unterlumen-readme-screenshots
description: Keep the README's Screenshots section current. Use after a user-visible feature is finished or a place's interface changed noticeably (new place, new dialog, redesigned screen, new phone layout), and when the user says "update the screenshots", "refresh the README pictures" or "add a screenshot for …". Retakes the affected pictures with the Playwright script and adds or rewrites their README entries.
allowed-tools: Bash, Read, Edit, Write
---

# README screenshots

The README's `## Screenshots` section shows one picture per important feature. The pictures are WebP files in `doc/screenshots/`, made by a script — never by hand, never from the real installation.

- `e2e/screenshots/take.mjs` — starts a throwaway server (port 8097) on a copy of `src/examples` with the folders renamed (Travel, 2017, 2025, Odds and ends), seeds a library "Pictures", destinations and galleries through the API, captures, converts with `cwebp`, deletes everything temporary.
- `e2e/screenshots/shots.mjs` — the list of shots, in README order. Each has a `name` (the file name), optional `dark`, `storage` (localStorage before load), phone settings (`...PHONE`), and a `take(page, { base, lib })` that opens a place and puts it into the state worth showing.

## When a feature is finished

1. **Decide what the README must show.**
   - A new place, dialog or clearly visible capability → a new shot and a new README entry.
   - A changed place → retake its existing shots (a changed Folders grid touches `folders`, `folders-dark`, `phone-folders`; the sidebar touches all of them).
   - Something invisible (a fix, a backend change) → nothing to do; say so and stop.

2. **Add or adjust the shot** in `shots.mjs`, following the ones there:
   - Reach the state the way a person would (clicks, keys) using the helpers `ready`, `openFolder`, `selectPhotos`, `openLibrary`, `selectionDialog`, `settle`.
   - Light theme. Only `folders-dark` shows the dark theme; do not add other dark shots unless the feature is about theming.
   - Show the feature with real content: photos selected, a dialog filled in, a panel open. Avoid focus rings, hover states and half-filled forms (blur the active element before capturing).
   - If the feature needs data the seed does not create, extend `seed()` in `take.mjs` through the API rather than clicking it together.

3. **Build and capture.**
   ```bash
   cd src && go build -o ../unterlumen . && cd ../e2e && npm run screenshots -- <name> [<name> …]
   ```
   Without names it retakes all of them (about 1–2 minutes). It needs `cwebp` and a network connection (map tiles).

4. **Look at every picture you made.** Convert each to PNG and read it:
   ```bash
   S=<scratchpad>/shots; mkdir -p $S
   for f in <names>; do dwebp -quiet doc/screenshots/$f.webp -o $S/$f.png; sips -Z 1440 $S/$f.png >/dev/null; done
   ```
   Check: thumbnails and map tiles fully loaded; no temporary path (`/private/var/…`, `unterlumen-shots-`) — `showHomePath` turns it into `~/Pictures`; the selected item is in view; nothing unreadable (dark text on the viewer's dark panel was a real bug found this way — fix such bugs in the app, not in the picture). Retake until it is right.

5. **Write the README entry** under `## Screenshots`, in the order of `shots.mjs`:
   - `#### Heading` in sentence case, named after the place or feature as the UI names it (Destination, Gallery, Publish — ADR-0029).
   - One to three plain sentences on what the picture shows and what you can do there. Specifics, no superlatives, no exclamation marks.
   - `![Alt text that describes the picture](doc/screenshots/<name>.webp)`. Several related pictures go side by side in a table; phone pictures use `<img … width="320">`.
   - Update the matching bullet under `## Features` if the feature is new.

6. **Check that nothing is orphaned:**
   ```bash
   for f in doc/screenshots/*.webp; do grep -q "$f" README.md || echo "unused: $f"; done
   grep -o 'doc/screenshots/[a-z-]*\.webp' README.md | sort -u | while read f; do [ -f "$f" ] || echo "missing: $f"; done
   ```
   Delete unused pictures and their shots.

7. **Report** which pictures changed and show the user the ones that are new. Do not commit unless asked; when asked, stage the exact files (`doc/screenshots/<name>.webp`, `README.md`, `e2e/screenshots/*.mjs`), never a whole directory.

## Not in scope

The product page on huepattl.de has its own copies of the pictures and its own skill in that repository. Mention to the user that the website may want the new pictures too, but do not edit `../huepattl.de` from here.
