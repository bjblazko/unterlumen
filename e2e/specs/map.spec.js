import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The Map place (ADR-0039): every located photo of every library, grouped by
// place, the newest photo of a group on top. folder-b holds photos with and
// without GPS, so the map has several groups and single photos.

const LIB_NAME = 'E2E Map';

// The tiles come from OpenFreeMap; the tests do not need them, only the
// markers. An empty style keeps them offline and fast.
const EMPTY_STYLE = {
    version: 8,
    sources: {},
    layers: [{ id: 'ground', type: 'background', paint: { 'background-color': '#dddddd' } }],
};

async function stubTiles(page) {
    await page.route('https://tiles.openfreemap.org/**', (route) =>
        route.fulfill({ contentType: 'application/json', body: JSON.stringify(EMPTY_STYLE) }));
}

// "12 photos" or "3 of 12 photos" → the numbers in it, thin spaces removed.
function countsIn(text) {
    return (text.match(/[\d\s]+(?= of| photo)/g) || []).map(n => Number(n.replace(/\s/g, '')));
}

async function markerTotal(page) {
    return page.locator('.map-marker').evaluateAll(markers =>
        markers.reduce((n, m) => n + Number((m.querySelector('.map-marker-count')?.textContent || '1').replace(/\s/g, '')), 0));
}

async function openMap(page) {
    await stubTiles(page);
    await page.goto('/#map');
    await waitForAppReady(page);
    await page.waitForSelector('.map-marker', { timeout: 20_000 });
}

