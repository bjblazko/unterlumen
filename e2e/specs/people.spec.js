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
  await page.route(`**/api/library/${LIB}/people/groups/*`, r => r.fulfill({ json: [
    { id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }, { id: 12, photoId: 'p2', x: 0.1, y: 0.1, w: 0.2, h: 0.3 }] }));
  await page.route(`**/api/library/${LIB}/people/1`, r => r.fulfill({ json: [{ id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }] }));
  await page.route(`**/api/library/${LIB}/thumb/*`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
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

  test('resting on a face shows it framed in its whole photo', async ({ page }) => {
    const state = { named: null, overview: () => ({ people: [], groups: [{ id: 0, size: 2, faces: [11, 12] }], groupCount: 1, pending: 0, finding: false }) };
    await fakePeople(page, state);
    await page.goto(`/#people/${LIB}/group/0`);
    await waitForAppReady(page);
    await page.locator('.face-tile').first().hover();
    const peek = page.locator('.face-peek');
    await expect(peek).toBeVisible();
    await expect(peek.locator('img')).toHaveAttribute('src', new RegExp(`/api/library/${LIB}/thumb/p1`));
    await expect(peek.locator('.face-peek-frame')).toHaveAttribute('style', /left:60\.00%;top:20\.00%;width:10\.00%;height:15\.00%/);
    await page.mouse.move(0, 0);
    await expect(peek).toHaveCount(0);
    // The keyboard gets the same view on the face it is on.
    await page.locator('.face-tile').first().focus();
    await page.keyboard.press('Tab');
    await expect(peek.locator('img')).toHaveAttribute('src', new RegExp(`/thumb/p2`));
  });

  test("a person's Show photos opens the library filter at that person", async ({ page }) => {
    const state = { named: null, overview: () => ({ people: [{ id: 1, name: 'Anna', faces: 1, photos: 1, cover: 11 }], groups: [], groupCount: 0, pending: 0, finding: false }) };
    await fakePeople(page, state);
    await page.goto(`/#people/${LIB}/person/1`);
    await waitForAppReady(page);
    await page.locator('.people-show').click();
    await expect(page).toHaveURL(/#libraries/);
    await expect(page.locator('.chip .chip-label')).toHaveText('Person: Anna');
  });

  test("a photo's info panel names a face and takes a name away", async ({ page }) => {
    const state = { named: null, overview: () => ({ people: [], groups: [{ id: 0, size: 2, faces: [11, 12] }], groupCount: 1, pending: 0, finding: false }) };
    await fakePeople(page, state);
    let unnamed = null;
    let faces = [{ id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }, { id: 13, photoId: 'p1', x: 0.1, y: 0.2, w: 0.1, h: 0.15, name: 'Ben' }];
    await page.route(`**/api/library/${LIB}/photo/p1/faces`, r => r.fulfill({ json: { faces, names: ['Anna', 'Ben'] } }));
    await page.route(`**/api/library/${LIB}/photo/p1/info`, r => r.fulfill({ json: { name: 'p1.jpg', size: 1000, modTime: '2026-01-01T00:00:00Z' } }));
    await page.route(`**/api/library/${LIB}/photo/p1`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
    await page.route(`**/api/library/${LIB}/faces/unname`, async r => { unnamed = r.request().postDataJSON(); await r.fulfill({ json: { skipped: 0 } }); });
    await page.goto(`/#people/${LIB}/group/0`);
    await waitForAppReady(page);
    await page.locator('.face-tile').first().dblclick();
    const people = page.locator('.info-people');
    await expect(people.locator('.info-face')).toHaveCount(2);
    await expect(people.locator('.info-face-name')).toHaveText('Ben');
    await people.locator('.info-face-input').fill('Anna');
    await people.locator('.info-face-input').press('Enter');
    await expect.poll(() => state.named).toEqual({ faces: [11], name: 'Anna' });
    await people.locator('.info-face-unname').click();
    await expect.poll(() => unnamed).toEqual({ faces: [13] });
  });

  test("typing a face's name in the viewer's info panel stays typing", async ({ page }) => {
    const state = { named: null, overview: () => ({ people: [], groups: [{ id: 0, size: 2, faces: [11, 12] }], groupCount: 1, pending: 0, finding: false }) };
    await fakePeople(page, state);
    await page.route(`**/api/library/${LIB}/photo/p1/faces`, r => r.fulfill({ json: { faces: [{ id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }], names: [] } }));
    await page.route(`**/api/library/${LIB}/photo/p1/info`, r => r.fulfill({ json: { name: 'p1.jpg', size: 1000, modTime: '2026-01-01T00:00:00Z' } }));
    await page.route(`**/api/library/${LIB}/photo/p1`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
    await page.goto(`/#people/${LIB}/group/0`);
    await waitForAppReady(page);
    await page.locator('.face-tile').first().dblclick();
    const input = page.locator('.info-people .info-face-input');
    await input.click();
    // I opens and closes the panel, F the film strip, H the interface,
    // Backspace marks the photo — none of them while typing a name.
    await input.pressSequentially('Iris Hoff');
    await input.press('Backspace');
    await expect(input).toHaveValue('Iris Hof');
    await expect(input).toBeVisible();
    await expect(page.locator('.viewer')).toBeVisible();
    await input.press('Escape');
    await expect(input).not.toBeFocused();
    await expect(page.locator('.viewer')).toBeVisible();
  });

  test("a person's name can be taken away from all their faces", async ({ page }) => {
    let forgotten = false;
    const state = { named: null, overview: () => ({ people: forgotten ? [] : [{ id: 1, name: 'Anna', faces: 1, photos: 1, cover: 11 }], groups: [], groupCount: 0, pending: 0, finding: false }) };
    await fakePeople(page, state);
    await page.route(`**/api/library/${LIB}/people/1`, r => {
      if (r.request().method() !== 'DELETE') return r.fulfill({ json: [{ id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }] });
      forgotten = true;
      return r.fulfill({ json: { skipped: 0 } });
    });
    await page.goto(`/#people/${LIB}/person/1`);
    await waitForAppReady(page);
    await page.getByRole('button', { name: 'Person options' }).click();
    await page.getByText('Take the name away…').click();
    await page.locator('.dialog-scrim button', { hasText: 'Take the name away' }).click();
    await expect(page).toHaveURL(new RegExp(`#people/${LIB}$`));
    expect(forgotten).toBe(true);
  });
  test("one face switched on becomes the person's cover", async ({ page }) => {
    const state = { cover: null, overview: () => ({ people: [{ id: 1, name: 'Anna', faces: 2, photos: 2, cover: state.cover ?? 11 }], groups: [], groupCount: 0, pending: 0, finding: false }) };
    await fakePeople(page, state);
    await page.route(`**/api/library/${LIB}/people/1`, r => r.fulfill({ json: [
      { id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }, { id: 12, photoId: 'p2', x: 0.1, y: 0.1, w: 0.2, h: 0.3 }] }));
    await page.route(`**/api/library/${LIB}/people/1/cover`, async r => {
      state.cover = r.request().postDataJSON().face;
      await r.fulfill({ status: 204 });
    });
    await page.goto(`/#people/${LIB}/person/1`);
    await waitForAppReady(page);
    const tiles = page.locator('.face-tile');
    await expect(tiles.nth(0).locator('.face-tile-cover')).toHaveText('Cover');
    await tiles.nth(0).click();
    await expect(page.locator('.people-cover')).toBeHidden();
    await tiles.nth(0).click();
    await tiles.nth(1).click();
    await page.locator('.people-cover').click();
    await expect(tiles.nth(1).locator('.face-tile-cover')).toHaveText('Cover');
    expect(state.cover).toBe(12);
    await expect(page.locator('.face-tile-cover')).toHaveCount(1);
  });

  // The page is drawn anew while the pointer rests on a face: its peek
  // must not stay behind, stuck to a tile that is gone.
  test('a face peek goes when the page is drawn anew under it', async ({ page }) => {
    const state = { cover: null, overview: () => ({ people: [{ id: 1, name: 'Anna', faces: 2, photos: 2, cover: state.cover ?? 11 }], groups: [], groupCount: 0, pending: 0, finding: false }) };
    await fakePeople(page, state);
    await page.route(`**/api/library/${LIB}/people/1`, r => r.fulfill({ json: [
      { id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }, { id: 12, photoId: 'p2', x: 0.1, y: 0.1, w: 0.2, h: 0.3 }] }));
    await page.route(`**/api/library/${LIB}/people/1/cover`, async r => {
      state.cover = r.request().postDataJSON().face;
      await r.fulfill({ status: 204 });
    });
    await page.goto(`/#people/${LIB}/person/1`);
    await waitForAppReady(page);
    await page.locator('.face-tile').nth(1).click();
    // The pointer rests on the other face, which is not clicked: its peek shows.
    await page.locator('.face-tile').nth(0).hover();
    await expect(page.locator('.face-peek')).toBeVisible();
    // The page is drawn anew without the pointer moving.
    await page.locator('.people-cover').evaluate(b => b.click());
    await expect(page.locator('.face-tile').nth(1).locator('.face-tile-cover')).toBeVisible();
    await expect(page.locator('.face-peek')).toHaveCount(0);
  });
});

