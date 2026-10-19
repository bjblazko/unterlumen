import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// People in all libraries (ADR-0051): the name links a person across
// libraries; the page only reads and sends the owner to each library to
// rename. The e2e server has no ONNX Runtime, so the People API is answered
// here.

const FAMILY = 'lib-family';
const NAS = 'lib-nas';
const OTHER = 'lib-other';
const WAITING = 'lib-waiting';
const tinyJPEG = Buffer.from(
  '/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=',
  'base64');

const anna = (lib, libraryName, person, faces) => ({ library: lib, libraryName, person, faces, photos: faces, cover: 11 });

async function fakeEveryone(page) {
  await page.route('**/api/faces', r => r.fulfill({ json: { installed: true, plan: { supported: true, bytes: 75_000_000 } } }));
  await page.route('**/api/library/', r => r.request().method() === 'GET'
    ? r.fulfill({ json: [
      { id: FAMILY, name: 'Family', sourcePath: '/a' }, { id: NAS, name: 'NAS', sourcePath: '/b' },
      { id: OTHER, name: 'Other spelling', sourcePath: '/c' }, { id: WAITING, name: 'Waiting', sourcePath: '/d' }] })
    : r.continue());
  await page.route('**/api/library/*/face/*', r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route('**/api/library/*/thumb/*', r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route('**/api/people', r => r.fulfill({ json: [
    { name: 'Anna', faces: 5, photos: 5, libraries: [anna(FAMILY, 'Family', 1, 3), anna(NAS, 'NAS', 7, 2)] },
    { name: 'Ben', faces: 1, photos: 1, libraries: [anna(FAMILY, 'Family', 2, 1)] },
  ] }));
  await page.route('**/api/people/someone?*', r => r.fulfill({ json: {
    name: 'Anna',
    in: [
      { ...anna(FAMILY, 'Family', 1, 3), shown: [{ id: 11, photoId: 'p1', x: 0.1, y: 0.1, w: 0.2, h: 0.2 }] },
      { ...anna(NAS, 'NAS', 7, 2), shown: [{ id: 21, photoId: 'q1', x: 0.1, y: 0.1, w: 0.2, h: 0.2 }] },
    ],
    notIn: [
      { library: OTHER, libraryName: 'Other spelling', looksLike: { id: 4, name: 'Anna Müller', faces: 2, photos: 2, cover: 31 } },
      { library: WAITING, libraryName: 'Waiting', group: { id: 3, size: 6, faces: [41, 42] } },
    ],
  } }));
  await page.route(`**/api/library/${FAMILY}/people`, r => r.fulfill({ json: {
    people: [{ id: 1, name: 'Anna', faces: 3, photos: 3, cover: 11 }], groups: [], groupCount: 0, pending: 0, finding: false } }));
  await page.route(`**/api/library/${FAMILY}/people/1`, r => r.fulfill({ json: [{ id: 11, photoId: 'p1', x: 0.1, y: 0.1, w: 0.2, h: 0.2 }] }));
}

test.describe('People in all libraries', () => {
  test('All libraries lists everyone by name, and a person shows where they are and who elsewhere may be them', async ({ page }) => {
    await fakeEveryone(page);
    await page.goto(`/#people/${FAMILY}`);
    await waitForAppReady(page);
    await page.locator('.people-library').selectOption({ label: 'All libraries' });
    await expect(page).toHaveURL(/#people\/all$/);
    const cards = page.locator('.people-card');
    await expect(cards).toHaveCount(2);
    await expect(cards.first().locator('.people-card-where')).toHaveText('in 2 libraries');
    await expect(cards.last().locator('.people-card-where')).toHaveText('Family');
    await expect(page.locator('.people-count')).toHaveText('2 people in 2 libraries');

    await cards.first().click();
    await expect(page).toHaveURL(/#people\/all\/person\/Anna$/);
    await expect(page.locator('.people-page-title')).toHaveText('Anna');
    await expect(page.locator('.people-in .people-section-title')).toHaveText(['Family', 'NAS']);
    await expect(page.locator('.people-hint').first()).toContainText('Names are given and changed in each library');
    await expect(page.locator('.people-hint').first().locator('a')).toHaveText(['Family', 'NAS']);
    // Read only: nothing here renames.
    await expect(page.locator('.menu-button, .people-input')).toHaveCount(0);

    const elsewhere = page.locator('.people-elsewhere');
    await expect(elsewhere.nth(0)).toContainText('Anna Müller there looks like Anna');
    await expect(elsewhere.nth(0).locator('a')).toHaveAttribute('href', `#people/${OTHER}/person/4`);
    await expect(elsewhere.nth(1)).toContainText('A group of 6 faces');
    await expect(elsewhere.nth(1).locator('a')).toHaveAttribute('href', `#people/${WAITING}/group/3`);
  });

  test("a person's page in one library says where else the name is, and renaming warns it is this library only", async ({ page }) => {
    await fakeEveryone(page);
    await page.goto(`/#people/${FAMILY}/person/1`);
    await waitForAppReady(page);
    const hint = page.locator('.people-hint');
    await expect(hint).toContainText('Anna is also named in NAS');
    await expect(hint.locator('a').last()).toHaveAttribute('href', '#people/all/person/Anna');
    await page.getByRole('button', { name: 'Person options' }).click();
    await page.getByRole('menuitem', { name: 'Rename…' }).click();
    await expect(page.locator('.dialog')).toContainText('This changes the name in Family only. Anna is also named in NAS: rename there too');
  });
});
