import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// Tests for the five library scan operations:
//   scan-new, reindex, regen-previews-missing, regen-previews-all, cleanup

async function driveSSE(request, url, timeout = 60_000) {
    const res = await request.post(url, { timeout });
    expect(res.status(), `SSE POST ${url}`).toBe(200);
    const ct = res.headers()['content-type'];
    expect(ct, 'content-type should be SSE').toContain('text/event-stream');
    const text = await res.text();
    expect(text, 'SSE stream should signal completion').toContain('"finished":true');
    return text;
}

test.describe('Library scan operations', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(
            existing
                .filter(l => l.name === 'E2E Scan Ops')
                .map(l => request.delete(`/api/library/${l.id}`))
        );
        const res = await request.post('/api/library/', {
            data: { name: 'E2E Scan Ops', description: '', sourcePath: 'folder-a' },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    // ── API contracts ────────────────────────────────────────────────────────

    test('POST /api/library/{id}/scan-new returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/scan-new`);
    });

    test('POST /api/library/{id}/reindex returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/reindex`);
    });

    test('POST /api/library/{id}/regen-previews-missing returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/regen-previews-missing`);
    });

    test('POST /api/library/{id}/regen-previews-all returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/regen-previews-all`);
    });

    test('POST /api/library/{id}/cleanup returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/cleanup`);
    });

    test('POST /api/library/{id}/scan-new with subfolder scopes scan', async ({ request }) => {
        const text = await driveSSE(
            request,
            `/api/library/${libID}/scan-new?subfolder=a1`
        );
        expect(text).toContain('"finished":true');
    });

    // ── Library card UI ──────────────────────────────────────────────────────

    test('library card shows all five scan operations', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Scan Ops' });
        await expect(card).toBeVisible({ timeout: 8_000 });

        // Primary scan button
        await expect(card.locator('.lib-scan-new')).toContainText('Scan for new photos');

        // Expand the secondary scan operations via the toggle chevron
        await card.locator('.lib-scan-toggle').click();
        const scanMenu = card.locator('.lib-scan-menu');
        await expect(scanMenu).toBeVisible({ timeout: 3_000 });
        await expect(scanMenu.locator('.lib-reindex')).toContainText('Rebuild metadata & previews');
        await expect(scanMenu.locator('.lib-regen-missing')).toContainText('Generate missing previews');
        await expect(scanMenu.locator('.lib-rebuild-all')).toContainText('Rebuild all previews');
        await expect(scanMenu.locator('.lib-cleanup')).toContainText('Remove deleted photos');
    });

    // ── Browse Tools dropdown ─────────────────────────────────────────────────

    test('browse Tools dropdown in library mode shows all five scan operations', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        // Open the library into the detail / browse pane
        const card = page.locator('.library-card', { hasText: 'E2E Scan Ops' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('#lib-pane', { timeout: 8_000 });

        // Open the Tools menu in the library pane (same .tools-menu-btn used elsewhere)
        await page.locator('#lib-pane .tools-menu-btn').click();

        // Scope to the library pane — there are two .tools-lib-scan-section elements in the DOM
        // (one per browse pane) and the lib-scan section is shown when App.mode === 'library'
        const libPane = page.locator('#lib-pane');
        const section = libPane.locator('.tools-lib-scan-section');
        await expect(section).toBeVisible({ timeout: 5_000 });

        // Primary button
        await expect(section.locator('[data-tool="lib-scan-new"]')).toContainText('Scan for new photos');

        // Expand the secondary items
        await section.locator('.lib-scan-tools-toggle').click();
        const menu = section.locator('.lib-scan-tools-menu');
        await expect(menu).toBeVisible({ timeout: 3_000 });

        await expect(menu.locator('[data-tool="lib-reindex"]')).toContainText('Rebuild metadata & previews');
        await expect(menu.locator('[data-tool="lib-regen-missing"]')).toContainText('Generate missing previews');
        await expect(menu.locator('[data-tool="lib-rebuild-all"]')).toContainText('Rebuild all previews');
        await expect(menu.locator('[data-tool="lib-cleanup"]')).toContainText('Remove deleted photos');
    });
});
