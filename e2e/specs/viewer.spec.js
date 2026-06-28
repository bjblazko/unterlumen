import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

test.describe('Image Viewer', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
  });

  async function openViewerOnFirst(page) {
    await page.locator('[data-type="image"]').first().dblclick();
    await expect(page.locator('.viewer')).toBeVisible({ timeout: 5_000 });
  }

  async function openViewer(page, imageName) {
    const item = page.locator(`[data-name="${imageName}"]`);
    await item.dblclick();
    await expect(page.locator('.viewer')).toBeVisible({ timeout: 5_000 });
  }

  test('double-click opens viewer with correct filename', async ({ page }) => {
    await openViewer(page, GPS_IMAGE);
    await expect(page.locator('.viewer-filename')).toContainText(GPS_IMAGE);
  });

  test('viewer shows image from /api/image', async ({ page }) => {
    await openViewer(page, GPS_IMAGE);
    const img = page.locator('.viewer-image-container img');
    await expect(img).toHaveAttribute('src', /\/api\/image/);
  });

  test('viewer counter shows position / total format', async ({ page }) => {
    await openViewer(page, GPS_IMAGE);
    const counter = page.locator('.viewer-counter');
    await expect(counter).toContainText(/\d+ \/ \d+/);
  });

  test('close viewer with back button', async ({ page }) => {
    await openViewer(page, GPS_IMAGE);
    await page.locator('.viewer-back').click();
    await expect(page.locator('.viewer')).not.toBeVisible({ timeout: 5_000 });
    await expect(page.locator('[data-type="image"]').first()).toBeVisible();
  });

  test('close viewer with Escape key', async ({ page }) => {
    await openViewer(page, GPS_IMAGE);
    await page.keyboard.press('Escape');
    await expect(page.locator('.viewer')).not.toBeVisible({ timeout: 5_000 });
  });

  test('navigate to next image with button', async ({ page }) => {
    await openViewer(page, GPS_IMAGE);
    const counterBefore = await page.locator('.viewer-counter').textContent();
    const nextBtn = page.locator('.viewer-next');
    await expect(nextBtn).not.toBeDisabled();
    await nextBtn.click();
    const counterAfter = await page.locator('.viewer-counter').textContent();
    expect(counterAfter).not.toBe(counterBefore);
  });

  test('navigate with ArrowRight and ArrowLeft keys', async ({ page }) => {
    // Open the first image so ArrowLeft is available after ArrowRight
    await openViewerOnFirst(page);
    const counterBefore = await page.locator('.viewer-counter').textContent();
    await page.keyboard.press('ArrowRight');
    const counterAfterRight = await page.locator('.viewer-counter').textContent();
    expect(counterAfterRight).not.toBe(counterBefore);
    await page.keyboard.press('ArrowLeft');
    await expect(page.locator('.viewer-counter')).toContainText(counterBefore.trim());
  });

  test('prev button is disabled on first image', async ({ page }) => {
    await openViewerOnFirst(page);
    await expect(page.locator('.viewer-counter')).toContainText('1 /');
    await expect(page.locator('.viewer-prev')).toBeDisabled();
  });

  test('mark for deletion from viewer updates wastebin count', async ({ page }) => {
    await openViewer(page, GPS_IMAGE);
    await page.locator('.viewer-delete').click();
    await expect(page.locator('#wastebin-count')).toHaveText('1', { timeout: 3_000 });
    await page.locator('.viewer-back').click();
  });

  test('trash icon appears on thumbnail after closing viewer', async ({ page }) => {
    await openViewer(page, GPS_IMAGE);
    await page.locator('.viewer-delete').click();
    await page.locator('.viewer-back').click();
    await expect(page.locator('.viewer')).not.toBeVisible({ timeout: 5_000 });
    await expect(page.locator(`[data-name="${GPS_IMAGE}"]`)).toHaveClass(/marked-for-deletion/, { timeout: 3_000 });
  });
});
