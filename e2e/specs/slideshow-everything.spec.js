import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';
import { reindexLibrary } from '../helpers/library.js';

// A slideshow takes everything it is asked for, not only what has been
// scrolled to: a folder with nothing selected takes its subfolders too, ⌘A
// takes the folders with the photos, and Select all in the filter's results
// reads the results not yet loaded. folder-a holds no photo of its own and
// 30 in a1, a2 and a3 (as git has them; see NOTES.md).

const FOLDER_A_PHOTOS = 30;

const slideshowCount = async (page) => {
    await page.locator('.browse-slideshow-btn:visible').click();
    const subtitle = page.locator('.slideshow-dialog .dialog-subtitle');
    await expect(subtitle).toBeVisible({ timeout: 15_000 });
    const text = await subtitle.textContent();
    await page.keyboard.press('Escape');
    return text;
};

test.describe('Slideshow of everything — Folders', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-a');
        await page.locator('.dir-item[data-name="a1"]').first().waitFor();
    });

    test('a folder of folders with nothing selected shows all their photos', async ({ page }) => {
        await expect(page.locator('.browse-slideshow-btn:visible')).toBeEnabled();
        expect(await slideshowCount(page)).toBe(`${FOLDER_A_PHOTOS} photos`);
    });

    test('⌘A selects the folders too, and the slideshow takes their photos', async ({ page }) => {
        await page.locator('.place-header:visible .place-title').click();
        await page.keyboard.press('ControlOrMeta+a');
        await expect(page.locator('.dir-item.selected')).toHaveCount(3);
        expect(await slideshowCount(page)).toBe(`${FOLDER_A_PHOTOS} photos`);
    });
});

test.describe('Slideshow of everything — results', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === 'E2E Slideshow Results').map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', {
            data: { name: 'E2E Slideshow Results', description: '', sourcePath: 'folder-a' },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test('Select all reads the results not yet loaded', async ({ page }) => {
        // Pages of 10 instead of 100, so 30 results are three pages.
        await page.route(/\/api\/library\/search\?/, (route) => {
            const url = new URL(route.request().url());
            if (url.searchParams.get('limit') === '100') url.searchParams.set('limit', '10');
            route.continue({ url: url.toString() });
        });
        await page.goto('/#folders');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });
        await page.locator('#lib-filter-btn').click();
        await page.waitForSelector('.lib-filter-select', { timeout: 20_000 });
        await page.locator('.lib-filter-select').first().selectOption(String(libID));
        const meta = page.locator('#lib-results-pane .place-meta');
        await expect(meta).toHaveText(`${FOLDER_A_PHOTOS} photos`, { timeout: 15_000 });
        expect(await page.locator('#lib-results-pane [data-type="image"]').count()).toBeLessThan(FOLDER_A_PHOTOS);

        await page.locator('#lib-results-pane .select-all-all').click();
        await expect(page.locator('#lib-results-pane .select-all-count')).toHaveText(`${FOLDER_A_PHOTOS} selected`);
        expect(await slideshowCount(page)).toBe(`${FOLDER_A_PHOTOS} photos`);
    });
});
