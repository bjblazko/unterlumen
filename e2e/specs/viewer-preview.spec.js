import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// A HEIF takes seconds and hundreds of MB to convert on a NAS; two at once
// stalled one for minutes. So a library photo opens on its stored preview and
// the full-size photo replaces it, and the photos ahead are fetched only
// where nothing has to be converted for them.

test.describe('The viewer shows the preview first', () => {
  const LIB_NAME = 'E2E Viewer Preview';
  let libID;

  test.beforeAll(async ({ request }) => {
    test.setTimeout(180_000);
    const existing = await (await request.get('/api/library/')).json();
    await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
    const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: 'folder-a/a2' } });
    libID = (await res.json()).id;
    await reindexLibrary(request, libID);
  });

  test.afterAll(async ({ request }) => {
    if (libID) await request.delete(`/api/library/${libID}`);
  });

  async function openFirstPhoto(page) {
    await page.goto('/#libraries');
    await waitForAppReady(page);
    await page.locator('.library-card', { hasText: LIB_NAME }).locator('.lib-open').click();
    await page.locator('#lib-pane [data-type="image"]').first().waitFor();
    await page.locator('#lib-pane [data-type="image"]').first().dblclick();
    await expect(page.locator('.viewer')).toBeVisible();
  }

  test('a library photo opens on its preview and then shows the full-size photo', async ({ page }) => {
    const firstSources = [];
    page.on('request', req => {
      if (req.resourceType() === 'image' && /\/api\/library\/[^/]+\/(thumb|photo)\//.test(req.url())) firstSources.push(req.url());
    });
    await openFirstPhoto(page);

    const img = page.locator('.viewer-image-container img');
    await expect(img).toHaveAttribute('src', /\/api\/library\/[^/]+\/photo\//, { timeout: 60_000 });
    expect(firstSources.some(u => u.includes('/thumb/'))).toBe(true);
    await expect(page.locator('.viewer-crop-btn')).toBeEnabled();
  });

  test('the photos ahead are asked for as a prefetch', async ({ page }) => {
    const prefetches = [];
    page.on('request', req => {
      if (req.headers()['x-prefetch'] === '1') prefetches.push(req.url());
    });
    await openFirstPhoto(page);
    await expect.poll(() => prefetches.length).toBeGreaterThan(0);
    expect(prefetches.every(u => /\/api\/library\/[^/]+\/photo\//.test(u))).toBe(true);
  });
});
