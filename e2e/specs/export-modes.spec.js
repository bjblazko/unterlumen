/**
 * Export modes — verifies export works in all four UI contexts:
 *   browse mode, library detail, in-library filter, cross-library search.
 *
 * API tests use actual pathHints (absolute paths) from the library search API
 * to confirm the backend fix for "invalid path" errors.
 * UI smoke tests confirm the tools-menu → export-modal path is wired up in
 * search-result panes (which previously had no onToolInvoke callback).
 */

import path from 'path';
import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';
import { GPS_IMAGE, NO_GPS_IMAGE, GPS_PATH, NO_GPS_PATH, navigateToFolder } from '../helpers/fixtures.js';

const EXAMPLES_PATH = path.resolve(new URL('../fixtures/photos', import.meta.url).pathname);

// ─── Scale / EXIF payloads ───────────────────────────────────────────────────

const SCALE_50PCT  = { mode: 'percent', percent: 50 };
const SCALE_2048W  = { mode: 'max_dim', maxDimension: 'width', maxValue: 2048 };

// ─── Helpers ─────────────────────────────────────────────────────────────────

async function runSaveExport(request, files, { sourcePath = '', scale = {}, exifMode = 'keep', format = 'jpeg' } = {}) {
    const res = await request.post('/api/export/save', {
        data: { files, format, quality: 85, scale, exifMode, destination: '/tmp', sourcePath },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(Array.isArray(body.results)).toBe(true);
    expect(body.results.length).toBe(files.length);
    for (const r of body.results) {
        expect(r.error ?? '').toBe('');
        expect(r.success).toBe(true);
    }
}

async function runZipExport(request, files, { sourcePath = '', scale = {}, format = 'jpeg' } = {}) {
    const streamRes = await request.post('/api/export/zip-stream', {
        data: { files, format, quality: 85, scale, exifMode: 'keep', sourcePath },
    });
    expect(streamRes.status()).toBe(200);

    const body = await streamRes.text();
    const events = body
        .split('\n')
        .filter(l => l.startsWith('data:'))
        .map(l => JSON.parse(l.slice('data:'.length).trim()));
    const complete = events.find(e => e.complete);
    expect(complete?.token).toBeTruthy();

    const dlRes = await request.get(`/api/export/zip-download?token=${complete.token}`);
    expect(dlRes.status()).toBe(200);
    expect(dlRes.headers()['content-type']).toContain('application/zip');
    const buf = await dlRes.body();
    // An empty ZIP is 22 bytes; a ZIP with real image content is several KB at minimum.
    expect(buf.length).toBeGreaterThan(1_000);
}

// ─── Test suite ──────────────────────────────────────────────────────────────

test.describe('Export modes', () => {
    let libID;
    let libSourcePath;
    let absPath1, absPath2;

    test.beforeAll(async ({ request }) => {
        test.setTimeout(240_000);

        // Clean up any leftover test library from a previous run.
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(
            existing
                .filter(l => l.name === 'E2E Export Modes Library')
                .map(l => request.delete(`/api/library/${l.id}`))
        );

        const res = await request.post('/api/library/', {
            data: { name: 'E2E Export Modes Library', description: '', sourcePath: EXAMPLES_PATH },
        });
        expect(res.status()).toBe(201);
        const lib = await res.json();
        libID = lib.id;
        libSourcePath = lib.sourcePath;

        await reindexLibrary(request, libID);

        // Fetch two known absolute pathHints to use in API tests.
        const searchRes = await request.get(`/api/library/search?ids=${libID}&limit=2`);
        expect(searchRes.status()).toBe(200);
        const { results } = await searchRes.json();
        expect(results.length).toBeGreaterThanOrEqual(2);
        absPath1 = results[0].pathHint;
        absPath2 = results[1].pathHint;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    // ── A1: browse — save, 50% scale, strip EXIF ─────────────────────────────

    test('A1 browse: save 2 files at 50% scale, strip EXIF', async ({ request }) => {
        await runSaveExport(request, [GPS_PATH, NO_GPS_PATH], {
            scale: SCALE_50PCT, exifMode: 'strip',
        });
    });

    // ── B1: browse — zip, JPEG, max 2048px width ─────────────────────────────

    test('B1 browse: zip 2 files as JPEG, max 2048px width', async ({ request }) => {
        await runZipExport(request, [GPS_PATH, NO_GPS_PATH], {
            scale: SCALE_2048W,
        });
    });

    // ── A2/A3: library/cross-lib search — save, absolute paths, no sourcePath ─

    test('A2 library search (no sourcePath): save 2 absolute-path files at 50% scale, strip EXIF', async ({ request }) => {
        await runSaveExport(request, [absPath1, absPath2], {
            scale: SCALE_50PCT, exifMode: 'strip', sourcePath: '',
        });
    });

    test('A3 cross-library search (no sourcePath): zip 2 absolute-path files, max 2048px', async ({ request }) => {
        await runZipExport(request, [absPath1, absPath2], {
            scale: SCALE_2048W, sourcePath: '',
        });
    });

    // ── B2/B3: library/cross-lib search — zip, absolute paths, no sourcePath ─

    test('B2 library search (no sourcePath): zip 2 absolute-path files as JPEG, max 2048px', async ({ request }) => {
        await runZipExport(request, [absPath1, absPath2], {
            scale: SCALE_2048W, sourcePath: '',
        });
    });

    test('B3 cross-library search (no sourcePath): save 2 absolute-path files at 50% scale', async ({ request }) => {
        await runSaveExport(request, [absPath1, absPath2], {
            scale: SCALE_50PCT, exifMode: 'strip', sourcePath: '',
        });
    });

    // ── A4/B4: in-library filter — absolute paths with library sourcePath ─────

    test('A4 in-library filter (with sourcePath): save 2 absolute-path files at 50% scale, strip EXIF', async ({ request }) => {
        await runSaveExport(request, [absPath1, absPath2], {
            scale: SCALE_50PCT, exifMode: 'strip', sourcePath: libSourcePath,
        });
    });

    test('B4 in-library filter (with sourcePath): zip 2 absolute-path files as JPEG, max 2048px', async ({ request }) => {
        await runZipExport(request, [absPath1, absPath2], {
            scale: SCALE_2048W, sourcePath: libSourcePath,
        });
    });

    // ── UI smoke: tools menu → export modal opens in each mode ───────────────
    //
    // These tests verify the frontend onToolInvoke wiring (our fix to
    // library.js / library-filter.js) by confirming the export modal appears
    // after clicking the tool button from search-result panes.

    test('UI browse: select 2 photos and open export modal', async ({ page }) => {
        await page.goto('/');
        await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 2);

        await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
        await page.locator(`[data-name="${NO_GPS_IMAGE}"]`).click({ modifiers: ['Meta'] });

        await page.locator('.tools-menu-btn').click();
        await page.locator('button.tool-item[data-tool="export"]').click();
        await expect(page.locator('.export-modal')).toBeVisible({ timeout: 5_000 });
        await page.keyboard.press('Escape');
    });

    test('UI library detail: select 2 photos and open export modal', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Export Modes Library' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });

        // Navigate into folder-b inside the library pane.
        await page.waitForSelector('#lib-pane [data-type="dir"]', { timeout: 15_000 });
        await page.locator('#lib-pane [data-name="folder-b"]').dblclick();
        await page.waitForSelector('#lib-pane [data-type="image"]', { timeout: 15_000 });

        const images = page.locator('#lib-pane [data-type="image"]');
        await images.nth(0).click();
        await images.nth(1).click({ modifiers: ['Meta'] });

        await page.locator('#lib-pane .tools-menu-btn').click();
        await page.locator('#lib-pane button.tool-item[data-tool="export"]').click();
        await expect(page.locator('.export-modal')).toBeVisible({ timeout: 5_000 });
        await page.keyboard.press('Escape');
    });

    test('UI in-library filter: photos from filter results open export modal', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Export Modes Library' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });

        // Open the in-detail filter panel and wait for search results.
        await page.locator('#lib-filter-btn').click();
        await page.waitForSelector('#lib-search-panel.visible', { timeout: 5_000 });
        await page.waitForSelector('.search-breadcrumb', { timeout: 15_000 });
        await page.waitForSelector('#lib-search-pane [data-type="image"]', { timeout: 15_000 });

        const images = page.locator('#lib-search-pane [data-type="image"]');
        await images.nth(0).click();
        await images.nth(1).click({ modifiers: ['Meta'] });

        await page.locator('#lib-search-pane .tools-menu-btn').click();
        await page.locator('#lib-search-pane button.tool-item[data-tool="export"]').click();
        await expect(page.locator('.export-modal')).toBeVisible({ timeout: 5_000 });
        await page.keyboard.press('Escape');
    });

    test('UI cross-library search: photos from list-view search open export modal', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        // Open the cross-library search panel (list view search).
        await page.locator('#lib-search-btn').click();
        await page.waitForSelector('#lib-search-panel.visible', { timeout: 5_000 });
        await page.waitForSelector('.lib-search-select', { timeout: 20_000 });

        // Scope to our test library to ensure results appear.
        await page.locator('.lib-search-select').first().selectOption(String(libID));
        await page.waitForFunction(
            () => {
                const el = document.querySelector('.lib-search-status');
                return el && el.textContent.includes('match');
            },
            { timeout: 15_000 },
        );
        await page.waitForSelector('#lib-search-results-area [data-type="image"]', { timeout: 15_000 });

        const images = page.locator('#lib-search-results-area [data-type="image"]');
        await images.nth(0).click();
        await images.nth(1).click({ modifiers: ['Meta'] });

        await page.locator('#lib-search-results-area .tools-menu-btn').click();
        await page.locator('#lib-search-results-area button.tool-item[data-tool="export"]').click();
        await expect(page.locator('.export-modal')).toBeVisible({ timeout: 5_000 });
        await page.keyboard.press('Escape');
    });
});
