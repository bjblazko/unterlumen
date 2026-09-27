import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';

// The ⋯ menu holds what need not be on screen all the time: the Names and
// Details switches, the slideshow and the rarer folder work.

test.describe('The ⋯ menu', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 2);
    });

    test('closes the breadcrumb row and names itself', async ({ page }) => {
        const btn = page.locator('.breadcrumb-row .menu-btn');
        await expect(btn).toHaveAccessibleName('More');
        await expect(btn).toHaveAttribute('aria-expanded', 'false');
        await btn.click();
        await expect(btn).toHaveAttribute('aria-expanded', 'true');
        await expect(page.getByRole('menu')).toBeVisible();
        // The switches and the slideshow no longer take room in the toolbar.
        await expect(page.locator('.controls .toggle')).toHaveCount(0);
        await expect(page.locator('.controls').getByText('Slideshow')).toHaveCount(0);
    });

    test('a switch flips in place and leaves the menu open', async ({ page }) => {
        await page.locator('.browse-more .menu-btn').click();
        const names = page.getByRole('menuitemcheckbox', { name: /Names/ });
        await expect(names).toHaveAttribute('aria-checked', 'false');
        await names.click();
        await expect(names).toHaveAttribute('aria-checked', 'true');
        await expect(page.locator('.item-name').first()).toBeVisible();
        await expect(page.getByRole('menu')).toBeVisible();
    });

    test('an action closes the menu', async ({ page }) => {
        await page.locator('.browse-more .menu-btn').click();
        await page.getByRole('menuitem', { name: 'Slideshow' }).click();
        await expect(page.getByRole('menu')).toHaveCount(0);
        await expect(page.locator('.dialog-scrim .dialog-title')).toHaveText('Slideshow');
        await page.keyboard.press('Escape');
    });

    test('keys move through it, and Escape closes only the menu', async ({ page }) => {
        await page.locator('[data-type="image"]').first().click();
        await expect(page.locator('.selection-bar')).toBeVisible();

        const btn = page.locator('.browse-more .menu-btn');
        await btn.focus();
        await page.keyboard.press('Enter');
        const items = page.locator('.menu .menu-item:not(:disabled)');
        await expect(items.first()).toBeFocused();
        await page.keyboard.press('ArrowDown');
        await expect(items.nth(1)).toBeFocused();
        await page.keyboard.press('End');
        await expect(items.last()).toBeFocused();
        await page.keyboard.press('ArrowDown');
        await expect(items.first()).toBeFocused();

        await page.keyboard.press('Escape');
        await expect(page.getByRole('menu')).toHaveCount(0);
        await expect(btn).toBeFocused();
        // The selection outlived the Escape that was meant for the menu.
        await expect(page.locator('.selection-bar')).toBeVisible();
    });

    test('a click outside closes it', async ({ page }) => {
        await page.locator('.browse-more .menu-btn').click();
        await expect(page.getByRole('menu')).toBeVisible();
        await page.locator('.breadcrumb').click();
        await expect(page.getByRole('menu')).toHaveCount(0);
    });

    test('Clear cache says what it would clear', async ({ page }) => {
        await page.locator('[data-type="image"]').nth(1).click({ modifiers: ['Shift'] });
        await page.locator('.browse-more .menu-btn').click();
        await expect(page.locator('.menu [data-id="clear-cache"]')).toHaveText(/^Clear cache · \d+ files?$/);
    });
});

test('Organize\'s source offers only the switches', async ({ page }) => {
    await page.goto('/#organize');
    await waitForAppReady(page);
    await page.waitForSelector('.organize-source .browse-header', { timeout: 10_000 });
    await page.locator('.organize-source .menu-btn').click();
    const ids = await page.locator('.menu .menu-item').evaluateAll(els => els.map(e => e.dataset.id));
    expect(ids).toEqual(['names', 'details']);
});
