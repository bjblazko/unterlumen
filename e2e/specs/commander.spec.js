import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';

test.describe('Commander (Organize mode)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await page.locator('#mode-commander').click();
    // Wait for both panes to load
    await page.waitForSelector('#left-pane', { timeout: 5_000 });
    await page.waitForSelector('#right-pane', { timeout: 5_000 });
  });

  // --- Layout ---

  test('keyboard shortcut 3 activates commander mode', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await page.keyboard.press('3');
    await expect(page.locator('#left-pane')).toBeVisible({ timeout: 3_000 });
    await expect(page.locator('#right-pane')).toBeVisible({ timeout: 3_000 });
  });

  test('commander has left pane, right pane, and action buttons', async ({ page }) => {
    await expect(page.locator('#left-pane')).toBeVisible();
    await expect(page.locator('#right-pane')).toBeVisible();
    await expect(page.locator('#cmd-copy')).toBeVisible();
    await expect(page.locator('#cmd-move')).toBeVisible();
    await expect(page.locator('#cmd-delete')).toBeVisible();
  });

  test('left pane defaults to grid view', async ({ page }) => {
    await expect(page.locator('#left-pane .grid')).toBeVisible({ timeout: 5_000 });
  });

  test('right pane defaults to list view', async ({ page }) => {
    await expect(page.locator('#right-pane table.list-view')).toBeVisible({ timeout: 5_000 });
  });

  test('left pane is initially active', async ({ page }) => {
    await expect(page.locator('#left-pane')).toHaveClass(/active/);
    await expect(page.locator('#right-pane')).not.toHaveClass(/active/);
  });

  test('both panes have breadcrumb navigation', async ({ page }) => {
    await expect(page.locator('#left-pane .breadcrumb')).toBeVisible({ timeout: 5_000 });
    await expect(page.locator('#right-pane .breadcrumb')).toBeVisible({ timeout: 5_000 });
  });

  test('both panes show the fixture images', async ({ page }) => {
    await waitForThumbnailsLoaded(page, 1);
    await expect(page.locator('#left-pane [data-name="gps-jpeg.jpg"]')).toBeVisible({ timeout: 5_000 });
    await expect(page.locator('#right-pane [data-name="gps-jpeg.jpg"]')).toBeVisible({ timeout: 5_000 });
  });

  // --- Pane focus ---

  test('Tab key switches focus to the right pane', async ({ page }) => {
    await page.keyboard.press('Tab');
    await expect(page.locator('#right-pane')).toHaveClass(/active/, { timeout: 3_000 });
    await expect(page.locator('#left-pane')).not.toHaveClass(/active/);
  });

  test('Tab again switches focus back to left pane', async ({ page }) => {
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await expect(page.locator('#left-pane')).toHaveClass(/active/, { timeout: 3_000 });
  });

  test('clicking right pane activates it', async ({ page }) => {
    await page.locator('#right-pane').click({ position: { x: 10, y: 10 } });
    await expect(page.locator('#right-pane')).toHaveClass(/active/, { timeout: 3_000 });
    await expect(page.locator('#left-pane')).not.toHaveClass(/active/);
  });

  // --- Keyboard navigation (non-destructive) ---

  test('ArrowRight moves focus in left pane', async ({ page }) => {
    // Wait for left pane specifically — waitForThumbnailsLoaded may count right-pane or hidden browse-pane items
    await page.waitForFunction(
      () => document.querySelectorAll('#left-pane [data-index]').length >= 1,
      { timeout: 10_000 },
    );
    await page.keyboard.press('ArrowRight');
    // Some item in left pane should gain .focused class
    await expect(page.locator('#left-pane [data-index].focused')).toBeVisible({ timeout: 3_000 });
  });

  test('Enter on a directory navigates into it', async ({ page }) => {
    await waitForThumbnailsLoaded(page, 1);
    // Focus the subdir item and press Enter
    const subdir = page.locator('#left-pane [data-name="subdir"]');
    await subdir.click();
    await page.keyboard.press('Enter');
    await expect(page.locator('#left-pane .crumb[data-path="subdir"]')).toBeVisible({ timeout: 3_000 });
  });

  test('breadcrumb click navigates back to root in left pane', async ({ page }) => {
    await waitForThumbnailsLoaded(page, 1);
    // Navigate into subdir first
    await page.locator('#left-pane [data-name="subdir"]').dblclick();
    await expect(page.locator('#left-pane .crumb[data-path="subdir"]')).toBeVisible({ timeout: 3_000 });
    // Click root crumb
    await page.locator('#left-pane .crumb[data-path=""]').click();
    await expect(page.locator('#left-pane [data-name="gps-jpeg.jpg"]')).toBeVisible({ timeout: 3_000 });
  });

  test('clicking an item selects it in left pane', async ({ page }) => {
    await waitForThumbnailsLoaded(page, 1);
    const item = page.locator('#left-pane [data-name="gps-jpeg.jpg"]');
    await item.click();
    await expect(item).toHaveClass(/selected/);
  });

  test('Space toggles selection on focused image in left pane', async ({ page }) => {
    // Wait for left pane specifically — waitForThumbnailsLoaded may count right-pane items
    await page.waitForFunction(
      () => document.querySelectorAll('#left-pane [data-type="image"]').length >= 1,
      { timeout: 10_000 },
    );
    // focusedIndex starts at 0 (subdir) after load(). One ArrowRight moves to index 1 = gps-jpeg.jpg.
    await page.keyboard.press('ArrowRight');
    const item = page.locator('#left-pane [data-name="gps-jpeg.jpg"]');
    await expect(item).toHaveClass(/focused/, { timeout: 3_000 });
    await expect(item).not.toHaveClass(/selected/);
    await page.keyboard.press('Space');
    await expect(item).toHaveClass(/selected/);
  });

  test('Ctrl+A selects all images in left pane', async ({ page }) => {
    await waitForThumbnailsLoaded(page, 1);
    // Click an image first to ensure keyboard events land in the app (not the mode button)
    await page.locator('#left-pane [data-name="gps-jpeg.jpg"]').click();
    // devices['Desktop Chrome'] uses Win32 platform so isMac=false; use Control+a
    await page.keyboard.press('Control+a');
    const selectedItems = page.locator('#left-pane [data-type="image"].selected');
    await expect(selectedItems.first()).toBeVisible({ timeout: 3_000 });
    const count = await selectedItems.count();
    expect(count).toBeGreaterThan(0);
  });

  // --- Action button states ---

  test('copy and move buttons are enabled when a focused item exists', async ({ page }) => {
    // focusedIndex=0 (subdir) is set on load, so getActionableFiles() is non-empty immediately
    await expect(page.locator('#cmd-copy')).toBeEnabled({ timeout: 3_000 });
    await expect(page.locator('#cmd-move')).toBeEnabled({ timeout: 3_000 });
  });

  test('copy and move buttons remain enabled after clicking a file', async ({ page }) => {
    await waitForThumbnailsLoaded(page, 1);
    await page.locator('#left-pane [data-name="gps-jpeg.jpg"]').click();
    await expect(page.locator('#cmd-copy')).toBeEnabled({ timeout: 3_000 });
    await expect(page.locator('#cmd-move')).toBeEnabled({ timeout: 3_000 });
  });

  // --- Double-click opens viewer ---

  test('double-click image in left pane opens the viewer', async ({ page }) => {
    await waitForThumbnailsLoaded(page, 1);
    await page.locator('#left-pane [data-name="gps-jpeg.jpg"]').dblclick();
    await expect(page.locator('.viewer')).toBeVisible({ timeout: 5_000 });
    await page.keyboard.press('Escape');
    await expect(page.locator('.viewer')).not.toBeVisible({ timeout: 3_000 });
  });

  // --- Resizer ---

  test('commander resizer is present between the panes', async ({ page }) => {
    await expect(page.locator('#cmd-resizer')).toBeVisible();
  });

  // --- Return to browse ---

  test('keyboard shortcut 1 returns to browse mode', async ({ page }) => {
    await page.keyboard.press('1');
    await expect(page.locator('#mode-browse')).toHaveClass(/active/, { timeout: 3_000 });
    await expect(page.locator('#left-pane')).not.toBeVisible();
  });
});
