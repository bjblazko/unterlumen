import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// The info panel is open from the start on a desk; closed, it stays closed
// everywhere for the session. A phone opens it with its Info button.

test.describe('Info panel default', () => {
  test('starts open on a desk', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    await expect(page.locator('.info-panel.expanded:visible')).toHaveCount(1);
  });

  test('closed once, it stays closed across places and a reload', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    await page.locator('.info-panel.expanded:visible .info-collapse-btn').click();
    await expect(page.locator('.info-panel.expanded:visible')).toHaveCount(0);

    await page.locator('[data-type="dir"][data-name="folder-b"]').dblclick();
    await page.locator('[data-type="image"]').first().dblclick();
    await expect(page.locator('.viewer')).toBeVisible();
    await expect(page.locator('.viewer .info-panel.collapsed')).toBeVisible();

    await page.reload();
    await waitForAppReady(page);
    await expect(page.locator('.info-panel.collapsed:visible')).toHaveCount(1);
    await expect(page.locator('.info-panel.expanded:visible')).toHaveCount(0);
  });

  test('starts closed on a phone', async ({ browser }) => {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
    const page = await context.newPage();
    await page.goto('/#folders');
    await page.locator('[data-type="dir"]').first().waitFor();
    await expect(page.locator('.info-panel.expanded:visible')).toHaveCount(0);
    await context.close();
  });
});
