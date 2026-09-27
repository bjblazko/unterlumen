import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// One way to say that something is happening, and a status line at the foot
// of the sidebar for work that outlives the page that started it (ADR-0036).

const LIB_NAME = 'E2E Activity Library';
const DEST_SLUG = 'e2e-activity-dest';

const delay = (ms) => new Promise(f => setTimeout(f, ms));

test.describe('Activity and the status line', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', {
            data: { name: LIB_NAME, description: '', sourcePath: 'folder-b' },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.delete(`/api/channels/${DEST_SLUG}`).catch(() => {});
    });

    test('a library scan shows in the status line and stays there on another page', async ({ page }) => {
        test.setTimeout(180_000);
        await page.goto('/#settings');
        await waitForAppReady(page);

        // Started without waiting: the scan streams until it is done.
        page.evaluate((id) => { fetch(`/api/library/${id}/reindex`, { method: 'POST' }).then(r => r.text()); }, libID);

        const job = page.locator('#status-line .status-job', { hasText: LIB_NAME });
        await expect(job).toBeVisible({ timeout: 10_000 });
        await expect(job).toHaveAttribute('href', '#libraries');

        // Moving to another place keeps it in view.
        await page.locator('#mode-browse').click();
        await expect(job).toBeVisible();

        // It ends with a sentence and no bar: yellow means time passing.
        await expect(job).toHaveAttribute('data-state', 'done', { timeout: 150_000 });
        await expect(job).toContainText('Finished.');
        await expect(job.locator('.activity-bar')).toBeHidden();
    });

    test('a quick load shows no activity at all', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await page.waitForSelector('.folder-tile.dir-item', { timeout: 5_000 });
        await expect(page.locator('.browse-content .activity:visible')).toHaveCount(0);
    });

    test('a slow load says what it is doing', async ({ page }) => {
        await page.route('**/api/browse?*', async (route) => { await delay(2_000); await route.continue(); });
        await page.goto('/#folders');
        await waitForAppReady(page);
        const activity = page.locator('.browse-loading .activity');
        await expect(activity).toBeVisible({ timeout: 2_000 });
        await expect(activity).toContainText('Reading the folder');
        // The count is not known while the folder is read.
        await expect(page.locator('.status-bar')).toHaveText('');
    });

    test('filter results a newer filter replaces are marked as stale', async ({ page }) => {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await page.locator('#lib-filter-btn').click();
        await page.waitForSelector('.lib-filter-select', { timeout: 20_000 });
        await page.locator('.lib-filter-select').first().selectOption(String(libID));

        const results = page.locator('#lib-results-pane');
        await page.locator('.lib-filter-panel select').nth(1).selectOption({ index: 1 });
        await expect(results.locator('[data-type="image"]').first()).toBeVisible({ timeout: 15_000 });

        await page.route('**/api/library/search*', async (route) => { await delay(2_500); await route.continue(); });
        await page.locator('.lib-filter-panel select').nth(1).selectOption({ index: 0 });
        await expect(results).toHaveClass(/is-stale/);
        await expect(page.locator('.lib-filter-status')).toHaveText('Searching…', { timeout: 2_000 });
        await expect(results).not.toHaveClass(/is-stale/, { timeout: 10_000 });
    });

    test('a failed search keeps the old results faded and says why', async ({ page }) => {
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await page.locator('#lib-filter-btn').click();
        await page.waitForSelector('.lib-filter-select', { timeout: 20_000 });
        await page.locator('.lib-filter-select').first().selectOption(String(libID));
        await page.locator('.lib-filter-panel select').nth(1).selectOption({ index: 1 });
        const results = page.locator('#lib-results-pane');
        await expect(results.locator('[data-type="image"]').first()).toBeVisible({ timeout: 15_000 });

        await page.route('**/api/library/search*', (route) => route.fulfill({ status: 500, body: 'database is locked' }));
        await page.locator('.lib-filter-panel select').nth(1).selectOption({ index: 0 });
        await expect(page.locator('.lib-filter-status-error')).toContainText('database is locked');
        await expect(results).toHaveClass(/is-stale/);
    });

    test('saving a destination twice in a row sends one request', async ({ page, request }) => {
        await request.delete(`/api/channels/${DEST_SLUG}`).catch(() => {});
        let posts = 0;
        await page.route('**/api/channels/', async (route) => {
            if (route.request().method() === 'POST') {
                posts++;
                await delay(1_000);
            }
            await route.continue();
        });
        await page.goto('/#destinations');
        await waitForAppReady(page);
        await page.locator('#dest-new').click();
        await page.locator('#chf-name').fill('E2E Activity Dest');

        const save = page.locator('#dest-save');
        await save.click();
        await expect(save).toBeDisabled();
        await expect(save).toHaveText('Saving…');
        await save.click({ force: true });

        await expect(page.locator('.dest-row', { hasText: 'E2E Activity Dest' })).toBeVisible({ timeout: 8_000 });
        expect(posts).toBe(1);
    });
});
