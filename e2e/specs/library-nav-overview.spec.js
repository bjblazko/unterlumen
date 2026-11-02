import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// A click on "Libraries" opens the place at its start — the overview — even
// when a library was open the last time it was shown.
test.describe('Libraries opens at the overview', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === 'E2E Nav Overview').map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', { data: { name: 'E2E Nav Overview', description: '', sourcePath: 'folder-a' } });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    async function openLibrary(page) {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await page.locator('.library-card', { hasText: 'E2E Nav Overview' }).locator('.lib-open').click();
        await expect(page.locator('.library-detail')).toBeVisible({ timeout: 8_000 });
    }

    test('from another place', async ({ page }) => {
        await openLibrary(page);
        await page.locator('#mode-map').click();
        await page.locator('#mode-library').click();
        await expect(page.locator('.library-list-view')).toBeVisible();
        await expect(page.locator('.library-detail')).toHaveCount(0);
    });

    test('from inside the open library', async ({ page }) => {
        await openLibrary(page);
        await page.locator('#mode-library').click();
        await expect(page.locator('.library-list-view')).toBeVisible();
    });

    test('the sidebar entry of a library still opens that library', async ({ page }) => {
        await page.goto('/#map');
        await waitForAppReady(page);
        await page.locator(`#nav-libraries [data-library-id="${libID}"]`).click();
        await expect(page.locator('.library-detail')).toBeVisible({ timeout: 8_000 });
    });
});
