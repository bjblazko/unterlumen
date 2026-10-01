import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The Colour space topic of Statistics: every analysed photo as a light in
// OKLab, drawn with WebGPU (ADR-0046). Headless Chromium may have no WebGPU;
// then the topic must say so, and the drawing itself is not tested.

const LIB_NAME = 'E2E Statistics Colour space';

test.describe('Statistics: Colour space', () => {
    let lib;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: 'folder-b' } });
        expect(res.status()).toBe(201);
        lib = await res.json();
        await reindexLibrary(request, lib.id);
        const analyse = await request.post(`/api/library/${lib.id}/analyse`, { timeout: 120_000 });
        expect(await analyse.text()).toContain('"finished":true');
    });

    test.afterAll(async ({ request }) => {
        if (lib) await request.delete(`/api/library/${lib.id}`);
    });

    test('the colour space API has one point per analysed photo and a path', async ({ request }) => {
        const body = await (await request.get(`/api/library/colour-space?ids=${lib.id}`)).json();
        expect(body.analysedPhotos).toBeGreaterThan(0);
        expect(body.libraries).toEqual([lib.id]);
        for (const col of ['id', 'lib', 'date', 'mono', 'l', 'a', 'b']) {
            expect(body.points[col]).toHaveLength(body.analysedPhotos);
        }
        expect(body.path.length).toBeGreaterThan(0);
        expect(body.path[0]).toEqual(expect.objectContaining({ period: expect.any(String), photos: expect.any(Number) }));
    });

    test('one photo is found by its id, as a click on a light finds it', async ({ request }) => {
        const body = await (await request.get(`/api/library/colour-space?ids=${lib.id}`)).json();
        const id = body.points.id[0];
        const found = await (await request.get(`/api/library/search?ids=${lib.id}&photo_id=${id}`)).json();
        expect(found.results.map(r => r.id)).toEqual([id]);
    });

    test('the topic draws the stage, or says that WebGPU is missing', async ({ page }) => {
        await page.goto(`/#statistics/colour-space?library=${lib.id}`);
        await waitForAppReady(page);
        await expect(page.locator('.stats-title')).toHaveText('Colour space');
        const hasGPU = await page.evaluate(async () => !!(navigator.gpu && await navigator.gpu.requestAdapter().catch(() => null)));
        if (hasGPU) {
            await expect(page.locator('.colour-space-canvas')).toBeVisible({ timeout: 15_000 });
            await expect(page.locator('.colour-space-label', { hasText: 'Sky blue' })).toBeAttached();
            await expect(page.locator('.colour-space-help')).toContainText('each at its main colour');
            await expect(page.locator('.colour-space-legend')).toContainText('One photo, at its main colour');
            await expect(page.locator('.colour-space-legend')).toContainText('The line joins the');
        } else {
            await expect(page.locator('.colour-space-nogpu')).toContainText('needs WebGPU', { timeout: 15_000 });
        }
    });

    test('Full view shows the chart and the photos over the window, dark, until Escape', async ({ page }) => {
        await page.emulateMedia({ colorScheme: 'light' });
        await page.goto(`/#statistics/colour-space?library=${lib.id}`);
        await waitForAppReady(page);
        const place = page.locator('.stats-place');
        const button = page.locator('.stats-stage-btn');
        await expect(button).toHaveText('Full view', { timeout: 15_000 });
        await button.click();
        await expect(place).toHaveClass(/stats-place--stage/);
        await expect(place).toHaveAttribute('data-theme', 'dark');
        await expect(page.locator('.stats-head')).toBeHidden();
        await expect(page.locator('#stats-photos')).toBeVisible();
        await expect(button).toHaveText('Leave full view');
        const box = await place.boundingBox();
        expect(box.x).toBe(0);
        expect(box.width).toBe(page.viewportSize().width);

        // Escape leaves the full view first and keeps the photos.
        await page.keyboard.press('Escape');
        await expect(place).not.toHaveClass(/stats-place--stage/);
        await expect(place).not.toHaveAttribute('data-theme', 'dark');
        await expect(page.locator('#stats-photos')).toBeVisible();
        await expect(button).toHaveText('Full view');
    });

    test('a selected stage owns the arrow keys until Escape', async ({ page }) => {
        await page.goto(`/#statistics/colour-space?library=${lib.id}`);
        await waitForAppReady(page);
        const canvas = page.locator('.colour-space-canvas');
        test.skip(!(await canvas.isVisible({ timeout: 15_000 }).catch(() => false)), 'no WebGPU in this browser');
        await canvas.focus();
        await expect(canvas).toHaveClass(/keyboard-owner/);
        await page.keyboard.press('Escape');
        await expect(canvas).not.toHaveClass(/keyboard-owner/);
    });
});
