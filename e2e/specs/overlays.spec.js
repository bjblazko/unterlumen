import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';

test.describe('Overlays and EXIF metadata', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await waitForThumbnailsLoaded(page, 1);
  });

  test('GPS JPEG gets GPS overlay badge', async ({ page }) => {
    // The meta overlay poll fires 300ms after load and retries until ready.
    const gpsBadge = page.locator('[data-name="gps-jpeg.jpg"] .overlay-badge-gps');
    await expect(gpsBadge).toBeVisible({ timeout: 10_000 });
  });

  test('non-GPS JPEG does not get GPS overlay badge', async ({ page }) => {
    // First wait for GPS badge on the GPS image to confirm polling has run
    await expect(
      page.locator('[data-name="gps-jpeg.jpg"] .overlay-badge-gps'),
    ).toBeVisible({ timeout: 10_000 });
    // Now the non-GPS image should have no GPS badge
    const noGpsBadge = page.locator('[data-name="no-gps-jpeg.jpg"] .overlay-badge-gps');
    await expect(noGpsBadge).not.toBeVisible();
  });

  test('JPEG files show JPEG file-type badge', async ({ page }) => {
    // File-type badges are rendered synchronously on first load
    const badge = page.locator('[data-name="gps-jpeg.jpg"] .overlay-badge').first();
    await expect(badge).toContainText('JPEG', { timeout: 5_000 });
  });

  test('HEIC file shows HEIF file-type badge', async ({ page }) => {
    const badge = page.locator('[data-name="heic-sample.heic"] .overlay-badge').first();
    await expect(badge).toContainText('HEIF', { timeout: 5_000 });
  });

  test('info panel shows Location section for GPS JPEG', async ({ page }) => {
    await page.locator('[data-name="gps-jpeg.jpg"]').click();
    await page.keyboard.press('i');
    await expect(page.locator('.info-panel.expanded, .info-panel[data-expanded="true"]')).toBeVisible({ timeout: 5_000 });
    // Wait for info to load (panel body no longer empty)
    await page.waitForFunction(
      () => document.querySelector('.info-panel') &&
            !document.querySelector('.info-panel').textContent.includes('Loading'),
      { timeout: 10_000 },
    );
    // Should have a location section
    const panelText = await page.locator('.info-panel').textContent();
    expect(panelText).toMatch(/location|latitude|lat/i);
  });

  test('info panel has no Location section for non-GPS JPEG', async ({ page }) => {
    await page.locator('[data-name="no-gps-jpeg.jpg"]').click();
    await page.keyboard.press('i');
    await expect(page.locator('.info-panel.expanded, .info-panel[data-expanded="true"]')).toBeVisible({ timeout: 5_000 });
    await page.waitForFunction(
      () => document.querySelector('.info-panel') &&
            !document.querySelector('.info-panel').textContent.includes('Loading'),
      { timeout: 10_000 },
    );
    const panelText = await page.locator('.info-panel').textContent();
    // Should show file info but no GPS coordinates
    expect(panelText).not.toMatch(/\d+\.\d+.*°/);
  });
});
