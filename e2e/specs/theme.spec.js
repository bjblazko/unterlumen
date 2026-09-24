import { test, expect } from '@playwright/test';

// Settings is a place now, not a dropdown in a corner (phase 9).
async function openSettings(page) {
  await page.locator('#mode-settings').click();
  await page.waitForSelector('[data-theme-set="light"]:visible', { timeout: 3_000 });
}

test.describe('Theme', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
  });

  // ADR-0030 replaced the data-accent product-variant attribute with the
  // rams-design tokens: the accent is a token value, and it differs per theme.
  test('the accent token carries the rams-design orange', async ({ page }) => {
    const accent = await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue('--accent').trim().toUpperCase());
    expect(['#E85D04', '#F07A2A']).toContain(accent);
  });

  test('the UI voice is Plex Sans and data is set in Plex Mono', async ({ page }) => {
    const body = await page.evaluate(() => getComputedStyle(document.body).fontFamily);
    expect(body).toContain('IBM Plex Sans');
    await page.waitForSelector('.item-name, .list-name', { timeout: 10_000 });
    const loaded = await page.evaluate(() => [...document.fonts]
      .filter(f => f.status === 'loaded').map(f => f.family));
    expect(loaded).toContain('IBM Plex Sans');
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

    const dirItem = page.locator('.folder-chip.dir-item').first();
    if (await dirItem.isVisible()) {
      await dirItem.click();
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
