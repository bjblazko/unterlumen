import { test, expect } from '@playwright/test';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_PATH, GPS_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

test.describe('Export', () => {
  // ── Estimate API ─────────────────────────────────────────────────────────

  test.describe('Export estimate API', () => {
    test('POST /api/export/estimate returns estimates array', async ({ request }) => {
      const res = await request.post('/api/export/estimate', {
        data: {
          files: [GPS_PATH],
          format: 'jpeg',
          quality: 85,
          scale: {},
          method: 'heuristic',
        },
      });
      expect(res.status()).toBe(200);
      const body = await res.json();
      expect(Array.isArray(body.estimates)).toBe(true);
      expect(body.estimates.length).toBe(1);
    });

    test('estimate entry has required fields', async ({ request }) => {
      const res = await request.post('/api/export/estimate', {
        data: {
          files: [GPS_PATH],
          format: 'jpeg',
          quality: 85,
          scale: {},
          method: 'heuristic',
        },
      });
      const body = await res.json();
      const entry = body.estimates[0];
      expect(entry).toHaveProperty('file');
      expect(entry).toHaveProperty('inputBytes');
      expect(typeof entry.inputBytes).toBe('number');
      expect(entry.inputBytes).toBeGreaterThan(0);
    });

    test('estimate with multiple files returns one entry per file', async ({ request }) => {
      const files = [
        'folder-b/2024-07-04_14-09-43_X-T50_DSCF3258.jpeg',
        'folder-b/2018-10-20_17-46-50_Canon EOS 500D_IMG_3826.jpeg',
      ];
      const res = await request.post('/api/export/estimate', {
        data: { files, format: 'jpeg', quality: 85, scale: {}, method: 'heuristic' },
      });
      expect(res.status()).toBe(200);
      const body = await res.json();
      expect(body.estimates.length).toBe(2);
    });
  });

  // ── ZIP stream API ────────────────────────────────────────────────────────

  test.describe('ZIP stream and download', () => {
    test('POST /api/export/zip-stream produces SSE events ending with a download token', async ({ request }) => {
      const res = await request.post('/api/export/zip-stream', {
        data: { files: [GPS_PATH], format: 'jpeg', quality: 85, scale: {}, exifMode: 'keep' },
      });
      expect(res.status()).toBe(200);
      expect(res.headers()['content-type']).toContain('text/event-stream');

      const body = await res.text();
      // SSE lines are "data: {...}\n\n" — find the complete event which carries the token
      const lines = body.split('\n').filter(l => l.startsWith('data:'));
      const events = lines.map(l => JSON.parse(l.slice('data:'.length).trim()));
      const completeEvent = events.find(e => e.complete);
      expect(completeEvent).toBeTruthy();
      expect(typeof completeEvent.token).toBe('string');
      expect(completeEvent.token.length).toBeGreaterThan(0);
    });

    test('GET /api/export/zip-download with valid token returns a ZIP file', async ({ request }) => {
      // Create a ZIP first
      const streamRes = await request.post('/api/export/zip-stream', {
        data: { files: [GPS_PATH], format: 'jpeg', quality: 85, scale: {}, exifMode: 'keep' },
      });
      const body = await streamRes.text();
      const lines = body.split('\n').filter(l => l.startsWith('data:'));
      const events = lines.map(l => JSON.parse(l.slice('data:'.length).trim()));
      const completeEvent = events.find(e => e.complete);
      const token = completeEvent?.token;

      expect(token).toBeTruthy();

      const dlRes = await request.get(`/api/export/zip-download?token=${token}`);
      expect(dlRes.status()).toBe(200);
      expect(dlRes.headers()['content-type']).toContain('application/zip');
      expect(dlRes.headers()['content-disposition']).toContain('attachment');
      const buf = await dlRes.body();
      expect(buf.length).toBeGreaterThan(0);
    });
  });

  // ── UI smoke test ─────────────────────────────────────────────────────────

  test('select an image in browse and open export dialog via toolbar', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, 'folder-b');
    await page.waitForSelector(`[data-name="${GPS_IMAGE}"]`, { timeout: 10_000 });
    await page.locator(`[data-name="${GPS_IMAGE}"]`).click();

    // Open the Tools dropdown then click "Convert & Export"
    await page.locator('.tools-menu-btn').click();
    const exportBtn = page.locator('button.tool-item[data-tool="export"]');
    await expect(exportBtn).toBeVisible({ timeout: 3_000 });
    await exportBtn.click();

    const exportModal = page.locator('.export-modal');
    await expect(exportModal).toBeVisible({ timeout: 5_000 });
    await page.keyboard.press('Escape');
    await expect(exportModal).not.toBeVisible({ timeout: 3_000 });
  });
});
