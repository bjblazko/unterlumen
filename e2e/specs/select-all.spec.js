import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, NO_GPS_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

// Wherever a selection is made, Select all and Select none sit beside the
// count, without a key to know. Faces in People are covered in people*.spec.

test.describe('Select all', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 2);
  });

  test('in a folder, selects every photo and none again', async ({ page }) => {
    const select = page.locator('.browse-select-all .select-all');
    const photos = await page.locator('[data-type="image"]').count();
    await expect(select.locator('.select-all-none')).toBeHidden();
    await select.locator('.select-all-all').click();
    await expect(page.locator('[data-type="image"].selected')).toHaveCount(photos);
    await expect(select.locator('.select-all-count')).toContainText(`${photos} selected`);
    await expect(select.locator('.select-all-all')).toBeHidden();
    await expect(page.locator('.selection-bar')).toBeVisible();

    await select.locator('.select-all-none').click();
    await expect(page.locator('[data-type="image"].selected')).toHaveCount(0);
    await expect(page.locator('.selection-bar')).toHaveCount(0);
    await expect(select.locator('.select-all-all')).toBeVisible();
  });

  test('a selection made by hand shows both', async ({ page }) => {
    const select = page.locator('.browse-select-all .select-all');
    await page.locator('[data-type="image"]').first().click();
    await expect(select.locator('.select-all-all')).toBeVisible();
    await expect(select.locator('.select-all-none')).toBeVisible();
    await expect(select.locator('.select-all-count')).toContainText('1 selected');
  });

  test('in Marked for deletion, selects every file and none again', async ({ page }) => {
    for (const name of [GPS_IMAGE, NO_GPS_IMAGE]) {
      await page.locator(`[data-name="${name}"]`).click();
      await page.keyboard.press('Delete');
    }
    await page.locator('#mode-wastebin').click();
    const select = page.locator('.wastebin-select .select-all');
    await expect(page.locator('.place-header:visible .place-meta')).toHaveText('2 photos');
    await select.locator('.select-all-all').click();
    await expect(page.locator('#wb-restore')).toHaveText('Restore 2');
    await expect(select.locator('.select-all-count')).toHaveText('2 selected');
    await page.locator('.wastebin-select .select-all-none').click();
    await expect(page.locator('#wb-restore')).toHaveText('Restore all');
  });
});
