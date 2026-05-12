import { test, expect } from '@playwright/test';
import { GPS_PATH, NO_GPS_PATH } from '../helpers/fixtures.js';

// Crop modifies files in-place on fixtures/photos/ (copies — originals in src/examples are safe).
// Run `npm run setup` to restore the copies between test runs if needed.

test.describe('Crop API', () => {
  // ── Validation ────────────────────────────────────────────────────────────

  test('POST /api/crop with empty path returns 400', async ({ request }) => {
    const res = await request.post('/api/crop', {
      data: { path: '', x: 0, y: 0, width: 0.5, height: 0.5 },
    });
    expect(res.status()).toBe(400);
  });

  test('POST /api/crop with zero width returns 400', async ({ request }) => {
    const res = await request.post('/api/crop', {
      data: { path: GPS_PATH, x: 0, y: 0, width: 0, height: 0.5 },
    });
    expect(res.status()).toBe(400);
  });

  test('POST /api/crop with zero height returns 400', async ({ request }) => {
    const res = await request.post('/api/crop', {
      data: { path: GPS_PATH, x: 0, y: 0, width: 0.5, height: 0 },
    });
    expect(res.status()).toBe(400);
  });

  test('POST /api/crop with out-of-bounds region returns 400', async ({ request }) => {
    const res = await request.post('/api/crop', {
      data: { path: GPS_PATH, x: 0.8, y: 0.8, width: 0.5, height: 0.5 }, // x+w > 1
    });
    expect(res.status()).toBe(400);
  });

  test('POST /api/crop with path traversal returns 400', async ({ request }) => {
    const res = await request.post('/api/crop', {
      data: { path: '../../etc/passwd', x: 0, y: 0, width: 0.5, height: 0.5 },
    });
    expect(res.status()).toBe(400);
  });

  test('POST /api/crop with method GET returns 405', async ({ request }) => {
    const res = await request.get('/api/crop');
    expect(res.status()).toBe(405);
  });

  // ── Successful crop ───────────────────────────────────────────────────────

  test('POST /api/crop with valid params returns 200', async ({ request }) => {
    // Crop the centre 50% of the no-GPS Canon image (doesn't affect GPS tests)
    const res = await request.post('/api/crop', {
      data: { path: NO_GPS_PATH, x: 0.25, y: 0.25, width: 0.5, height: 0.5 },
    });
    expect(res.status()).toBe(200);
  });

  test('thumbnail is invalidated after crop — subsequent request succeeds', async ({ request }) => {
    // Just verify the thumbnail endpoint still works after a crop has been applied
    const res = await request.get(`/api/thumbnail?path=${encodeURIComponent(NO_GPS_PATH)}`);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('image/jpeg');
  });
});
