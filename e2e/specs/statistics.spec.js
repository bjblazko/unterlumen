import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The Statistics place (ADR-0043): an overview with a card per topic, the
// topics as sidebar sub-entries, a scope in the address, and the photos of a
// clicked value in a column beside the charts. folder-b holds photos of many
// cameras and years.

const LIB_NAME = 'E2E Statistics';

test.describe('Statistics', () => {
    let lib;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: 'folder-b' } });
        expect(res.status()).toBe(201);
        lib = await res.json();
        await reindexLibrary(request, lib.id);
    });

    test.afterAll(async ({ request }) => {
        if (lib) await request.delete(`/api/library/${lib.id}`);
    });

    // The overview or a topic, scoped to the test library: the shared
    // database may hold other libraries (e2e/NOTES.md).
    async function open(page, topic = '') {
        await page.goto(`/#statistics${topic ? '/' + topic : ''}?library=${lib.id}`);
        await waitForAppReady(page);
        await expect(page.locator('.stats-count')).toContainText('photos', { timeout: 15_000 });
    }

    test('the statistics API has the shape the charts read', async ({ request }) => {
        const body = await (await request.get(`/api/library/statistics?ids=${lib.id}`)).json();
        expect(body.totalPhotos).toBeGreaterThan(0);
        for (const key of ['formats', 'filmSims', 'focalLengths', 'focalLengths35', 'apertures', 'isos', 'cameraLens']) {
            expect(Array.isArray(body[key])).toBe(true);
        }
        expect(body.shootingHours).toHaveLength(24);
        expect(typeof body.shootingDays).toBe('object');
    });

    test('search finds what the charts counted: hour, frame shape and folder', async ({ request }) => {
        const stats = await (await request.get(`/api/library/statistics?ids=${lib.id}`)).json();
        const hour = stats.shootingHours.findIndex(n => n > 0);
        const byHour = await (await request.get(`/api/library/search?ids=${lib.id}&hour=${hour}`)).json();
        expect(byHour.total).toBe(stats.shootingHours[hour]);

        const tl = await (await request.get(`/api/library/timeline?ids=${lib.id}`)).json();
        const shape = tl.aspectRatios[0];
        const counted = shape.counts.reduce((a, b) => a + b, 0);
        const byShape = await (await request.get(`/api/library/search?ids=${lib.id}&aspect=${encodeURIComponent(shape.ratio)}`)).json();
        // The timeline counts dated photos only.
        expect(byShape.total).toBeGreaterThanOrEqual(counted);

        const inFolder = await (await request.get(`/api/library/search?ids=${lib.id}&pathPrefix=${encodeURIComponent(lib.sourcePath)}`)).json();
        expect(inFolder.total).toBe(stats.totalPhotos);
        const elsewhere = await (await request.get(`/api/library/search?ids=${lib.id}&pathPrefix=${encodeURIComponent(lib.sourcePath + '-not')}`)).json();
        expect(elsewhere.total).toBe(0);
    });

    test('a camera is listed once, and "Other" at most once', async ({ request }) => {
        const tl = await (await request.get('/api/library/timeline')).json();
        const names = tl.cameraUsage.map(c => c.camera);
        expect(new Set(names).size).toBe(names.length);
        expect(names.length).toBeLessThanOrEqual(6);
    });

    test('the sidebar lists the topics, and they have addresses', async ({ page }) => {
        await open(page);
        await expect(page.locator('#mode-statistics')).toHaveAttribute('aria-current', 'page');
        const topics = page.locator('#nav-statistics .nav-sub');
        await expect(topics).toHaveText(['Equipment', 'Exposure', 'Time', 'Frame', 'Colour', 'Places']);

        await topics.filter({ hasText: /^\s*Exposure\s*$/ }).click();
        await expect(page).toHaveURL(new RegExp(`#statistics/exposure\\?library=${lib.id}$`));
        await expect(topics.filter({ hasText: /^\s*Exposure\s*$/ })).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.stats-title')).toHaveText('Exposure');
        await expect(page.locator('.stats-chart-title', { hasText: 'Aperture over time' })).toBeVisible();

        // The address alone brings the topic back.
        await page.reload();
        await waitForAppReady(page);
        await expect(page.locator('.stats-title')).toHaveText('Exposure');
        await expect(page.locator('.stats-library')).toHaveValue(lib.id);
    });

    test('the overview leads to each topic, and back', async ({ page }) => {
        await open(page);
        const cards = page.locator('.stats-card');
        await expect(cards).toHaveCount(6);
        await cards.filter({ hasText: 'Frame' }).click();
        await expect(page.locator('.stats-title')).toHaveText('Frame');
        await page.locator('.stats-back').click();
        await expect(page.locator('.stats-title')).toHaveText('Statistics');
        await page.goBack();
        await expect(page.locator('.stats-title')).toHaveText('Frame');
    });

    test('with the sidebar collapsed the topics are reached from the overview', async ({ page }) => {
        await open(page);
        await page.locator('#sidebar-collapse').click();
        await expect(page.locator('#nav-statistics .nav-sub').first()).toBeHidden();
        await expect(page.locator('.stats-card')).toHaveCount(6);
        await page.locator('#sidebar-collapse').click();
    });

    test('the topics show in the sidebar only while Statistics is open', async ({ page }) => {
        await open(page);
        await expect(page.locator('#nav-statistics .nav-sub').first()).toBeVisible();
        await page.locator('#mode-map').click();
        await expect(page.locator('#nav-statistics .nav-sub').first()).toBeHidden();
    });

    test('9 opens Statistics', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await page.keyboard.press('9');
        await expect(page.locator('#mode-statistics')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.stats-title')).toHaveText('Statistics');
    });

    test("a library's Statistics button opens the place for that library", async ({ page }) => {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        const card = page.locator('.library-card', { hasText: LIB_NAME });
        await card.waitFor({ timeout: 15_000 });
        await card.locator('.lib-open').click();
        await page.locator('#lib-detail-stats-btn').click();
        await expect(page).toHaveURL(new RegExp(`#statistics\\?library=${lib.id}$`));
        await expect(page.locator('.stats-library')).toHaveValue(lib.id);
    });

    test('camera usage is a line chart with a legend', async ({ page }) => {
        await open(page, 'equipment');
        const chart = page.locator('.stats-chart', { hasText: 'Camera usage' });
        const legend = chart.locator('.stats-legend-item');
        await expect(legend.first()).toBeVisible();
        const names = await legend.allTextContents();
        expect(new Set(names.map(n => n.trim())).size).toBe(names.length);
        await expect(chart.locator('path[fill="none"]')).toHaveCount(names.length);
    });

    test('clicking a camera shows its photos beside the charts', async ({ page, request }) => {
        await open(page, 'equipment');
        const cell = page.locator('.stats-chart', { hasText: 'Camera and lens' }).locator('.lens-cell .stats-pickable').first();
        const label = await cell.getAttribute('aria-label');
        const counted = Number(label.match(/([\d\s,.]+) photos?$/)[1].replace(/\D/g, ''));
        await cell.click();

        const column = page.locator('#stats-photos');
        await expect(column).toBeVisible();
        await expect(column.locator('.photo-column-subject')).toHaveText(label.split(',')[0]);
        await expect(column.locator('.photo-column-title')).toContainText('photo');
        const tiles = column.locator('.photo-column-tile');
        await expect(tiles.first()).toBeVisible();
        expect(await tiles.count()).toBeGreaterThanOrEqual(Math.min(counted, 1));

        await page.keyboard.press('Escape');
        await expect(column).toBeHidden();
    });

    // folder-b has years whose median is ISO 20; the axis used to start at 50
    // and drew them below it.
    test('ISO over time stays inside its plot', async ({ page }) => {
        await open(page, 'exposure');
        const chart = page.locator('.stats-chart').filter({ has: page.locator('.stats-chart-title', { hasText: /^ISO over time$/ }) });
        const plot = chart.locator('svg > g').first();
        await expect(plot.locator('circle').first()).toBeAttached();
        const escaped = await plot.evaluate((g) => {
            const axis = [...g.children].find(c => c.getAttribute('transform')?.startsWith('translate(0,'));
            const iH = Number(axis.getAttribute('transform').match(/translate\(0,([\d.]+)\)/)[1]);
            return [...g.querySelectorAll('circle')].map(c => Number(c.getAttribute('cy'))).filter(cy => !(cy >= 0 && cy <= iH));
        });
        expect(escaped).toEqual([]);
    });

    // The lens name drawn on a cell took the click, so clicking the name
    // showed nothing.
    test('clicking the name on a cell picks the cell', async ({ page }) => {
        await open(page, 'equipment');
        const label = page.locator('.stats-chart', { hasText: 'Camera and lens' }).locator('.lens-cell text').first();
        // Where the name is drawn, as a person would click it; the name itself
        // takes no pointer events.
        const box = await label.boundingBox();
        await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
        await expect(page.locator('#stats-photos')).toBeVisible();
    });

    test('a bar can be picked with the keyboard', async ({ page }) => {
        await open(page, 'exposure');
        const iso = page.locator('.stats-chart').filter({ has: page.locator('.stats-chart-title', { hasText: /^ISO$/ }) });
        const bar = iso.locator('.stats-pickable').first();
        await bar.focus();
        await page.keyboard.press('Enter');
        await expect(page.locator('#stats-photos')).toBeVisible();
        await expect(page.locator('#stats-photos .photo-column-subject')).toContainText('ISO');
    });

    test('the photo column starts open on the desk with every photo of the scope', async ({ page }) => {
        await open(page, 'time');
        const column = page.locator('#stats-photos');
        await expect(column).toBeVisible();
        await expect(column.locator('.photo-column-subject')).toHaveText(`All photos in ${LIB_NAME}`);
        await expect(column.locator('.photo-column-tile').first()).toBeVisible();
    });

    test('once closed, the photo column stays closed until a value is picked', async ({ page }) => {
        await open(page, 'exposure');
        await page.locator('#stats-photos .photo-column-close').click();
        await expect(page.locator('#stats-photos')).toBeHidden();
        await page.reload();
        await expect(page.locator('.stats-count')).toContainText('photos', { timeout: 15_000 });
        await expect(page.locator('#stats-photos')).toBeHidden();
        const iso = page.locator('.stats-chart').filter({ has: page.locator('.stats-chart-title', { hasText: /^ISO$/ }) });
        await iso.locator('.stats-pickable').first().click();
        await expect(page.locator('#stats-photos')).toBeVisible();
    });

    test.describe('on a phone', () => {
        test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

        test('the photo column waits for a picked value', async ({ page }) => {
            await open(page, 'time');
            await expect(page.locator('#stats-photos')).toBeHidden();
        });
    });
});
