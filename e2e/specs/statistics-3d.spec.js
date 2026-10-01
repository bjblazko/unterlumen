import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The Exposure space, Daylight, Character and Space and time topics of
// Statistics: 3D stages drawn with WebGPU (ADR-0046). Headless Chromium may have no WebGPU; then the
// topics must say so, and the drawing itself is not tested.

const LIB_NAME = 'E2E Statistics 3D';

test.describe('Statistics: the 3D topics', () => {
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

    test('the exposure space API has one point per photo with all three settings', async ({ request }) => {
        const body = await (await request.get(`/api/library/exposure-space?ids=${lib.id}`)).json();
        expect(body.photos).toBeGreaterThan(0);
        expect(body.libraries).toEqual([lib.id]);
        for (const col of ['id', 'lib', 'date', 'camera', 'focal', 'fnum', 'iso']) {
            expect(body.points[col]).toHaveLength(body.photos);
        }
        expect(body.cameras.length).toBeGreaterThan(0);
        for (const name of body.cameras) expect(name).not.toMatch(/^"|"$/);
        expect(Math.max(...body.points.camera)).toBeLessThan(body.cameras.length);
        expect(body.path.length).toBeGreaterThan(0);
    });

    test('the colour space points carry what Daylight and Character draw', async ({ request }) => {
        const body = await (await request.get(`/api/library/colour-space?ids=${lib.id}`)).json();
        for (const col of ['lum', 'contrast', 'colourful']) {
            expect(body.points[col]).toHaveLength(body.analysedPhotos);
            for (const v of body.points[col]) expect(v).toBeGreaterThanOrEqual(0);
        }
    });

    test('the space and time API has every located, dated photo', async ({ request }) => {
        const body = await (await request.get(`/api/library/space-time?ids=${lib.id}`)).json();
        expect(body.libraries).toEqual([lib.id]);
        for (const col of ['id', 'lib', 'date', 'lat', 'lon', 'l', 'a', 'b']) {
            expect(body.points[col]).toHaveLength(body.photos);
        }
        for (const lat of body.points.lat) expect(Math.abs(lat)).toBeLessThanOrEqual(90);
    });

    test('the world outline for Space and time is served', async ({ request }) => {
        const res = await request.get('/data/natural-earth-lines.json');
        expect(res.ok()).toBe(true);
        const body = await res.json();
        expect(body.coast.length).toBeGreaterThan(100);
        expect(body.borders.length).toBeGreaterThan(100);
    });

    for (const [topic, title, legend] of [
        ['exposure-space', 'Exposure space', 'the median focal length, aperture and ISO'],
        ['daylight', 'Daylight', 'One month of the year'],
        ['character', 'Character', 'the average brightness, contrast and colourfulness'],
    ]) {
        test(`${title} draws its stage, or says that WebGPU is missing`, async ({ page }) => {
            await page.goto(`/#statistics/${topic}?library=${lib.id}`);
            await waitForAppReady(page);
            await expect(page.locator('.stats-title')).toHaveText(title);
            await expect(page.locator('.stats-stage-btn')).toHaveText('Full view');
            const hasGPU = await page.evaluate(async () => !!(navigator.gpu && await navigator.gpu.requestAdapter().catch(() => null)));
            if (hasGPU) {
                await expect(page.locator('.point-stage-canvas')).toBeVisible({ timeout: 15_000 });
                await expect(page.locator('.point-stage-legend')).toContainText(legend);
            } else {
                await expect(page.locator('.point-stage-nogpu')).toContainText('needs WebGPU', { timeout: 15_000 });
            }
        });
    }

    test('Space and time draws its stage or says why not', async ({ page }) => {
        await page.goto(`/#statistics/space-time?library=${lib.id}`);
        await waitForAppReady(page);
        await expect(page.locator('.stats-title')).toHaveText('Space and time');
        await expect(page.locator('.point-stage-canvas, .point-stage-nogpu, .stats-nodata').first()).toBeAttached({ timeout: 15_000 });
    });

    test('Daylight has no Periods select, the Exposure space has', async ({ page }) => {
        await page.goto(`/#statistics/daylight?library=${lib.id}`);
        await waitForAppReady(page);
        await expect(page.locator('.stats-title')).toHaveText('Daylight');
        await expect(page.locator('.stats-granularity')).toHaveCount(0);
        await page.goto(`/#statistics/exposure-space?library=${lib.id}`);
        await expect(page.locator('.stats-title')).toHaveText('Exposure space');
        await expect(page.locator('.stats-granularity')).toHaveCount(1);
    });
});
