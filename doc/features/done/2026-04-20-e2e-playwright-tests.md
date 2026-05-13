# E2E Integration Tests with Playwright

*Last modified: 2026-05-12*

## Summary

Adds a Playwright-based end-to-end test suite that covers core user flows: browse, image viewer, EXIF overlay badges, and the wastebin mark/restore workflow. Tests run headlessly and are fully integrated into GitHub Actions CI.

## Details

- **Framework**: Playwright (Node.js), Chromium only.
- **Test directory**: `e2e/` at the repository root.
- **Fixtures**: `src/examples` (79 real-world images, 2004–2026, multiple cameras/lenses, GPS and non-GPS) is copied into `e2e/fixtures/photos/` by `npm run setup`. Copies are gitignored; originals in `src/examples` are never modified by tests.
- **Server**: Tests spin up the compiled `./unterlumen` binary on port 8082 via Playwright's `webServer` config, pointing at `e2e/fixtures/photos/`.
- **CI workflow**: `.github/workflows/e2e.yml` — triggers on every push and pull request, builds the binary, runs setup (copy), installs Playwright, runs tests, and uploads the HTML report as an artifact.

**Test specs:**
- `api.spec.js` — HTTP contract tests (config, browse, thumbnail, image, info, tools/check, path traversal)
- `browse.spec.js` — Directory listing, thumbnail rendering, view mode switching, selection
- `browse-navigation.spec.js` — Three-level folder navigation, mixed dir+image layout, breadcrumbs, status counts
- `thumbnails.spec.js` — JPEG/HIF thumbnail delivery, size parameter, justified/grid layout
- `viewer.spec.js` — Fullscreen viewer open/close, prev/next navigation, keyboard shortcuts, mark-for-deletion
- `overlays.spec.js` — Async GPS badge, HEIF badge, info panel with/without location data
- `wastebin.spec.js` — Mark for deletion, switch to wastebin mode, select, restore (never deletes)
- `commander.spec.js` — Dual-pane organiser mode, pane focus, keyboard navigation, action buttons
- `filmstrip.spec.js` — Film strip show/hide, deferred thumbnail loading, navigation sync
- `gps-editing.spec.js` — Add GPS coordinates to a non-GPS image, remove GPS from a GPS image, UI badge sync
- `export.spec.js` — Export estimate API, ZIP stream/download, UI dialog smoke test
- `crop.spec.js` — Crop API validation, successful crop, thumbnail invalidation
- `aspect-ratio.spec.js` — Justified/grid/list rendering across portrait, landscape, 4:3, 3:2 aspect ratios
- `library.spec.js` — Library card UI, detail view navigation, delete confirmation
- `library-search.spec.js` — EXIF range sliders, camera model filter, ISO filter, cross-library search API
- `statistics.spec.js` — Statistics modal UI, chart cards, library filter dropdown

## Acceptance Criteria

- [x] `cd e2e && npm ci && npm run setup && npm test` passes locally with all specs green
- [x] GitHub Actions `E2E Tests` workflow passes on push/PR
- [x] HIF/HEIC tests are skipped gracefully if ffmpeg is unavailable
- [x] GPS editing tests are skipped gracefully if exiftool is unavailable
- [x] No test ever calls `POST /api/delete` or clicks `#wb-delete`
- [x] Test fixtures are gitignored and recreated from `src/examples` on `npm run setup`
- [x] Originals in `src/examples` are never modified by tests
- [x] Playwright HTML report is uploaded as a CI artifact on every run
