import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';

// Every action on a selection lives in one bar that appears when something is
// selected (ADR-0029). What differs by context is the action list, because the
// contexts differ: "Add to gallery" needs library photos.

test.describe('Selection bar', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 2);
    });

    test('appears with the selection and goes away with it', async ({ page }) => {
        await expect(page.locator('.selection-bar')).toHaveCount(0);

        await page.locator('[data-type="image"]').first().click();
        await expect(page.locator('.selection-bar')).toBeVisible();
        await expect(page.locator('.selection-bar-count')).toHaveText('1 selected');

        await page.locator('[data-type="image"]').nth(1).click({ modifiers: ['Meta'] });
        await expect(page.locator('.selection-bar-count')).toHaveText('2 selected');

        await page.locator('.selection-bar [data-action="clear"]').click();
        await expect(page.locator('.selection-bar')).toHaveCount(0);
    });

    test('Escape clears the selection and the bar with it', async ({ page }) => {
        await page.locator('[data-type="image"]').first().click();
        await expect(page.locator('.selection-bar')).toBeVisible();
        await page.keyboard.press('Escape');
        await expect(page.locator('.selection-bar')).toHaveCount(0);
    });

    test('a folder offers no "Add to gallery", because a folder is not a library', async ({ page }) => {
        await page.locator('[data-type="image"]').first().click();
        await expect(page.locator('.selection-bar [data-action="export"]')).toBeVisible();
        await expect(page.locator('.selection-bar [data-action="collect"]')).toHaveCount(0);
    });

    test('the Tools menu keeps only what acts on the folder or the library', async ({ page }) => {
        await page.locator('[data-type="image"]').first().click();
        await page.locator('.tools-menu-btn').click();
        const menu = page.locator('.tools-menu');
        await expect(menu).toBeVisible();
        for (const tool of ['export', 'batch-rename', 'rename', 'set-location', 'remove-location']) {
            await expect(menu.locator(`[data-tool="${tool}"]`)).toHaveCount(0);
        }
    });

    test('the status bar counts, and no longer carries its own Deselect', async ({ page }) => {
        await page.locator('[data-type="image"]').first().click();
        await expect(page.locator('.status-bar')).toContainText('1 selected');
        await expect(page.locator('.status-bar .btn')).toHaveCount(0);
    });
});
