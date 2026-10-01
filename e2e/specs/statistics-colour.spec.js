import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The Colour topic of Statistics: black and white against colour, the colour
// of each period, the main colours and warm against cool, each showing its
// photos when clicked. folder-b holds one black-and-white photo, IMG_6120.

const LIB_NAME = 'E2E Statistics Colour';

test.describe('Statistics: Colour', () => {
    let lib;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: 'folder-b' } });
        expect(res.status()).toBe(201);
        lib = await res.json();
        await reindexLibrary(request, lib.id);
        // The pass after the scan runs on its own; this one returns when
        // every photo is measured.
        const analyse = await request.post(`/api/library/${lib.id}/analyse`, { timeout: 120_000 });
        expect(await analyse.text()).toContain('"finished":true');
    });

    test.afterAll(async ({ request }) => {
        if (lib) await request.delete(`/api/library/${lib.id}`);
    });

    async function open(page) {
        await page.goto(`/#statistics/colour?library=${lib.id}`);
        await waitForAppReady(page);
        await expect(page.locator('.stats-title')).toHaveText('Colour');
        await expect(page.locator('.stats-chart')).toHaveCount(4, { timeout: 15_000 });
    }

    const chart = (page, title) => page.locator('.stats-chart')
        .filter({ has: page.locator('.stats-chart-title', { hasText: new RegExp(`^${title}$`) }) });

    test('the colour API has the shape the charts read', async ({ request }) => {
        const body = await (await request.get(`/api/library/colour?ids=${lib.id}`)).json();
        expect(body.analysedPhotos).toBeGreaterThan(0);
        expect(body.unanalysedPhotos).toBe(0);
        expect(body.classes.map(c => c.class)).toEqual(['mono', 'tinted', 'colour']);
        expect(body.classes[0].counts.reduce((a, b) => a + b, 0)).toBe(1);
        expect(body.periodColours.length).toBeGreaterThan(0);
        expect(body.hueWheel.length).toBeGreaterThan(0);
        expect(body.seasons).toHaveLength(12);
    });

    test('the overview has a Colour card with the strip', async ({ page }) => {
        await page.goto(`/#statistics?library=${lib.id}`);
        await waitForAppReady(page);
        const card = page.locator('.stats-card', { hasText: 'Colour' });
        await expect(card.locator('svg rect').first()).toBeAttached({ timeout: 15_000 });
    });

    test('the black-and-white photo is found from its share', async ({ request }) => {
        // The line chart takes its clicks by position; the criterion it
        // builds is the search below.
        const body = await (await request.get(`/api/library/search?ids=${lib.id}&mono=mono`)).json();
        expect(body.results.map(r => r.filename)).toEqual(['IMG_6120.jpeg']);
    });

    test('a swatch of the strip shows the colour photos of its period', async ({ page }) => {
        await open(page);
        const swatch = chart(page, 'Colour of each period').locator('.stats-pickable').first();
        const label = await swatch.getAttribute('aria-label');
        await swatch.click();
        const column = page.locator('#stats-photos');
        await expect(column).toBeVisible();
        await expect(column.locator('.photo-column-subject')).toHaveText(`Colour photos · ${label.split(',')[0]}`);
        await expect(column.locator('.photo-column-tile').first()).toBeVisible();
    });

    test('a sector of the wheel names its hue and shows its photos', async ({ page }) => {
        await open(page);
        const sector = chart(page, 'Main colours').locator('.stats-pickable').first();
        const hue = (await sector.getAttribute('aria-label')).split(',')[0];
        await sector.click();
        const column = page.locator('#stats-photos');
        await expect(column.locator('.photo-column-subject')).toContainText(hue, { ignoreCase: true });
        await expect(column.locator('.photo-column-tile').first()).toBeVisible();
    });

    test('a warm or cool bar can be picked with the keyboard', async ({ page }) => {
        await open(page);
        const bar = chart(page, 'Warm and cool through the year').locator('.stats-pickable').first();
        await bar.focus();
        await page.keyboard.press('Enter');
        await expect(page.locator('#stats-photos .photo-column-subject')).toHaveText(/^(Warm|Cool) · \w+$/);
        await expect(page.locator('#stats-photos .photo-column-tile').first()).toBeVisible();
    });

    test('warm and cool can be narrowed to one year', async ({ page }) => {
        await open(page);
        const card = chart(page, 'Warm and cool through the year');
        const select = card.locator('.stats-season-year');
        const year = await select.locator('option').nth(1).getAttribute('value');
        await select.selectOption(year);
        const bar = card.locator('.stats-pickable').first();
        await expect(bar).toHaveAttribute('aria-label', new RegExp(` ${year}: `));
        await bar.click();
        await expect(page.locator('#stats-photos .photo-column-subject')).toHaveText(new RegExp(`^(Warm|Cool) · \\w+ ${year}$`));
        await expect(page.locator('#stats-photos .photo-column-tile').first()).toBeVisible();
    });
});
