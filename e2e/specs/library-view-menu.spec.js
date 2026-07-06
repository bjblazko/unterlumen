import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE } from '../helpers/fixtures.js';
import { reindexLibrary } from '../helpers/library.js';

// Regression coverage for the View menu (Layout: Grid/Justified/List, Show
// names, Show details) inside an open library folder view (#lib-pane).
// LibraryPane extends BrowsePane and inherits this menu unmodified, but it
// was previously only covered by e2e tests in plain browse mode — never
// inside a library, which is where users primarily interact with it.
test.describe('View menu — library folder view', () => {
  let libID;

  test.beforeAll(async ({ request }) => {
    test.setTimeout(200_000);
    const existing = await (await request.get('/api/library/')).json();
    await Promise.all(
      existing.filter(l => l.name === 'E2E View Menu Library').map(l => request.delete(`/api/library/${l.id}`))
    );
    const res = await request.post('/api/library/', {
      data: { name: 'E2E View Menu Library', description: '', sourcePath: '/' },
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
    const card = page.locator('.library-card', { hasText: 'E2E View Menu Library' });
    await card.waitFor({ state: 'visible', timeout: 10_000 });
    await card.locator('.lib-open').click();
    await page.waitForSelector('.library-detail', { timeout: 8_000 });
  });

  // Regression test: the library root (sourcePath '/') contains only
  // subdirectories (folder-a, folder-b), no images. LibraryPane builds
  // synthetic directory entries with `date: new Date(0)` (a Date object)
  // instead of an ISO string, which crashes formatDate()'s `iso.replace()`
  // call as soon as list view tries to render a directory row — silently
  // aborting the render, which looks like "the button doesn't react".
  test('switch to list view at the library root (folders only) does not crash', async ({ page }) => {
    const errors = [];
    page.on('pageerror', (err) => errors.push(err.message));

    await page.waitForSelector('#lib-pane .grid-item.dir-item[data-name="folder-a"]', { timeout: 10_000 });

    await page.locator('#lib-pane .view-menu-btn').click();
    await page.locator('#lib-pane button[data-view="list"]').click();

    await expect(page.locator('#lib-pane table.list-view')).toBeVisible({ timeout: 5_000 });
    const dirRow = page.locator('#lib-pane tr[data-type="dir"][data-name="folder-a"]');
    await expect(dirRow).toBeVisible({ timeout: 5_000 });
    // The date cell must render without throwing, even for the placeholder date.
    await expect(dirRow.locator('.list-date')).toBeVisible();

    expect(errors, `console/page errors switching to list view at library root: ${errors.join('\n')}`).toEqual([]);
  });

  test.describe('inside folder-b', () => {
    test.beforeEach(async ({ page }) => {
      const folderB = page.locator('#lib-pane .grid-item.dir-item[data-name="folder-b"]');
      await folderB.waitFor({ state: 'visible', timeout: 10_000 });
      await folderB.dblclick();
      await waitForThumbnailsLoaded(page, 1);
    });

    test('switch to list view inside a library folder', async ({ page }) => {
      const errors = [];
      page.on('pageerror', (err) => errors.push(err.message));

      await page.locator('#lib-pane .view-menu-btn').click();
      await page.locator('#lib-pane button[data-view="list"]').click();

      await expect(page.locator('#lib-pane table.list-view')).toBeVisible({ timeout: 5_000 });
      await expect(page.locator('#lib-pane tr[data-type="image"]').first()).toBeVisible({ timeout: 5_000 });

      expect(errors, `console/page errors: ${errors.join('\n')}`).toEqual([]);
    });

    test('switch back to grid view inside a library folder', async ({ page }) => {
      await page.locator('#lib-pane .view-menu-btn').click();
      await page.locator('#lib-pane button[data-view="list"]').click();
      await expect(page.locator('#lib-pane table.list-view')).toBeVisible({ timeout: 5_000 });

      await page.locator('#lib-pane .view-menu-btn').click();
      await page.locator('#lib-pane button[data-view="grid"]').click();
      await expect(page.locator('#lib-pane .grid-item.image-item').first()).toBeVisible({ timeout: 5_000 });
    });

    test('Show names toggle shows and hides filename labels in a library folder', async ({ page }) => {
      const item = page.locator(`#lib-pane [data-name*="${GPS_IMAGE}"], #lib-pane .grid-item.image-item`).first();
      await expect(item.locator('.item-name')).toHaveCount(0);

      await page.locator('#lib-pane .view-menu-btn').click();
      const namesToggle = page.locator('#lib-pane .toggle-names-wrap .toggle');
      await namesToggle.click();

      await expect(item.locator('.item-name')).toBeVisible({ timeout: 5_000 });

      await page.locator('#lib-pane .view-menu-btn').click();
      await namesToggle.click();
      await expect(item.locator('.item-name')).toHaveCount(0);
    });

    test('Show details toggle shows and hides overlay badges in a library folder', async ({ page }) => {
      const gpsItem = page.locator(`#lib-pane [data-name="${GPS_IMAGE}"]`);
      await expect(gpsItem.locator('.overlay-badges')).toBeVisible({ timeout: 5_000 });

      await page.locator('#lib-pane .view-menu-btn').click();
      const overlaysToggle = page.locator('#lib-pane .toggle-overlays-wrap .toggle');
      await overlaysToggle.click();

      await expect(gpsItem.locator('.overlay-badges')).toHaveCount(0);

      await page.locator('#lib-pane .view-menu-btn').click();
      await overlaysToggle.click();
      await expect(gpsItem.locator('.overlay-badges')).toBeVisible({ timeout: 5_000 });
    });
  });
});
