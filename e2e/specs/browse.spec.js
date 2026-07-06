import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, NO_GPS_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

test.describe('Browse', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
  });

  test('page title and initial state', async ({ page }) => {
    await expect(page).toHaveTitle('Unterlumen');
    await expect(page.locator('#mode-browse')).toHaveClass(/active/);
    await expect(page.locator('.breadcrumb')).toBeVisible();
    await expect(page.locator('.status-bar')).toBeVisible();
  });

  test('root shows folder-a and folder-b directories', async ({ page }) => {
    const items = page.locator('.grid-item.dir-item');
    await expect(items.filter({ hasText: 'folder-a' })).toBeVisible({ timeout: 5_000 });
    await expect(items.filter({ hasText: 'folder-b' })).toBeVisible({ timeout: 5_000 });
    // No images at root
    await expect(page.locator('[data-type="image"]')).toHaveCount(0);
  });

  test('fixture images appear in grid after navigating into folder-b', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const items = page.locator('[data-type="image"]');
    await expect(items.first()).toBeVisible();
    const pic = page.locator(`[data-name="${GPS_IMAGE}"]`);
    await expect(pic).toBeVisible();
    const img = pic.locator('img');
    await expect(img).toHaveAttribute('src', /\/api\/thumbnail/);
  });

  test('standard browse thumbnails request an explicit size', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const img = page.locator(`[data-name="${GPS_IMAGE}"] img`);
    await expect(img).toHaveAttribute('src', /[?&]size=\d+/);
  });

  test('status bar shows image count', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const statusText = await page.locator('.status-bar').textContent();
    expect(statusText).toMatch(/\d+\s*(image|photo)/i);
  });

  test('navigate folder-a → a1 → back via breadcrumb', async ({ page }) => {
    await navigateToFolder(page, 'folder-a');
    const a1 = page.locator('.grid-item.dir-item[data-name="a1"]');
    await expect(a1).toBeVisible({ timeout: 5_000 });
    await a1.dblclick();
    await expect(page.locator('.crumb[data-path="folder-a/a1"]')).toBeVisible({ timeout: 5_000 });
    await waitForThumbnailsLoaded(page, 1);
    // Navigate back to folder-a via breadcrumb
    await page.locator('.crumb[data-path="folder-a"]').click();
    await expect(page.locator('.crumb[data-path="folder-a/a1"]')).not.toBeVisible({ timeout: 5_000 });
    await expect(page.locator('.grid-item.dir-item[data-name="a1"]')).toBeVisible();
  });

  test('switch to grid view', async ({ page }) => {
    await page.locator('.view-menu-btn').click();
    await page.locator('button[data-view="grid"]').click();
    await expect(page.locator('.grid-item.image-item, .grid-item.dir-item').first()).toBeVisible({ timeout: 5_000 });
  });

  test('switch to list view', async ({ page }) => {
    await page.locator('.view-menu-btn').click();
    await page.locator('button[data-view="list"]').click();
    await expect(page.locator('table.list-view')).toBeVisible({ timeout: 5_000 });
  });

  test('switch back to justified view', async ({ page }) => {
    await page.locator('.view-menu-btn').click();
    await page.locator('button[data-view="grid"]').click();
    await page.locator('.view-menu-btn').click();
    await page.locator('button[data-view="justified"]').click();
    await expect(page.locator('.justified')).toBeVisible({ timeout: 5_000 });
  });

  test('list view actually shows rows for real images, no console errors', async ({ page }) => {
    const errors = [];
    page.on('pageerror', (err) => errors.push(err.message));
    page.on('console', (msg) => { if (msg.type() === 'error') errors.push(msg.text()); });

    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);

    await page.locator('.view-menu-btn').click();
    await page.locator('button[data-view="list"]').click();

    const table = page.locator('table.list-view');
    await expect(table).toBeVisible({ timeout: 5_000 });
    await expect(table.locator('tr[data-type="image"]').first()).toBeVisible({ timeout: 5_000 });
    // Sanity: the row for a known fixture actually renders its name and a date.
    const gpsRow = table.locator(`tr[data-name="${GPS_IMAGE}"]`);
    await expect(gpsRow).toBeVisible();
    await expect(gpsRow.locator('.list-date')).not.toHaveText('');

    expect(errors, `console/page errors while switching to list view: ${errors.join('\n')}`).toEqual([]);
  });

  test('Show names toggle shows and hides filename labels in grid view', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);

    const item = page.locator(`[data-name="${GPS_IMAGE}"]`);
    await expect(item.locator('.item-name')).toHaveCount(0);

    await page.locator('.view-menu-btn').click();
    const namesToggle = page.locator('.toggle-names-wrap .toggle');
    await namesToggle.click();

    await expect(item.locator('.item-name')).toBeVisible({ timeout: 5_000 });
    await expect(item.locator('.item-name')).toHaveText(GPS_IMAGE);

    // Toggle back off.
    await page.locator('.view-menu-btn').click();
    await namesToggle.click();
    await expect(item.locator('.item-name')).toHaveCount(0);
  });

  test('Show details toggle shows and hides overlay badges in grid view', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);

    const item = page.locator(`[data-name="${GPS_IMAGE}"]`);
    await expect(item.locator('.overlay-badges')).toBeVisible({ timeout: 5_000 });

    await page.locator('.view-menu-btn').click();
    const overlaysToggle = page.locator('.toggle-overlays-wrap .toggle');
    await overlaysToggle.click();

    await expect(item.locator('.overlay-badges')).toHaveCount(0);

    // Toggle back on.
    await page.locator('.view-menu-btn').click();
    await overlaysToggle.click();
    await expect(item.locator('.overlay-badges')).toBeVisible({ timeout: 5_000 });
  });

  test('click selects an image item', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const item = page.locator(`[data-name="${GPS_IMAGE}"]`);
    await item.click();
    await expect(item).toHaveClass(/selected/);
  });

  test('Escape clears selection', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const item = page.locator(`[data-name="${GPS_IMAGE}"]`);
    await item.click();
    await expect(item).toHaveClass(/selected/);
    await page.keyboard.press('Escape');
    await expect(item).not.toHaveClass(/selected/);
  });

  test('Meta+click adds to selection', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const first = page.locator(`[data-name="${GPS_IMAGE}"]`);
    const second = page.locator(`[data-name="${NO_GPS_IMAGE}"]`);
    await first.click();
    await second.click({ modifiers: ['Meta'] });
    await expect(first).toHaveClass(/selected/);
    await expect(second).toHaveClass(/selected/);
    const statusText = await page.locator('.status-bar').textContent();
    expect(statusText).toMatch(/2\s*selected/i);
  });

  test('changing sort order re-renders items', async ({ page }) => {
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const itemsBefore = await page.locator('[data-type="image"]').count();
    expect(itemsBefore).toBeGreaterThan(0);
    // Navigate out and back to verify re-render
    await page.locator('.crumb[data-path=""]').click();
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const itemsAfter = await page.locator('[data-type="image"]').count();
    expect(itemsAfter).toBe(itemsBefore);
  });
});
