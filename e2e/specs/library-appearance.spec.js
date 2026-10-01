import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// What a photo looks like is measured from its thumbnail after every scan
// (ADR-0044); "Analyse photos again" in Edit library measures it anew.

const LIB_NAME = 'E2E Appearance';

// lastEvent reads an SSE body and returns its last progress event.
function lastEvent(text) {
    const events = text.split('\n')
        .filter(l => l.startsWith('data: '))
        .map(l => JSON.parse(l.slice(6)));
    return events[events.length - 1];
}

test.describe('Photo appearance', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing
            .filter(l => l.name === LIB_NAME)
            .map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', {
            data: { name: LIB_NAME, description: '', sourcePath: 'folder-a' },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
        const scan = await request.post(`/api/library/${libID}/scan-new`, { timeout: 120_000 });
        expect(await scan.text()).toContain('"finished":true');
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test('POST /api/library/{id}/analyse measures every photo again', async ({ request }) => {
        const res = await request.post(`/api/library/${libID}/analyse`, { timeout: 120_000 });
        expect(res.status()).toBe(200);
        const last = lastEvent(await res.text());
        expect(last.finished).toBe(true);
        expect(last.error).toBeUndefined();
        expect(last.total).toBeGreaterThan(0);
        expect(last.done).toBe(last.total);
    });

    test('POST /api/library/{id}/analyse rejects a folder outside the library', async ({ request }) => {
        const res = await request.post(`/api/library/${libID}/analyse?subfolder=${encodeURIComponent('../..')}`);
        expect(lastEvent(await res.text()).error).toBe('invalid subfolder path');
    });

    test('Edit library runs Analyse photos again and says when it is finished', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });
        await page.locator('.library-card', { hasText: LIB_NAME }).locator('.lib-open').click();
        await page.waitForSelector('#lib-pane', { timeout: 8_000 });

        await page.locator('#lib-edit-btn').click();
        const dialog = page.locator('.library-dialog');
        await expect(dialog).toBeVisible({ timeout: 5_000 });

        const button = dialog.locator('[data-maint="analyseAgain"]');
        await expect(button).toHaveText('Analyse photos again');
        await button.click();
        await expect(dialog.locator('#lib-edit-maint-progress')).toContainText(/Finished\. \d+ photos?\./, { timeout: 60_000 });
        await expect(button).toBeEnabled();

        await dialog.locator('#lib-edit-cancel').click();
    });
});
