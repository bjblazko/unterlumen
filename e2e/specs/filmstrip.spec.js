import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';

test.describe('Film strip', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await waitForThumbnailsLoaded(page, 1);
    // Open viewer on first image
    await page.locator('[data-name="gps-jpeg.jpg"]').dblclick();
    await expect(page.locator('.viewer')).toBeVisible({ timeout: 5_000 });
  });

  test('film strip is hidden by default when viewer opens', async ({ page }) => {
    const strip = page.locator('.viewer-filmstrip');
    await expect(strip).toBeAttached();
    // Hidden via display:none
    const display = await strip.evaluate((el) => el.style.display);
    expect(display).toBe('none');
  });

  test('F key shows the film strip', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });
  });

  test('F key toggles the film strip off again', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });
    await page.keyboard.press('f');
    const display = await page.locator('.viewer-filmstrip').evaluate((el) => el.style.display);
    expect(display).toBe('none');
  });

  test('film strip has one thumb per image in the fixture', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });
    // We have 3 images: gps-jpeg, heic-sample, no-gps-jpeg
    const thumbs = page.locator('.filmstrip-thumb');
    await expect(thumbs).toHaveCount(3);
  });

  test('first thumb is marked active when viewer opens on first image', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });
    await expect(page.locator('.filmstrip-thumb.filmstrip-active')).toHaveCount(1);
    await expect(page.locator('.filmstrip-thumb[data-index="0"]')).toHaveClass(/filmstrip-active/);
  });

  test('clicking a film strip thumb navigates the viewer', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });

    const counterBefore = await page.locator('.viewer-counter').textContent();
    await page.locator('.filmstrip-thumb[data-index="1"]').click();

    const counterAfter = await page.locator('.viewer-counter').textContent();
    expect(counterAfter).not.toBe(counterBefore);
    expect(counterAfter).toMatch(/^2 \//);
  });

  test('active thumb updates after clicking a different thumb', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });

    await page.locator('.filmstrip-thumb[data-index="2"]').click();
    await expect(page.locator('.filmstrip-thumb[data-index="2"]')).toHaveClass(/filmstrip-active/, { timeout: 3_000 });
    await expect(page.locator('.filmstrip-thumb[data-index="0"]')).not.toHaveClass(/filmstrip-active/);
  });

  test('active thumb updates when navigating via Next button', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });

    await page.locator('.viewer-next').click();
    await expect(page.locator('.filmstrip-thumb[data-index="1"]')).toHaveClass(/filmstrip-active/, { timeout: 3_000 });
    await expect(page.locator('.filmstrip-thumb[data-index="0"]')).not.toHaveClass(/filmstrip-active/);
  });

  test('active thumb updates when navigating via ArrowRight key', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });

    await page.keyboard.press('ArrowRight');
    await expect(page.locator('.filmstrip-thumb[data-index="1"]')).toHaveClass(/filmstrip-active/, { timeout: 3_000 });
  });

  test('film strip thumb has a thumbnail image', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });
    const img = page.locator('.filmstrip-thumb[data-index="0"] img');
    await expect(img).toHaveAttribute('src', /\/api\/thumbnail/);
  });

  test('film strip stays visible after viewer navigation', async ({ page }) => {
    await page.keyboard.press('f');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible({ timeout: 3_000 });
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await expect(page.locator('.viewer-filmstrip')).toBeVisible();
  });
});
