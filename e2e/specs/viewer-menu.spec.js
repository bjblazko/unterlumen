import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The full view's ⋯ menu: what an overview does to a selection, for the photo
// on screen — from a library and from the Map, Statistics and the Timeline,
// which open the same viewer (feature doc 2026-10-02-viewer-menu).
//
// The library is folder-a, not the served folder, so a path relative to the
// library would be wrong on the server: the menu must resolve it.

const LIB_NAME = 'E2E Viewer Menu';
const PHOTO = '_DSF2027.jpg'; // folder-a/a3, located, so it is on the Map

const EMPTY_STYLE = {
    version: 8,
    sources: {},
    layers: [{ id: 'ground', type: 'background', paint: { 'background-color': '#dddddd' } }],
};

async function openFromMap(page) {
    await page.route('https://tiles.openfreemap.org/**', (route) =>
        route.fulfill({ contentType: 'application/json', body: JSON.stringify(EMPTY_STYLE) }));
    await page.goto('/#map');
    await waitForAppReady(page);
    const tile = page.locator(`#map-photos .photo-column-tile[aria-label^="${PHOTO}"]`);
    await tile.first().click({ timeout: 20_000 });
    await expect(page.locator('.viewer-filename')).toHaveText(new RegExp(PHOTO.replace('.', '\\.')));
}

async function choose(page, id) {
    await page.locator('.viewer .menu-btn').click();
    await page.locator(`.menu .menu-item[data-id="${id}"]`).click();
}

test.describe('Full view menu', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: 'folder-a' } });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test('a photo from the Map has the library actions', async ({ page }) => {
        await openFromMap(page);
        await page.locator('.viewer .menu-btn').click();
        const ids = await page.locator('.menu .menu-item').evaluateAll(els => els.map(e => e.dataset.id));
        expect(ids).toEqual(['collect', 'export', 'rename', 'location', 'organize', 'library']);
    });

    test('Show in library opens its folder with the photo selected', async ({ page }) => {
        await openFromMap(page);
        await choose(page, 'library');
        await expect(page.locator('.viewer')).toHaveCount(0);
        await expect(page.locator('#mode-library')).toHaveAttribute('aria-current', 'page');
        const selected = page.locator('#lib-pane [data-type="image"].selected');
        await expect(selected).toHaveCount(1, { timeout: 10_000 });
        await expect(selected).toHaveAttribute('data-name', PHOTO);
        await expect(selected).toHaveClass(/focused/);
    });

    test('Add to gallery asks for the gallery, and Escape leaves the photo open', async ({ page }) => {
        await openFromMap(page);
        await choose(page, 'collect');
        await expect(page.locator('.dialog-scrim')).toContainText('Add 1 photo to a gallery');
        await page.keyboard.press('Escape');
        await expect(page.locator('.dialog-scrim')).toHaveCount(0);
        await expect(page.locator('.viewer')).toBeVisible();
    });

    test('Set location reads the photo by its path under the served folder', async ({ page }) => {
        await openFromMap(page);
        const info = page.waitForRequest(r => r.url().includes('/api/info?path=') &&
            decodeURIComponent(r.url().split('path=')[1]) === `folder-a/a3/${PHOTO}`);
        await choose(page, 'location');
        await info;
        await expect(page.locator('.dialog-scrim')).toBeVisible();
    });

    test('a library\'s own Set location sends paths under the served folder, not the library', async ({ page }) => {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await page.locator('.library-card', { hasText: LIB_NAME }).locator('.lib-open').click();
        await page.locator('#lib-pane .folder-tile[data-name="a3"]').dblclick();
        await page.locator(`#lib-pane [data-name="${PHOTO}"]`).click();
        const info = page.waitForRequest(r => r.url().includes('/api/info?path=') &&
            decodeURIComponent(r.url().split('path=')[1]) === `folder-a/a3/${PHOTO}`);
        await page.locator('.selection-bar [data-action="location"]').click();
        await info;
    });

    test('a photo in Folders has what Folders offers, and Show in Organize goes there', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await page.locator('[data-type="dir"][data-name="folder-a"]').dblclick();
        await page.locator('[data-type="dir"][data-name="a3"]').dblclick();
        await page.locator(`[data-type="image"][data-name="${PHOTO}"]`).dblclick();
        await expect(page.locator('.viewer')).toBeVisible();
        await page.locator('.viewer .menu-btn').click();
        const ids = await page.locator('.menu .menu-item').evaluateAll(els => els.map(e => e.dataset.id));
        expect(ids).toEqual(['export', 'rename', 'location', 'organize']);
        await page.locator('.menu .menu-item[data-id="organize"]').click();
        await expect(page.locator('.viewer')).toHaveCount(0);
        await expect(page).toHaveURL(/#organize$/);
    });
});
