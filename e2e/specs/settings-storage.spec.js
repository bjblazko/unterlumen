import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// Settings → Storage: what the cache and the data folder hold, part by part.
// The e2e server has a cache of its own (playwright.config.js), so clearing
// it here touches nothing of the installed app's.

test.describe('Settings: Storage', () => {
  test('the storage API names the cache parts, its limit and the data folder', async ({ request }) => {
    const s = await (await request.get('/api/storage')).json();
    expect(s.cache.segments.map(p => p.id)).toEqual(['thumbnails', 'images', 'older']);
    expect(s.cache.limit).toBeGreaterThanOrEqual(0);
    expect(Array.isArray(s.data.libraries)).toBe(true);
    expect((await request.post('/api/storage/clear', { data: { segment: 'everything' } })).status()).toBe(400);
  });

  test('previews fill as photos are shown and are cleared on their own', async ({ page, request }) => {
    const browse = await (await request.get('/api/browse?path=folder-b&sort=name&order=asc')).json();
    const photo = browse.entries.find(e => e.type === 'image');
    expect((await request.get(`/api/thumbnail?path=${encodeURIComponent(`folder-b/${photo.name}`)}&size=137`)).ok()).toBe(true);
    await expect.poll(async () => {
      const s = await (await request.get('/api/storage')).json();
      return s.cache.segments.find(p => p.id === 'thumbnails').bytes;
    }, { timeout: 15_000 }).toBeGreaterThan(0);

    await page.goto('/#settings');
    await waitForAppReady(page);
    const previews = page.locator('.storage-row[data-segment="thumbnails"]');
    await expect(previews.locator('.storage-label')).toHaveText('Previews');
    await expect(previews.locator('.storage-hint')).toContainText('Made again as you browse');
    await previews.getByRole('button', { name: 'Clear' }).click();
    await previews.locator('.storage-confirm').getByRole('button', { name: 'Clear' }).click();
    await expect(page.locator('.storage-row[data-segment="thumbnails"] .storage-size')).toHaveText('0 KB');
    await expect(page.locator('.storage-row[data-segment="thumbnails"] .storage-clear')).toHaveCount(0);
  });

  test('the limit is chosen and kept', async ({ page, request }) => {
    await page.goto('/#settings');
    await waitForAppReady(page);
    await page.locator('#storage-limit').selectOption({ label: '10 GB' });
    await expect.poll(async () => (await (await request.get('/api/storage')).json()).cache.limit).toBe(10 * 1073741824);
    await request.put('/api/storage/limit', { data: { bytes: 5 * 1073741824 } });
  });

  test('Folder previews says what Fast and Sharp do', async ({ page }) => {
    await page.goto('/#settings');
    await waitForAppReady(page);
    const field = page.locator('.form-field', { has: page.locator('.form-label', { hasText: 'Folder previews' }) });
    await expect(field).toContainText('Only for Folders');
    await expect(field.locator('.toggle')).toContainText('Fast');
    await expect(field.locator('.toggle')).toContainText('Sharp');
  });
});
