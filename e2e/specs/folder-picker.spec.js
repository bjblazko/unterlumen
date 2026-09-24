import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The folder picker is the one dialog that asks for a folder (Organize's
// targets, a destination's output folder, an export folder). It looks in two
// places: the filesystem and the libraries.

test.describe('Folder picker', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === 'E2E Picker Library')
            .map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', {
            data: { name: 'E2E Picker Library', description: '', sourcePath: 'folder-a' },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test.beforeEach(async ({ page }) => {
        // Always start in the filesystem, whatever the last session used.
        await page.addInitScript(() => window.localStorage.removeItem('folderPicker.source'));
        await page.goto('/#organize');
        await waitForAppReady(page);
        await page.waitForSelector('.organize-source .browse-header', { timeout: 10_000 });
        await page.locator('[data-org="add"]').click();
        await expect(page.locator('.fp-dialog')).toBeVisible({ timeout: 5_000 });
    });

    test('opens in the filesystem with a clickable path', async ({ page }) => {
        await expect(page.locator('.fp-sources [data-source="fs"]')).toHaveAttribute('aria-pressed', 'true');
        await expect(page.locator('.fp-crumbs')).toBeVisible();
        await page.locator('.fp-row', { hasText: 'folder-a' }).click();
        await expect(page.locator('.fp-crumb-here')).toHaveText('folder-a');
        await expect(page.locator('.fp-dialog .dialog-note')).toContainText('folder-a');
    });

    test('a library is one click away, and lands in its folder', async ({ page }) => {
        await page.locator('.fp-sources [data-source="libs"]').click();
        const row = page.locator('.fp-row', { hasText: 'E2E Picker Library' });
        await expect(row).toBeVisible({ timeout: 8_000 });

        await row.click();
        // Back in the filesystem view, inside the library's own folder.
        await expect(page.locator('.fp-sources [data-source="fs"]')).toHaveAttribute('aria-pressed', 'true');
        await expect(page.locator('.fp-crumb-here')).toHaveText('folder-a');
        await expect(page.locator('.fp-dialog .dialog-note')).toContainText('folder-a');
    });

    test('Select returns the folder, Cancel returns nothing', async ({ page }) => {
        await page.locator('.fp-row', { hasText: 'folder-b' }).click();
        await page.locator('#fp-select').click();
        await expect(page.locator('.fp-dialog')).toHaveCount(0);
        await expect(page.locator('.org-target', { hasText: 'folder-b' })).toBeVisible();

        await page.locator('[data-org="add"]').click();
        await page.locator('#fp-cancel').click();
        await expect(page.locator('.fp-dialog')).toHaveCount(0);
        await expect(page.locator('.org-target')).toHaveCount(2); // folder-b and "Mark for deletion"
    });

    test('Home goes back to the folder the server was started with', async ({ page }) => {
        // Walk away from home first; the button is disabled while you are there.
        await page.locator('.fp-row', { hasText: 'folder-a' }).click();
        await expect(page.locator('.fp-crumb-here')).toHaveText('folder-a');

        await page.locator('.fp-home').click();
        await expect(page.locator('.fp-crumbs')).toContainText('Root');
        await expect(page.locator('.fp-home')).toBeDisabled();
        await expect(page.locator('.fp-dialog .dialog-note')).toContainText('browse root');
    });

    test('Escape closes it without choosing', async ({ page }) => {
        await page.keyboard.press('Escape');
        await expect(page.locator('.fp-dialog')).toHaveCount(0);
        await expect(page.locator('.org-target')).toHaveCount(1); // only "Mark for deletion"
    });
});
