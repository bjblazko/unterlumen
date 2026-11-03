import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// Memories (ADR-0064): photos of this day in earlier years, and cards for a
// trip, a person, this month back then and rediscovered photos.
// The page follows the browser's date, so the clock is set to a month and
// day the fixtures have photos on, in 2026.

const LIB_NAME = 'E2E Memories';
let today = '';
let libID = '';

// daysWithPhotos reads the dates the timeline has photos on.
async function daysWithPhotos(request) {
    const sk = await (await request.get(`/api/timeline?ids=${libID}`)).json();
    const start = Date.parse(sk.start + 'T00:00:00Z');
    return sk.days.flatMap((n, i) => (n > 0 ? [new Date(start + i * 86400000).toISOString().slice(0, 10)] : []));
}

test.beforeAll(async ({ request }) => {
    test.setTimeout(240_000); // indexing every fixture takes a while
    const existing = await (await request.get('/api/library/')).json();
    await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
    const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: '/' } });
    expect(res.status()).toBe(201);
    libID = (await res.json()).id;
    await reindexLibrary(request, libID);
    // The first month and day of an earlier year that has memories.
    for (const day of await daysWithPhotos(request)) {
        if (day >= '2026' || day.slice(5) === '02-29') continue;
        const candidate = '2026' + day.slice(4);
        const o = await (await request.get(`/api/memories?day=${candidate}&ids=${libID}`)).json();
        if (o.years.length) { today = candidate; break; }
    }
    expect(today, 'a day of the fixtures with photos in an earlier year').not.toBe('');
});

// It covers every fixture, so later specs would find each photo in one library more.
test.afterAll(async ({ request }) => {
    const existing = await (await request.get('/api/library/')).json();
    await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
});

test.beforeEach(async ({ page }) => {
    await page.clock.setFixedTime(new Date(`${today}T12:00:00`));
    await page.addInitScript(() => { try { localStorage.removeItem('scope.filter'); } catch { /* none */ } });
});

test.describe('Memories', () => {
    test('opens from the sidebar and shows on-this-day rows', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        const entry = page.locator('#mode-memories');
        await expect(entry).toHaveAttribute('href', '#memories');
        await entry.click();
        await expect(page).toHaveURL(/#memories$/);
        await expect(entry).toHaveAttribute('aria-current', 'page');
        const year = page.locator('.memories-year').first();
        await expect(year).toBeVisible({ timeout: 20_000 });
        await expect(year.locator('.memories-year-title')).toContainText(/On this day (a year|\d+ years) ago/);
        await expect(year.locator('.memories-tile').first()).toBeVisible();
        await expect(page.locator('.btn-accent')).toHaveCount(0);
    });

    test('a row leads to its day and a tile opens the viewer', async ({ page }) => {
        await page.goto('/#memories');
        await waitForAppReady(page);
        const head = page.locator('.memories-year-head').first();
        await expect(head).toBeVisible({ timeout: 20_000 });
        const href = await head.getAttribute('href');
        expect(href).toMatch(/^#memories\/day\/\d{4}-\d{2}-\d{2}$/);
        await head.click();
        await expect(page).toHaveURL(new RegExp(href + '$'));
        await expect(page.locator('.place-header .gal-crumbs a', { hasText: 'Memories' })).toBeVisible();
        await page.locator('.memories-picks .memories-tile').first().click();
        await expect(page.locator('#viewer-container')).toBeVisible();
        await page.keyboard.press('Escape');
        // The title is the way back.
        await page.locator('.place-header .gal-crumbs a', { hasText: 'Memories' }).click();
        await expect(page).toHaveURL(/#memories$/);
        await expect(page.locator('.memories-year').first()).toBeVisible();
    });

    test('a day page has its address, and a day without memories says so', async ({ page }) => {
        await page.goto('/#memories');
        await waitForAppReady(page);
        const href = await page.locator('.memories-year-head').first().getAttribute('href', { timeout: 20_000 });
        await page.goto('/' + href);
        await waitForAppReady(page);
        await expect(page.locator('.memories-picks .memories-tile').first()).toBeVisible({ timeout: 20_000 });
        await page.goto('/#memories/trip/1900-01-01');
        await expect(page.locator('.memories-body')).toContainText('not among today');
    });

    test('the shared filter narrows the memories', async ({ page }) => {
        await page.goto('/#memories');
        await waitForAppReady(page);
        await expect(page.locator('.memories-year').first()).toBeVisible({ timeout: 20_000 });
        // Memories show no videos, so asking for videos only leaves nothing.
        await page.locator('.scope-filter-toggle:visible').click();
        await page.locator('.scope-filter-kind:visible').selectOption('video');
        await expect(page.locator('.memories-empty')).toBeVisible({ timeout: 20_000 });
        await expect(page.locator('.memories-year')).toHaveCount(0);
        await page.locator('.scope-filter-chips:visible .lib-filter-chip', { hasText: 'Kind' }).click();
        await expect(page.locator('.memories-year').first()).toBeVisible({ timeout: 20_000 });
    });

    test('on a phone the tab bar holds Memories beside the other five, in German too', async ({ browser }) => {
        const ctx = await browser.newContext({ locale: 'de-DE', viewport: { width: 360, height: 740 }, hasTouch: true, isMobile: true });
        const page = await ctx.newPage();
        await page.clock.setFixedTime(new Date(`${today}T12:00:00`));
        await page.goto('/#memories');
        await waitForAppReady(page);
        const tab = page.locator('#tab-memories');
        await expect(tab).toBeVisible();
        await expect(tab).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.tabbar-item:visible')).toHaveCount(6);
        // Every label is its own word: none runs into the next.
        const labels = await page.locator('.tabbar-item:visible').evaluateAll(els => els.map(e => {
            const tabBox = e.getBoundingClientRect();
            const label = e.querySelector('span');
            const box = label.getBoundingClientRect();
            return { tab: tabBox.toJSON(), left: box.left, right: box.right };
        }));
        for (const [i, l] of labels.entries()) {
            expect(l.tab.left).toBeGreaterThanOrEqual(0);
            expect(l.tab.right).toBeLessThanOrEqual(360);
            // Space between two labels, so they read as two words.
            if (i > 0) expect(l.left - labels[i - 1].right, `gap before label ${i}`).toBeGreaterThanOrEqual(4);
        }
        expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360);
        await expect(page.locator('.memories-year').first()).toBeVisible({ timeout: 20_000 });
        await ctx.close();
    });
});
