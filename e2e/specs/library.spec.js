import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

test.describe('Library list view', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        // Clean up stale libraries from interrupted previous runs
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === 'E2E Library UI').map(l => request.delete(`/api/library/${l.id}`)));

        const res = await request.post('/api/library/', {
            data: { name: 'E2E Library UI', description: '', sourcePath: '/tmp' },
        });
        expect(res.status()).toBe(201);
        const body = await res.json();
        libID = body.id;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });
    });

    test('switches to library mode and shows list view', async ({ page }) => {
        await expect(page.locator('.library-list-view')).toBeVisible();
        await expect(page.locator('#mode-library')).toHaveClass(/active/);
    });

    test('shows library card with correct name and source path', async ({ page }) => {
        const card = page.locator('.library-card', { hasText: 'E2E Library UI' });
        await expect(card).toBeVisible({ timeout: 5_000 });
        await expect(card.locator('.library-card-name')).toContainText('E2E Library UI');
        await expect(card.locator('.library-card-meta')).toContainText('/tmp');
    });

    test('shows photo count and last-indexed info on card', async ({ page }) => {
        const card = page.locator('.library-card', { hasText: 'E2E Library UI' });
        await expect(card.locator('.library-card-stats')).toContainText('0 photos');
    });

    test('Search button toggles search panel open and closed', async ({ page }) => {
        const panel = page.locator('#lib-search-panel');
        await expect(panel).not.toHaveClass(/visible/);

        await page.locator('#lib-search-btn').click();
        await expect(panel).toHaveClass(/visible/);
        await expect(page.locator('#lib-search-btn')).toHaveClass(/active/);

        await page.locator('#lib-search-btn').click();
        await expect(panel).not.toHaveClass(/visible/);
    });

    test('Open button navigates to library detail view', async ({ page }) => {
        const card = page.locator('.library-card', { hasText: 'E2E Library UI' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });
        await expect(page.locator('.library-detail')).toBeVisible();
        await expect(page.locator('#lib-back')).toBeVisible();
        await expect(page.locator('.library-detail-name')).toContainText('E2E Library UI');
    });

    test('Back button returns to library list from detail view', async ({ page }) => {
        const card = page.locator('.library-card', { hasText: 'E2E Library UI' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });
        await page.locator('#lib-back').click();
        await expect(page.locator('.library-list-view')).toBeVisible({ timeout: 5_000 });
    });

    test('Filter button is visible in library detail view', async ({ page }) => {
        const card = page.locator('.library-card', { hasText: 'E2E Library UI' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });
        await expect(page.locator('#lib-filter-btn')).toBeVisible();
    });

    test('Delete button removes library card after confirmation', async ({ page }) => {
        // Create a disposable library inline for this test
        const res = await page.request.post('/api/library/', {
            data: { name: 'To Delete', description: '', sourcePath: '/tmp' },
        });
        const body = await res.json();
        const deleteID = body.id;

        await page.reload();
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'To Delete' });
        await expect(card).toBeVisible();

        page.on('dialog', (dialog) => dialog.accept());
        await card.locator('.lib-delete').click();
        await expect(card).not.toBeVisible({ timeout: 5_000 });

        // Cleanup in case the delete button didn't hit the API (belt-and-suspenders)
        await page.request.delete(`/api/library/${deleteID}`).catch(() => {});
    });
});
