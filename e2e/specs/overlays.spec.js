import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, NO_GPS_IMAGE, HIF_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

test.describe('Overlays and EXIF metadata — folder-b (JPEG)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
  });

  test('GPS JPEG gets GPS overlay badge', async ({ page }) => {
    const gpsBadge = page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge-gps`);
    await expect(gpsBadge).toBeVisible({ timeout: 10_000 });
  });

  test('non-GPS JPEG does not get GPS overlay badge', async ({ page }) => {
    // Wait for GPS badge on a GPS image to confirm polling has run
    await expect(
      page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge-gps`),
    ).toBeVisible({ timeout: 10_000 });
    const noGpsBadge = page.locator(`[data-name="${NO_GPS_IMAGE}"] .overlay-badge-gps`);
    await expect(noGpsBadge).not.toBeVisible();
  });

  test('JPEG files show JPEG file-type badge', async ({ page }) => {
    const badge = page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge`, { hasText: 'JPEG' });
    await expect(badge).toBeVisible({ timeout: 5_000 });
  });

  test('info panel shows Location section for GPS JPEG', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('i');
    await expect(page.locator('.info-panel.expanded, .info-panel[data-expanded="true"]')).toBeVisible({ timeout: 5_000 });
    await page.waitForFunction(
      () => document.querySelector('.info-panel') &&
            !document.querySelector('.info-panel').textContent.includes('Loading'),
      { timeout: 10_000 },
    );
    const panelText = await page.locator('.info-panel').textContent();
    expect(panelText).toMatch(/location|latitude|lat/i);
  });

  test('info panel has no Location section for non-GPS JPEG', async ({ page }) => {
    await page.locator(`[data-name="${NO_GPS_IMAGE}"]`).click();
    await page.keyboard.press('i');
    await expect(page.locator('.info-panel.expanded, .info-panel[data-expanded="true"]')).toBeVisible({ timeout: 5_000 });
    await page.waitForFunction(
      () => document.querySelector('.info-panel') &&
            !document.querySelector('.info-panel').textContent.includes('Loading'),
      { timeout: 10_000 },
    );
    const panelText = await page.locator('.info-panel').textContent();
    expect(panelText).not.toMatch(/\d+\.\d+.*°/);
  });
});

test.describe('Overlays — folder-a/a1 (HIF)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, 'folder-a');
    const a1 = page.locator('.grid-item.dir-item[data-name="a1"]');
    await a1.dblclick();
    await page.waitForSelector('.crumb[data-path="folder-a/a1"]', { timeout: 5_000 });
    await waitForThumbnailsLoaded(page, 1);
  });

  test('HIF file shows HEIF file-type badge', async ({ page }) => {
    const badge = page.locator(`[data-name="${HIF_IMAGE}"] .overlay-badge`, { hasText: 'HEIF' });
    await expect(badge).toBeVisible({ timeout: 5_000 });
  });
});
