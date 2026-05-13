import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import {
    GPS_EDIT_PATH, NO_GPS_EDIT_PATH,
    GPS_EDIT_IMAGE, NO_GPS_EDIT_IMAGE,
    navigateToFolder,
} from '../helpers/fixtures.js';

// All edits target fixtures/photos/ (copies of src/examples) — originals are never modified.
// GPS_EDIT_PATH and NO_GPS_EDIT_PATH are dedicated images used only by this spec.
// afterAll restores original GPS state so re-runs without `npm run setup` work correctly.

test.describe('GPS editing — set-location / remove-location APIs', () => {
    let exiftoolAvailable = false;

    test.beforeAll(async ({ request }) => {
        const res = await request.get('/api/tools/check');
        const tools = await res.json();
        exiftoolAvailable = !!tools.exiftool;
    });

    test.beforeEach(async ({}, testInfo) => {
        if (!exiftoolAvailable) testInfo.skip();
    });

    // Restore GPS state after the suite so subsequent runs don't start in a dirty state.
    test.afterAll(async ({ request }) => {
        if (!exiftoolAvailable) return;
        // Restore GPS_EDIT_PATH (should have GPS)
        await request.post('/api/set-location', {
            data: { files: [GPS_EDIT_PATH], latitude: 39.376, longitude: 3.333 },
        }).catch(() => {});
        // Restore NO_GPS_EDIT_PATH (should have no GPS)
        await request.post('/api/remove-location', {
            data: { files: [NO_GPS_EDIT_PATH] },
        }).catch(() => {});
    });

    // ── Add GPS to a non-GPS image ───────────────────────────────────────────

    test('POST /api/set-location adds GPS to a non-GPS JPEG', async ({ request }) => {
        const before = await (await request.get(`/api/info?path=${encodeURIComponent(NO_GPS_EDIT_PATH)}`)).json();
        expect(before.exif?.latitude == null).toBe(true);

        const res = await request.post('/api/set-location', {
            data: { files: [NO_GPS_EDIT_PATH], latitude: 51.5074, longitude: -0.1278 },
        });
        expect(res.status()).toBe(200);
        const body = await res.json();
        expect(body.results[0].success).toBe(true);

        const after = await (await request.get(`/api/info?path=${encodeURIComponent(NO_GPS_EDIT_PATH)}`)).json();
        expect(typeof after.exif?.latitude).toBe('number');
        expect(typeof after.exif?.longitude).toBe('number');
        expect(after.exif.latitude).toBeCloseTo(51.5074, 2);
    });

    test('GPS badge appears after adding GPS to an image', async ({ page, request }) => {
        await request.post('/api/set-location', {
            data: { files: [NO_GPS_EDIT_PATH], latitude: 51.5074, longitude: -0.1278 },
        });

        await page.goto('/');
        await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);

        const gpsBadge = page.locator(`[data-name="${NO_GPS_EDIT_IMAGE}"] .overlay-badge-gps`);
        await expect(gpsBadge).toBeVisible({ timeout: 10_000 });
    });

    // ── Remove GPS from a GPS image ──────────────────────────────────────────

    test('POST /api/remove-location removes GPS from a GPS JPEG', async ({ request }) => {
        const before = await (await request.get(`/api/info?path=${encodeURIComponent(GPS_EDIT_PATH)}`)).json();
        expect(typeof before.exif?.latitude).toBe('number');

        const res = await request.post('/api/remove-location', {
            data: { files: [GPS_EDIT_PATH] },
        });
        expect(res.status()).toBe(200);
        const body = await res.json();
        expect(body.results[0].success).toBe(true);

        const after = await (await request.get(`/api/info?path=${encodeURIComponent(GPS_EDIT_PATH)}`)).json();
        expect(after.exif?.latitude == null).toBe(true);
    });

    test('GPS badge disappears after removing GPS from an image', async ({ page, request }) => {
        await request.post('/api/remove-location', {
            data: { files: [GPS_EDIT_PATH] },
        });

        await page.goto('/');
        await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);

        await page.waitForTimeout(1_500);
        const gpsBadge = page.locator(`[data-name="${GPS_EDIT_IMAGE}"] .overlay-badge-gps`);
        await expect(gpsBadge).not.toBeVisible();
    });

    // ── Validation ────────────────────────────────────────────────────────────

    test('POST /api/set-location rejects out-of-range latitude', async ({ request }) => {
        const res = await request.post('/api/set-location', {
            data: { files: [NO_GPS_EDIT_PATH], latitude: 999, longitude: 0 },
        });
        expect(res.status()).toBe(400);
    });

    test('POST /api/set-location with empty files returns 400', async ({ request }) => {
        const res = await request.post('/api/set-location', {
            data: { files: [], latitude: 51.0, longitude: 0 },
        });
        expect(res.status()).toBe(400);
    });

    test('POST /api/remove-location with empty files returns 400', async ({ request }) => {
        const res = await request.post('/api/remove-location', {
            data: { files: [] },
        });
        expect(res.status()).toBe(400);
    });
});
