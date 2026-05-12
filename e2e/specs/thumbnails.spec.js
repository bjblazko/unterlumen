import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { HIF_PATH, FOLDER_B_IMAGE_COUNT, navigateToFolder } from '../helpers/fixtures.js';

test.describe('Thumbnails', () => {
  // ── folder-b: JPEG thumbnails ────────────────────────────────────────────

  test.describe('folder-b JPEG thumbnails', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/');
      await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
      await navigateToFolder(page, 'folder-b');
      await waitForThumbnailsLoaded(page, 1);
    });

    test('all image items have a thumbnail src pointing to /api/thumbnail', async ({ page }) => {
      const imgs = page.locator('[data-type="image"] img');
      const count = await imgs.count();
      expect(count).toBeGreaterThan(0);
      for (let i = 0; i < count; i++) {
        const src = await imgs.nth(i).getAttribute('src');
        expect(src).toMatch(/\/api\/thumbnail/);
      }
    });

    test('all thumbnail URLs include an explicit size parameter', async ({ page }) => {
      const imgs = page.locator('[data-type="image"] img');
      const count = await imgs.count();
      expect(count).toBeGreaterThan(0);
      for (let i = 0; i < count; i++) {
        const src = await imgs.nth(i).getAttribute('src');
        expect(src).toMatch(/[?&]size=\d+/);
      }
    });

    test('folder-b shows all 50 image items', async ({ page }) => {
      await expect(page.locator('[data-type="image"]')).toHaveCount(FOLDER_B_IMAGE_COUNT);
    });

    test('justified view renders items without horizontal overflow', async ({ page }) => {
      const overflowing = await page.locator('.justified').evaluate((el) => {
        return el.scrollWidth > el.clientWidth + 2; // 2px tolerance for subpixel rounding
      });
      expect(overflowing).toBe(false);
    });

    test('grid view renders thumbnail cells with consistent structure', async ({ page }) => {
      await page.locator('.view-menu-btn').click();
      await page.locator('button[data-view="grid"]').click();
      await expect(page.locator('.grid-item.image-item').first()).toBeVisible({ timeout: 5_000 });
      const items = page.locator('.grid-item.image-item');
      const count = await items.count();
      expect(count).toBe(FOLDER_B_IMAGE_COUNT);
    });
  });

  // ── folder-a/a1: HIF thumbnail (requires ffmpeg) ─────────────────────────

  test.describe('HIF thumbnail conversion', () => {
    test('HIF thumbnail is served as image/jpeg', async ({ request }) => {
      const toolRes = await request.get('/api/tools/check');
      const tools = await toolRes.json();
      if (!tools.ffmpeg) {
        test.skip(true, 'ffmpeg not available');
        return;
      }
      const res = await request.get(`/api/thumbnail?path=${encodeURIComponent(HIF_PATH)}`);
      expect(res.status()).toBe(200);
      expect(res.headers()['content-type']).toContain('image/jpeg');
      const buf = await res.body();
      expect(buf.length).toBeGreaterThan(0);
    });

    test('HIF thumbnail appears in the grid after navigating to folder-a/a1', async ({ page }) => {
      const toolRes = await page.request.get('/api/tools/check');
      const tools = await toolRes.json();
      if (!tools.ffmpeg) {
        test.skip(true, 'ffmpeg not available');
        return;
      }
      await page.goto('/');
      await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
      await navigateToFolder(page, 'folder-a');
      await page.locator('.grid-item.dir-item[data-name="a1"]').dblclick();
      await page.waitForSelector('.crumb[data-path="folder-a/a1"]', { timeout: 5_000 });
      await waitForThumbnailsLoaded(page, 1);

      const hifItem = page.locator('[data-name="2026-04-24_X-T50-XF23mmF2-R-WR-DSCF6850.hif"]');
      await expect(hifItem).toBeVisible({ timeout: 5_000 });
      const img = hifItem.locator('img');
      await expect(img).toHaveAttribute('src', /\/api\/thumbnail/);
    });
  });
});
