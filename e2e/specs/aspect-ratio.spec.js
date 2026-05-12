import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { navigateToFolder } from '../helpers/fixtures.js';

// src/examples/folder-b contains images from cameras spanning 2004–2026 with
// varying aspect ratios: 4:3 (compact cameras), 3:2 (DSLRs, X-T50), 4:3/16:9 (iPhones).

test.describe('Aspect ratio rendering', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
  });

  // ── Justified view ────────────────────────────────────────────────────────

  test('justified view renders rows without horizontal overflow', async ({ page }) => {
    const overflowing = await page.locator('.justified').evaluate((el) => {
      return el.scrollWidth > el.clientWidth + 2;
    });
    expect(overflowing).toBe(false);
  });

  test('images in justified view have distinct widths (aspect-ratio-aware layout)', async ({ page }) => {
    const widths = await page.locator('.justified [data-type="image"]').evaluateAll((items) =>
      items.slice(0, 10).map((el) => el.getBoundingClientRect().width),
    );
    // If all widths were identical, layout is ignoring aspect ratios
    const uniqueWidths = new Set(widths.map((w) => Math.round(w)));
    expect(uniqueWidths.size).toBeGreaterThan(1);
  });

  // ── Grid view ─────────────────────────────────────────────────────────────

  test('grid view renders all image cells with positive height', async ({ page }) => {
    await page.locator('.view-menu-btn').click();
    await page.locator('button[data-view="grid"]').click();
    await expect(page.locator('.grid-item.image-item').first()).toBeVisible({ timeout: 5_000 });

    const heights = await page.locator('.grid-item.image-item').evaluateAll((items) =>
      items.slice(0, 10).map((el) => Math.round(el.getBoundingClientRect().height)),
    );
    // All cells must have positive height (grid is rendering them correctly).
    // Row heights vary by aspect ratio — this is by design, not a defect.
    expect(heights.every((h) => h > 0)).toBe(true);
  });

  test('grid view has no horizontal overflow', async ({ page }) => {
    await page.locator('.view-menu-btn').click();
    await page.locator('button[data-view="grid"]').click();
    await expect(page.locator('.grid-item.image-item').first()).toBeVisible({ timeout: 5_000 });

    const overflowing = await page.locator('.grid').evaluate((el) => {
      return el.scrollWidth > el.clientWidth + 2;
    });
    expect(overflowing).toBe(false);
  });

  // ── List view ─────────────────────────────────────────────────────────────

  test('list view renders all rows with consistent height', async ({ page }) => {
    await page.locator('.view-menu-btn').click();
    await page.locator('button[data-view="list"]').click();
    await expect(page.locator('table.list-view')).toBeVisible({ timeout: 5_000 });

    const rows = page.locator('table.list-view tbody tr');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);

    const heights = await rows.evaluateAll((trs) =>
      trs.slice(0, 10).map((tr) => Math.round(tr.getBoundingClientRect().height)),
    );
    const uniqueHeights = new Set(heights);
    expect(uniqueHeights.size).toBe(1);
  });
});
