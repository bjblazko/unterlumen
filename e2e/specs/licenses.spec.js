import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// About › Licenses and thanks lists what Unterlumen is built on, and the
// license texts come with the program (feature doc 2026-10-02-licenses-and-thanks).

test.describe('Licenses and thanks', () => {
  test('About leads to the place', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    await page.locator('#about-trigger').click();
    await page.locator('.about-topics a[data-mode="licenses"]').click();
    await expect(page).toHaveURL(/#licenses$/);
    await expect(page.locator('.guide-pane h1:visible')).toHaveText('Licenses and thanks');
  });

  test('every entry names its license and website, and built-in ones link their text', async ({ page }) => {
    await page.goto('/#licenses');
    const items = page.locator('.credits-item');
    await expect(items.first()).toBeVisible();
    for (const name of ['modernc.org/sqlite', 'MapLibre GL JS 6.11.2', 'IBM Plex Sans and Mono', 'FFmpeg', 'OpenStreetMap']) {
      await expect(page.locator('.credits-name', { hasText: name })).toHaveCount(1);
    }
    const count = await items.count();
    for (let i = 0; i < count; i++) {
      await expect(items.nth(i).locator('.credits-license')).not.toBeEmpty();
      await expect(items.nth(i).locator('a', { hasText: 'Website' })).toHaveCount(1);
    }
    const maplibre = page.locator('.credits-item', { hasText: 'MapLibre' });
    await expect(maplibre.locator('a', { hasText: 'Support the project' })).toHaveAttribute('href', 'https://maplibre.org/sponsors/');
  });

  test('the license texts are served from inside the binary', async ({ page, request }) => {
    await page.goto('/#licenses');
    const links = page.locator('.credits-links a', { hasText: 'License text' });
    await expect(links.first()).toBeVisible();
    for (const href of await links.evaluateAll(as => as.map(a => a.getAttribute('href')))) {
      const resp = await request.get(href);
      expect(resp.ok(), href).toBe(true);
      expect((await resp.text()).length, href).toBeGreaterThan(100);
    }
  });
});
