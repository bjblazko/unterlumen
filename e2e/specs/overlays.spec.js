import path from 'path';
import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, NO_GPS_IMAGE, HIF_IMAGE, navigateToFolder } from '../helpers/fixtures.js';
import { reindexLibrary } from '../helpers/library.js';

const EXAMPLES_PATH = path.resolve(new URL('../fixtures/photos', import.meta.url).pathname);

test.describe('Overlays and EXIF metadata — folder-b (JPEG)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, 'folder-b');
    await waitForThumbnailsLoaded(page, 1);
  });

  test('GPS JPEG gets GPS overlay badge', async ({ page }) => {
    const gpsBadge = page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge-gps`);
    await expect(gpsBadge).toBeVisible({ timeout: 10_000 });
  });

  test('non-GPS JPEG does not get GPS overlay badge', async ({ page }) => {
    // Wait for GPS badge on a GPS image to confirm polling has run
    await expect(
      page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge-gps`),
    ).toBeVisible({ timeout: 10_000 });
    const noGpsBadge = page.locator(`[data-name="${NO_GPS_IMAGE}"] .overlay-badge-gps`);
    await expect(noGpsBadge).not.toBeVisible();
  });

  test('JPEG files show JPEG file-type badge', async ({ page }) => {
    const badge = page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge`, { hasText: 'JPEG' });
    await expect(badge).toBeVisible({ timeout: 5_000 });
  });

  test('info panel shows Location section for GPS JPEG', async ({ page }) => {
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
    await page.keyboard.press('i');
    await expect(page.locator('.info-panel.expanded, .info-panel[data-expanded="true"]')).toBeVisible({ timeout: 5_000 });
    await page.waitForFunction(
      () => document.querySelector('.info-panel') &&
            !document.querySelector('.info-panel').textContent.includes('Loading'),
      { timeout: 10_000 },
    );
    const panelText = await page.locator('.info-panel').textContent();
    expect(panelText).toMatch(/location|latitude|lat/i);
  });

  test('info panel has no Location section for non-GPS JPEG', async ({ page }) => {
    await page.locator(`[data-name="${NO_GPS_IMAGE}"]`).click();
    await page.keyboard.press('i');
    await expect(page.locator('.info-panel.expanded, .info-panel[data-expanded="true"]')).toBeVisible({ timeout: 5_000 });
    await page.waitForFunction(
      () => document.querySelector('.info-panel') &&
            !document.querySelector('.info-panel').textContent.includes('Loading'),
      { timeout: 10_000 },
    );
    const panelText = await page.locator('.info-panel').textContent();
    expect(panelText).not.toMatch(/\d+\.\d+.*°/);
  });
});

test.describe('Overlays — folder-a/a1 (HIF)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, 'folder-a');
    const a1 = page.locator('.grid-item.dir-item[data-name="a1"]');
    await a1.dblclick();
    await page.waitForSelector('.crumb[data-path="folder-a/a1"]', { timeout: 5_000 });
    await waitForThumbnailsLoaded(page, 1);
  });

  test('HIF file shows HEIF file-type badge', async ({ page }) => {
    const badge = page.locator(`[data-name="${HIF_IMAGE}"] .overlay-badge`, { hasText: 'HEIF' });
    await expect(badge).toBeVisible({ timeout: 5_000 });
  });
});

test.describe('Overlays — library folder view', () => {
  let libID;

  test.beforeAll(async ({ request }) => {
    test.setTimeout(200_000);
    const existing = await (await request.get('/api/library/')).json();
    await Promise.all(
      existing.filter(l => l.name === 'E2E Overlay Library').map(l => request.delete(`/api/library/${l.id}`))
    );
    const res = await request.post('/api/library/', {
      data: { name: 'E2E Overlay Library', description: '', sourcePath: EXAMPLES_PATH },
    });
    expect(res.status()).toBe(201);
    libID = (await res.json()).id;
    await reindexLibrary(request, libID);
  });

  test.afterAll(async ({ request }) => {
    if (libID) await request.delete(`/api/library/${libID}`);
  });

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await page.locator('#mode-library').click();
    const card = page.locator('.library-card', { hasText: 'E2E Overlay Library' });
    await card.waitFor({ state: 'visible', timeout: 10_000 });
    await card.locator('.lib-open').click();
    await page.waitForSelector('.library-detail', { timeout: 8_000 });

    // Navigate into folder-b
    const folderB = page.locator('#lib-pane .grid-item.dir-item[data-name="folder-b"]');
    await folderB.waitFor({ state: 'visible', timeout: 10_000 });
    await folderB.dblclick();
    await waitForThumbnailsLoaded(page, 1);
  });

  test('GPS JPEG gets GPS overlay badge in library view', async ({ page }) => {
    const gpsBadge = page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge-gps`);
    await expect(gpsBadge).toBeVisible({ timeout: 5_000 });
  });

  test('non-GPS JPEG does not get GPS overlay badge in library view', async ({ page }) => {
    await expect(page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge-gps`)).toBeVisible({ timeout: 5_000 });
    await expect(page.locator(`[data-name="${NO_GPS_IMAGE}"] .overlay-badge-gps`)).not.toBeVisible();
  });

  test('JPEG files show JPEG file-type badge in library view', async ({ page }) => {
    const badge = page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge`, { hasText: 'JPEG' });
    await expect(badge).toBeVisible({ timeout: 5_000 });
  });

  test('Fujifilm X-T50 JPEG shows film-simulation badge in library view', async ({ page }) => {
    // GPS_IMAGE is a Fujifilm X-T50 shot — always records a film simulation in MakerNote
    const filmBadge = page.locator(`[data-name="${GPS_IMAGE}"] .overlay-badge`).filter({ hasNotText: 'JPEG' }).first();
    await expect(filmBadge).toBeVisible({ timeout: 5_000 });
  });
});
