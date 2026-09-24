import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { GPS_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

// Every dialog is built by the same component (ADR-0033), so this spec checks
// the behaviour once per representative dialog instead of once per feature.

async function selectAPhoto(page) {
    await page.goto('/');
    await waitForAppReady(page);
    await navigateToFolder(page, 'folder-b');
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await expect(page.locator('.selection-bar')).toBeVisible();
}

test.describe('Dialogs — one frame, one behaviour', () => {
    test('a dialog has a title, one accent action and no closing cross', async ({ page }) => {
        await selectAPhoto(page);
        await page.locator('.selection-bar [data-action="export"]').click();

        const dialog = page.locator('.dialog');
        await expect(dialog).toBeVisible({ timeout: 5_000 });
        await expect(dialog.locator('.dialog-title')).toHaveText('Export');
        await expect(dialog.locator('.dialog-foot .btn-accent')).toHaveCount(1);
        // Cancel closes it; there is no second control doing the same.
        await expect(dialog.locator('.modal-close, .modal-close-btn')).toHaveCount(0);
        await expect(dialog).toHaveAttribute('role', 'dialog');
        await expect(dialog).toHaveAttribute('aria-modal', 'true');
    });

    test('Escape closes it and the focus goes back to the button that opened it', async ({ page }) => {
        await selectAPhoto(page);
        const opener = page.locator('.selection-bar [data-action="rename"]');
        await opener.click();
        await expect(page.locator('.dialog')).toBeVisible({ timeout: 5_000 });

        // The focus is inside the dialog while it is open.
        const insideBefore = await page.evaluate(() => !!document.activeElement?.closest('.dialog'));
        expect(insideBefore).toBe(true);

        await page.keyboard.press('Escape');
        await expect(page.locator('.dialog')).toHaveCount(0);
        await expect(opener).toBeFocused();
    });

    test('a click on the scrim cancels', async ({ page }) => {
        await selectAPhoto(page);
        await page.locator('.selection-bar [data-action="location"]').click();
        await expect(page.locator('.dialog')).toBeVisible({ timeout: 5_000 });

        await page.locator('.dialog-scrim').click({ position: { x: 5, y: 5 } });
        await expect(page.locator('.dialog')).toHaveCount(0);
    });

    test('the global shortcuts stand back while a dialog is open', async ({ page }) => {
        await selectAPhoto(page);
        await page.locator('.selection-bar [data-action="export"]').click();
        await expect(page.locator('.dialog')).toBeVisible({ timeout: 5_000 });

        // "2" would go to Marked for deletion if the dialog did not own the keys.
        await page.keyboard.press('2');
        await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.dialog')).toBeVisible();
    });

    test('dialogs stack: the folder picker sits on top and Escape closes only it', async ({ page }) => {
        await selectAPhoto(page);
        await page.locator('.selection-bar [data-action="export"]').click();
        await expect(page.locator('.dialog')).toBeVisible({ timeout: 5_000 });

        await page.locator('.export-destination-btn').click();
        await expect(page.locator('.dialog-scrim')).toHaveCount(2);

        await page.keyboard.press('Escape');
        await expect(page.locator('.dialog-scrim')).toHaveCount(1);
        await expect(page.locator('.dialog-title')).toHaveText('Export');
    });

    test('Remove location asks before it removes', async ({ page }) => {
        await selectAPhoto(page);
        await page.locator('.selection-bar [data-action="location"]').click();
        await expect(page.locator('.dialog')).toBeVisible({ timeout: 5_000 });

        // Used to throw: the button called a method that did not exist.
        await page.locator('#loc-remove').click();
        await expect(page.locator('.location-confirm-msg')).toContainText('Remove the location');
        await expect(page.locator('.dialog-foot .btn-danger')).toContainText('Remove from 1 photo');
    });
});

test.describe('Charts take their colours from the tokens (ADR-0034)', () => {
    test('the chart ramp is defined and the dead --border token is gone', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);

        const tokens = await page.evaluate(() => {
            const read = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();
            return {
                cats: Array.from({ length: 8 }, (_, i) => read(`--chart-${i + 1}`)),
                seq: Array.from({ length: 5 }, (_, i) => read(`--chart-seq-${i + 1}`)),
                grid: read('--chart-grid'),
                film: read('--film-velvia'),
                border: read('--border'),
            };
        });
        expect(tokens.cats.every(c => /^#[0-9a-f]{6}$/i.test(c))).toBe(true);
        expect(new Set(tokens.cats).size).toBe(8);
        expect(tokens.seq.every(c => /^#[0-9a-f]{6}$/i.test(c))).toBe(true);
        expect(tokens.grid).not.toBe('');
        expect(tokens.film).not.toBe('');
        expect(tokens.border).toBe(''); // it was renamed in ADR-0030 and is not used any more
    });
});
