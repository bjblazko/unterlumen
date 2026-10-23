import { test, expect } from '@playwright/test';
import { createHash } from 'crypto';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// People with real face recognition (ADR-0051): ONNX Runtime and the models,
// installed by the app itself, on photos of invented people (faces/ABOUT.md)
// served by a server of their own on port 8086 (playwright.config.js, project "faces").
// The tests run in order on one library: the faces found, a person named,
// a face marked by hand, the filter narrowed to the person.
//
// What the tests rely on, as the detector sees these photos: the woman with
// glasses of portraits-12, -22 and -44 is one group, each near enough
// to her other faces to be suggested as her, and her face in party-33,
// turned away in the dark, is not found.

const PHOTOS = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'fixtures', 'faces-photos');
const NAME = 'Mei';

function photoFile(folder, tile) {
  const file = fs.readdirSync(path.join(PHOTOS, folder)).find(f => f.endsWith(`_${tile}.jpg`));
  return path.join(PHOTOS, folder, file);
}

// A library photo's ID is its file's SHA-256.
function photoID(folder, tile) {
  return createHash('sha256').update(fs.readFileSync(photoFile(folder, tile))).digest('hex');
}

function sidecarOf(folder, tile) {
  const file = photoFile(folder, tile).replace(/\.jpg$/, '.xmp');
  return fs.existsSync(file) ? fs.readFileSync(file, 'utf8') : '';
}

// Sidecars from an earlier run would bring its names back.
function removeSidecars(dir = PHOTOS) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) removeSidecars(p);
    else if (e.name.endsWith('.xmp')) fs.rmSync(p);
  }
}

const tileOf = f => f.file.replace(/^.*_/, '').replace(/\.jpg$/, '');

