import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

test.describe('Library list view', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        // Clean up stale libraries from interrupted previous runs
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(
            existing
                .filter(l => l.name === 'E2E Library UI' || l.name === 'To Delete')
                .map(l => request.delete(`/api/library/${l.id}`))
        );

        const res = await request.post('/api/library/', {
            data: { name: 'E2E Library UI', description: '', sourcePath: 'folder-a' },
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
        await expect(page.locator('#mode-library')).toHaveAttribute('aria-current', 'page');
    });

    test('shows library card with correct name and source path', async ({ page }) => {
        const card = page.locator('.library-card', { hasText: 'E2E Library UI' });
        await expect(card).toBeVisible({ timeout: 8_000 });
        await expect(card.locator('.library-card-name')).toContainText('E2E Library UI');
        await expect(card.locator('.library-card-meta')).toContainText('folder-a');
    });

    test('the row carries the photo count and when it was last indexed', async ({ page }) => {
        const card = page.locator('.library-card', { hasText: 'E2E Library UI' });
        await expect(card.locator('.library-card-count')).toContainText('photo');
        await expect(card.locator('.library-card-indexed')).toContainText('Indexed');
    });

    // The overview has the same filter as a library, starting with every
    // library in scope: opening it sets nothing, so the list stays until a
    // criterion is set, and Done closes it from its own head.
    test('the filter opens beside the list of libraries and closes itself', async ({ page }) => {
        const panel = page.locator('#lib-filter-panel');
        await expect(panel).not.toHaveClass(/visible/);

        await page.locator('#lib-filter-btn').click();
        await expect(panel).toHaveClass(/visible/);
        await expect(page.locator('.lib-filter-title')).toHaveText('Filter');
        await expect(page.locator('.lib-filter-select').first()).toHaveValue('');
        await expect(page.locator('#lib-list-body')).toBeVisible();
        await expect(page.locator('#lib-results-pane')).toBeHidden();

        await page.locator('.lib-filter-close').click();
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

    // The filter is a panel: a button in the head opens the column beside
    // the photos, Done closes it again.
    test('the filter opens from the head and gives the room back on Done', async ({ page }) => {
        const card = page.locator('.library-card', { hasText: 'E2E Library UI' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });
        await expect(page.locator('#lib-filter-panel')).not.toHaveClass(/visible/);

        await page.locator('#lib-filter-btn').click();
        await expect(page.locator('#lib-filter-panel')).toHaveClass(/visible/);
        await expect(page.locator('.lib-filter-title')).toHaveText('Filter');

        await page.locator('.lib-filter-close').click();
        await expect(page.locator('#lib-filter-panel')).not.toHaveClass(/visible/);
    });

    // Deleting a library moved out of its row and into the library itself
    // (phase 9): never a Delete button sitting next to Edit, and no
    // confirm() box — the question is asked in the dialog.
    test('deleting a library happens in Edit library, in two steps', async ({ page }) => {
        const res = await page.request.post('/api/library/', {
            data: { name: 'To Delete', description: '', sourcePath: 'folder-a' },
        });
        const deleteID = (await res.json()).id;

        await page.reload();
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'To Delete' });
        await expect(card).toBeVisible({ timeout: 8_000 });
        // The row itself opens the library — no accent button per row.
        await expect(card.locator('.btn-accent')).toHaveCount(0);
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });

        await page.locator('#lib-edit-btn').click();
        await page.locator('#lib-delete-start').click();
        await expect(page.locator('.gal-danger-question')).toContainText('To Delete');
        await page.locator('#lib-delete-confirm').click();

        await expect(page.locator('.library-card', { hasText: 'To Delete' })).toHaveCount(0, { timeout: 8_000 });

        await page.request.delete(`/api/library/${deleteID}`).catch(() => {});
    });
});
