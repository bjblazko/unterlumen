import { test, expect } from '@playwright/test';
import { GPS_PATH, NO_GPS_PATH, HIF_PATH } from '../helpers/fixtures.js';

test.describe('API contract', () => {
  test('GET /api/config returns startPath and serverRole', async ({ request }) => {
    const res = await request.get('/api/config');
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty('startPath');
    expect(typeof body.serverRole).toBe('boolean');
  });

  test('GET /api/browse returns top-level folder entries', async ({ request }) => {
    const res = await request.get('/api/browse?path=&sort=name&order=asc');
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(Array.isArray(body.entries)).toBe(true);
    const names = body.entries.map((e) => e.name);
    expect(names).toContain('folder-a');
    expect(names).toContain('folder-b');
  });

  test('GET /api/browse entries have required fields', async ({ request }) => {
    const res = await request.get('/api/browse?path=folder-b&sort=name&order=asc');
    const body = await res.json();
    for (const entry of body.entries) {
      expect(entry).toHaveProperty('name');
      expect(entry).toHaveProperty('type');
      expect(entry).toHaveProperty('date');
      if (entry.type === 'image') {
        expect(entry).toHaveProperty('size');
      }
    }
    const imageEntry = body.entries.find((e) => e.type === 'image');
    expect(imageEntry).toBeTruthy();
  });

  test('GET /api/browse with sort=size&order=desc returns valid response', async ({ request }) => {
    const res = await request.get('/api/browse?path=folder-b&sort=size&order=desc');
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(Array.isArray(body.entries)).toBe(true);
  });

  test('GET /api/thumbnail for JPEG returns image/jpeg', async ({ request }) => {
    const res = await request.get(`/api/thumbnail?path=${encodeURIComponent(GPS_PATH)}`);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('image/jpeg');
    const buf = await res.body();
    expect(buf.length).toBeGreaterThan(0);
  });

  test('GET /api/thumbnail for HIF returns image/jpeg (converted)', async ({ request }) => {
    const toolRes = await request.get('/api/tools/check');
    const tools = await toolRes.json();
    if (!tools.ffmpeg) {
      test.skip(true, 'ffmpeg not available');
      return;
    }
    const res = await request.get(`/api/thumbnail?path=${encodeURIComponent(HIF_PATH)}`);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('image/jpeg');
  });

  test('GET /api/image for JPEG returns image/jpeg', async ({ request }) => {
    const res = await request.get(`/api/image?path=${encodeURIComponent(GPS_PATH)}`);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('image/jpeg');
  });

  test('GET /api/image for HIF returns image/jpeg (converted)', async ({ request }) => {
    const toolRes = await request.get('/api/tools/check');
    const tools = await toolRes.json();
    if (!tools.ffmpeg) {
      test.skip(true, 'ffmpeg not available');
      return;
    }
    const res = await request.get(`/api/image?path=${encodeURIComponent(HIF_PATH)}`);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('image/jpeg');
  });

  test('GET /api/info for GPS JPEG returns latitude and longitude', async ({ request }) => {
    const res = await request.get(`/api/info?path=${encodeURIComponent(GPS_PATH)}`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.exif).toBeTruthy();
    expect(typeof body.exif.latitude).toBe('number');
    expect(typeof body.exif.longitude).toBe('number');
  });

  test('GET /api/info for non-GPS JPEG has no latitude', async ({ request }) => {
    const res = await request.get(`/api/info?path=${encodeURIComponent(NO_GPS_PATH)}`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    const lat = body.exif?.latitude;
    expect(lat == null).toBe(true);
  });

  test('GET /api/tools/check returns tool availability', async ({ request }) => {
    const res = await request.get('/api/tools/check');
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty('ffmpeg');
  });

  test('GET /api/browse/meta returns ready field', async ({ request }) => {
    const res = await request.get('/api/browse/meta?path=');
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty('ready');
    expect(typeof body.ready).toBe('boolean');
    // 'meta' is only present once EXIF scanning is complete — not asserted here
  });

  test('path traversal on /api/browse returns 400', async ({ request }) => {
    const res = await request.get('/api/browse?path=../../etc');
    expect(res.status()).toBe(400);
  });

  test('path traversal on /api/thumbnail returns 400', async ({ request }) => {
    const res = await request.get('/api/thumbnail?path=../../../etc/passwd');
    expect(res.status()).toBe(400);
  });
});