test.describe.serial('Face recognition', () => {
  let lib;

  test.beforeAll(async ({ request }) => {
    test.setTimeout(600_000);
    removeSidecars();
    const status = await (await request.get('/api/faces')).json();
    if (!status.installed) {
      const r = await request.post('/api/faces/install', { timeout: 540_000 });
      expect(r.status(), await r.text()).toBe(204);
    }
    for (const l of await (await request.get('/api/library/')).json()) {
      await request.delete(`/api/library/${l.id}`);
    }
    const made = await request.post('/api/library/', { data: { name: 'Friends', description: '', sourcePath: '.' } });
    expect(made.ok()).toBeTruthy();
    lib = (await made.json()).id;
    await reindexLibrary(request, lib);
    await expect.poll(async () => (await (await request.get(`/api/library/${lib}/people`)).json()).pending,
      { timeout: 120_000, intervals: [1_000] }).toBe(0);
  });

  async function groupWith(request, tile) {
    const overview = await (await request.get(`/api/library/${lib}/people`)).json();
    for (const g of overview.groups) {
      const faces = await (await request.get(`/api/library/${lib}/people/groups/${g.id}`)).json();
      if (faces.some(f => tileOf(f) === tile)) return { ...g, tiles: faces.map(tileOf) };
    }
    return null;
  }

  test('finds the faces and puts one person together across photos', async ({ page, request }) => {
    const mei = await groupWith(request, 'portraits-12');
    expect(mei, 'no group holds the face of portraits-12').not.toBeNull();
    expect(mei.tiles).toEqual(expect.arrayContaining(['portraits-22', 'portraits-44']));
    expect(await groupWith(request, 'party-33'), 'a face was found in party-33').toBeNull();

    await page.goto('/#people');
    await waitForAppReady(page);
    await page.locator(`a[href="#people/name/${lib}"]`).first().click();
    const row = page.locator(`.people-group[data-group="${mei.id}"]`);
    await expect(row).toBeVisible({ timeout: 20_000 });
    await expect(row.locator('.people-group-count')).toContainText(String(mei.size));
  });

  // Two of her faces are left out when the group is named: People then
  // offers them as maybe her, one is taken, the other gets a No that keeps
  // it from being asked about again.
  test('a named person is suggested for faces like hers, and a No is kept', async ({ page, request }) => {
    const mei = await groupWith(request, 'portraits-12');
    const faces = await (await request.get(`/api/library/${lib}/people/groups/${mei.id}`)).json();
    const left = Object.fromEntries(faces.filter(f => ['portraits-22', 'portraits-44'].includes(tileOf(f))).map(f => [tileOf(f), f.id]));
    expect(Object.keys(left).sort()).toEqual(['portraits-22', 'portraits-44']);

    await page.goto(`/#people/name/${lib}/group/${mei.id}`);
    await waitForAppReady(page);
    await page.getByRole('button', { name: 'Select all' }).click();
    for (const id of Object.values(left)) await page.locator(`.face-tile[data-face="${id}"]`).click();
    await page.locator('#people-group-name').fill(NAME);
    await page.locator('.people-submit').click();
    // The two left out stay on the group's page, now asked about as her.
    await expect(page.locator('.face-tile')).toHaveCount(2);
    await expect(page.locator(`.people-accept[data-name="${NAME}"]`)).toBeVisible();
    expect(sidecarOf('portraits', 'portraits-12')).toContain(`<mwg-rs:Name>${NAME}</mwg-rs:Name>`);
    expect(sidecarOf('portraits', 'portraits-44')).not.toContain(NAME);

    // Maybe Mei on her page: the two left out; one of them is her.
    await page.goto(`/#people/person/${NAME}`);
    await waitForAppReady(page);
    const maybe = page.locator('.people-maybe');
    await expect(maybe.locator('.people-section-title')).toHaveText(`Maybe ${NAME}`, { timeout: 20_000 });
    await expect(maybe.locator(`.face-tile[data-face="${left['portraits-44']}"]`)).toBeVisible();
    await maybe.locator(`.face-tile[data-face="${left['portraits-44']}"]`).click();
    await maybe.getByRole('button', { name: `This is ${NAME}` }).click();
    await expect(maybe.locator(`.face-tile[data-face="${left['portraits-44']}"]`)).toHaveCount(0);
    expect(sidecarOf('portraits', 'portraits-44')).toContain(NAME);

    // The other waits with "Is this Mei?"; No is kept, on reload and on her page.
    const row = () => page.locator('.people-group', { has: page.locator(`.people-not[data-person]`) })
      .filter({ has: page.locator(`.people-accept[data-name="${NAME}"]`) });
    await page.goto(`/#people/name/${lib}`);
    await waitForAppReady(page);
    await expect(row()).toHaveCount(1, { timeout: 20_000 });
    await row().locator('.people-not').click();
    await expect(row()).toHaveCount(0);
    await page.reload();
    await waitForAppReady(page);
    await expect(page.locator('.people-group').first()).toBeVisible();
    await expect(row()).toHaveCount(0);
    await page.goto(`/#people/person/${NAME}`);
    await waitForAppReady(page);
    await expect(page.locator('.people-faces .face-tile').first()).toBeVisible();
    await expect(page.locator(`.people-maybe .face-tile[data-face="${left['portraits-22']}"]`)).toHaveCount(0);
  });

  test('a face that was not found is marked by hand and stays after reading the library again', async ({ page, request }) => {
    await page.setViewportSize({ width: 1400, height: 900 });
    await page.goto('/#libraries');
    await waitForAppReady(page);
    await page.locator(`.nav-sub[data-library-id="${lib}"]`).click();
    await page.locator('#lib-pane .folder-tile.dir-item[data-name="party"]').dblclick();
    await page.locator(`#lib-pane .image-item[data-name="${path.basename(photoFile('party', 'party-33'))}"]`).dblclick();
    await expect(page.locator('.viewer')).toBeVisible();

    await page.locator('.viewer .info-face-mark').click({ timeout: 20_000 });
    const area = await page.locator('.face-mark-overlay').boundingBox();
    // Her face, turned towards the man on the right.
    await page.mouse.move(area.x + area.width * 0.29, area.y + area.height * 0.16);
    await page.mouse.down();
    await page.mouse.move(area.x + area.width * 0.46, area.y + area.height * 0.63, { steps: 5 });
    await page.mouse.up();
    await page.locator('.face-mark-input').fill(NAME);
    await page.locator('.face-mark-input').press('Enter');
    await expect(page.locator('.face-mark-overlay')).toHaveCount(0);
    await expect(page.locator('.viewer .info-face-name')).toHaveText([NAME]);
    await expect(page.locator('.viewer .info-face-unname')).toHaveAttribute('title', `Remove ${NAME}'s frame`);
    expect(sidecarOf('party', 'party-33')).toContain(NAME);

    await reindexLibrary(request, lib);
    const r = await (await request.get(`/api/library/${lib}/photo/${photoID('party', 'party-33')}/faces`)).json();
    expect(r.faces.map(f => [f.name, !!f.manual])).toEqual([[NAME, true]]);
  });

  test('the filter of Map, Timeline and Statistics narrows to her photos', async ({ page, request }) => {
    const found = await (await request.get(`/api/library/search?ids=${lib}&person=${NAME}&limit=500`)).json();
    const photos = found.results;
    const all = photos.length;
    expect(all).toBeGreaterThan(5);
    // The party photos have no place.
    const located = photos.filter(p => !/party-/.test(p.filename)).length;

    await page.route('https://tiles.openfreemap.org/**', r => r.fulfill({ json: { version: 8, sources: {}, layers: [] } }));
    await page.goto('/#map');
    await waitForAppReady(page);
    await page.locator('.map-pane .scope-filter-toggle').click();
    await page.locator('.scope-filter-panel:visible').getByRole('combobox', { name: 'Add a person to the filter' }).selectOption(NAME);
    // "8 photos", "3 of 8 photos", "14 photos · 2024-10 – 2025-07": the first number.
    const number = async (sel) => Number((((await page.locator(sel).textContent()) || '').match(/\d[\d\s]*/) || ['0'])[0].replace(/\s/g, ''));
    await expect.poll(() => number('.map-count')).toBe(located);

    // The filter is shared: the Timeline and Statistics count her photos too.
    await page.goto('/#timeline');
    await waitForAppReady(page);
    await expect(page.locator('.timeline-place .scope-filter-chips .lib-filter-chip', { hasText: `Person: ${NAME}` })).toBeVisible();
    await expect.poll(() => number('.timeline-count')).toBe(all);
    await page.goto('/#statistics');
    await waitForAppReady(page);
    await expect.poll(() => number('.stats-count')).toBe(all);
  });
});
