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
