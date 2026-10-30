import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';
import { reindexLibrary } from '../helpers/library.js';

// Phase 8 of the Rams redesign: on a phone Unterlumen is for looking —
// libraries, folders, photos, metadata and the people in them.
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
    test('a tab bar replaces the sidebar, and Libraries is where it starts', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);

        await expect(page.locator('.tabbar')).toBeVisible();
        await expect(page.locator('.sidebar')).toBeHidden();

        // The same places as the sidebar, with the same addresses; Libraries first.
        await expect(page.locator('#tab-library')).toHaveAttribute('href', '#libraries');
        await expect(page.locator('#tab-library')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('#tab-statistics')).toHaveAttribute('href', '#statistics');
        await expect(page.locator('#tab-people')).toHaveAttribute('href', '#people');

        await page.locator('#tab-people').tap();
        await expect(page).toHaveURL(/#people$/);
        await expect(page.locator('#tab-people')).toHaveAttribute('aria-current', 'page');
        // Publishing is desktop work, and photos are found in libraries:
        // Statistics has the place Folders had. Folders stays reachable by its links.
        await expect(page.locator('.tabbar-item')).toHaveText(['Libraries', 'Map', 'Timeline', 'People', 'Statistics']);
    });

    // The photos keep the screen: a place's head is one row on a phone —
    // its setting a small select, its sentence behind "i".
    test("a place's head is one row", async ({ page }) => {
        for (const place of ['#libraries', '#map', '#timeline', '#statistics', '#folders']) {
            await page.goto('/' + place);
            await waitForAppReady(page);
            const head = page.locator('#app .place-header:visible').first();
            await expect(head).toBeVisible();
            const box = await head.boundingBox();
            expect(box.height, place).toBeLessThanOrEqual(60);
        }
        await page.goto('/#map');
        await waitForAppReady(page);
        await expect(page.getByRole('combobox', { name: 'Map' })).toBeVisible();
        await page.getByRole('button', { name: 'Explain' }).first().tap();
        await expect(page.locator('.place-lede-open .place-lede-text').first()).toBeVisible();
    });

    // A search shows its photos; the filter, which covers the screen on a
    // phone, stays closed and counts its condition on its button.
    test('a search shows its photos, not the filter', async ({ page }) => {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await page.evaluate(() => App.showPhotos({ thing: 'boat' }));
        await expect(page.locator('.place-header:visible', { hasText: 'Results' })).toBeVisible();
        await expect(page.locator('.lib-filter-panel.visible')).toHaveCount(0);
        await expect(page.locator('.lib-filter-btn-count:visible').first()).toHaveText('1');
    });

    // The row height shrinks with the width, so two landscape photos share a
    // row instead of one photo filling the screen; the margins stay thin.
    test('two photos share a row, with thin margins', async ({ page }) => {
        await page.goto('/#folders');
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
        // Indexing folder-a decodes its HEIFs one at a time (media.oneFullDecode),
        // which on a two-core CI runner took longer than the default minute.
        test.setTimeout(180_000);
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
                // The mosaic is square and ends above the name. Safari once let
                // the photos run over the name and the counts.
                const tile = await page.locator('.folder-tile').first().evaluate(t => {
                    const m = t.querySelector('.folder-tile-mosaic').getBoundingClientRect();
                    const n = t.querySelector('.item-name').getBoundingClientRect();
                    return { w: Math.round(m.width), h: Math.round(m.height), gap: Math.round(n.top - m.bottom) };
                });
                expect(Math.abs(tile.w - tile.h)).toBeLessThanOrEqual(1);
                expect(tile.gap).toBeGreaterThanOrEqual(0);
                const barPadding = await page.locator('.tabbar').evaluate(e => getComputedStyle(e).paddingBottom);
                expect(barPadding).toBe('4px');

                // Statistics has no room in the head; it is in the pane's ⋯.
                await expect(page.locator('#lib-detail-stats-btn')).toBeHidden();
                await page.locator('#lib-pane .menu-btn').tap();
                await page.locator('.menu [data-id="statistics"]').tap();
                await expect(page.locator('.stats-place')).toBeVisible({ timeout: 15_000 });
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
        await expect(page).toHaveURL(/#libraries$/);
        await expect(page.locator('#tab-library')).toHaveAttribute('aria-current', 'page');
    });

    test('selecting a photo offers no actions', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);

        await page.locator('[data-type="image"]').first().tap();
        await expect(page.locator('.selection-bar')).toBeHidden();
        // The controls row is gone; the ⋯ offers what only looks. Making a
        // library or clearing a cache changes things: desktop work.
        await expect(page.locator('.controls')).toBeHidden();
        // Without the controls, the row under the head still keeps a gap above the photos.
        const gap = await page.locator('.browse-bar').evaluate(e => getComputedStyle(e).paddingBottom);
        expect(gap).toBe('8px');
        await page.locator('.browse-more .menu-btn').tap();
        await expect(page.locator('.menu .menu-item')).toHaveCount(3);
        const ids = await page.locator('.menu .menu-item').evaluateAll(els => els.map(e => e.dataset.id));
        expect(ids).toEqual(['names', 'details', 'videos']);
        const itemHeight = await page.locator('.menu .menu-item').first().evaluate(e => e.getBoundingClientRect().height);
        expect(itemHeight).toBeGreaterThanOrEqual(44);
    });

    test('the viewer fills the screen, swipes and shows info from below', async ({ page }) => {
        await page.goto('/#folders');
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

        // Saving the original is looking, not editing: it stays, on screen.
        const download = page.locator('.viewer-download-btn');
        await expect(download).toBeVisible();
        const dl = await download.boundingBox();
        expect(dl.x + dl.width).toBeLessThanOrEqual(390);

        await page.locator('.viewer-info-btn').tap();
        const sheet = page.locator('.viewer-info-container .info-panel.expanded');
        await expect(sheet).toBeVisible({ timeout: 8_000 });
        // It comes from the bottom and leaves the photo above it visible.
        const sheetBox = await sheet.boundingBox();
        const viewport = page.viewportSize();
        expect(sheetBox.y).toBeGreaterThan(viewport.height * 0.2);
    });

    test('the slideshow covers the tab bar and its controls fit the width', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);
        await page.locator('.browse-slideshow-btn').tap();
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

    test('statistics fit the width of the screen', async ({ page }) => {
        await page.goto('/#statistics/equipment');
        await waitForAppReady(page);
        const place = page.locator('.stats-place');
        await expect(place).toBeVisible({ timeout: 15_000 });
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
        expect(overflow).toBeLessThanOrEqual(0);
        const box = await place.boundingBox();
        expect(Math.round(box.width)).toBe(page.viewportSize().width);
    });

    // The sentence under a place's title folds behind Explain on a phone.
    test('a place says what it is only when asked', async ({ page }) => {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        const lede = page.locator('.place-header .place-lede');
        await expect(lede.locator('.place-lede-text')).toBeHidden();
        const info = lede.locator('.place-lede-toggle');
        await expect(info).toHaveAttribute('aria-expanded', 'false');
        await info.tap();
        await expect(lede.locator('.place-lede-text')).toBeVisible();
        await expect(lede.locator('.place-lede-text')).toContainText('every folder inside it');
        await info.tap();
        await expect(lede.locator('.place-lede-text')).toBeHidden();
    });

    // The style beside the title and the sentence behind Explain, so the map keeps its height.
    test('the map keeps its options out of the way', async ({ page, request }) => {
        // folder-b holds photos with a location.
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === 'E2E Phone map').map(l => request.delete(`/api/library/${l.id}`)));
        const lib = await (await request.post('/api/library/', { data: { name: 'E2E Phone map', description: '', sourcePath: 'folder-b' } })).json();
        await reindexLibrary(request, lib.id);
        try {
            await page.route('https://tiles.openfreemap.org/**', (route) => route.fulfill({
                contentType: 'application/json',
                body: JSON.stringify({ version: 8, sources: {}, layers: [{ id: 'bg', type: 'background', paint: { 'background-color': '#ddd' } }] }),
            }));
            await page.goto('/#map');
            await waitForAppReady(page);
            await page.waitForSelector('.map-marker', { timeout: 15_000 });
            const text = page.locator('.place-header .place-lede-text');
            await expect(text).toBeHidden();
            await page.locator('.place-header .place-lede-toggle').tap();
            await expect(text).toBeVisible();
            // The style is the head's one setting, beside the title as a small
            // select, not a row of its own.
            const style = page.locator('.map-pane .place-header').getByRole('combobox', { name: 'Map' });
            await expect(style).toBeVisible();
            const [title, seg] = [await page.locator('.map-pane .place-header h1, .map-pane .place-header .place-title').first().boundingBox(), await style.boundingBox()];
            expect(Math.abs(title.y + title.height / 2 - (seg.y + seg.height / 2))).toBeLessThan(12);
        } finally {
            await request.delete(`/api/library/${lib.id}`);
        }
    });
});
