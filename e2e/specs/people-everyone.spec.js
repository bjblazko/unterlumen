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
    with: [{ name: 'Ben', photos: 2 }],
  } }));
  await page.route('**/api/people/with?*', r => r.fulfill({ json: [{ name: 'Ben', photos: 2 }] }));
  await page.route(`**/api/library/${FAMILY}/people`, r => r.fulfill({ json: {
    people: [{ id: 1, name: 'Anna', faces: 3, photos: 3, cover: 11 }], groups: [], groupCount: 0, pending: 0, finding: false } }));
  await page.route(`**/api/library/${FAMILY}/people/1`, r => r.fulfill({ json: [{ id: 11, photoId: 'p1', x: 0.1, y: 0.1, w: 0.2, h: 0.2 }] }));
  const birthdays = {};
  await page.route('**/api/people/birthdays', async r => {
    if (r.request().method() === 'PUT') {
      const { name, birthday } = r.request().postDataJSON();
      birthdays[name] = birthday;
      return r.fulfill({ status: 204 });
    }
    return r.fulfill({ json: birthdays });
  });
  return { birthdays };
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
    // Read only: nothing here renames; the menu only has the birthday,
    // which is the person's on every library.
    await expect(page.locator('.people-input')).toHaveCount(0);
    await page.getByRole('button', { name: 'Person options' }).click();
    await expect(page.getByRole('menuitem')).toHaveText(['Birthday…']);
    await page.keyboard.press('Escape');

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
  test("a person's birthday is given from their page and shows under the name", async ({ page }) => {
    const { birthdays } = await fakeEveryone(page);
    await page.goto(`/#people/${FAMILY}/person/1`);
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
  test('Often with leads to the photos of both, from a library and from all libraries', async ({ page }) => {
    await fakeEveryone(page);
    for (const address of [`/#people/${FAMILY}/person/1`, '/#people/all/person/Anna']) {
      await page.goto(address);
      await waitForAppReady(page);
      const chip = page.locator('.people-with-chip', { hasText: 'Ben' });
      await expect(chip).toContainText('2');
      await chip.click();
      await expect(page).toHaveURL(/#libraries/);
      await expect(page.locator('.chip .chip-label')).toHaveText(['Person: Anna', 'Person: Ben']);
    }
  });
});
