import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// People across libraries (ADR-0051): the name links a person across
// libraries; People starts with everyone, a person has one page by name with
// a section per library, and a library without the name is put right in
// place. The e2e server has no ONNX Runtime, so the People API is answered
// here.

const FAMILY = 'lib-family';
const NAS = 'lib-nas';
const OTHER = 'lib-other';
const WAITING = 'lib-waiting';
const PROJECTS = 'lib-projects';
const tinyJPEG = Buffer.from(
  '/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=',
  'base64');

const anna = (lib, libraryName, person, faces) => ({ library: lib, libraryName, person, faces, photos: faces, cover: 11 });

async function fakeEveryone(page) {
  const changes = [];
  await page.route('**/api/faces', r => r.fulfill({ json: { installed: true, plan: { supported: true, bytes: 75_000_000 } } }));
  await page.route('**/api/library/', r => r.request().method() === 'GET'
    ? r.fulfill({ json: [
      { id: FAMILY, name: 'Family', sourcePath: '/a' }, { id: NAS, name: 'NAS', sourcePath: '/b' },
      { id: OTHER, name: 'Other spelling', sourcePath: '/c' }, { id: WAITING, name: 'Waiting', sourcePath: '/d' },
      { id: PROJECTS, name: 'Projects', sourcePath: '/e' }] })
    : r.continue());
  await page.route('**/api/library/*/face/*', r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route('**/api/library/*/thumb/*', r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route('**/api/people', r => r.fulfill({ json: [
    { name: 'Anna', faces: 5, photos: 5, libraries: [anna(FAMILY, 'Family', 1, 3), anna(NAS, 'NAS', 7, 2)], cover: { library: NAS, face: 21 } },
    { name: 'Ben', faces: 1, photos: 1, libraries: [anna(FAMILY, 'Family', 2, 1)], cover: { library: FAMILY, face: 11 } },
  ] }));
  await page.route('**/api/people/to-name', r => r.fulfill({ json: [
    { library: FAMILY, libraryName: 'Family', groups: 0, pending: 0, finding: false },
    { library: WAITING, libraryName: 'Waiting', groups: 1, pending: 0, finding: false },
  ] }));
  await page.route('**/api/people/someone?*', r => r.fulfill({ json: {
    name: 'Anna',
    in: [anna(FAMILY, 'Family', 1, 3), anna(NAS, 'NAS', 7, 2)],
    cover: { library: NAS, face: 21 },
    // Newest first, whichever library: the same face number in two libraries is two faces.
    faces: [
      { library: NAS, id: 21, photoId: 'q1', date: '2025-01-01T00:00:00', x: 0.1, y: 0.1, w: 0.2, h: 0.2 },
      { library: FAMILY, id: 11, photoId: 'p1', date: '2024-01-01T00:00:00', x: 0.1, y: 0.1, w: 0.2, h: 0.2 },
      { library: NAS, id: 11, photoId: 'q2', date: '2023-01-01T00:00:00', x: 0.1, y: 0.1, w: 0.2, h: 0.2 },
    ],
    notIn: [
      { library: OTHER, libraryName: 'Other spelling', looksLike: { id: 4, name: 'Anna Müller', faces: 2, photos: 2, cover: 31 } },
      { library: WAITING, libraryName: 'Waiting', group: { id: 3, size: 6, faces: [41, 42] } },
      { library: PROJECTS, libraryName: 'Projects' },
    ],
    with: [{ name: 'Ben', photos: 2, cover: { library: FAMILY, face: 11 } }],
  } }));
  await page.route('**/api/people/with?*', r => r.fulfill({ json: [{ name: 'Ben', photos: 2 }] }));
  await page.route('**/api/library/*/people/*', r => {
    if (r.request().method() !== 'PATCH') return r.continue();
    changes.push({ url: new URL(r.request().url()).pathname, ...r.request().postDataJSON() });
    return r.fulfill({ json: { skipped: 0 } });
  });
  const birthdays = {};
  await page.route('**/api/people/birthdays', async r => {
    if (r.request().method() === 'PUT') {
      const { name, birthday } = r.request().postDataJSON();
      birthdays[name] = birthday;
      return r.fulfill({ status: 204 });
    }
    return r.fulfill({ json: birthdays });
  });
  return { birthdays, changes };
}

test.describe('People across libraries', () => {
  test('People starts with everyone, found by name, and the groups waiting for a name', async ({ page }) => {
    await fakeEveryone(page);
    await page.goto('/#people');
    await waitForAppReady(page);
    const cards = page.locator('.people-card');
    await expect(cards).toHaveCount(2);
    await expect(cards.first().locator('.people-card-where')).toHaveText('in 2 libraries');
    await expect(cards.last().locator('.people-card-where')).toHaveText('Family');
    await expect(page.locator('.people-count')).toHaveText('2 people');
    await expect(page.locator('.people-to-name')).toContainText('Groups waiting for a name');
    await expect(page.locator('.people-to-name-link')).toHaveText(['Waiting 1']);
    await page.locator('#people-find').fill('be');
    await expect(cards.locator('visible=true')).toHaveCount(1);
    await page.locator('#people-find').fill('zz');
    await expect(page.locator('.people-none')).toBeVisible();
  });

  test('a person has one page with all their faces, and who elsewhere may be them', async ({ page }) => {
    await fakeEveryone(page);
    await page.goto('/#people');
    await waitForAppReady(page);
    await page.locator('.people-card', { hasText: 'Anna' }).click();
    await expect(page).toHaveURL(/#people\/person\/Anna$/);
    await expect(page.locator('.people-page-title')).toHaveText('Anna');
    await expect(page.locator('.people-faces .people-hint')).toHaveText('3 faces in Family and 2 faces in NAS.');
    const tiles = page.locator('.people-faces .face-tile');
    await expect(tiles).toHaveCount(3);
    await expect(tiles.nth(0)).toHaveAttribute('data-library', NAS);
    await expect(tiles.nth(1)).toHaveAttribute('data-library', FAMILY);
    // The cover the overview shows is the one marked.
    await expect(page.locator('.face-tile-cover')).toHaveCount(1);
    await expect(tiles.nth(0).locator('.face-tile-cover')).toBeVisible();
    // The bar says which libraries the selected faces are in.
    await tiles.nth(0).click();
    await expect(page.locator('.people-bar-where')).toHaveText('In NAS');
    await tiles.nth(1).click();
    await expect(page.locator('.people-bar-where')).toHaveText('In 2 libraries');
    await expect(page.locator('.people-unname')).toHaveText('2 are not Anna');
    await page.locator('.people-faces .select-all-none').click();
    await expect(page.locator('.people-bar')).toBeHidden();
    await page.getByRole('button', { name: 'Person options' }).click();
    await expect(page.getByRole('menuitem')).toHaveText(['Rename…', 'Birthday…', 'Take the name away…']);
    await page.keyboard.press('Escape');

    const elsewhere = page.locator('.people-elsewhere');
    await expect(elsewhere).toHaveCount(2);
    await expect(elsewhere.nth(0)).toContainText('In Other spelling, Anna Müller looks like Anna');
    await expect(elsewhere.nth(0).locator('.people-rename-there')).toHaveText('Rename to Anna in Other spelling (2 photos)');
    await expect(elsewhere.nth(1)).toContainText('6 faces waiting for a name look like Anna');
    await expect(elsewhere.nth(1).locator('.people-name-there')).toHaveText('Name them Anna');
    // A library where nobody looks like her is one line.
    await expect(page.locator('.people-hint', { hasText: 'Not in Projects.' })).toBeVisible();
  });

  test('Select all takes every face, and a change goes to each library', async ({ page }) => {
    await fakeEveryone(page);
    const unnamed = [];
    await page.route('**/api/library/*/faces/unname', r => {
      unnamed.push({ lib: r.request().url().split('/api/library/')[1].split('/')[0], ...r.request().postDataJSON() });
      return r.fulfill({ json: { skipped: 0 } });
    });
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    const select = page.locator('.people-faces .select-all');
    await expect(select.locator('.select-all-count')).toHaveText('3 faces');
    await expect(select.locator('.select-all-none')).toBeHidden();
    await select.locator('.select-all-all').click();
    await expect(select.locator('.select-all-count')).toHaveText('3 faces · 3 selected');
    await expect(select.locator('.select-all-all')).toBeHidden();
    await expect(page.locator('.people-faces .face-tile[aria-pressed="true"]')).toHaveCount(3);
    await expect(page.locator('.people-unname')).toHaveText('3 are not Anna');
    await page.locator('.people-unname').click();
    await expect.poll(() => unnamed).toEqual([{ lib: NAS, faces: [21, 11] }, { lib: FAMILY, faces: [11] }]);
  });

  test("one face selected opens its photo, or shows it in its library's folder", async ({ page }) => {
    await fakeEveryone(page);
    await page.route('**/api/library/*/photo/*/info', r => r.fulfill({ json: { path: '/b/2025/q1.jpg', name: 'q1.jpg' } }));
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    await page.evaluate(() => { window.__shown = []; App.showInLibrary = (...a) => window.__shown.push(a); });
    const tiles = page.locator('.people-faces .face-tile');
    await tiles.nth(0).click();
    await expect(page.locator('.people-open')).toBeVisible();
    await expect(page.locator('.people-in-library')).toBeVisible();
    await tiles.nth(2).click();
    await expect(page.locator('.people-open')).toBeHidden();
    await tiles.nth(2).click();

    await page.locator('.people-in-library').click();
    await expect.poll(() => page.evaluate(() => window.__shown)).toEqual([[NAS, '2025', 'q1.jpg']]);

    await page.locator('.people-open').click();
    await expect(page.locator('.viewer')).toBeVisible();
  });

  test('someone named otherwise elsewhere is renamed there in place', async ({ page }) => {
    const { changes } = await fakeEveryone(page);
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    await page.locator('.people-rename-there').click();
    await expect.poll(() => changes).toEqual([{ url: `/api/library/${OTHER}/people/4`, name: 'Anna' }]);
  });

  test('Rename chooses the libraries, all of them ticked', async ({ page }) => {
    const { changes } = await fakeEveryone(page);
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    await page.getByRole('button', { name: 'Person options' }).click();
    await page.getByRole('menuitem', { name: 'Rename…' }).click();
    const choices = page.locator('.people-library-choice input');
    await expect(choices).toHaveCount(2);
    await expect(choices.nth(0)).toBeChecked();
    await expect(choices.nth(1)).toBeChecked();
    // Someone else is Anna in NAS.
    await choices.nth(1).uncheck();
    await page.locator('#people-rename').fill('Anna Berg');
    await page.locator('.dialog').getByRole('button', { name: 'Rename' }).click();
    await expect(page).toHaveURL(/#people\/person\/Anna%20Berg$/);
    expect(changes).toEqual([{ url: `/api/library/${FAMILY}/people/1`, name: 'Anna Berg' }]);
  });

  test("a person's birthday is given from their page and shows under the name", async ({ page }) => {
    const { birthdays } = await fakeEveryone(page);
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    await expect(page.locator('.people-born')).toHaveCount(0);
    await page.getByRole('button', { name: 'Person options' }).click();
    await page.getByRole('menuitem', { name: 'Birthday…' }).click();
    await page.locator('#people-birthday').fill('2015-05-12');
    await page.locator('.dialog').getByRole('button', { name: 'Save' }).click();
    await expect(page.locator('.people-born')).toContainText('Born');
    await expect(page.locator('.people-born')).toContainText('2015');
    expect(birthdays).toEqual({ Anna: '2015-05-12' });
  });

  test('Often with leads to the photos of both', async ({ page }) => {
    await fakeEveryone(page);
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    const chip = page.locator('.people-with-chip', { hasText: 'Ben' });
    await expect(chip).toContainText('2');
    await expect(chip.locator('img.people-with-face')).toHaveAttribute('src', `/api/library/${FAMILY}/face/11`);
    await expect(page.locator('.people-with .people-label')).toHaveText('Often in photos together with · number of photos');
    await chip.click();
    await expect(page).toHaveURL(/#libraries/);
    await expect(page.locator('.chip .chip-label')).toHaveText(['Person: Anna', 'Person: Ben']);
  });
});
