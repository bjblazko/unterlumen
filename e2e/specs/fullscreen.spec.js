import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';

// The browser's own full screen hides the address bar and the system bars.
// A slideshow enters it when it starts; the viewer has a button for it. Each
// leaves only the full screen it entered.

const isFullscreen = (page) => page.evaluate(() => !!document.fullscreenElement);

test.describe('Full screen', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 2);
    });

    test('a slideshow starts in full screen and Close leaves it', async ({ page }) => {
        await page.locator('.browse-slideshow-btn').click();
        await page.locator('.dialog .btn-accent').click();
        await expect(page.locator('.ss-hud')).toBeVisible();
        await expect.poll(() => isFullscreen(page)).toBe(true);

        await page.locator('.ss-btn-close').click();
        await expect.poll(() => isFullscreen(page)).toBe(false);
    });

    // On a computer full screen is in the viewer's ⋯; its entry says which
    // way it goes.
    const fullScreenItem = async (page) => {
        await page.locator('.viewer .menu-btn').click();
        return page.locator('.menu [data-id="fullscreen"]');
    };

    test('the viewer\'s menu enters and leaves full screen', async ({ page }) => {
        await page.locator('[data-type="image"]').first().dblclick();
        let item = await fullScreenItem(page);
        await expect(item).toHaveText('Full screen');
        await item.click();
        await expect.poll(() => isFullscreen(page)).toBe(true);

        // Moving to the next photo re-renders the viewer; the state holds.
        await page.keyboard.press('ArrowRight');
        item = await fullScreenItem(page);
        await expect(item).toHaveText('Leave full screen');
        await item.click();
        await expect.poll(() => isFullscreen(page)).toBe(false);
    });

    test('leaving the viewer leaves the full screen it entered', async ({ page }) => {
        await page.locator('[data-type="image"]').first().dblclick();
        await (await fullScreenItem(page)).click();
        await expect.poll(() => isFullscreen(page)).toBe(true);
        await page.locator('.viewer-back').click();
        await expect(page.locator('.viewer')).toHaveCount(0);
        await expect.poll(() => isFullscreen(page)).toBe(false);
    });
});
