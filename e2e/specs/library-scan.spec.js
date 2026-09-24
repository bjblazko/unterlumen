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

    test('library row offers Scan for new photos and the indexed date', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Scan Ops' });
        await expect(card).toBeVisible({ timeout: 8_000 });

        await expect(card.locator('.lib-scan-new')).toContainText('Scan for new photos');
        await expect(card.locator('.library-card-indexed')).toContainText('Indexed');

        // The rarer maintenance runs are not on the row any more.
        await expect(card.locator('.lib-scan-toggle')).toHaveCount(0);
        await expect(card.locator('.lib-reindex')).toHaveCount(0);
    });

    // ── Maintenance in "Edit library…" ───────────────────────────────────────

    test('Edit library holds the four maintenance runs', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Scan Ops' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('#lib-pane', { timeout: 8_000 });

        await page.locator('#lib-edit-btn').click();
        const dialog = page.locator('.library-dialog');
        await expect(dialog).toBeVisible({ timeout: 5_000 });

        const actions = dialog.locator('#lib-edit-maint-actions');
        await expect(actions.locator('[data-maint="scanNew"]')).toContainText('Scan for new photos');
        await expect(actions.locator('[data-maint="reindex"]')).toContainText('Rebuild metadata & previews');
        await expect(actions.locator('[data-maint="regenMissingPreviews"]')).toContainText('Generate missing previews');
        await expect(actions.locator('[data-maint="rebuildAllPreviews"]')).toContainText('Rebuild all previews');
        await expect(actions.locator('[data-maint="cleanup"]')).toContainText('Remove deleted photos');

        await dialog.locator('#lib-edit-cancel').click();
    });

    // ── Scanning the folder you have open ────────────────────────────────────

    test('library page head keeps Scan for new photos, without a menu', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Scan Ops' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('#lib-pane', { timeout: 8_000 });

        const libPane = page.locator('#lib-pane');
        await expect(libPane.locator('[data-tool="lib-scan-new"]')).toContainText('Scan for new photos');
        await expect(libPane.locator('.lib-scan-tools-menu')).toHaveCount(0);
    });
});
