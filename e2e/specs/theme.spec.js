import { test, expect } from '@playwright/test';

async function openSettings(page) {
  // Close dropdown if open by clicking away, then open settings
  const settingsMenu = page.locator('#settings-menu');
  if (await settingsMenu.isVisible()) {
    await page.locator('body').click({ position: { x: 10, y: 10 } });
    await page.waitForFunction(() => {
      const m = document.getElementById('settings-menu');
      return m && m.style.display === 'none';
    }, { timeout: 2_000 }).catch(() => {});
  }
  await page.locator('#settings-btn').click();
  await page.waitForSelector('[data-theme-set="light"]:visible', { timeout: 3_000 });
}

test.describe('Theme', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
  });

  test('orange accent attribute is set on html element', async ({ page }) => {
    const accent = await page.locator('html').getAttribute('data-accent');
    expect(accent).toBe('orange');
  });

  test('theme buttons exist in settings dropdown', async ({ page }) => {
    await openSettings(page);
    await expect(page.locator('[data-theme-set="light"]')).toBeVisible();
    await expect(page.locator('[data-theme-set="auto"]')).toBeVisible();
    await expect(page.locator('[data-theme-set="dark"]')).toBeVisible();
  });

  test('selecting light theme sets data-theme on html', async ({ page }) => {
    await openSettings(page);
    await page.locator('[data-theme-set="light"]').click();
    const theme = await page.locator('html').getAttribute('data-theme');
    expect(theme).not.toBe('dark');
  });

  test('selecting dark theme sets data-theme="dark" on html', async ({ page }) => {
    await openSettings(page);
    await page.locator('[data-theme-set="dark"]').click();
    const theme = await page.locator('html').getAttribute('data-theme');
    expect(theme).toBe('dark');
  });

  test('theme toggle switches correctly between light and dark', async ({ page }) => {
    await openSettings(page);
    await page.locator('[data-theme-set="light"]').click();
    expect(await page.locator('html').getAttribute('data-theme')).not.toBe('dark');

    await openSettings(page);
    await page.locator('[data-theme-set="dark"]').click();
    expect(await page.locator('html').getAttribute('data-theme')).toBe('dark');

    await openSettings(page);
    await page.locator('[data-theme-set="light"]').click();
    expect(await page.locator('html').getAttribute('data-theme')).not.toBe('dark');
  });

  test('viewer uses dark background regardless of theme', async ({ page }) => {
    await openSettings(page);
    await page.locator('[data-theme-set="light"]').click();

    const dirItem = page.locator('.grid-item.dir-item').first();
    if (await dirItem.isVisible()) {
      await dirItem.dblclick();
      await page.waitForSelector('[data-type="image"]', { timeout: 5_000 }).catch(() => {});
    }
    const imageItem = page.locator('[data-type="image"]').first();
    if (await imageItem.isVisible()) {
      await imageItem.dblclick();
      await page.waitForSelector('.viewer', { timeout: 5_000 });
      await expect(page.locator('.viewer')).toBeVisible();
      const bg = await page.locator('.viewer').evaluate(el => getComputedStyle(el).backgroundColor);
      const [r, g, b] = bg.match(/\d+/g).map(Number);
      // Viewer must be dark (sum of rgb channels < 150 for dark gray/black)
      expect(r + g + b).toBeLessThan(150);
    }
  });
});
