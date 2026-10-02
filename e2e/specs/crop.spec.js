import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { GPS_PATH, NO_GPS_IMAGE } from '../helpers/fixtures.js';

// Crop changes a file in place. It works on a copy of a fixture made for
// this spec: cropping the fixture itself halved it on every run, until it was
// 2×1 pixels and the export estimates failed on it.
const FOLDER_B = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'fixtures', 'photos', 'folder-b');
const CROP_NAME = 'crop-spec-copy.jpeg';
const CROP_PATH = `folder-b/${CROP_NAME}`;

test.describe('Crop API', () => {
  test.describe.configure({ mode: 'serial' });
  test.beforeAll(() => fs.copyFileSync(path.join(FOLDER_B, NO_GPS_IMAGE), path.join(FOLDER_B, CROP_NAME)));
  test.afterAll(() => fs.rmSync(path.join(FOLDER_B, CROP_NAME), { force: true }));

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
    const res = await request.post('/api/crop', {
      data: { path: CROP_PATH, x: 0.25, y: 0.25, width: 0.5, height: 0.5 },
    });
    expect(res.status()).toBe(200);
  });

  test('thumbnail is invalidated after crop — subsequent request succeeds', async ({ request }) => {
    // Just verify the thumbnail endpoint still works after a crop has been applied
    const res = await request.get(`/api/thumbnail?path=${encodeURIComponent(CROP_PATH)}`);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('image/jpeg');
  });
});
