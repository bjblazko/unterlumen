# E2E Integration Tests with Playwright

*Last modified: 2026-04-20*

## Summary

Adds a Playwright-based end-to-end test suite that covers core user flows: browse, image viewer, EXIF overlay badges, and the wastebin mark/restore workflow. Tests run headlessly and are fully integrated into GitHub Actions CI.

## Details

- **Framework**: Playwright (Node.js), Chromium only.
- **Test directory**: `e2e/` at the repository root.
- **Fixtures**: Royalty-free sample images downloaded by `e2e/fixtures/setup.sh` (not committed). Sources: `ianare/exif-samples` (MIT) for JPEGs, `strukturag/libheif` (Apache 2.0) for a HEIC sample.
- **Server**: Tests spin up the compiled `./unterlumen` binary on port 8082 via Playwright's `webServer` config, pointing at the `e2e/fixtures/` directory.
- **CI workflow**: `.github/workflows/e2e.yml` — triggers on every push and pull request, builds the binary, downloads fixtures, installs Playwright, runs tests, and uploads the HTML report as an artifact.

**Test specs:**
- `api.spec.js` — HTTP contract tests (config, browse, thumbnail, image, info, tools/check, path traversal)
- `browse.spec.js` — Directory listing, thumbnail rendering, subdirectory navigation, view mode switching, selection
- `viewer.spec.js` — Fullscreen viewer open/close, prev/next navigation, keyboard shortcuts, mark-for-deletion
- `overlays.spec.js` — Async GPS badge, HEIF badge, info panel with/without location data
- `wastebin.spec.js` — Mark for deletion, switch to wastebin mode, select, restore (never deletes)

## Acceptance Criteria

- [x] `cd e2e && npm ci && npm run setup && npm test` passes locally with all specs green
- [x] GitHub Actions `E2E Tests` workflow passes on push/PR
- [x] HEIC tests are skipped gracefully if ffmpeg is unavailable
- [x] No test ever calls `POST /api/delete` or clicks `#wb-delete`
- [x] Test fixtures are gitignored and downloaded on demand
- [x] Playwright HTML report is uploaded as a CI artifact on every run
