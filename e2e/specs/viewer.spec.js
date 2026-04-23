import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';

test.describe('Image Viewer', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await waitForThumbnailsLoaded(page, 1);
  });

  async function openViewer(page, imageName) {
    const item = page.locator(`[data-name="${imageName}"]`);
    await item.dblclick();
    await expect(page.locator('.viewer')).toBeVisible({ timeout: 5_000 });
  }

  test('double-click opens viewer with correct filename', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    await expect(page.locator('.viewer-filename')).toContainText('gps-jpeg.jpg');
  });

  test('viewer shows image from /api/image', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    const img = page.locator('.viewer-image-container img');
    await expect(img).toHaveAttribute('src', /\/api\/image/);
  });

  test('viewer counter shows 1 / N', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    const counter = page.locator('.viewer-counter');
    await expect(counter).toContainText('1 /');
  });

  test('close viewer with back button', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    await page.locator('.viewer-back').click();
    await expect(page.locator('.viewer')).not.toBeVisible({ timeout: 5_000 });
    await expect(page.locator('[data-type="image"]').first()).toBeVisible();
  });

  test('close viewer with Escape key', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    await page.keyboard.press('Escape');
    await expect(page.locator('.viewer')).not.toBeVisible({ timeout: 5_000 });
  });

  test('navigate to next image with button', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    const counterBefore = await page.locator('.viewer-counter').textContent();
    const nextBtn = page.locator('.viewer-next');
    await expect(nextBtn).not.toBeDisabled();
    await nextBtn.click();
    const counterAfter = await page.locator('.viewer-counter').textContent();
    expect(counterAfter).not.toBe(counterBefore);
    expect(counterAfter).toMatch(/^2 \//);
  });

  test('navigate with ArrowRight and ArrowLeft keys', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    await page.keyboard.press('ArrowRight');
    await expect(page.locator('.viewer-counter')).toContainText('2 /');
    await page.keyboard.press('ArrowLeft');
    await expect(page.locator('.viewer-counter')).toContainText('1 /');
  });

  test('prev button is disabled on first image', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    // Ensure we're on the first image
    await expect(page.locator('.viewer-counter')).toContainText('1 /');
    await expect(page.locator('.viewer-prev')).toBeDisabled();
  });

  test('mark for deletion from viewer updates wastebin count', async ({ page }) => {
    await openViewer(page, 'gps-jpeg.jpg');
    await page.locator('.viewer-delete').click();
    // Wastebin badge should now show 1
    await expect(page.locator('#wastebin-count')).toHaveText('1', { timeout: 3_000 });
    await page.locator('.viewer-back').click();
    // Browse pane is still showing; item will have marked-for-deletion after next render
    // (the pane doesn't re-render on viewer close — verified via wastebin count above)
  });
});
