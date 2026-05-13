import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, NO_GPS_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

test.describe('Wastebin (non-destructive)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
  });

  test('wastebin count is initially hidden', async ({ page }) => {
    const badge = page.locator('#wastebin-count');
    const isHidden = await badge.evaluate(
      (el) => el.style.display === 'none' || el.textContent.trim() === '',
    );
    expect(isHidden).toBe(true);
  });

  test('marking an image for deletion updates the badge', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');
    await expect(page.locator('#wastebin-count')).toHaveText('1', { timeout: 3_000 });
    await expect(page.locator(`[data-name="${GPS_IMAGE}"]`)).toHaveClass(/marked-for-deletion/);
  });

  test('marking a second image increments the badge', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');
    await page.locator(`[data-name="${NO_GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');
    await expect(page.locator('#wastebin-count')).toHaveText('2', { timeout: 3_000 });
  });

  test('wastebin mode shows marked images', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');
    await page.locator(`[data-name="${NO_GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();

    await expect(page.locator('.wastebin-header')).toContainText('2 files', { timeout: 3_000 });

    const items = page.locator('.grid-item.image-item');
    await expect(items).toHaveCount(2);

    await expect(page.locator('#wb-restore')).toBeDisabled();
    await expect(page.locator('#wb-delete')).toBeDisabled();
  });

  test('selecting a wastebin item enables Restore button', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();
    await expect(page.locator('.wastebin-header')).toBeVisible({ timeout: 3_000 });

    const item = page.locator('.grid-item.image-item').first();
    await item.click();
    await expect(item).toHaveClass(/selected/);
    await expect(page.locator('#wb-restore')).toBeEnabled();
    await expect(page.locator('#wb-restore')).toContainText('Restore (1)');
  });

  test('restore removes item from wastebin and decrements badge', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');
    await page.locator(`[data-name="${NO_GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();
    await expect(page.locator('.wastebin-header')).toContainText('2 files', { timeout: 3_000 });

    await page.locator('.grid-item.image-item').first().click();
    await page.locator('#wb-restore').click();

    await expect(page.locator('.wastebin-header')).toContainText('1 file', { timeout: 3_000 });
    await expect(page.locator('#wastebin-count')).toHaveText('1');
  });

  test('restoring all items shows empty wastebin state', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();
    await expect(page.locator('.wastebin-header')).toBeVisible({ timeout: 3_000 });

    await page.locator('.grid-item.image-item').first().click();
    await page.locator('#wb-restore').click();

    await expect(page.locator('.wastebin-empty')).toBeVisible({ timeout: 3_000 });
    const badge = page.locator('#wastebin-count');
    const isHidden = await badge.evaluate(
      (el) => el.style.display === 'none' || el.textContent.trim() === '',
    );
    expect(isHidden).toBe(true);
  });

  test('restored image loses marked-for-deletion class in browse after re-entering folder', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();
    await page.locator('.grid-item.image-item').first().click();
    await page.locator('#wb-restore').click();

    // Switch back to browse, navigate out and back into folder-b to force pane re-render
    await page.locator('#mode-browse').click();
    await page.locator('.crumb[data-path=""]').click();
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);

    const item = page.locator(`[data-name="${GPS_IMAGE}"]`);
    await expect(item).not.toHaveClass(/marked-for-deletion/);
  });

  // Safety: the Delete button is intentionally never clicked in these tests.
});
