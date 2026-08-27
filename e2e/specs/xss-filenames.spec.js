import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_PATH, navigateToFolder } from '../helpers/fixtures.js';

// Regression test: browse-grid.js, browse-list.js, and browse-justified.js
// used to interpolate raw filenames into HTML (data-name, alt, item-name)
// with no escaping, unlike comparable templates elsewhere that already used
// the existing escapeHtml() helper. A filename containing `">` or a tag
// broke out of the attribute / injected markup into the DOM — a stored XSS
// reachable from any untrusted source of files (SD card import, shared
// drive, zip extraction), even though the app itself has no login (ADR-0006).
//
// Uses a dedicated throwaway subfolder (created + torn down here) so the
// dangerous filename never touches shared fixtures used by other specs.

const XSS_FOLDER = 'e2e-xss-test';
const DANGEROUS_NAME = '"><img src=x onerror="window.__xssFired=true">.jpg';

test.describe('Filename XSS safety', () => {
    test.beforeAll(async ({ request }) => {
        // Fresh folder each run: clean up any leftovers from an interrupted prior run.
        await request.post('/api/delete', { data: { files: [XSS_FOLDER] } }).catch(() => {});

        const mkdirRes = await request.post('/api/mkdir', { data: { path: XSS_FOLDER } });
        expect(mkdirRes.status()).toBe(200);

        const copyRes = await request.post('/api/copy', {
            data: { files: [GPS_PATH], destination: XSS_FOLDER },
        });
        expect(copyRes.status()).toBe(200);
        const copyBody = await copyRes.json();
        expect(copyBody.results[0].success).toBe(true);

        const copiedName = GPS_PATH.split('/').pop();
        const renameRes = await request.post('/api/rename', {
            data: { path: `${XSS_FOLDER}/${copiedName}`, name: DANGEROUS_NAME },
        });
        expect(renameRes.status()).toBe(200);
    });

    test.afterAll(async ({ request }) => {
        await request.post('/api/delete', { data: { files: [XSS_FOLDER] } });
    });

    test('grid view renders a dangerous filename as text, not markup', async ({ page }) => {
        page.on('dialog', (d) => {
            throw new Error(`Unexpected dialog (would indicate script execution): ${d.message()}`);
        });

        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, XSS_FOLDER);
        await waitForThumbnailsLoaded(page, 1);

        const item = page.locator('.image-item[data-type="image"]').first();
        await expect(item).toBeVisible();

        // The onerror handler must never have run — the string must render as
        // an attribute-safe, inert filename, not be parsed into a live <img>.
        const xssFired = await page.evaluate(() => window.__xssFired === true);
        expect(xssFired).toBe(false);

        // Names aren't shown as visible text in grid/justified view by default
        // (the "Show names" toggle is off), so assert against the DOM directly:
        // the escaped payload must round-trip as a plain attribute value/text
        // node, and the item must contain exactly the one legitimate <img>
        // (the thumbnail) — no second <img> injected by the payload.
        expect(await item.getAttribute('data-name')).toBe(DANGEROUS_NAME);
        const thumbImg = item.locator('img');
        await expect(thumbImg).toHaveCount(1);
        expect(await thumbImg.getAttribute('alt')).toBe(DANGEROUS_NAME);
    });

    test('list view renders a dangerous filename as text, not markup', async ({ page }) => {
        page.on('dialog', (d) => {
            throw new Error(`Unexpected dialog (would indicate script execution): ${d.message()}`);
        });

        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, XSS_FOLDER);
        await page.locator('.view-menu-btn').click();
        await page.locator('button[data-view="list"]').click();
        await waitForThumbnailsLoaded(page, 1);

        const nameCell = page.locator('tr.image-row .list-name').first();
        await expect(nameCell).toBeVisible();

        const xssFired = await page.evaluate(() => window.__xssFired === true);
        expect(xssFired).toBe(false);
        expect(await nameCell.locator('img').count()).toBe(0);
        await expect(nameCell).toContainText(DANGEROUS_NAME);
    });
});
