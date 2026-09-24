import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';

// Phase 8 of the Rams redesign: on a phone Unterlumen is for looking —
// libraries, folders, photos, metadata and how the galleries are doing.
// Everything that changes files or settings is desktop work and is not
// offered here rather than offered and refused.

// Phone shape only — the devices[] descriptors also pick WebKit, which this
// suite does not install (see playwright.config.js: chromium only).
test.use({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 3,
    isMobile: true,
    hasTouch: true,
});

test.describe('Phone', () => {
    test('a tab bar replaces the sidebar', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);

        await expect(page.locator('.tabbar')).toBeVisible();
        await expect(page.locator('.sidebar')).toBeHidden();

        // The same places as the sidebar, with the same addresses.
        await expect(page.locator('#tab-browse')).toHaveAttribute('href', '#folders');
        await expect(page.locator('#tab-library')).toHaveAttribute('href', '#libraries');
        await expect(page.locator('#tab-published')).toHaveAttribute('href', '#galleries');
        await expect(page.locator('#tab-browse')).toHaveAttribute('aria-current', 'page');

        await page.locator('#tab-published').tap();
        await expect(page).toHaveURL(/#galleries$/);
        await expect(page.locator('#tab-published')).toHaveAttribute('aria-current', 'page');
    });

    test('a desktop-only place says so instead of showing empty chrome', async ({ page }) => {
        await page.goto('/#organize');
        await waitForAppReady(page);

        await expect(page.locator('.desk-only-notice')).toBeVisible();
        await expect(page.locator('.desk-only-notice')).toContainText('desktop');
        // And it offers the way back to something a phone can do.
        await page.locator('.desk-only-notice a').tap();
        await expect(page.locator('.desk-only-notice')).toBeHidden();
        await expect(page.locator('.browse-container')).toBeVisible();
    });

    test('selecting a photo offers no actions', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);

        await page.locator('[data-type="image"]').first().tap();
        await expect(page.locator('.selection-bar')).toBeHidden();
        // Making a library or clearing a cache changes things: desktop work.
        await expect(page.locator('.folder-tool').first()).toBeHidden();
        await expect(page.locator('.slideshow-btn')).toBeHidden();
    });

    test('the viewer fills the screen, swipes and shows info from below', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 2);

        const first = page.locator('[data-type="image"]').first();
        await first.tap();
        await first.tap(); // a double tap opens the viewer, as on the desktop
        await expect(page.locator('.viewer')).toBeVisible({ timeout: 8_000 });

        const counter = page.locator('.viewer-counter');
        await expect(counter).toHaveText(/^1 \/ /);

        // A swipe to the left is the next photo.
        const box = await page.locator('.viewer-body').boundingBox();
        await page.evaluate(({ x, y, w }) => {
            const el = document.querySelector('.viewer-body');
            const touch = (cx) => new Touch({ identifier: 1, target: el, clientX: cx, clientY: y });
            el.dispatchEvent(new TouchEvent('touchstart', { touches: [touch(x + w * 0.8)], bubbles: true }));
            el.dispatchEvent(new TouchEvent('touchend', { changedTouches: [touch(x + w * 0.2)], bubbles: true }));
        }, { x: box.x, y: box.y + box.height / 2, w: box.width });
        await expect(counter).toHaveText(/^2 \/ /);

        // Editing tools are desktop work; Info is not.
        await expect(page.locator('.viewer-crop-btn')).toBeHidden();
        await expect(page.locator('.viewer-delete')).toBeHidden();
        await expect(page.locator('.viewer-zoom-group')).toBeHidden();

        await page.locator('.viewer-info-btn').tap();
        const sheet = page.locator('.viewer-info-container .info-panel.expanded');
        await expect(sheet).toBeVisible({ timeout: 8_000 });
        // It comes from the bottom and leaves the photo above it visible.
        const sheetBox = await sheet.boundingBox();
        const viewport = page.viewportSize();
        expect(sheetBox.y).toBeGreaterThan(viewport.height * 0.2);
    });

    test('statistics fill the screen', async ({ page }) => {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await expect(page.locator('#lib-stats-btn')).toBeVisible({ timeout: 8_000 });
        await page.locator('#lib-stats-btn').tap();

        const modal = page.locator('.stats-modal');
        await expect(modal).toBeVisible({ timeout: 8_000 });
        const box = await modal.boundingBox();
        const viewport = page.viewportSize();
        expect(Math.round(box.width)).toBe(viewport.width);
    });
});
