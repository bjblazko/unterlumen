import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// Every place says in one sentence what it is and what it does to your files,
// and the places it names are links (feature doc 2026-09-28-explain-the-model).

const PLACES = [
  ['#folders', '.folder-title', 'on disk'],
  ['#organize', '.organize-head', 'move on disk'],
  ['#libraries', '.library-list-body', 'every folder inside it'],
  ['#map', '.map-head', 'from all'],
  ['#timeline', '.timeline-head', 'one time axis'],
  ['#galleries', '.gal-head', 'published together'],
  ['#destinations', '.gal-head', 'where galleries go'],
];

test.describe('Explaining the model', () => {
  for (const [hash, scope, words] of PLACES) {
    test(`${hash} says what it is`, async ({ page }) => {
      await page.goto('/' + hash);
      await waitForAppReady(page);
      await expect(page.locator(`${scope} .place-lede`)).toContainText(words);
    });
  }

  test('a place named in a sentence is a link that goes there, and back returns', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    await page.locator('.folder-title .place-link[data-mode="library"]').click();
    await expect(page).toHaveURL(/#libraries$/);
    await expect(page.locator('#mode-library')).toHaveAttribute('aria-current', 'page');
    await page.goBack();
    await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
  });

  test('a subfolder does not repeat the sentence', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    await expect(page.locator('.folder-title .place-lede')).toBeVisible();
    await page.locator('.folder-row [data-type="dir"], [data-type="dir"]').first().dblclick();
    await expect(page.locator('.folder-title h1')).not.toHaveText('Root');
    await expect(page.locator('.folder-title .place-lede')).toHaveCount(0);
  });

  test('#guide opens directly and its boxes lead to the places', async ({ page }) => {
    // #guide has no sidebar entry, so waitForAppReady (which waits for one) does not apply.
    await page.goto('/#guide');
    await expect(page.locator('.guide-pane h1')).toHaveText('How Unterlumen works');
    await expect(page.locator('.guide-diagram')).toBeVisible();
    // A library holds subfolders, and libraries need no folder in common
    // (ADR-0047); the guide shows both with an example tree.
    await expect(page.locator('.guide-tree')).toContainText('Pictures/Projects/');
    await expect(page.locator('.guide-tree')).toContainText('nas/Travel/');
    await page.locator('a.guide-node[data-mode="destinations"]').click();
    await expect(page.locator('#mode-destinations')).toHaveAttribute('aria-current', 'page');
  });

  test('every sentence leads to the guide', async ({ page }) => {
    await page.goto('/#map');
    await waitForAppReady(page);
    await page.locator('.map-head .place-link[data-mode="guide"]').click();
    await expect(page).toHaveURL(/#guide$/);
    await expect(page.locator('.guide-pane')).toBeVisible();
    await page.goBack();
    await expect(page.locator('#mode-map')).toHaveAttribute('aria-current', 'page');
  });

  test('Settings and About lead to the guide', async ({ page }) => {
    await page.goto('/#settings');
    await waitForAppReady(page);
    await page.locator('.settings-guide .place-link').click();
    await expect(page.locator('.guide-pane')).toBeVisible();

    await page.locator('#about-trigger').click();
    await page.locator('.about-place[data-mode="guide"]').click();
    await expect(page.locator('.dialog-scrim')).toHaveCount(0);
    await expect(page).toHaveURL(/#guide$/);
  });

  test('the guide reads on a phone', async ({ browser }) => {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
    await page.goto('/#guide');
    await expect(page.locator('.guide-pane')).toBeVisible();
    await expect(page.locator('.desk-only-notice')).toBeHidden();
    const box = await page.locator('.guide-diagram').boundingBox();
    expect(box.width).toBeLessThanOrEqual(390);
    await page.close();
  });

  test.describe('a folder and its library point at each other', () => {
    const LIB_NAME = 'E2E Explain Folder A';
    const OUTER_NAME = 'E2E Explain Everything';

    async function removeLib(request) {
      const libs = await (await request.get('/api/library/')).json();
      await Promise.all(libs.filter(l => [LIB_NAME, OUTER_NAME].includes(l.name)).map(l => request.delete(`/api/library/${l.id}`)));
    }

    test.beforeAll(async ({ request }) => {
      await removeLib(request);
      for (const [name, sourcePath] of [[LIB_NAME, 'folder-a'], [OUTER_NAME, '/']]) {
        const res = await request.post('/api/library/', { data: { name, description: '', sourcePath } });
        expect(res.ok()).toBeTruthy();
      }
    });

    test('a folder in two libraries names both', async ({ page }) => {
      await page.goto('/#folders');
      await waitForAppReady(page);
      await page.locator('[data-type="dir"][data-path="folder-a"], [data-type="dir"]:has-text("folder-a")').first().dblclick();
      const badge = page.locator('.browse-library-badge');
      await expect(badge.locator('.library-link', { hasText: LIB_NAME })).toBeVisible();
      await expect(badge.locator('.library-link', { hasText: OUTER_NAME })).toBeVisible();
      await expect(badge).toContainText(/^In libraries /);
    });

    test.afterAll(async ({ request }) => { await removeLib(request); });

    test('Folders names the library, and the library opens its folder', async ({ page }) => {
      await page.goto('/#folders');
      await waitForAppReady(page);
      await page.locator('[data-type="dir"][data-path="folder-a"], [data-type="dir"]:has-text("folder-a")').first().dblclick();
      // Other specs may leave a library over the same folder; every one is named.
      const badge = page.locator('.browse-library-badge');
      await expect(badge).toContainText(/^In librar(y|ies) /);
      await badge.locator('.library-link', { hasText: LIB_NAME }).click();
      await expect(page.locator('.library-detail-name')).toHaveText(LIB_NAME);

      await page.locator('.library-detail-folder .folder-link').click();
      await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
      await expect(page.locator('.folder-title h1')).toHaveText('folder-a');
    });
  });

  test('Galleries points at Destinations', async ({ page }) => {
    await page.goto('/#galleries');
    await waitForAppReady(page);
    await page.locator('.gal-head .place-link[data-mode="destinations"]').click();
    await expect(page.locator('#mode-destinations')).toHaveAttribute('aria-current', 'page');
  });
});
