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
        await expect(page.locator('.select-all-count')).toHaveText('1 selected');

        await page.locator('[data-type="image"]').nth(1).click({ modifiers: ['Meta'] });
        await expect(page.locator('.select-all-count')).toHaveText('2 selected');

        // Select none, beside the count, ends the selection and the bar.
        await page.locator('.select-all-none').click();
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

    test('only folder-level actions are left in the ⋯ menu', async ({ page }) => {
        await page.locator('[data-type="image"]').first().click();
        // What acts on a selection is in the bar; the ⋯ holds what acts on
        // the folder, and says what it would act on.
        await expect(page.locator('.tools-menu-btn')).toHaveCount(0);
        await page.locator('.browse-more .menu-btn').click();
        // The cache is cleared in Settings › Storage, not here.
        await expect(page.locator('.menu [data-id="clear-cache"]')).toHaveCount(0);
        for (const tool of ['export', 'batch-rename', 'rename', 'set-location', 'remove-location']) {
            await expect(page.locator(`.menu [data-id="${tool}"]`)).toHaveCount(0);
        }
    });

    test('the count beside Select all says how many are selected', async ({ page }) => {
        await page.locator('[data-type="image"]').first().click();
        await expect(page.locator('.select-all-count')).toContainText('1 selected');
    });
});
