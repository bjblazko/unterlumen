import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The Timeline place (ADR-0040): every dated library photo on one axis.
// Desktop: a band in rows over a time bar with a frame; phone: a list and a scrubber.

const LIB_NAME = 'E2E Timeline';

async function openTimeline(page) {
    await page.goto('/#timeline');
    await waitForAppReady(page);
    await page.waitForSelector('.timeline-tile img', { timeout: 20_000 });
}

// One library over all fixtures, shared by the desktop and phone tests.
test.beforeAll(async ({ request }) => {
    test.setTimeout(240_000); // indexing every fixture takes a while
    const existing = await (await request.get('/api/library/')).json();
    await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
    const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: '/' } });
    expect(res.status()).toBe(201);
    await reindexLibrary(request, (await res.json()).id);
});

test.describe('Timeline', () => {
    // Narrow enough that the fixtures' photos overflow the band, so it scrolls.
    test.use({ viewport: { width: 800, height: 640 } });

    test('is a place in Explore, below Map', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        const entry = page.locator('#mode-timeline');
        await expect(entry).toHaveAttribute('href', '#timeline');
        await entry.click();
        await expect(entry).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.timeline-title')).toHaveText('Timeline');
    });

    test('shows the photos in rows over a graph with a frame', async ({ page }) => {
        await openTimeline(page);
        await expect(page.locator('.timeline-count')).toContainText('photos');
        await expect(page.locator('.timeline-frame')).toBeVisible();
        const box = await page.locator('.timeline-axis canvas').boundingBox();
        expect(box.width).toBeGreaterThan(200);
    });

    test('scrolling the band moves the frame, dragging the frame scrolls the band', async ({ page }) => {
        await openTimeline(page);
        const band = page.locator('.timeline-band');
        const frame = page.locator('.timeline-frame');
        await band.evaluate(el => { el.scrollLeft = 0; });
        await expect.poll(() => frame.evaluate(el => parseFloat(el.style.left))).toBeLessThan(5);
        const axis = await page.locator('.timeline-axis').boundingBox();
        await page.mouse.click(axis.x + axis.width - 4, axis.y + 20);
        await expect.poll(() => band.evaluate(el => el.scrollLeft)).toBeGreaterThan(0);
    });

    test('Rows in the menu changes the band and is remembered', async ({ page }) => {
        await openTimeline(page);
        await page.locator('.timeline-head .menu-btn').click();
        await page.getByRole('menuitem', { name: '2 rows' }).click();
        await expect.poll(() => page.evaluate(() => localStorage.getItem('timeline-rows'))).toBe('2');
    });

    test('a click shows the info panel, a double-click opens the viewer', async ({ page }) => {
        await openTimeline(page);
        const tile = page.locator('.timeline-tile').first();
        await tile.click();
        await expect(tile).toHaveClass(/is-selected/);
        await tile.dblclick();
        await expect(page.locator('#viewer-container')).toBeVisible();
    });

    test('keeps the number of tiles bounded while scrolling everything', async ({ page }) => {
        await openTimeline(page);
        const band = page.locator('.timeline-band');
        const counts = [];
        const width = await band.evaluate(el => el.scrollWidth);
        for (let x = 0; x <= width; x += 800) {
            await band.evaluate((el, v) => { el.scrollLeft = v; }, x);
            await page.waitForTimeout(50);
            counts.push(await page.locator('.timeline-tile').count());
        }
        expect(Math.max(...counts)).toBeLessThan(400);
    });

    test('explains an empty timeline', async ({ page }) => {
        await page.route('**/api/timeline', route => route.fulfill({ contentType: 'application/json',
            body: JSON.stringify({ version: 'v', start: '', days: [], ratios: [], undated: 3 }) }));
        await page.goto('/#timeline');
        await waitForAppReady(page);
        await expect(page.locator('.timeline-note')).toContainText('date taken');
    });

    test('reloads when the timeline changed', async ({ page }) => {
        let skeletons = 0;
        await page.route('**/api/timeline', route => { skeletons++; route.continue(); });
        await page.route('**/api/timeline/photos?*', route =>
            skeletons === 1 ? route.fulfill({ status: 409, body: 'changed' }) : route.continue());
        await openTimeline(page);
        expect(skeletons).toBeGreaterThanOrEqual(2);
    });
});

test.describe('Timeline on a phone', () => {
    test.use({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 3, isMobile: true, hasTouch: true });

    test('newest first, a scrubber, a tap opens the photo', async ({ page }) => {
        await openTimeline(page);
        await expect(page.locator('#tab-timeline')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.timeline-scrubber')).toBeVisible();
        await expect(page.locator('.timeline-band')).toHaveCount(0);
        const scrub = await page.locator('.timeline-scrubber').boundingBox();
        await page.mouse.move(scrub.x + 30, scrub.y + scrub.height - 20);
        await page.mouse.down();
        await expect(page.locator('.timeline-scrubber-bubble')).toBeVisible();
        await page.mouse.up();
        await page.locator('.timeline-tile').first().click();
        await expect(page.locator('#viewer-container')).toBeVisible();
    });
});
