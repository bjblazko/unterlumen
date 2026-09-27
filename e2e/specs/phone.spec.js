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

    // The row height shrinks with the width, so two landscape photos share a
    // row instead of one photo filling the screen; the margins stay thin.
    test('two photos share a row, with thin margins', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 4);
        await page.waitForFunction(() => {
            const tops = [...document.querySelectorAll('.justified-item')].slice(0, 2)
                .map(e => Math.round(e.getBoundingClientRect().top));
            return tops.length === 2 && tops[0] === tops[1];
        }, null, { timeout: 10_000 });
        const pad = await page.evaluate(() => parseFloat(getComputedStyle(document.querySelector('.browse-container')).paddingLeft));
        expect(pad).toBeLessThanOrEqual(8);
        const bar = await page.locator('.tabbar').boundingBox();
        expect(bar.height).toBeLessThanOrEqual(56);
    });

    // In a library, folders are tiles; two share a row. The phone width rule
    // once sat above the base rule and lost to its fixed 176px.
    test('a library shows two folder tiles to a row, and no inset under the tab bar', async ({ page, request }) => {
        const NAME = 'E2E Phone Tiles';
        const clean = async () => {
            const libs = await (await request.get('/api/library/')).json();
            await Promise.all(libs.filter(l => l.name === NAME).map(l => request.delete(`/api/library/${l.id}`)));
        };
        await clean();
        const lib = await (await request.post('/api/library/', { data: { name: NAME, description: '', sourcePath: 'folder-a' } })).json();
        await request.post(`/api/library/${lib.id}/reindex`, { timeout: 120_000 });
        try {
            await page.goto('/#libraries');
            await waitForAppReady(page);
            await page.locator('.library-card', { hasText: NAME }).locator('.lib-open').tap();
            await page.waitForSelector('.folder-tile', { timeout: 15_000 });
            const tiles = await page.locator('.folder-tile').evaluateAll(els => els.slice(0, 2).map(e => {
                const r = e.getBoundingClientRect(); return { left: Math.round(r.left), top: Math.round(r.top) };
            }));
            expect(tiles[0].top).toBe(tiles[1].top);
            expect(tiles[0].left).toBeLessThanOrEqual(8);
            const barPadding = await page.locator('.tabbar').evaluate(e => getComputedStyle(e).paddingBottom);
            expect(barPadding).toBe('4px');

            // Statistics has no room in the head; it is in the pane's ⋯.
            await expect(page.locator('#lib-detail-stats-btn')).toBeHidden();
            await page.locator('#lib-pane .menu-btn').tap();
            await page.locator('.menu [data-id="statistics"]').tap();
            await expect(page.locator('.stats-dialog')).toBeVisible({ timeout: 15_000 });
        } finally {
            await clean();
        }
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
        // The controls row is gone; the ⋯ offers what only looks. Making a
        // library or clearing a cache changes things: desktop work.
        await expect(page.locator('.controls')).toBeHidden();
        // Without the toolbar row, the header still keeps a gap above the photos.
        const gap = await page.locator('.browse-header').evaluate(e => getComputedStyle(e).paddingBottom);
        expect(gap).toBe('8px');
        await page.locator('.browse-more .menu-btn').tap();
        await expect(page.locator('.menu .menu-item')).toHaveCount(3);
        const ids = await page.locator('.menu .menu-item').evaluateAll(els => els.map(e => e.dataset.id));
        expect(ids).toEqual(['names', 'details', 'slideshow']);
        const itemHeight = await page.locator('.menu .menu-item').first().evaluate(e => e.getBoundingClientRect().height);
        expect(itemHeight).toBeGreaterThanOrEqual(44);
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

    test('the slideshow covers the tab bar and its controls fit the width', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);
        await page.locator('.browse-more .menu-btn').tap();
        await page.locator('.menu [data-id="slideshow"]').tap();
        await page.locator('.dialog .btn-accent').tap();
        await expect(page.locator('.ss-hud')).toBeVisible();

        const width = page.viewportSize().width;
        const boxes = await page.locator('.ss-hud .ss-btn').evaluateAll(els => els.map(e => e.getBoundingClientRect().toJSON()));
        expect(boxes).toHaveLength(4);
        for (const b of boxes) {
            expect(b.left).toBeGreaterThanOrEqual(0);
            expect(b.right).toBeLessThanOrEqual(width);
            expect(b.height).toBeGreaterThanOrEqual(44);
        }
        // What is on top at the Close button is the button, not the tab bar.
        const close = boxes[3];
        const onTop = await page.evaluate(([x, y]) => document.elementFromPoint(x, y)?.closest('.ss-btn-close') !== null,
            [close.left + close.width / 2, close.top + close.height / 2]);
        expect(onTop).toBe(true);
    });

    test('statistics fill the screen', async ({ page }) => {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await expect(page.locator('#lib-stats-btn')).toBeVisible({ timeout: 8_000 });
        await page.locator('#lib-stats-btn').tap();

        const modal = page.locator('.stats-dialog');
        await expect(modal).toBeVisible({ timeout: 8_000 });
        const box = await modal.boundingBox();
        const viewport = page.viewportSize();
        expect(Math.round(box.width)).toBe(viewport.width);
    });
});
