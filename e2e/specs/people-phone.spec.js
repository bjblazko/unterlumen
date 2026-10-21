import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// People on a phone (ADR-0051): find a person, see their photos by year and
// who is often with them; nothing to name or manage. The People API is
// answered here, since the e2e server has no face recognition.

test.use({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 3, isMobile: true, hasTouch: true });

const LIB = 'lib-family';
const tinyJPEG = Buffer.from(
  '/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=',
  'base64');
const named = (name, person, photos) => ({ name, faces: photos, photos, libraries: [{ library: LIB, libraryName: 'Family', person, faces: photos, photos, cover: person * 10 }], cover: { library: LIB, face: person * 10 } });

async function fakePeople(page, { slowFirst = 0 } = {}) {
  const searches = [];
  await page.route('**/api/faces', r => r.fulfill({ json: { installed: true, plan: { supported: true, bytes: 1 } } }));
  await page.route('**/api/library/', r => r.request().method() === 'GET'
    ? r.fulfill({ json: [{ id: LIB, name: 'Family', sourcePath: '/a' }] }) : r.continue());
  await page.route('**/api/library/*/face/*', r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route('**/api/library/*/thumb/*', r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route('**/api/people', r => r.fulfill({ json: [named('Anna', 1, 3), named('Ben', 2, 2), named('Cleo', 3, 1)] }));
  await page.route('**/api/people/birthdays', r => r.fulfill({ json: { Anna: '2015-05-12' } }));
  await page.route('**/api/people/someone?*', r => r.fulfill({ json: {
    name: 'Anna', in: named('Anna', 1, 3).libraries, cover: { library: LIB, face: 10 }, faces: [], notIn: [],
    with: [{ name: 'Ben', photos: 1, cover: { library: LIB, face: 20 } }],
  } }));
  await page.route(`**/api/library/${LIB}/people`, r => r.fulfill({ json: {
    people: [{ id: 1, name: 'Anna', faces: 3, photos: 3, cover: 10 }], groups: [], groupCount: 0, pending: 0, finding: false } }));
  await page.route('**/api/library/search*', async r => {
    const q = new URL(r.request().url()).searchParams;
    searches.push(q.getAll('person'));
    if (slowFirst && searches.length === 1) await new Promise(res => setTimeout(res, slowFirst));
    const all = [
      { libraryID: LIB, id: 'p1', filename: 'a.jpg', dateTaken: '2024-06-01T10:00:00' },
      { libraryID: LIB, id: 'p2', filename: 'b.jpg', dateTaken: '2023-02-01T10:00:00' },
      { libraryID: LIB, id: 'p3', filename: 'c.jpg', dateTaken: '2023-01-01T10:00:00' },
    ];
    const results = q.getAll('person').length > 1 ? all.slice(0, 1) : all;
    return r.fulfill({ json: { total: results.length, results } });
  });
  return { searches };
}

test.describe('People on a phone', () => {
  test('everyone named is found by typing a name, with nothing to manage', async ({ page }) => {
    await fakePeople(page);
    await page.goto('/#people');
    await waitForAppReady(page);
    await expect(page.locator('.desk-only-notice')).toBeHidden();
    const cards = page.locator('.people-phone-grid .people-card');
    await expect(cards).toHaveCount(3);
    await page.locator('#people-find').fill('be');
    await expect(cards.locator('visible=true')).toHaveCount(1);
    await expect(cards.locator('visible=true').locator('.people-card-name')).toHaveText('Ben');
    await page.locator('#people-find').fill('zz');
    await expect(page.locator('.people-phone-none')).toBeVisible();
    await expect(page.locator('.people-place').locator('.people-group, .people-input, .menu-btn, .people-library').locator('visible=true')).toHaveCount(0);
  });

  test('a person shows their photos by year, and with someone often with them', async ({ page }) => {
    const { searches } = await fakePeople(page);
    await page.goto('/#people');
    await waitForAppReady(page);
    await page.locator('.people-card', { hasText: 'Anna' }).tap();
    await expect(page).toHaveURL(/#people\/person\/Anna$/);
    await expect(page.locator('.people-phone-hero .people-page-title')).toHaveText('Anna');
    await expect(page.locator('.people-phone-hero .people-born')).toContainText('2015');
    await expect(page.locator('.people-year')).toHaveText(['2024', '2023']);
    await expect(page.locator('.people-phone-photo')).toHaveCount(3);
    expect(searches.at(-1)).toEqual(['Anna']);

    await page.locator('.people-with-chip', { hasText: 'Ben' }).tap();
    await expect(page.locator('.people-phone-subject')).toHaveText('Photos of Anna and Ben together');
    await expect(page.locator('.people-phone-photo')).toHaveCount(1);
    expect(searches.at(-1)).toEqual(['Anna', 'Ben']);

    await page.locator('.people-phone-photo').first().tap();
    await expect(page.locator('.viewer')).toBeVisible();
  });

  // On a slow network the first photos are still on their way when a
  // name under Often with is tapped: the photos of both must still come.
  test('choosing someone while the first photos load still shows the photos of both', async ({ page }) => {
    await fakePeople(page, { slowFirst: 1500 });
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    await page.locator('.people-with-chip', { hasText: 'Ben' }).tap();
    await expect(page.locator('.people-phone-subject')).toHaveText('Photos of Anna and Ben together');
    await expect(page.locator('.people-phone-photo')).toHaveCount(1, { timeout: 5000 });
    await page.waitForTimeout(2000);
    // The late answer for Anna alone does not add her other photos.
    await expect(page.locator('.people-phone-photo')).toHaveCount(1);
  });

  test("a library's person page leads to the same page by name", async ({ page }) => {
    await fakePeople(page);
    await page.goto(`/#people/${LIB}/person/1`);
    await waitForAppReady(page);
    await expect(page).toHaveURL(/#people\/person\/Anna$/);
    await expect(page.locator('.people-phone-hero .people-page-title')).toHaveText('Anna');
  });

  test('Galleries is desktop work on a phone', async ({ page }) => {
    await page.goto('/#galleries');
    await waitForAppReady(page);
    await expect(page.locator('.desk-only-notice')).toBeVisible();
    await expect(page.locator('.desk-only-notice')).toContainText('find the people in them');
  });
});
