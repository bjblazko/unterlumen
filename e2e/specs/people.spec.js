import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// People: faces found in a library, grouped, named by the owner (ADR-0051,
// feature doc 2026-10-18-people-faces). The e2e server has no ONNX Runtime,
// so the first test sees the place as it is before the download; the others
// answer the People API themselves to walk through naming.

const LIB = 'lib-1';
const tinyJPEG = Buffer.from(
  '/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=',
  'base64');

async function fakePeople(page, state) {
  await page.route('**/api/faces', r => r.fulfill({ json: { installed: true, plan: { supported: true, bytes: 75_000_000 } } }));
  await page.route('**/api/library/', r => r.request().method() === 'GET'
    ? r.fulfill({ json: [{ id: LIB, name: 'Family', sourcePath: '/photos' }] }) : r.continue());
  await page.route(`**/api/library/${LIB}/face/*`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route(`**/api/library/${LIB}/people`, r => r.fulfill({ json: state.overview() }));
  await page.route(`**/api/library/${LIB}/people/groups/*`, r => r.fulfill({ json: [{ id: 11, photoId: 'p1' }, { id: 12, photoId: 'p2' }] }));
  await page.route(`**/api/library/${LIB}/faces/name`, async r => {
    state.named = r.request().postDataJSON();
    await r.fulfill({ json: { skipped: 0 } });
  });
}

test.describe('People', () => {
  test('the sidebar entry and the 0 key open People, which offers the download', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    await page.keyboard.press('0');
    await expect(page).toHaveURL(/#people/);
    await expect(page.locator('#mode-people')).toHaveAttribute('aria-current', 'page');
    await expect(page.locator('.people-title')).toHaveText('People');
    await expect(page.locator('.people-install')).toHaveText('Download and turn on');
    await expect(page.locator('.people-note')).toContainText('no photo and no face leaves this computer');
  });

  test('a group is named from the overview and becomes a person', async ({ page }) => {
    const state = {
      named: null,
      overview() {
        return this.named
          ? { people: [{ id: 1, name: this.named.name, faces: 2, photos: 2, cover: 11 }], groups: [], groupCount: 0, pending: 0, finding: false }
          : { people: [], groups: [{ id: 0, size: 2, faces: [11, 12] }], groupCount: 1, pending: 0, finding: false };
      },
    };
    await fakePeople(page, state);
    await page.goto('/#people');
    await waitForAppReady(page);
    const group = page.locator('.people-group');
    await expect(group).toHaveCount(1);
    await expect(group.locator('.people-group-count')).toHaveText('2 faces');
    await group.locator('.people-input').fill('Anna');
    await group.locator('button[type=submit]').click();
    await expect(page.locator('.people-card-name')).toHaveText('Anna');
    expect(state.named).toEqual({ faces: [11, 12], name: 'Anna' });
    await expect(page.locator('.people-empty').last()).toHaveText('No faces are waiting for a name.');
  });

  test('on a group page a face switched off is left out of the name', async ({ page }) => {
    const state = {
      named: null,
      overview: () => ({ people: [], groups: [{ id: 0, size: 2, faces: [11, 12] }], groupCount: 1, pending: 0, finding: false }),
    };
    await fakePeople(page, state);
    await page.goto(`/#people/${LIB}/group/0`);
    await waitForAppReady(page);
    const tiles = page.locator('.face-tile');
    await expect(tiles).toHaveCount(2);
    await expect(page.locator('.people-submit')).toHaveText('Name 2 faces');
    await tiles.nth(1).click();
    await expect(tiles.nth(1)).toHaveAttribute('aria-pressed', 'false');
    await expect(page.locator('.people-submit')).toHaveText('Name 1 face');
    await page.locator('#people-group-name').fill('Ben');
    await page.locator('.people-submit').click();
    await expect(page).toHaveURL(new RegExp(`#people/${LIB}$`));
    expect(state.named).toEqual({ faces: [11], name: 'Ben' });
  });
});
