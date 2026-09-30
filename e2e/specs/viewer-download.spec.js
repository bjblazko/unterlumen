import { test, expect } from '@playwright/test';
import { statSync, readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';
import { GPS_IMAGE, GPS_PATH, HIF_IMAGE, HIF_PATH } from '../helpers/fixtures.js';

// The viewer's download button saves the original file under its own name,
// wherever the viewer was opened from — a HEIF included, which the viewer
// itself shows as JPEG.

const PHOTOS = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../fixtures/photos');

async function downloadFromViewer(page) {
    const [download] = await Promise.all([
        page.waitForEvent('download'),
        page.locator('.viewer-download-btn').click(),
    ]);
    return { name: download.suggestedFilename(), bytes: readFileSync(await download.path()) };
}

async function downloadSelection(page) {
    const [download] = await Promise.all([
        page.waitForEvent('download', { timeout: 30_000 }),
        page.locator('.selection-bar [data-action="download"]').click(),
    ]);
    return { name: download.suggestedFilename(), file: await download.path() };
}

// Entry names of a ZIP, and one entry's bytes.
const zipNames = (file) => execFileSync('unzip', ['-Z1', file]).toString().trim().split('\n');
const zipEntry = (file, name) => execFileSync('unzip', ['-p', file, name], { maxBuffer: 100 * 1024 * 1024 });

async function openInFolder(page, folderPath, name) {
    await page.goto('/');
    await waitForAppReady(page);
    // Nested folders: each step's crumb carries the path so far.
    const parts = folderPath.split('/').slice(0, -1);
    for (let i = 0; i < parts.length; i++) {
        await page.locator(`.dir-item[data-name="${parts[i]}"]`).first().dblclick();
        await page.waitForSelector(`.crumb[data-path="${parts.slice(0, i + 1).join('/')}"]`);
    }
    await waitForThumbnailsLoaded(page, 1);
    await page.locator(`[data-name="${name}"]`).dblclick();
    await expect(page.locator('.viewer')).toBeVisible();
}

test.describe('Download the original from the viewer', () => {
    test('from Folders, the file as it is on disk', async ({ page }) => {
        await openInFolder(page, GPS_PATH, GPS_IMAGE);
        await expect(page.locator('.viewer-download-btn')).toHaveAttribute('aria-label', 'Download the original file');
        const { name, bytes } = await downloadFromViewer(page);
        expect(name).toBe(GPS_IMAGE);
        expect(bytes.length).toBe(statSync(path.join(PHOTOS, GPS_PATH)).size);
    });

    test('a HEIF file comes as HEIF, not as the JPEG the viewer shows', async ({ page }) => {
        await openInFolder(page, HIF_PATH, HIF_IMAGE);
        const { name, bytes } = await downloadFromViewer(page);
        expect(name).toBe(HIF_IMAGE);
        expect(bytes.equals(readFileSync(path.join(PHOTOS, HIF_PATH)))).toBe(true);
    });

    test.describe('from a library and from the map', () => {
        const LIB_NAME = 'E2E Download';
        let libID;

        test.beforeAll(async ({ request }) => {
            const existing = await (await request.get('/api/library/')).json();
            await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
            const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: 'folder-b' } });
            libID = (await res.json()).id;
            await reindexLibrary(request, libID);
        });

        test.afterAll(async ({ request }) => {
            if (libID) await request.delete(`/api/library/${libID}`);
        });

        test('from a library', async ({ page }) => {
            await page.goto('/#libraries');
            await waitForAppReady(page);
            await page.locator('.library-card', { hasText: LIB_NAME }).locator('.lib-open').click();
            await page.locator(`#lib-pane [data-type="image"]`).first().waitFor();
            await page.locator(`#lib-pane [data-type="image"]`).first().dblclick();
            await expect(page.locator('.viewer')).toBeVisible();
            const shown = (await page.locator('.viewer-filename').textContent()).trim();
            const { name, bytes } = await downloadFromViewer(page);
            expect(name).toBe(shown);
            expect(bytes.length).toBe(statSync(path.join(PHOTOS, 'folder-b', shown)).size);
        });

        test('several photos of a library as a ZIP of the originals', async ({ page }) => {
            await page.goto('/#libraries');
            await waitForAppReady(page);
            await page.locator('.library-card', { hasText: LIB_NAME }).locator('.lib-open').click();
            const images = page.locator('#lib-pane [data-type="image"]');
            await images.nth(2).waitFor();
            await images.nth(0).click();
            await images.nth(1).click({ modifiers: ['ControlOrMeta'] });
            const labels = await page.locator('#lib-pane [data-type="image"].selected').evaluateAll(els => els.map(e => e.dataset.name));
            const { name, file } = await downloadSelection(page);
            expect(name).toMatch(/\.zip$/);
            expect(zipNames(file)).toHaveLength(2);
            for (const entry of zipNames(file)) {
                expect(zipEntry(file, entry).equals(readFileSync(path.join(PHOTOS, 'folder-b', entry)))).toBe(true);
            }
            expect(labels).toHaveLength(2);
        });

        // Filter results carry absolute paths, which a server refuses; the
        // page sends library and photo IDs instead.
        test('filter results go by library and ID, not by path', async ({ page }) => {
            await page.goto('/#libraries');
            await waitForAppReady(page);
            await page.locator('.library-card', { hasText: LIB_NAME }).locator('.lib-open').click();
            await page.locator('#lib-filter-btn').click();
            const camera = page.locator('.lib-text-filter-select').first();
            await camera.waitFor();
            await camera.selectOption({ index: 1 });
            const results = page.locator('#lib-results-pane [data-type="image"]');
            await results.nth(1).waitFor();
            await results.nth(0).click();
            await results.nth(1).click({ modifiers: ['ControlOrMeta'] });

            const [request] = await Promise.all([
                page.waitForRequest('**/api/export/zip-stream'),
                downloadSelection(page).then(d => { downloaded = d; }),
            ]);
            const body = request.postDataJSON();
            expect(body.photos).toHaveLength(2);
            expect(body.files).toEqual([]);
            expect(zipNames(downloaded.file)).toHaveLength(2);
        });
        let downloaded;

        test('Export… from filter results sends them by ID as well', async ({ page }) => {
            await page.goto('/#libraries');
            await waitForAppReady(page);
            await page.locator('.library-card', { hasText: LIB_NAME }).locator('.lib-open').click();
            await page.locator('#lib-filter-btn').click();
            const camera = page.locator('.lib-text-filter-select').first();
            await camera.waitFor();
            await camera.selectOption({ index: 1 });
            const results = page.locator('#lib-results-pane [data-type="image"]');
            await results.nth(1).waitFor();
            await results.nth(0).click();
            await results.nth(1).click({ modifiers: ['ControlOrMeta'] });
            const [estimate] = await Promise.all([
                page.waitForRequest('**/api/export/estimate'),
                page.locator('.selection-bar [data-action="export"]').click(),
            ]);
            // The size estimates find them by ID too, answered under their path.
            const est = estimate.postDataJSON();
            expect(est.files).toEqual([]);
            expect(est.photos).toHaveLength(2);
            expect(est.photos.every(p => p.key && p.id && p.library)).toBe(true);
            await expect(page.locator('.export-total-in')).not.toHaveText(/…|—/);
            await page.locator('.export-dialog input[name="output-mode"][value="zip"]').check();
            const [request] = await Promise.all([
                page.waitForRequest('**/api/export/zip-stream'),
                page.waitForEvent('download', { timeout: 30_000 }),
                page.locator('.export-dialog .btn-accent').click(),
            ]);
            const body = request.postDataJSON();
            expect(body.photos).toHaveLength(2);
            expect(body.files).toEqual([]);
        });

        test('from the map, where the viewer is otherwise read-only', async ({ page }) => {
            await page.route('https://tiles.openfreemap.org/**', (route) => route.fulfill({
                contentType: 'application/json',
                body: JSON.stringify({ version: 8, sources: {}, layers: [{ id: 'bg', type: 'background', paint: { 'background-color': '#ddd' } }] }),
            }));
            await page.goto('/#map');
            await waitForAppReady(page);
            await page.locator('.map-photos-toggle').click();
            await page.locator('#map-photos .photo-column-tile').first().click();
            await expect(page.locator('.viewer')).toBeVisible();
            await expect(page.locator('.viewer-delete')).toHaveCount(0);
            const shown = (await page.locator('.viewer-filename').textContent()).trim();
            const { name } = await downloadFromViewer(page);
            expect(name).toBe(shown);
        });
    });

    test.describe('from the selection bar', () => {
        async function selectInFolderB(page, count) {
            await page.goto('/');
            await waitForAppReady(page);
            await page.locator('.dir-item[data-name="folder-b"]').first().dblclick();
            await waitForThumbnailsLoaded(page, count);
            const images = page.locator('[data-type="image"]');
            await images.nth(0).click();
            for (let i = 1; i < count; i++) await images.nth(i).click({ modifiers: ['ControlOrMeta'] });
            return Promise.all([...Array(count).keys()].map(i => images.nth(i).getAttribute('data-name')));
        }

        test('one photo downloads as its file', async ({ page }) => {
            const [name] = await selectInFolderB(page, 1);
            const { name: saved, file } = await downloadSelection(page);
            expect(saved).toBe(name);
            expect(readFileSync(file).equals(readFileSync(path.join(PHOTOS, 'folder-b', name)))).toBe(true);
        });

        test('a selected folder comes as a ZIP with everything in it, in its folders', async ({ page }) => {
            await page.goto('/');
            await waitForAppReady(page);
            await page.locator('.folder-tile[data-name="folder-a"]').click();
            // Only folders: Download works, the photo-only actions say why not.
            await expect(page.locator('.selection-bar-count')).toHaveText('1 selected');
            await expect(page.locator('.selection-bar [data-action="export"]')).toBeDisabled();
            await expect(page.locator('.selection-bar [data-action="export"]')).toHaveAttribute('title', /Works on photos/);
            const { name, file } = await downloadSelection(page);
            expect(name).toBe('folder-a.zip');
            const entries = zipNames(file);
            expect(entries).toContain('folder-a/folder-a-sample.jpeg');
            expect(entries).toContain(`folder-a/a1/${HIF_IMAGE}`);
            expect(entries.every(e => e.startsWith('folder-a/'))).toBe(true);
            expect(zipEntry(file, `folder-a/a1/${HIF_IMAGE}`).equals(readFileSync(path.join(PHOTOS, HIF_PATH)))).toBe(true);
        });

        test('several photos come as a ZIP of the originals, named after the folder', async ({ page }) => {
            const names = await selectInFolderB(page, 3);
            const { name, file } = await downloadSelection(page);
            expect(name).toBe('folder-b.zip');
            expect(zipNames(file).sort()).toEqual([...names].sort());
            expect(zipEntry(file, names[1]).equals(readFileSync(path.join(PHOTOS, 'folder-b', names[1])))).toBe(true);
        });
    });
});
