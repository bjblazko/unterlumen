import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// Regression test: batch-rename on a library-mode photo used to fail with
// "invalid path" whenever the server's browse boundary wasn't literally "/"
// (e.g. this NAS install's UNTERLUMEN_ROOT_PATH=/photos, or these e2e
// fixtures' UNTERLUMEN_ROOT_PATH=e2e/fixtures/photos). app.js turned the
// library's absolute sourcePath into a boundary-relative path by naively
// stripping only the leading "/", which happens to be correct only when
// boundary is "/" itself — otherwise it produces a doubled, nonexistent
// path that pathguard.SafePath correctly rejects. Fixed by reusing the
// same absolute-to-boundary-relative conversion commander.js's "Jump to
// library" feature already did correctly (api.js's
// absPathRelativeToBoundary).
test.describe('Batch rename in library mode', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(
            existing
                .filter(l => l.name === 'E2E Batch Rename Library')
                .map(l => request.delete(`/api/library/${l.id}`)),
        );

        // sourcePath 'folder-b' resolves server-side to an absolute path that
        // is a strict subfolder of the browse boundary — the scenario that
        // triggered the bug (boundary != sourcePath, boundary != "/").
        const res = await request.post('/api/library/', {
            data: { name: 'E2E Batch Rename Library', description: '', sourcePath: 'folder-b' },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test('batch-rename preview resolves a valid new name, not "invalid path"', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Batch Rename Library' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });
        await waitForThumbnailsLoaded(page, 1);

        await page.locator('[data-type="image"]').first().click();
        await page.locator('.tools-menu-btn:visible').click();
        await page.locator('button.tool-item[data-tool="batch-rename"]:visible').click();

        await page.waitForSelector('.batch-rename-preview-list', { timeout: 8_000 });
        // The preview only (re-)renders on an 'input' event; re-fill the
        // already-prefilled pattern to trigger it.
        const patternInput = page.locator('.batch-rename-input');
        await patternInput.fill('{YYYY}-{MM}-{DD}_{original}');

        const row = page.locator('.batch-rename-row').first();
        await expect(row).toBeVisible({ timeout: 8_000 });
        await expect(row).not.toHaveClass(/batch-rename-row-error/);
        await expect(row).not.toContainText('invalid path');
    });
});
