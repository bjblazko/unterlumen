import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder, FOLDER_B_IMAGE_COUNT } from '../helpers/fixtures.js';

// The global shortcuts of app-keyboard.js that no other spec presses.
// ControlOrMeta follows the host: the app reads navigator.platform, which is
// MacIntel in headless Chrome on a Mac and so expects Cmd there.

async function openFolderB(page) {
  await page.goto('/');
  await waitForAppReady(page);
  await navigateToFolder(page, 'folder-b');
  await waitForThumbnailsLoaded(page, 1);
  // Shortcuts need focus on the page, not on a button.
  await page.locator('[data-type="image"]').first().click();
}

test.describe('Keyboard shortcuts', () => {
  test('number keys and comma switch places', async ({ page }) => {
    await page.goto('/');
    await waitForAppReady(page);
    await page.locator('.breadcrumb').click();
    const places = [['3', 'organize'], ['4', 'library'], ['5', 'published'], ['6', 'destinations'], [',', 'settings'], ['1', 'browse']];
    for (const [key, place] of places) {
      await page.keyboard.press(key);
      await expect(page.locator(`#mode-${place}`)).toHaveAttribute('aria-current', 'page');
    }
  });

  test('Cmd/Ctrl+A selects every photo, Escape clears the selection', async ({ page }) => {
    await openFolderB(page);
    await page.keyboard.press('ControlOrMeta+a');
    await expect(page.locator('[data-type="image"].selected')).toHaveCount(FOLDER_B_IMAGE_COUNT);
    await page.keyboard.press('Escape');
    await expect(page.locator('[data-type="image"].selected')).toHaveCount(0);
    // With nothing selected, Escape goes up one folder.
    await expect(page.locator('.crumb[data-path="folder-b"]')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('.folder-chip[data-name="folder-b"]')).toBeVisible({ timeout: 5_000 });
  });

  test('Cmd/Ctrl+D marks the selection for deletion', async ({ page }) => {
    await openFolderB(page);
    const first = page.locator('[data-type="image"]').first();
    await expect(first).toHaveClass(/selected/);
    await page.keyboard.press('ControlOrMeta+d');
    await expect(first).toHaveClass(/marked-for-deletion/);
    await expect(page.locator('[data-type="image"].selected')).toHaveCount(0);
  });

  test('arrow keys move the focus and Space selects the focused photo', async ({ page }) => {
    await openFolderB(page);
    await page.keyboard.press('Escape'); // clear the click's selection
    const focusedIndex = () => page.locator('[data-type="image"].focused').first().getAttribute('data-index');
    await page.keyboard.press('ArrowRight');
    const afterRight = Number(await focusedIndex());
    await page.keyboard.press('ArrowDown');
    const afterDown = Number(await focusedIndex());
    expect(afterDown).toBeGreaterThan(afterRight);
    // Rows of the justified view differ in length, so Up need not undo Down.
    await page.keyboard.press('ArrowUp');
    expect(Number(await focusedIndex())).toBeLessThan(afterDown);
    await page.keyboard.press('ArrowLeft');
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press(' ');
    await expect(page.locator('[data-type="image"].selected')).toHaveCount(1);
    await expect(page.locator('[data-type="image"].selected.focused')).toHaveCount(1);
  });

  test('Enter opens the focused photo in the viewer', async ({ page }) => {
    await openFolderB(page);
    await page.keyboard.press('Escape');
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Enter');
    await expect(page.locator('.viewer')).toBeVisible({ timeout: 5_000 });
  });

  test('H hides and shows the interface', async ({ page }) => {
    await openFolderB(page);
    await page.keyboard.press('h');
    await expect(page.locator('body')).toHaveClass(/ui-hidden/);
    await page.keyboard.press('h');
    await expect(page.locator('body')).not.toHaveClass(/ui-hidden/);
  });

  test('Backspace marks the selection for deletion', async ({ page }) => {
    await openFolderB(page);
    const first = page.locator('[data-type="image"]').first();
    await page.keyboard.press('Backspace');
    await expect(first).toHaveClass(/marked-for-deletion/);
  });
});
