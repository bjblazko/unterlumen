import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// People: faces found in a library, grouped, named by the owner (ADR-0051,
// feature docs 2026-10-18-people-faces and 2026-10-20-people-one-place).
// The e2e server has no ONNX Runtime, so the first test sees the place as it
// is before the download; the others answer the People API themselves to
// walk through naming and managing.

const LIB = 'lib-1';
const tinyJPEG = Buffer.from(
  '/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=',
  'base64');
const FACE_11 = { id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15, file: 'p1.jpg' };
const FACE_12 = { id: 12, photoId: 'p2', x: 0.1, y: 0.1, w: 0.2, h: 0.3, file: 'p2.jpg' };

// state.overview() is the library's People; state.anna, when set, the faces
// of Anna (person 1) and her cover.
async function fakePeople(page, state) {
  const people = () => state.overview().people;
  await page.route('**/api/faces', r => r.fulfill({ json: { installed: true, plan: { supported: true, bytes: 75_000_000 } } }));
  await page.route('**/api/library/', r => r.request().method() === 'GET'
    ? r.fulfill({ json: [{ id: LIB, name: 'Family', sourcePath: '/photos' }] }) : r.continue());
  await page.route(`**/api/library/${LIB}/face/*`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route(`**/api/library/${LIB}/thumb/*`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
  await page.route(`**/api/library/${LIB}/people`, r => r.fulfill({ json: state.overview() }));
  await page.route(`**/api/library/${LIB}/people/groups/*`, r => r.fulfill({ json: [FACE_11, FACE_12] }));
  await page.route('**/api/library/person-names*', r => r.fulfill({ json: ['Anna', 'Ben', 'Cleo from elsewhere'] }));
  await page.route('**/api/people/to-name', r => r.fulfill({ json: [
    { library: LIB, libraryName: 'Family', groups: state.overview().groupCount, pending: 0, finding: false }] }));
  await page.route('**/api/people/birthdays', r => r.fulfill({ json: {} }));
  await page.route('**/api/people', r => r.fulfill({ json: people().map(p => ({
    name: p.name, faces: p.faces, photos: p.photos,
    libraries: [{ library: LIB, libraryName: 'Family', person: p.id, faces: p.faces, photos: p.photos, cover: p.cover }],
    cover: { library: LIB, face: p.cover } })) }));
  await page.route('**/api/people/someone?*', r => {
    const p = people().find(x => x.name === 'Anna');
    return r.fulfill({ json: p ? {
      name: 'Anna',
      in: [{ library: LIB, libraryName: 'Family', person: 1, faces: p.faces, photos: p.photos, cover: p.cover }],
      cover: { library: LIB, face: p.cover },
      faces: (state.anna || [FACE_11]).map(f => ({ ...f, library: LIB })),
      notIn: [], with: [],
    } : { name: 'Anna', in: [], faces: [], notIn: [], with: [] } });
  });
  await page.route('**/api/people/with?*', r => r.fulfill({ json: [] }));
  await page.route(`**/api/library/${LIB}/faces/name`, async r => {
    state.named = r.request().postDataJSON();
    await r.fulfill({ json: { skipped: 0 } });
  });
}

const waiting = (extra = {}) => ({ people: [], groups: [{ id: 0, size: 2, faces: [11, 12], ...extra }], groupCount: 1, pending: 0, finding: false });
const annaNamed = (cover = 11, faces = 1) => ({ people: [{ id: 1, name: 'Anna', faces, photos: faces, cover }], groups: [], groupCount: 0, pending: 0, finding: false });

test.describe('People', () => {
  test('the sidebar entry and the 0 key open People, which offers the download', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    await page.keyboard.press('0');
    await expect(page).toHaveURL(/#people/);
    await expect(page.locator('#mode-people')).toHaveAttribute('aria-current', 'page');
    await expect(page.locator('.place-title:visible')).toHaveText('People');
    await expect(page.locator('.people-install')).toHaveText('Download and turn on');
    await expect(page.locator('.people-note')).toContainText('no photo and no face leaves this computer');
  });

  test('a group waiting for a name is named and becomes a person', async ({ page }) => {
    const state = { named: null, overview() { return this.named ? annaNamed(11, 2) : waiting(); } };
    await fakePeople(page, state);
    await page.goto('/#people');
    await waitForAppReady(page);
    // Nobody is named yet: People says what it is, and where faces wait.
    await expect(page.locator('.people-lede .place-lede')).toBeVisible();
    await page.locator('.people-to-name-link', { hasText: 'Family' }).click();
    await expect(page).toHaveURL(new RegExp(`#people/name/${LIB}$`));
    await expect(page.locator('.people-page-title')).toHaveText('Waiting for a name in Family');
    const group = page.locator('.people-group');
    await expect(group).toHaveCount(1);
    await expect(group.locator('.people-group-count')).toHaveText('2 faces');
    // Names from every library are offered.
    await expect(page.locator('#people-names option')).toHaveCount(3);
    await group.locator('.people-input').fill('Anna');
    await group.locator('button[type=submit]').click();
    await expect(page.locator('.people-empty')).toHaveText('No faces are waiting for a name here.');
    expect(state.named).toEqual({ faces: [11, 12], name: 'Anna' });
    await page.locator('.people-back').click();
    await expect(page.locator('.people-card-name')).toHaveText('Anna');
    await expect(page.locator('.people-lede .place-lede')).toHaveCount(0);
  });

  test('a suggested name is given with its own button', async ({ page }) => {
    const state = { named: null, overview() { return this.named ? annaNamed(11, 2) : waiting({ suggestion: { id: 1, name: 'Anna' } }); } };
    await fakePeople(page, state);
    await page.goto(`/#people/name/${LIB}`);
    await waitForAppReady(page);
    await page.locator('.people-accept', { hasText: 'Anna' }).click();
    await expect.poll(() => state.named).toEqual({ faces: [11, 12], name: 'Anna' });
    await expect(page.locator('.people-group-error')).toHaveCount(0);
  });

  test('on a group page nothing is selected: all are named, or only those selected', async ({ page }) => {
    const state = { named: null, overview: () => waiting() };
    await fakePeople(page, state);
    await page.goto(`/#people/name/${LIB}/group/0`);
    await waitForAppReady(page);
    const tiles = page.locator('.face-tile');
    await expect(tiles).toHaveCount(2);
    await expect(tiles.nth(0)).toHaveAttribute('aria-pressed', 'false');
    await expect(page.locator('.people-submit')).toHaveText('Name all 2');
    await tiles.nth(1).click();
    await expect(tiles.nth(1)).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('.people-submit')).toHaveText('Name 1');
    await expect(page.locator('.people-hide')).toHaveText('Hide 1');
    await page.locator('#people-group-name').fill('Ben');
    await page.locator('.people-submit').click();
    await expect(page).toHaveURL(new RegExp(`#people/name/${LIB}$`));
    expect(state.named).toEqual({ faces: [12], name: 'Ben' });
  });

  test('older addresses lead to their new place', async ({ page }) => {
    const state = { named: null, overview: () => annaNamed() };
    await fakePeople(page, state);
    await page.goto(`/#people/${LIB}/person/1`);
    await waitForAppReady(page);
    await expect(page).toHaveURL(/#people\/person\/Anna$/);
    await expect(page.locator('.people-page-title')).toHaveText('Anna');
    await page.goto(`/#people/${LIB}`);
    await expect(page.locator('.people-page-title')).toHaveText('Waiting for a name in Family');
    await page.goto('/#people/all');
    await expect(page.locator('.people-card-name')).toHaveText('Anna');
  });

  test('resting on a face shows it framed in its whole photo', async ({ page }) => {
    const state = { named: null, overview: () => waiting() };
    await fakePeople(page, state);
    await page.goto(`/#people/name/${LIB}/group/0`);
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
    const state = { named: null, overview: () => annaNamed() };
    await fakePeople(page, state);
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    await page.locator('.people-show').click();
    await expect(page).toHaveURL(/#libraries/);
    await expect(page.locator('.chip .chip-label')).toHaveText('Person: Anna');
  });

  test("a photo's info panel names a face and takes a name away", async ({ page }) => {
    const state = { named: null, overview: () => waiting() };
    await fakePeople(page, state);
    let unnamed = null;
    const faces = [{ id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }, { id: 13, photoId: 'p1', x: 0.1, y: 0.2, w: 0.1, h: 0.15, name: 'Ben' }];
    await page.route(`**/api/library/${LIB}/photo/p1/faces`, r => r.fulfill({ json: { faces, names: ['Anna', 'Ben'] } }));
    await page.route(`**/api/library/${LIB}/photo/p1/info`, r => r.fulfill({ json: { name: 'p1.jpg', size: 1000, modTime: '2026-01-01T00:00:00Z' } }));
    await page.route(`**/api/library/${LIB}/photo/p1`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
    await page.route(`**/api/library/${LIB}/faces/unname`, async r => { unnamed = r.request().postDataJSON(); await r.fulfill({ json: { skipped: 0 } }); });
    await page.goto(`/#people/name/${LIB}/group/0`);
    await waitForAppReady(page);
    await page.locator('.face-tile').first().dblclick();
    // The viewer names the photo's file, not its ID.
    await expect(page.locator('.viewer-filename')).toHaveText('p1.jpg');
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
    const state = { named: null, overview: () => waiting() };
    await fakePeople(page, state);
    await page.route(`**/api/library/${LIB}/photo/p1/faces`, r => r.fulfill({ json: { faces: [{ id: 11, photoId: 'p1', x: 0.6, y: 0.2, w: 0.1, h: 0.15 }], names: [] } }));
    await page.route(`**/api/library/${LIB}/photo/p1/info`, r => r.fulfill({ json: { name: 'p1.jpg', size: 1000, modTime: '2026-01-01T00:00:00Z' } }));
    await page.route(`**/api/library/${LIB}/photo/p1`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
    await page.goto(`/#people/name/${LIB}/group/0`);
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

  test('a face that was not found is marked on the photo and named', async ({ page }) => {
    const state = { named: null, overview: () => waiting() };
    await fakePeople(page, state);
    let faces = [{ id: 11, photoId: 'p1', x: 0.1, y: 0.2, w: 0.1, h: 0.15, name: 'Anna' }];
    const marks = [];
    await page.route(`**/api/library/${LIB}/photo/p1/faces`, async r => {
      if (r.request().method() !== 'POST') return r.fulfill({ json: { faces, names: ['Anna', 'Ben'] } });
      const box = r.request().postDataJSON();
      marks.push(box);
      if (marks.length === 1) return r.fulfill({ status: 409, body: 'There is a face there already; name it in the list.' });
      faces = [...faces, { id: 14, photoId: 'p1', x: box.x, y: box.y, w: box.w, h: box.h, name: box.name, manual: true }];
      return r.fulfill({ json: { skipped: 0 } });
    });
    await page.route(`**/api/library/${LIB}/photo/p1/info`, r => r.fulfill({ json: { name: 'p1.jpg', size: 1000, modTime: '2026-01-01T00:00:00Z' } }));
    await page.route(`**/api/library/${LIB}/photo/p1`, r => r.fulfill({ body: tinyJPEG, contentType: 'image/jpeg' }));
    await page.goto(`/#people/name/${LIB}/group/0`);
    await waitForAppReady(page);
    await page.locator('.face-tile').first().dblclick();

    // Escape leaves the marking and nothing else.
    await page.locator('.info-face-mark').click();
    await expect(page.locator('.viewer-face-hint')).toBeVisible();
    await expect(page.locator('.face-mark-known-name')).toHaveText('Anna');
    await page.keyboard.press('Escape');
    await expect(page.locator('.face-mark-overlay')).toHaveCount(0);
    await expect(page.locator('.viewer')).toBeVisible();

    await page.locator('.info-face-mark').click();
    const area = await page.locator('.face-mark-overlay').boundingBox();
    await page.mouse.move(area.x + area.width * 0.6, area.y + area.height * 0.2);
    await page.mouse.down();
    await page.mouse.move(area.x + area.width * 0.7, area.y + area.height * 0.35, { steps: 4 });
    await page.mouse.up();
    const input = page.locator('.face-mark-input');
    await expect(input).toBeFocused();
    await input.fill('Ben');
    await input.press('Enter');
    // The server's reason stays beside the frame; the marking goes on.
    await expect(page.locator('.face-mark-error')).toHaveText('There is a face there already; name it in the list.');
    await input.press('Enter');
    await expect(page.locator('.face-mark-overlay')).toHaveCount(0);
    expect(marks[1].name).toBe('Ben');
    expect(marks[1].x).toBeCloseTo(0.6, 1);
    expect(marks[1].h).toBeCloseTo(0.15, 1);
    const people = page.locator('.info-people');
    await expect(people.locator('.info-face-name')).toHaveText(['Anna', 'Ben']);
    await expect(people.locator('.info-face-unname').last()).toHaveAttribute('title', "Remove Ben's frame");
  });

  test("a person's name can be taken away from all their faces", async ({ page }) => {
    let forgotten = false;
    const state = { named: null, overview: () => (forgotten ? { ...annaNamed(), people: [] } : annaNamed()) };
    await fakePeople(page, state);
    await page.route(`**/api/library/${LIB}/people/1`, r => {
      if (r.request().method() !== 'DELETE') return r.continue();
      forgotten = true;
      return r.fulfill({ json: { skipped: 0 } });
    });
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    await page.getByRole('button', { name: 'Person options' }).click();
    await page.getByText('Take the name away…').click();
    await page.locator('.dialog-scrim button', { hasText: 'Take the name away' }).click();
    await expect(page.locator('.people-note')).toContainText('Nobody is named Anna any more');
    expect(forgotten).toBe(true);
  });

  test('selected faces are given to someone else in one step', async ({ page }) => {
    const state = { named: null, anna: [FACE_11, FACE_12], overview: () => annaNamed(11, 2) };
    await fakePeople(page, state);
    await page.goto('/#people/person/Anna');
    await waitForAppReady(page);
    await expect(page.locator('.people-bar')).toBeHidden();
    await page.locator('.face-tile').nth(1).click();
    await expect(page.locator('.people-unname')).toHaveText('Not Anna');
    await page.locator('#people-instead').fill('Ben');
    await page.locator('.people-move').click();
    await expect.poll(() => state.named).toEqual({ faces: [12], name: 'Ben' });
  });

  test("one face switched on becomes the person's cover", async ({ page }) => {
    const state = { cover: null, anna: [FACE_11, FACE_12], overview: () => annaNamed(state.cover ?? 11, 2) };
    await fakePeople(page, state);
    await page.route(`**/api/library/${LIB}/people/1/cover`, async r => {
      state.cover = r.request().postDataJSON().face;
      await r.fulfill({ status: 204 });
    });
    await page.goto('/#people/person/Anna');
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
    const state = { cover: null, anna: [FACE_11, FACE_12], overview: () => annaNamed(state.cover ?? 11, 2) };
    await fakePeople(page, state);
    await page.route(`**/api/library/${LIB}/people/1/cover`, async r => {
      state.cover = r.request().postDataJSON().face;
      await r.fulfill({ status: 204 });
    });
    await page.goto('/#people/person/Anna');
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
