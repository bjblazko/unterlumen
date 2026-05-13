import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { FOLDER_B_IMAGE_COUNT, FOLDER_A_A1_IMAGE_COUNT, navigateToFolder } from '../helpers/fixtures.js';

test.describe('Folder navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
  });

  // ── Root level ───────────────────────────────────────────────────────────

  test('root shows exactly two directory entries and no images', async ({ page }) => {
    const dirs = page.locator('.grid-item.dir-item');
    await expect(dirs).toHaveCount(2, { timeout: 5_000 });
    await expect(dirs.filter({ hasText: 'folder-a' })).toHaveCount(1);
    await expect(dirs.filter({ hasText: 'folder-b' })).toHaveCount(1);
    await expect(page.locator('[data-type="image"]')).toHaveCount(0);
  });

  test('status bar at root reflects zero images', async ({ page }) => {
    await page.waitForSelector('.grid-item.dir-item', { timeout: 5_000 });
    const statusText = await page.locator('.status-bar').textContent();
    // Either shows "0 images" or only shows directories
    expect(statusText).toMatch(/0\s*(image|photo)|director/i);
  });

  // ── folder-b (flat, images only) ─────────────────────────────────────────

  test('folder-b contains images only, no subdirectories', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    await expect(page.locator('.grid-item.dir-item')).toHaveCount(0);
    const imageCount = await page.locator('[data-type="image"]').count();
    expect(imageCount).toBe(FOLDER_B_IMAGE_COUNT);
  });

  test('breadcrumb shows folder-b path after navigation', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await expect(page.locator('.crumb[data-path="folder-b"]')).toBeVisible();
    await expect(page.locator('.crumb[data-path=""]')).toBeVisible();
  });

  test('breadcrumb root click returns to root from folder-b', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await page.locator('.crumb[data-path=""]').click();
    await expect(page.locator('.crumb[data-path="folder-b"]')).not.toBeVisible({ timeout: 3_000 });
    await expect(page.locator('.grid-item.dir-item[data-name="folder-a"]')).toBeVisible();
  });

  test('status bar inside folder-b shows 50 images', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const statusText = await page.locator('.status-bar').textContent();
    expect(statusText).toMatch(new RegExp(`${FOLDER_B_IMAGE_COUNT}\\s*(image|photo)`, 'i'));
  });

  // ── folder-a (directory-only at top level) ───────────────────────────────

  test('folder-a shows three subdirs plus the sample image', async ({ page }) => {
    await navigateToFolder(page, 'folder-a');
    await expect(page.locator('.grid-item.dir-item')).toHaveCount(3, { timeout: 5_000 }); // a1, a2, a3
    await expect(page.locator('.grid-item.dir-item[data-name="a1"]')).toBeVisible();
    await expect(page.locator('.grid-item.dir-item[data-name="a2"]')).toBeVisible();
    await expect(page.locator('.grid-item.dir-item[data-name="a3"]')).toBeVisible();
    await expect(page.locator('[data-type="image"]')).toHaveCount(1); // folder-a-sample.jpeg
  });

  // ── folder-a (mixed: subdirs + image at same level) ──────────────────────

  test('folder-a shows both subdirectories and a sample image (mixed layout)', async ({ page }) => {
    await navigateToFolder(page, 'folder-a');
    // folder-a-sample.jpeg was added by setup.sh to test mixed dir+image rendering
    await expect(page.locator('[data-name="folder-a-sample.jpeg"]')).toBeVisible({ timeout: 5_000 });
    await expect(page.locator('.grid-item.dir-item[data-name="a1"]')).toBeVisible();
  });

  // ── Three-level deep navigation ──────────────────────────────────────────

  test('navigate root → folder-a → a1 → back to folder-a → back to root', async ({ page }) => {
    // Level 1: root → folder-a
    await navigateToFolder(page, 'folder-a');
    await expect(page.locator('.crumb[data-path="folder-a"]')).toBeVisible();

    // Level 2: folder-a → a1
    const a1 = page.locator('.grid-item.dir-item[data-name="a1"]');
    await a1.dblclick();
    await page.waitForSelector('.crumb[data-path="folder-a/a1"]', { timeout: 5_000 });
    await waitForThumbnailsLoaded(page, 1);
    const a1Images = await page.locator('[data-type="image"]').count();
    expect(a1Images).toBe(FOLDER_A_A1_IMAGE_COUNT);

    // Back to folder-a via breadcrumb
    await page.locator('.crumb[data-path="folder-a"]').click();
    await expect(page.locator('.crumb[data-path="folder-a/a1"]')).not.toBeVisible({ timeout: 3_000 });
    await expect(page.locator('.grid-item.dir-item[data-name="a1"]')).toBeVisible();

    // Back to root via breadcrumb
    await page.locator('.crumb[data-path=""]').click();
    await expect(page.locator('.crumb[data-path="folder-a"]')).not.toBeVisible({ timeout: 3_000 });
    await expect(page.locator('.grid-item.dir-item[data-name="folder-a"]')).toBeVisible();
    await expect(page.locator('.grid-item.dir-item[data-name="folder-b"]')).toBeVisible();
  });

  test('folder-a/a1 has correct image count', async ({ page }) => {
    await navigateToFolder(page, 'folder-a');
    const a1 = page.locator('.grid-item.dir-item[data-name="a1"]');
    await a1.dblclick();
    await page.waitForSelector('.crumb[data-path="folder-a/a1"]', { timeout: 5_000 });
    await waitForThumbnailsLoaded(page, 1);
    await expect(page.locator('[data-type="image"]')).toHaveCount(FOLDER_A_A1_IMAGE_COUNT);
    await expect(page.locator('.grid-item.dir-item')).toHaveCount(0);
  });

  test('breadcrumb path reflects each navigation level', async ({ page }) => {
    await navigateToFolder(page, 'folder-a');
    await page.locator('.grid-item.dir-item[data-name="a1"]').dblclick();
    await page.waitForSelector('.crumb[data-path="folder-a/a1"]', { timeout: 5_000 });

    // All three crumbs should be visible: root, folder-a, folder-a/a1
    await expect(page.locator('.crumb[data-path=""]')).toBeVisible();
    await expect(page.locator('.crumb[data-path="folder-a"]')).toBeVisible();
    await expect(page.locator('.crumb[data-path="folder-a/a1"]')).toBeVisible();
  });
});
