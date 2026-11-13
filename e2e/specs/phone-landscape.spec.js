import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';

// A phone turned on its side is wider than 700px but still a phone: low,
// touched, held in two hands. It keeps the phone's layout, and its viewer
// shows the photo alone — a tap brings the toolbar, a swipe the next photo.

test.use({
    viewport: { width: 844, height: 390 },
    deviceScaleFactor: 3,
    isMobile: true,
    hasTouch: true,
});

const tapAt = (page, sel, fx, fy) => page.evaluate(({ sel, fx, fy }) => {
    const el = document.querySelector(sel);
    const r = el.getBoundingClientRect();
    const touch = (x) => new Touch({ identifier: 1, target: el, clientX: x, clientY: r.top + r.height * fy });
    el.dispatchEvent(new TouchEvent('touchstart', { touches: [touch(r.left + r.width * fx)], bubbles: true }));
    el.dispatchEvent(new TouchEvent('touchend', { changedTouches: [touch(r.left + r.width * fx)], bubbles: true }));
}, { sel, fx, fy });

const swipeLeft = (page) => page.evaluate(() => {
    const el = document.querySelector('.viewer-body');
    const r = el.getBoundingClientRect();
    const y = r.top + r.height / 2;
    const touch = (x) => new Touch({ identifier: 1, target: el, clientX: x, clientY: y });
    el.dispatchEvent(new TouchEvent('touchstart', { touches: [touch(r.left + r.width * 0.8)], bubbles: true }));
    el.dispatchEvent(new TouchEvent('touchend', { changedTouches: [touch(r.left + r.width * 0.2)], bubbles: true }));
});

test.describe('Phone on its side', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 2);
    });

    test('keeps the phone layout, with more than one row of photos', async ({ page }) => {
        await expect(page.locator('.tabbar')).toBeVisible();
        await expect(page.locator('.sidebar')).toBeHidden();

        await page.locator('[data-type="image"]').first().tap();
        await expect(page.locator('.selection-bar')).toBeHidden();

        // A row aims at a third of the low screen; justified to the width it
        // grows a little, but never to the one row that was all there was.
        const height = await page.locator('.justified-item').first().evaluate(e => e.getBoundingClientRect().height);
        expect(height).toBeLessThan(390 * 0.4);
    });

    test('the viewer shows the photo alone; a tap brings the toolbar, a swipe the next photo', async ({ page }) => {
        const first = page.locator('[data-type="image"]').first();
        await first.tap();
        await first.tap();
        await expect(page.locator('.viewer')).toBeVisible({ timeout: 8_000 });

        // The photo has the whole screen: over the search and the tab bar.
        const body = await page.locator('.viewer-body').boundingBox();
        expect(body).toEqual({ x: 0, y: 0, width: 844, height: 390 });
        const toolbar = page.locator('.viewer-toolbar');
        await expect(toolbar).toHaveCSS('opacity', '0');
        await expect(toolbar).toHaveCSS('pointer-events', 'none');

        await tapAt(page, '.viewer-body', 0.5, 0.5);
        await expect(toolbar).toHaveCSS('opacity', '1');
        await expect(page.locator('.viewer-counter')).toHaveText(/^1 \/ /);

        // The toolbar stays from photo to photo until the next tap.
        await swipeLeft(page);
        await expect(page.locator('.viewer-counter')).toHaveText(/^2 \/ /);
        await expect(toolbar).toHaveCSS('opacity', '1');

        await tapAt(page, '.viewer-body', 0.5, 0.5);
        await expect(toolbar).toHaveCSS('opacity', '0');

        // Opened again, the photo is alone again.
        await tapAt(page, '.viewer-body', 0.5, 0.5);
        await page.locator('.viewer-back').tap();
        await first.tap();
        await first.tap();
        await expect(page.locator('.viewer-toolbar')).toHaveCSS('opacity', '0');
    });

    test('info still comes from below', async ({ page }) => {
        const first = page.locator('[data-type="image"]').first();
        await first.tap();
        await first.tap();
        await expect(page.locator('.viewer')).toBeVisible({ timeout: 8_000 });
        await tapAt(page, '.viewer-body', 0.5, 0.5);
        await page.locator('.viewer-info-btn').tap();
        const sheet = page.locator('.viewer-info-container .info-panel.expanded');
        await expect(sheet).toBeVisible({ timeout: 8_000 });
        const box = await sheet.boundingBox();
        expect(box.y).toBeGreaterThan(390 * 0.2);
    });
});
