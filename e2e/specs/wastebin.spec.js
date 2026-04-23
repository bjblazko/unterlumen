import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';

test.describe('Wastebin (non-destructive)', () => {
  test.beforeEach(async ({ page }) => {
    // Fresh page load ensures clean wastebin state
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await waitForThumbnailsLoaded(page, 1);
  });

  test('wastebin count is initially hidden', async ({ page }) => {
    const badge = page.locator('#wastebin-count');
    // Either hidden via display:none or empty text
    const isHidden = await badge.evaluate(
      (el) => el.style.display === 'none' || el.textContent.trim() === '',
    );
    expect(isHidden).toBe(true);
  });

  test('marking an image for deletion updates the badge', async ({ page }) => {
    await page.locator('[data-name="gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');
    await expect(page.locator('#wastebin-count')).toHaveText('1', { timeout: 3_000 });
    await expect(page.locator('[data-name="gps-jpeg.jpg"]')).toHaveClass(/marked-for-deletion/);
  });

  test('marking a second image increments the badge', async ({ page }) => {
    await page.locator('[data-name="gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');
    await page.locator('[data-name="no-gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');
    await expect(page.locator('#wastebin-count')).toHaveText('2', { timeout: 3_000 });
  });

  test('wastebin mode shows marked images', async ({ page }) => {
    await page.locator('[data-name="gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');
    await page.locator('[data-name="no-gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();

    // Should show the "N files marked" header
    await expect(page.locator('.wastebin-header')).toContainText('2 files', { timeout: 3_000 });

    // Both images should be visible in wastebin grid
    const items = page.locator('.grid-item.image-item');
    await expect(items).toHaveCount(2);

    // Action buttons exist but are disabled with no selection
    await expect(page.locator('#wb-restore')).toBeDisabled();
    await expect(page.locator('#wb-delete')).toBeDisabled();
  });

  test('selecting a wastebin item enables Restore button', async ({ page }) => {
    await page.locator('[data-name="gps-jpeg.jpg"]').click();
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
    await page.locator('[data-name="gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');
    await page.locator('[data-name="no-gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();
    await expect(page.locator('.wastebin-header')).toContainText('2 files', { timeout: 3_000 });

    // Select first item and restore it
    await page.locator('.grid-item.image-item').first().click();
    await page.locator('#wb-restore').click();

    // Wastebin should now show 1 item
    await expect(page.locator('.wastebin-header')).toContainText('1 file', { timeout: 3_000 });
    await expect(page.locator('#wastebin-count')).toHaveText('1');
  });

  test('restoring all items shows empty wastebin state', async ({ page }) => {
    await page.locator('[data-name="gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();
    await expect(page.locator('.wastebin-header')).toBeVisible({ timeout: 3_000 });

    await page.locator('.grid-item.image-item').first().click();
    await page.locator('#wb-restore').click();

    // Wastebin should show empty state
    await expect(page.locator('.wastebin-empty')).toBeVisible({ timeout: 3_000 });
    // Badge should be hidden
    const badge = page.locator('#wastebin-count');
    const isHidden = await badge.evaluate(
      (el) => el.style.display === 'none' || el.textContent.trim() === '',
    );
    expect(isHidden).toBe(true);
  });

  test('restored image loses marked-for-deletion class in browse after reload', async ({ page }) => {
    await page.locator('[data-name="gps-jpeg.jpg"]').click();
    await page.keyboard.press('Delete');

    await page.locator('#mode-wastebin').click();
    await page.locator('.grid-item.image-item').first().click();
    await page.locator('#wb-restore').click();

    // Switch back to browse and navigate to subdir + back to force pane re-render
    // (browse pane doesn't auto-update marked-for-deletion class on restore)
    await page.locator('#mode-browse').click();
    await page.locator('.grid-item.dir-item[data-name="subdir"]').dblclick();
    await expect(page.locator('.crumb[data-path="subdir"]')).toBeVisible({ timeout: 3_000 });
    await page.locator('.crumb[data-path=""]').click();
    await waitForThumbnailsLoaded(page, 1);

    const item = page.locator('[data-name="gps-jpeg.jpg"]');
    await expect(item).not.toHaveClass(/marked-for-deletion/);
  });

  // Safety: the Delete button is intentionally never clicked in these tests.
});
