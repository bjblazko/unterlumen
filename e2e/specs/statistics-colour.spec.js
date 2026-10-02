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
        // Two stages (Colour space, Character) and six charts.
        await expect(page.locator('.stats-chart')).toHaveCount(8, { timeout: 15_000 });
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

    test('the overview has a Colour card with the Colour space as its picture', async ({ page }) => {
        await page.goto(`/#statistics?library=${lib.id}`);
        await waitForAppReady(page);
        const card = page.locator('.stats-card', { has: page.locator('.stats-card-title', { hasText: /^Colour$/ }) });
        await expect(card.locator('.point-stage--preview')).toBeAttached({ timeout: 15_000 });
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

    // Colour combinations (ADR-0049): a photo has a hue when its swatches of
    // it cover a tenth of the frame; the server counts photos per set of hues.
    async function aCombination(request) {
        const body = await (await request.get(`/api/library/colour?ids=${lib.id}`)).json();
        expect(body.hueSets.length).toBeGreaterThan(0);
        const set = body.hueSets.find(s => s.hues.length >= 2) ?? body.hueSets[0];
        const hues = set.hues.slice(0, 2);
        const found = await (await request.get(`/api/library/search?ids=${lib.id}&hues=${hues.join(',')}&limit=200`)).json();
        return { hues, total: found.total };
    }

    test('a well-known combination shows its photos', async ({ page }) => {
        await open(page);
        const row = chart(page, 'Colour combinations').locator('button.combo-item').first();
        await expect(row).toBeVisible();
        const count = Number((await row.locator('.combo-count').textContent()).replace(/\D/g, ''));
        const name = await row.locator('.combo-name').textContent();
        await row.click();
        const column = page.locator('#stats-photos');
        await expect(column.locator('.photo-column-subject')).toContainText(name);
        await expect(column.locator('.photo-column-tile')).toHaveCount(count);
    });

    test('your combination counts the hues chosen on the wheel', async ({ page, request }) => {
        const { hues, total } = await aCombination(request);
        await open(page);
        const picker = chart(page, 'Your combination');
        const sector = name => picker.locator(`.stats-pickable[aria-label="${name}"]`);
        // Start from nothing chosen, then choose the combination's hues.
        const pressed = picker.locator('.stats-pickable[aria-pressed="true"]');
        while (await pressed.count()) await pressed.first().click();
        await expect(picker.locator('.combo-sentence')).toHaveText('Choose two or three colours on the wheel.');
        const names = await page.evaluate(h => h.map(hueName), hues);
        for (const name of names) await sector(name).click();
        await expect(picker.locator('.combo-sentence')).toContainText(total === 0 ? 'No photo is' : `${total} photo`);
        if (total > 0) {
            await picker.getByRole('button', { name: 'Show photos' }).click();
            await expect(page.locator('#stats-photos .photo-column-tile')).toHaveCount(total);
        }
    });

    test('the library filter takes a colour combination', async ({ page, request }) => {
        const { hues, total } = await aCombination(request);
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await page.locator('.library-card', { hasText: LIB_NAME }).locator('.lib-open').click();
        await page.locator('#lib-filter-btn').click();
        for (const bin of hues) await page.locator(`.lib-colour-chip[data-bin="${bin}"]`).click();
        await expect(page.locator(`.lib-colour-chip[data-bin="${hues[0]}"]`)).toHaveAttribute('aria-pressed', 'true');
        await expect(page.locator('.lib-filter-chip', { hasText: 'Colours:' })).toBeVisible();
        await expect(page.locator('#lib-results-pane [data-type="image"]')).toHaveCount(total, { timeout: 15_000 });
    });
});