test.describe('Map', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: 'folder-b' } });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test('lists the located photos of the library', async ({ request }) => {
        const geo = await (await request.get('/api/library/geo')).json();
        const lib = geo.libraries.find(l => l.id === libID);
        expect(lib.points.length).toBeGreaterThan(1);
        const [id, lat, lon, , filename] = lib.points[0];
        expect(id).toMatch(/^[0-9a-f]{64}$/);
        expect(Math.abs(lat)).toBeLessThanOrEqual(90);
        expect(Math.abs(lon)).toBeLessThanOrEqual(180);
        expect(filename).toMatch(/\.(jpe?g|hif|heic)$/i);
    });

    test('a photo in two libraries is one point on the map', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        const ids = await page.evaluate(() => mapPoints({ libraries: [
            { id: 'A', points: [['same', 48.1, 11.5, '2024-05-01T10:30:00', 'a.jpg'], ['own', 1, 1, '', 'b.jpg']] },
            { id: 'B', points: [['same', 48.1, 11.5, '2024-05-01T10:30:00', 'a.jpg']] },
        ] }).map(p => `${p.lib}/${p.id}`));
        expect(ids.sort()).toEqual(['A/own', 'A/same']);
    });

    test('is a place in the sidebar, at #map and on key 7', async ({ page }) => {
        await stubTiles(page);
        await page.goto('/');
        await waitForAppReady(page);

        await page.locator('#mode-map').click();
        await expect(page.locator('#mode-map')).toHaveAttribute('aria-current', 'page');
        await expect(page).toHaveURL(/#map$/);

        await page.locator('#mode-browse').click();
        await page.keyboard.press('7');
        await expect(page.locator('#mode-map')).toHaveAttribute('aria-current', 'page');
    });

    test('every located photo is in exactly one marker', async ({ page }) => {
        await openMap(page);
        const [total] = countsIn(await page.locator('.map-count').textContent());
        expect(total).toBeGreaterThan(1);
        await expect.poll(() => markerTotal(page)).toBe(total);
        // Groups show how many; a single photo does not.
        await expect(page.locator('.map-marker .map-marker-count').first()).toBeVisible();
    });

    test('a group opens by zooming in, or in the viewer when its photos share one spot', async ({ page }) => {
        await openMap(page);
        const labelsBefore = await page.locator('.map-marker').evaluateAll(ms => ms.map(m => m.getAttribute('aria-label')));
        await page.locator('.map-marker:has(.map-marker-count)').first().click();

        await expect.poll(async () => {
            if (await page.locator('.viewer').isVisible()) return 'viewer';
            const labels = await page.locator('.map-marker').evaluateAll(ms => ms.map(m => m.getAttribute('aria-label')));
            return labels.some(l => !labelsBefore.includes(l)) ? 'zoomed' : 'waiting';
        }, { timeout: 10_000 }).not.toBe('waiting');
    });

    test('a single photo opens in a read-only viewer, and Escape comes back', async ({ page }) => {
        await openMap(page);
        // Zoom in until a marker without a count exists.
        for (let i = 0; i < 6 && await page.locator('.map-marker:not(:has(.map-marker-count))').count() === 0; i++) {
            await page.locator('.map-marker').first().click();
            if (await page.locator('.viewer').isVisible()) break;
            await page.waitForTimeout(500);
        }
        if (!await page.locator('.viewer').isVisible()) {
            const single = page.locator('.map-marker:not(:has(.map-marker-count))').first();
            const name = (await single.getAttribute('aria-label')).split(',')[0];
            await single.click();
            await expect(page.locator('.viewer-filename')).toHaveText(name, { timeout: 10_000 });
        }
        await expect(page.locator('.viewer-crop-btn')).toHaveCount(0);
        await expect(page.locator('.viewer-delete')).toHaveCount(0);

        await page.keyboard.press('Escape');
        await expect(page.locator('.viewer')).toHaveCount(0);
        await expect(page.locator('.map-marker').first()).toBeVisible();
        await expect(page.locator('#mode-map')).toHaveAttribute('aria-current', 'page');
    });

    test('the time range narrows the photos on the map', async ({ page }) => {
        await openMap(page);
        const [total] = countsIn(await page.locator('.map-count').textContent());

        await page.locator('.range-handle--min').focus();
        await page.keyboard.press('End');
        await expect(page.locator('.map-count')).toContainText(' of ');
        const [shown, of] = countsIn(await page.locator('.map-count').textContent());
        expect(of).toBe(total);
        expect(shown).toBeLessThan(total);
        await expect.poll(() => markerTotal(page)).toBe(shown);

        await page.keyboard.press('Home');
        await expect(page.locator('.map-count')).not.toContainText(' of ');
        await expect.poll(() => markerTotal(page)).toBe(total);
    });

    // The photos in view: a column that is closed until asked for, follows
    // the map, and closes again with Done or Escape.
    // The panel's own button opened it empty ("Select an image to view
    // info"); only the I key loaded the photo.
    test('the info panel opened by its button in the viewer shows the photo', async ({ page }) => {
        await openMap(page);
        await page.locator('.map-photos-toggle').click();
        const tile = page.locator('#map-photos .photo-column-tile').first();
        const name = (await tile.getAttribute('aria-label')).split(',')[0];
        await tile.click();
        await expect(page.locator('.viewer')).toBeVisible();
        await page.locator('.viewer .info-toggle-btn').click();
        const body = page.locator('.viewer .info-panel-body');
        await expect(body).toContainText(name, { timeout: 10_000 });
        await expect(body).not.toContainText('Select an image');
    });

    test('the photos column shows what is in view and closes again', async ({ page }) => {
        await openMap(page);
        const btn = page.locator('.map-photos-toggle');
        await expect(page.locator('#map-photos')).toBeHidden();
        const [total] = countsIn(await page.locator('.map-count').textContent());
        await expect(btn.locator('.lib-filter-btn-count')).toHaveText(String(total).replace(/\B(?=(\d{3})+(?!\d))/g, ' '));

        await btn.click();
        await expect(page.locator('#map-photos')).toBeVisible();
        await expect(btn).toHaveAttribute('aria-expanded', 'true');
        await expect(page.locator('#map-photos .photo-column-tile')).toHaveCount(Math.min(total, 120));

        // Zooming into a group leaves fewer photos in view.
        await page.locator('.map-marker:has(.map-marker-count)').first().click();
        await expect.poll(async () => countsIn(await page.locator('#map-photos .photo-column-title').textContent())[0]).toBeLessThan(total);

        const tile = page.locator('#map-photos .photo-column-tile').nth(1);
        const name = (await tile.getAttribute('aria-label')).split(',')[0];
        await tile.click();
        await expect(page.locator('.viewer-filename')).toHaveText(name);
        await expect(page.locator('.viewer-counter')).toContainText('2 /');

        await page.keyboard.press('Escape');
        await expect(page.locator('.viewer')).toHaveCount(0);
        await expect(page.locator('#map-photos')).toBeVisible();
        await page.keyboard.press('Escape');
        await expect(page.locator('#map-photos')).toBeHidden();

        await btn.click();
        await page.locator('#map-photos .photo-column-close').click();
        await expect(page.locator('#map-photos')).toBeHidden();
    });

    test('the photos column stays open across a reload', async ({ page }) => {
        await openMap(page);
        await page.locator('.map-photos-toggle').click();
        await page.reload();
        await page.waitForSelector('.map-marker', { timeout: 20_000 });
        await expect(page.locator('#map-photos')).toBeVisible();
    });

    // Grey by default; the switch brings colour and is remembered.
    test('switches between grey and colour tiles', async ({ page }) => {
        const styles = [];
        page.on('request', (r) => {
            const m = /\/styles\/([a-z]+)/.exec(r.url());
            if (m) styles.push(m[1]);
        });
        await openMap(page);
        expect(styles).toEqual(['positron']);

        const toggle = page.locator('.map-style .toggle');
        await expect(toggle).toHaveAttribute('aria-checked', 'false');
        await expect(page.locator('.map-style')).toContainText('Style');
        await toggle.click();
        await expect.poll(() => styles).toContain('liberty');
        await expect(page.locator('.map-marker').first()).toBeVisible();

        await page.reload();
        await page.waitForSelector('.map-marker', { timeout: 20_000 });
        await expect(page.locator('.map-style .toggle')).toHaveAttribute('aria-checked', 'true');
        expect(styles.at(-1)).toBe('liberty');
    });
});

test.describe('Map on a phone', () => {
    test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

    test('is one of the tabs', async ({ page }) => {
        await stubTiles(page);
        await page.goto('/');
        await waitForAppReady(page);
        await expect(page.locator('#tab-map')).toHaveAttribute('href', '#map');
        await page.locator('#tab-map').tap();
        await expect(page.locator('.map-pane')).toBeVisible();
        await expect(page.locator('.desk-only-notice')).toBeHidden();
    });
});
