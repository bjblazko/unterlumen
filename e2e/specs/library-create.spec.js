import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// Creating a library through its dialog. The e2e server shares ~/.unterlumen
// with real libraries, so the one made here is removed by name.
const NAME = 'E2E Created In Dialog';

async function removeCreated(request) {
  const libs = await (await request.get('/api/library/')).json();
  await Promise.all(libs.filter(l => l.name === NAME).map(l => request.delete(`/api/library/${l.id}`)));
}

test.describe('New library dialog', () => {
  test.beforeAll(async ({ request }) => removeCreated(request));
  test.afterAll(async ({ request }) => removeCreated(request));

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await waitForAppReady(page);
    await page.locator('#mode-library').click();
    await page.waitForSelector('#lib-new-btn', { timeout: 8_000 });
    await page.locator('#lib-new-btn').click();
    await expect(page.locator('#lib-dlg-name')).toBeVisible();
  });

  test('asks for a name and a folder before creating anything', async ({ page }) => {
    await page.locator('#lib-dlg-create').click();
    await expect(page.locator('#lib-dlg-progress')).toHaveText('A library needs a name and a folder to read.');
    await expect(page.locator('#lib-dlg-name')).toBeFocused();
    await page.locator('#lib-dlg-cancel').click();
    await expect(page.locator('#lib-dlg-name')).toHaveCount(0);
  });

  test('the folder is chosen with the folder picker, and names the library', async ({ page }) => {
    await page.locator('#lib-dlg-choose').click();
    await page.locator('.fp-row', { hasText: 'folder-a' }).click();
    await page.locator('.fp-row', { hasText: 'a3' }).click();
    await expect(page.locator('.fp-crumb-here')).toHaveText('a3');
    await page.locator('#fp-select').click();
    await expect(page.locator('#lib-dlg-path')).toHaveValue('/folder-a/a3');
    await expect(page.locator('#lib-dlg-name')).toHaveValue('a3');
    await page.locator('#lib-dlg-cancel').click();
  });

  test('creates the library, indexes it and opens it', async ({ page, request }) => {
    await page.locator('#lib-dlg-name').fill(NAME);
    // A small folder: the dialog waits for the first index, which on a CI
    // runner took longer than 30 s for the 50 photos of folder-b.
    await page.locator('#lib-dlg-path').fill('"folder-a/a3"');
    await page.locator('#lib-dlg-create').click();
    await expect(page.locator('.library-detail')).toBeVisible({ timeout: 90_000 });
    await expect(page.locator('#lib-dlg-name')).toHaveCount(0);
    const libs = await (await request.get('/api/library/')).json();
    const made = libs.filter(l => l.name === NAME);
    expect(made).toHaveLength(1);
    // Quotes pasted around a path are stripped.
    expect(made[0].sourcePath).not.toContain('"');
  });
});

// The installed app asks the system's folder dialog first. Its answer is
// stubbed here: a test cannot click a dialog outside the page.
test.describe('New library dialog with the system folder dialog', () => {
  let boundary;

  async function openWithSystemDialog(page, chosen) {
    await page.route('**/api/config', async (route) => {
      const response = await route.fetch();
      const cfg = await response.json();
      boundary = cfg.boundary;
      await route.fulfill({ response, json: { ...cfg, folderDialog: true } });
    });
    await page.route('**/api/folder-dialog', (route) => route.fulfill({ json: chosen(boundary) }));
    await page.goto('/');
    await waitForAppReady(page);
    await page.locator('#mode-library').click();
    await page.locator('#lib-new-btn').click();
    await page.locator('#lib-dlg-choose').click();
  }

  test('a folder inside the photo folder is taken as chosen', async ({ page }) => {
    await openWithSystemDialog(page, (root) => ({ path: `${root}/folder-a/a3/`, cancelled: false }));
    await expect(page.locator('#lib-dlg-path')).toHaveValue('/folder-a/a3');
    await expect(page.locator('#lib-dlg-name')).toHaveValue('a3');
    await expect(page.locator('.fp-dialog')).toHaveCount(0);
  });

  test('a folder outside it opens the picker, which says why', async ({ page }) => {
    await openWithSystemDialog(page, () => ({ path: '/Volumes/elsewhere', cancelled: false }));
    await expect(page.locator('.fp-notice')).toContainText('/Volumes/elsewhere is outside the photo folder');
    await page.locator('#fp-cancel').click();
    await expect(page.locator('#lib-dlg-path')).toHaveValue('');
  });

  test('cancelling the system dialog changes nothing', async ({ page }) => {
    await openWithSystemDialog(page, () => ({ path: '', cancelled: true }));
    await expect(page.locator('.fp-dialog')).toHaveCount(0);
    await expect(page.locator('#lib-dlg-path')).toHaveValue('');
  });
});
