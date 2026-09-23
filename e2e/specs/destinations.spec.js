import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// Destinations are a place of their own now, and their form asks for the type
// first and then shows only what that type needs (ADR-0029).

const SLUG = 'e2e-destinations-share';

test.describe('Destinations', () => {
    test.afterAll(async ({ request }) => {
        await request.delete(`/api/channels/${SLUG}`).catch(() => {});
    });

    test('creating one: type first, slug derived from the name', async ({ page, request }) => {
        await request.delete(`/api/channels/${SLUG}`).catch(() => {});

        await page.goto('/#destinations');
        await waitForAppReady(page);
        await page.locator('#dest-new').click();

        const form = page.locator('#dest-form');
        await expect(form).toBeVisible();
        // Share links is the default and the first choice offered.
        await expect(form).toHaveAttribute('data-type', 'share');

        await form.locator('#chf-name').fill('E2E Destinations Share');
        // The slug is derived and shown, never typed — changing one would move
        // published output and break links already shared.
        await expect(form.locator('#chf-slug-preview')).toHaveText('e2e-destinations-share');
        await expect(form.locator('#chf-slug')).toHaveCount(1);
        await expect(form.locator('input[type="text"]#chf-slug')).toHaveCount(0);

        await page.locator('#dest-save').click();

        const row = page.locator('.dest-row', { hasText: 'E2E Destinations Share' });
        await expect(row).toBeVisible({ timeout: 8_000 });
        await expect(row).toContainText('Share links');

        const created = await (await request.get('/api/channels/')).json();
        const ch = created.find(c => c.slug === SLUG);
        expect(ch).toBeTruthy();
        expect(ch.galleryExport).toBe(true);
        expect(ch.siteExport ?? false).toBe(false);
    });

    test('the Files type hides the upload and website sections', async ({ page }) => {
        await page.goto('/#destinations');
        await waitForAppReady(page);
        await page.locator('#dest-new').click();

        const form = page.locator('#dest-form');
        await form.locator('input[name="dest-type"][value="files"]').check();
        await expect(form).toHaveAttribute('data-type', 'files');
        await expect(form.locator('.dest-when-online').first()).toBeHidden();
        await expect(form.locator('.dest-when-site')).toBeHidden();
        // A files destination is exactly the one that needs an output folder.
        await expect(form.locator('.dest-when-files')).toBeVisible();
    });

    test('accounts and handler key/values sit under Advanced', async ({ page }) => {
        await page.goto('/#destinations');
        await waitForAppReady(page);
        await page.locator('#dest-new').click();

        // An empty accounts editor has no height of its own, so the button
        // beside it is what tells collapsed from open.
        const advanced = page.locator('.dest-advanced');
        await expect(advanced.locator('#chf-account-add')).toBeHidden();
        await advanced.locator('summary').click();
        await expect(advanced.locator('#chf-account-add')).toBeVisible();
        await expect(advanced.locator('#chf-handler')).toBeVisible();
    });

    // A folder picked with the browser is relative to the browse root; stored
    // that way it would mean a different directory depending on where the
    // server was started from.
    test('a chosen output folder is stored as an absolute path', async ({ page, request }) => {
        const slug = 'e2e-destinations-files';
        await request.delete(`/api/channels/${slug}`).catch(() => {});

        await page.goto('/#destinations');
        await waitForAppReady(page);
        await page.locator('#dest-new').click();
        const form = page.locator('#dest-form');
        await form.locator('input[name="dest-type"][value="files"]').check();
        await form.locator('#chf-name').fill('E2E Destinations Files');
        await form.locator('#chf-output-path').fill('folder-a/exports');
        await page.locator('#dest-save').click();
        await expect(page.locator('.dest-row', { hasText: 'E2E Destinations Files' })).toBeVisible({ timeout: 8_000 });

        const channels = await (await request.get('/api/channels/')).json();
        const ch = channels.find(c => c.slug === slug);
        expect(ch.outputPath.startsWith('/')).toBe(true);
        expect(ch.outputPath.endsWith('folder-a/exports')).toBe(true);
        expect(ch.outputDir).toBe(ch.outputPath);

        await request.delete(`/api/channels/${slug}`).catch(() => {});
    });

    test('is a place with an address of its own', async ({ page }) => {
        await page.goto('/#destinations');
        await waitForAppReady(page);
        await expect(page.locator('#mode-destinations')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.dest-table')).toBeVisible({ timeout: 8_000 });

        // The old entry point — a modal reachable only from an opened library
        // — is gone.
        await page.locator('#mode-library').click();
        await expect(page.locator('#lib-channels-btn')).toHaveCount(0);
    });
});
