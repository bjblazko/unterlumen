import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, navigatePaneToFolder } from '../helpers/fixtures.js';

test.describe('Commander (Organize mode)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await page.locator('#mode-commander').click();
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

  test('both panes show fixture images after navigating into folder-b', async ({ page }) => {
    await navigatePaneToFolder(page, '#left-pane', 'folder-b', 'folder-b');
    await navigatePaneToFolder(page, '#right-pane', 'folder-b', 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    await expect(page.locator(`#left-pane [data-name="${GPS_IMAGE}"]`)).toBeVisible({ timeout: 5_000 });
    await expect(page.locator(`#right-pane [data-name="${GPS_IMAGE}"]`)).toBeVisible({ timeout: 5_000 });
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
    await page.waitForFunction(
      () => document.querySelectorAll('#left-pane [data-index]').length >= 1,
      { timeout: 10_000 },
    );
    await page.keyboard.press('ArrowRight');
    await expect(page.locator('#left-pane [data-index].focused')).toBeVisible({ timeout: 3_000 });
  });

  test('Enter on a directory navigates into it', async ({ page }) => {
    // Root shows folder-a and folder-b; click folder-a and press Enter
    const folderA = page.locator('#left-pane [data-name="folder-a"]');
    await expect(folderA).toBeVisible({ timeout: 5_000 });
    await folderA.click();
    await page.keyboard.press('Enter');
    await expect(page.locator('#left-pane .crumb[data-path="folder-a"]')).toBeVisible({ timeout: 3_000 });
  });

  test('breadcrumb click navigates back to root in left pane', async ({ page }) => {
    await navigatePaneToFolder(page, '#left-pane', 'folder-b', 'folder-b');
    await page.locator('#left-pane .crumb[data-path=""]').click();
    await expect(page.locator('#left-pane [data-name="folder-a"]')).toBeVisible({ timeout: 3_000 });
  });

  test('clicking an item selects it in left pane', async ({ page }) => {
    await navigatePaneToFolder(page, '#left-pane', 'folder-b', 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    const item = page.locator(`#left-pane [data-name="${GPS_IMAGE}"]`);
    await item.click();
    await expect(item).toHaveClass(/selected/);
  });

  test('Space toggles selection on focused image in left pane', async ({ page }) => {
    await navigatePaneToFolder(page, '#left-pane', 'folder-b', 'folder-b');
    await page.waitForFunction(
      () => document.querySelectorAll('#left-pane [data-type="image"]').length >= 1,
      { timeout: 10_000 },
    );
    // focusedIndex starts at 0 (first image in folder-b) after navigation
    await page.keyboard.press('ArrowRight');
    const focused = page.locator('#left-pane [data-index].focused');
    await expect(focused).toBeVisible({ timeout: 3_000 });
    await expect(focused).not.toHaveClass(/selected/);
    await page.keyboard.press('Space');
    await expect(focused).toHaveClass(/selected/);
  });

  test('Ctrl+A selects all images in left pane', async ({ page }) => {
    await navigatePaneToFolder(page, '#left-pane', 'folder-b', 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    await page.locator(`#left-pane [data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('Control+a');
    const selectedItems = page.locator('#left-pane [data-type="image"].selected');
    await expect(selectedItems.first()).toBeVisible({ timeout: 3_000 });
    const count = await selectedItems.count();
    expect(count).toBeGreaterThan(0);
  });

  // --- Action button states ---

  test('copy and move buttons are enabled when a focused item exists', async ({ page }) => {
    // focusedIndex=0 (folder-a) is set on root load — getActionableFiles() is non-empty immediately
    await expect(page.locator('#cmd-copy')).toBeEnabled({ timeout: 3_000 });
    await expect(page.locator('#cmd-move')).toBeEnabled({ timeout: 3_000 });
  });

  test('copy and move buttons remain enabled after clicking a file', async ({ page }) => {
    await navigatePaneToFolder(page, '#left-pane', 'folder-b', 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    await page.locator(`#left-pane [data-name="${GPS_IMAGE}"]`).click();
    await expect(page.locator('#cmd-copy')).toBeEnabled({ timeout: 3_000 });
    await expect(page.locator('#cmd-move')).toBeEnabled({ timeout: 3_000 });
  });

  // --- Double-click opens viewer ---

  test('double-click image in left pane opens the viewer', async ({ page }) => {
    await navigatePaneToFolder(page, '#left-pane', 'folder-b', 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
    await page.locator(`#left-pane [data-name="${GPS_IMAGE}"]`).dblclick();
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
