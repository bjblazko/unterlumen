import { test, expect } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';
import { waitForAppReady } from '../helpers/wait.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FIXTURES_PATH = path.resolve(__dirname, '../fixtures/photos');
// Use only folder-a for library mode tests — faster indexing (~30s) since
// all of FIXTURES_PATH takes 3+ minutes due to EXIF extraction for many files.
const FIXTURES_FOLDER_A = path.resolve(__dirname, '../fixtures/photos/folder-a');

async function openInfoPanel(page) {
    await page.keyboard.press('i');
    await page.waitForSelector('.info-panel.expanded', { timeout: 5_000 });
}

// folder-a has exactly 3 immediate subdirectories: a1, a2, a3
const FOLDER_A_SUBDIR_COUNT = 3;

test.describe('Folder info panel — browse mode', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
        await openInfoPanel(page);
    });

    test('single-clicking a folder shows folder name, contents, and size map sections', async ({ page }) => {
        const folderA = page.locator('.grid-item.dir-item[data-name="folder-a"]');
        await folderA.waitFor({ state: 'visible', timeout: 5_000 });
        await folderA.click();

        await page.waitForSelector('.info-panel .info-section', { timeout: 8_000 });

        const panel = page.locator('.info-panel.expanded');
        await expect(panel).toContainText('folder-a');
        await expect(panel).toContainText('Contents');
        await expect(panel).toContainText('Subfolders');
        await expect(panel).toContainText('Size Map');
    });

    test('treemap renders one cell per immediate subfolder of folder-a', async ({ page }) => {
        const folderA = page.locator('.grid-item.dir-item[data-name="folder-a"]');
        await folderA.waitFor({ state: 'visible', timeout: 5_000 });
        await folderA.click();

        await page.waitForSelector('.folder-treemap-cell', { timeout: 8_000 });
        await expect(page.locator('.folder-treemap-cell')).toHaveCount(FOLDER_A_SUBDIR_COUNT);
    });

    test('clicking a treemap cell navigates into that subfolder', async ({ page }) => {
        const folderA = page.locator('.grid-item.dir-item[data-name="folder-a"]');
        await folderA.waitFor({ state: 'visible', timeout: 5_000 });
        await folderA.click();

        await page.waitForSelector('.folder-treemap-cell', { timeout: 8_000 });
        const firstCell = page.locator('.folder-treemap-cell').first();
        const cellPath = await firstCell.getAttribute('data-path');
        await firstCell.click();

        await expect(page.locator(`.crumb[data-path="${cellPath}"]`)).toBeVisible({ timeout: 8_000 });
    });

    test('file types chart shows extension breakdown for folder-a', async ({ page }) => {
        const folderA = page.locator('.grid-item.dir-item[data-name="folder-a"]');
        await folderA.waitFor({ state: 'visible', timeout: 5_000 });
        await folderA.click();

        await page.waitForSelector('.folder-type-chart', { timeout: 8_000 });
        // folder-a contains .jpeg, .jpg (a2, a3) and .hif (a1, a2) files
        await expect(page.locator('.folder-type-chart')).toContainText('JPEG');
        await expect(page.locator('.folder-type-chart')).toContainText('HIF');
    });

    test('clicking a photo after a folder replaces folder info with photo info', async ({ page }) => {
        // Focus a folder first
        const folderA = page.locator('.grid-item.dir-item[data-name="folder-a"]');
        await folderA.waitFor({ state: 'visible', timeout: 5_000 });
        await folderA.click();
        await page.waitForSelector('.folder-treemap-cell', { timeout: 8_000 });

        // Navigate into folder-a (contains folder-a-sample.jpeg at its root)
        await folderA.dblclick();
        await page.waitForSelector('.crumb[data-path="folder-a"]', { timeout: 5_000 });

        const photo = page.locator('[data-type="image"]').first();
        await photo.waitFor({ state: 'visible', timeout: 5_000 });
        await photo.click();

        await expect(page.locator('.folder-treemap')).not.toBeVisible({ timeout: 5_000 });
        await expect(page.locator('.info-panel.expanded')).not.toContainText('Size Map');
    });
});

test.describe('Folder info panel — library mode', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(
            existing
                .filter(l => l.name === 'E2E Folder Info')
                .map(l => request.delete(`/api/library/${l.id}`))
        );

        const res = await request.post('/api/library/', {
            data: { name: 'E2E Folder Info', description: '', sourcePath: FIXTURES_FOLDER_A },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;

        // Trigger full reindex; the SSE stream closes when the scan is done,
        // so awaiting this request blocks until indexing is complete.
        test.setTimeout(120_000);
        const reindexRes = await request.post(`/api/library/${libID}/reindex`, { timeout: 90_000 });
        expect(reindexRes.ok()).toBe(true);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Folder Info' });
        await card.waitFor({ state: 'visible', timeout: 8_000 });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });

        // Open the info panel in library mode
        await page.keyboard.press('i');
        await page.waitForSelector('#lib-info-panel .info-panel.expanded', { timeout: 5_000 });
    });

    test('single-clicking a folder in library mode shows folder sections in info panel', async ({ page }) => {
        // Library source = folder-a; its subdirs a1, a2, a3 appear as dir items at root.
        // Scope to #lib-pane to avoid strict-mode collision with the hidden browse pane.
        const a1 = page.locator('#lib-pane .grid-item.dir-item[data-name="a1"]');
        await a1.waitFor({ state: 'visible', timeout: 8_000 });
        await a1.click();

        await page.waitForSelector('#lib-info-panel .info-section', { timeout: 8_000 });

        const panel = page.locator('#lib-info-panel');
        await expect(panel).toContainText('a1');
        await expect(panel).toContainText('Contents');
        await expect(panel).toContainText('Files');
    });

    test('library folder info shows EXIF stats sections for indexed photos', async ({ page }) => {
        // a1 has 7 indexed photos; library stats should show Photos and Formats sections
        const a1 = page.locator('#lib-pane .grid-item.dir-item[data-name="a1"]');
        await a1.waitFor({ state: 'visible', timeout: 8_000 });
        await a1.click();

        await page.waitForSelector('#lib-info-panel .info-section', { timeout: 8_000 });

        const panel = page.locator('#lib-info-panel');
        await expect(panel).toContainText('Photos');
        await expect(panel).toContainText('Formats');
    });
});
