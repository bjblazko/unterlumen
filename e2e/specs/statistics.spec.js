import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

test.describe('Statistics modal', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        // Create a test library (unindexed — stats will be empty but modal structure is testable)
        const res = await request.post('/api/library/', {
            data: { name: 'Stats test library', description: '', sourcePath: '/tmp' },
        });
        expect(res.status()).toBe(201);
        const body = await res.json();
        libID = body.id;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test('Statistics API returns expected shape', async ({ request }) => {
        const res = await request.get(`/api/library/statistics?ids=${libID}`);
        expect(res.status()).toBe(200);
        const body = await res.json();
        expect(typeof body.totalPhotos).toBe('number');
        expect(Array.isArray(body.formats)).toBe(true);
        expect(Array.isArray(body.filmSims)).toBe(true);
        // focalLengths, focalLengths35, apertures, isos are {value, count} pairs
        expect(Array.isArray(body.focalLengths)).toBe(true);
        expect(Array.isArray(body.focalLengths35)).toBe(true);
        expect(Array.isArray(body.apertures)).toBe(true);
        expect(Array.isArray(body.isos)).toBe(true);
        for (const arr of [body.focalLengths, body.focalLengths35, body.apertures, body.isos]) {
            for (const item of arr) {
                expect(typeof item.value).toBe('number');
                expect(typeof item.count).toBe('number');
            }
        }
        expect(Array.isArray(body.cameraLens)).toBe(true);
        expect(Array.isArray(body.shootingHours)).toBe(true);
        expect(body.shootingHours).toHaveLength(24);
        expect(typeof body.shootingDays).toBe('object');
    });

    test('Statistics API with no ids returns all libraries', async ({ request }) => {
        const res = await request.get('/api/library/statistics');
        expect(res.status()).toBe(200);
        const body = await res.json();
        expect(typeof body.totalPhotos).toBe('number');
        expect(Array.isArray(body.shootingHours)).toBe(true);
    });

    test('Statistics button opens modal in library mode', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const statsBtn = page.locator('#lib-stats-btn');
        await expect(statsBtn).toBeVisible();
        await statsBtn.click();

        await page.waitForSelector('.stats-overlay', { timeout: 5_000 });
        await expect(page.locator('.stats-overlay')).toBeVisible();
        await expect(page.locator('.modal-title')).toContainText('Statistics');
    });

    test('Statistics modal shows chart cards', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });
        await page.locator('#lib-stats-btn').click();
        await page.waitForSelector('.stats-grid', { timeout: 15_000 });

        const cards = page.locator('.stats-chart');
        await expect(cards).toHaveCount(8);
    });

    test('Statistics modal has library filter dropdown', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });
        await page.locator('#lib-stats-btn').click();
        await page.waitForSelector('.stats-lib-select', { timeout: 5_000 });
        await expect(page.locator('.stats-lib-select')).toBeVisible();
    });

    test('Escape closes statistics modal', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });
        await page.locator('#lib-stats-btn').click();
        await page.waitForSelector('.stats-overlay', { timeout: 5_000 });

        await page.keyboard.press('Escape');
        await expect(page.locator('.stats-overlay')).not.toBeVisible({ timeout: 3_000 });
    });

    test('Close button dismisses modal', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });
        await page.locator('#lib-stats-btn').click();
        await page.waitForSelector('.stats-overlay', { timeout: 5_000 });

        await page.locator('#stats-close').click();
        await expect(page.locator('.stats-overlay')).not.toBeVisible({ timeout: 3_000 });
    });
});
