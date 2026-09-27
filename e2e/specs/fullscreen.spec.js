import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';

// The browser's own full screen hides the address bar and the system bars.
// A slideshow enters it when it starts; the viewer has a button for it. Each
// leaves only the full screen it entered.

const isFullscreen = (page) => page.evaluate(() => !!document.fullscreenElement);

test.describe('Full screen', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 2);
    });

    test('a slideshow starts in full screen and Close leaves it', async ({ page }) => {
        await page.locator('.browse-more .menu-btn').click();
        await page.locator('.menu [data-id="slideshow"]').click();
        await page.locator('.dialog .btn-accent').click();
        await expect(page.locator('.ss-hud')).toBeVisible();
        await expect.poll(() => isFullscreen(page)).toBe(true);

        await page.locator('.ss-btn-close').click();
        await expect.poll(() => isFullscreen(page)).toBe(false);
    });

    test('the viewer\'s button enters and leaves full screen', async ({ page }) => {
        await page.locator('[data-type="image"]').first().dblclick();
        const btn = page.locator('.viewer-fullscreen-btn');
        await expect(btn).toHaveAccessibleName('Full screen');
        await expect(btn).toHaveAttribute('aria-pressed', 'false');

        await btn.click();
        await expect.poll(() => isFullscreen(page)).toBe(true);
        await expect(btn).toHaveAccessibleName('Leave full screen');
        await expect(btn).toHaveAttribute('aria-pressed', 'true');

        // Moving to the next photo re-renders the viewer; the state holds.
        await page.keyboard.press('ArrowRight');
        await expect(page.locator('.viewer-fullscreen-btn')).toHaveAttribute('aria-pressed', 'true');

        await page.locator('.viewer-fullscreen-btn').click();
        await expect.poll(() => isFullscreen(page)).toBe(false);
        await expect(page.locator('.viewer-fullscreen-btn')).toHaveAccessibleName('Full screen');
    });

    test('leaving the viewer leaves the full screen it entered', async ({ page }) => {
        await page.locator('[data-type="image"]').first().dblclick();
        await page.locator('.viewer-fullscreen-btn').click();
        await expect.poll(() => isFullscreen(page)).toBe(true);
        await page.locator('.viewer-back').click();
        await expect(page.locator('.viewer')).toHaveCount(0);
        await expect.poll(() => isFullscreen(page)).toBe(false);
    });
});
