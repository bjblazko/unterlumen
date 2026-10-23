import { test, expect } from '@playwright/test';
import fs from 'fs';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// Things with the real models (ADR-0054), on the invented people of the
// faces server (playwright.config.js, project "faces"), after
// face-recognition.spec.js named one of them. What the tests rely on, as
// CLIP sees these photos: most portraits are "portrait", and "people at a
// party" finds the party photos first.

test.describe.serial('Things', () => {
  let lib;

  test.beforeAll(async ({ request }) => {
    test.setTimeout(900_000);
    const status = await (await request.get('/api/things')).json();
    if (!status.installed) {
      const r = await request.post('/api/things/install', { timeout: 840_000 });
      expect(r.status(), await r.text()).toBe(204);
    }
    const libs = await (await request.get('/api/library/')).json();
    lib = libs[0]?.id;
    if (!lib) {
      const made = await request.post('/api/library/', { data: { name: 'Friends', description: '', sourcePath: '.' } });
      lib = (await made.json()).id;
    }
    await reindexLibrary(request, lib);
    await expect.poll(async () => {
      const ts = await (await request.get('/api/library/things-stats')).json();
      const all = (await (await request.get(`/api/library/search?ids=${lib}&limit=1`)).json()).total;
      return ts.looked > 0 && ts.looked === all;
    }, { timeout: 300_000, intervals: [2_000] }).toBe(true);
  });

  test('the search bar suggests a thing and opens its photos with the filter set', async ({ page }) => {
    await page.goto('/#libraries');
    await waitForAppReady(page);
    await page.keyboard.press('/');
    await expect(page.locator('#search-bar-input')).toBeFocused();
    await page.keyboard.type('portr');
    const option = page.locator('.search-bar-option', { hasText: 'portrait' });
    await expect(option).toBeVisible();
    await expect(option.locator('.search-bar-count')).toContainText('photos');
    await page.keyboard.press('Enter');
    await expect(page.locator('.lib-filter-chip', { hasText: 'portrait' }).first()).toBeVisible();
    await expect(page.locator('.place-header:visible')).toContainText('Results');
  });

  test('the search bar finds a person', async ({ page }) => {
    await page.goto('/#map');
    await waitForAppReady(page);
    await page.locator('#search-bar-input').fill('Me');
    await expect(page.locator('.search-bar-kind', { hasText: 'People' })).toBeVisible();
    await page.locator('.search-bar-option', { hasText: 'Mei' }).click();
    await expect(page.locator('.lib-filter-chip', { hasText: 'Mei' }).first()).toBeVisible();
  });

  test('words find the photos that show them, best first', async ({ request }) => {
    const r = await (await request.get('/api/library/search?text=people%20at%20a%20party&limit=3')).json();
    expect(r.total).toBeGreaterThan(0);
    expect(r.results[0].pathHint).toContain('/party/');
    expect(r.results[0].match).toBeGreaterThanOrEqual(r.results[r.results.length - 1].match);
  });

  test('a thing taken away in the info panel stays away, in the sidecar too', async ({ page, request }) => {
    await page.goto('/#libraries');
    await waitForAppReady(page);
    await page.locator('#search-bar-input').fill('portrait');
    await page.keyboard.press('Enter');
    const first = page.locator('.image-item').first();
    await first.click();
    const file = await first.getAttribute('data-path'); // the results' photos are named by their file
    await page.getByRole('button', { name: 'Info', exact: true }).click();
    const row = page.locator('.info-thing', { hasText: 'portrait' });
    await expect(row).toBeVisible();
    await row.locator('.info-thing-remove').click();
    await expect(row).toHaveCount(0);
    const found = await (await request.get(`/api/library/search?ids=${lib}&limit=200`)).json();
    const photo = found.results.find(p => p.pathHint === file);
    const sidecar = fs.readFileSync(photo.pathHint.replace(/\.jpg$/, '.xmp'), 'utf8');
    expect(sidecar).toMatch(/<ul:ThingsNot>[\s\S]*portrait[\s\S]*<\/ul:ThingsNot>/);
    expect(sidecar).not.toMatch(/<dc:subject>[\s\S]*<rdf:li>portrait<\/rdf:li>[\s\S]*<\/dc:subject>/);
    const things = await (await request.get(`/api/library/${lib}/photo/${photo.id}/things`)).json();
    expect(things.map(t => t.name)).not.toContain('portrait');
  });

  test('the Things topic counts the things met most', async ({ page }) => {
    await page.goto('/#statistics/things');
    await waitForAppReady(page);
    await expect(page.locator('.stats-chart', { hasText: 'Things met most' }).locator('svg rect').first()).toBeVisible();
    await expect(page.locator('.stats-chart', { hasText: 'Things met most' })).toContainText('portrait');
  });

  test('on a phone the search stands above the place', async ({ browser }) => {
    const phone = await browser.newPage({ viewport: { width: 390, height: 800 } });
    await phone.goto('/#libraries');
    await waitForAppReady(phone);
    const bar = await phone.locator('#search-bar-input').boundingBox();
    expect(bar.y).toBeLessThan(60);
    expect(bar.width).toBeGreaterThan(300);
    await phone.close();
  });
});
