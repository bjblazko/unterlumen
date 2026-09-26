import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, NO_GPS_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

// The export dialog estimates the output size: quickly by default, or by
// encoding each file when asked.

const SIZE = /^~?\d+(\.\d+)?\s?(B|KB|MB|GB)$/;

async function openExportForTwo(page) {
  await page.goto('/');
  await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
  await navigateToFolder(page, 'folder-b');
  await waitForThumbnailsLoaded(page, 2);
  await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
  await page.locator(`[data-name="${NO_GPS_IMAGE}"]`).click({ modifiers: ['Meta'] });
  await page.locator('.selection-bar [data-action="export"]').click();
  await expect(page.locator('.export-dialog')).toBeVisible({ timeout: 5_000 });
}

test.describe('Export estimates', () => {
  test('the quick estimate fills the totals and every row', async ({ page }) => {
    await openExportForTwo(page);
    await expect(page.locator('.export-total-in')).toHaveText(SIZE, { timeout: 10_000 });
    await expect(page.locator('.export-total-out')).toHaveText(SIZE);
    await expect(page.locator('.export-file-row .export-file-out')).toHaveText([SIZE, SIZE]);
  });

  test('the exact estimate measures each file and ends with totals', async ({ page }) => {
    await openExportForTwo(page);
    await expect(page.locator('.export-total-out')).toHaveText(SIZE, { timeout: 10_000 });
    await page.locator('[data-estimate="encode"]').click();
    await expect(page.locator('[data-estimate="encode"]')).toHaveClass(/active/);
    await expect(page.locator('.export-file-row .export-file-out')).toHaveText([SIZE, SIZE], { timeout: 20_000 });
    await expect(page.locator('.export-total-in')).toHaveText(SIZE);
    await expect(page.locator('.export-total-out')).toHaveText(SIZE);
  });
});
