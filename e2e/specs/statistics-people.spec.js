import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';
import { fakePeopleStats } from '../helpers/people-stats.js';

// The People topic of Statistics (ADR-0051). The e2e server has no face
// recognition, so the API is checked for its empty answer and the charts are
// drawn from a made-up one; the 3D views need WebGPU, which headless Chromium
// may not have.

const LIB_NAME = 'E2E Statistics People';

test.describe('Statistics: People', () => {
  let lib;

  test.beforeAll(async ({ request }) => {
    const existing = await (await request.get('/api/library/')).json();
    await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
    const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: 'folder-b' } });
    expect(res.status()).toBe(201);
    lib = await res.json();
    await reindexLibrary(request, lib.id);
  });

  test.afterAll(async ({ request }) => {
    if (lib) await request.delete(`/api/library/${lib.id}`);
  });

  test('the API answers with nobody when no face was found, and the search takes a number of faces', async ({ request }) => {
    const body = await (await request.get(`/api/library/people-stats?ids=${lib.id}`)).json();
    expect(body.libraries).toEqual([lib.id]);
    expect(body.people).toEqual([]);
    expect(body.crowd.buckets.map(b => b.label)).toEqual(['No face', '1 face', '2 faces', '3 to 5 faces', '6 or more']);
    expect(body.faceSpace.face).toEqual([]);
    const search = await (await request.get(`/api/library/search?ids=${lib.id}&faces_min=1`)).json();
    expect(search.total).toBe(0);
  });

  const chart = (page, title) => page.locator('.stats-chart').filter({ has: page.locator('.stats-chart-title', { hasText: new RegExp(`^${title}$`) }) });

  async function openPeople(page, birthdays = {}) {
    const ps = fakePeopleStats(lib.id);
    await page.route('**/api/library/people-stats*', r => r.fulfill({ json: ps }));
    await page.route('**/api/people/birthdays', r => r.fulfill({ json: birthdays }));
    const searches = [];
    await page.route('**/api/library/search*', r => {
      searches.push(new URL(r.request().url()).searchParams);
      return r.fulfill({ json: { total: 0, results: [] } });
    });
    await page.goto(`/#statistics/people?library=${lib.id}`);
    await waitForAppReady(page);
    await expect(page.locator('.stats-title')).toHaveText('People');
    return { ps, searches };
  }

  test('who appears most and who with whom show the photos of one person and of two', async ({ page }) => {
    const { ps, searches } = await openPeople(page);
    const bars = chart(page, 'Who appears most').locator('rect.stats-pickable');
    await expect(bars).toHaveCount(10);
    await bars.first().click();
    await expect.poll(() => searches.at(-1)?.getAll('person')).toEqual([ps.people[0].name]);

    const cells = chart(page, 'Who with whom').locator('rect.stats-pickable');
    await cells.first().click();
    await expect.poll(() => searches.at(-1)?.getAll('person').length).toBe(2);
  });

  test('alone or in a group looks for photos by their number of faces', async ({ page }) => {
    const { searches } = await openPeople(page);
    await chart(page, 'Alone or in a group').getByRole('button', { name: /^2 faces/ }).click();
    await expect.poll(() => searches.at(-1)?.get('faces_min')).toBe('2');
    expect(searches.at(-1).get('faces_max')).toBe('2');
  });

  test('the Face space, the Circle of people and Life lines are drawn, or say that WebGPU is missing', async ({ page }) => {
    await openPeople(page);
    const hasGPU = await page.evaluate(async () => !!(navigator.gpu && await navigator.gpu.requestAdapter().catch(() => null)));
    for (const title of ['Face space', 'Circle of people', 'Life lines']) {
      await expect(page.locator('.stats-stages')).toContainText(title);
      if (hasGPU) await expect(chart(page, title).locator('.point-stage-canvas')).toBeVisible({ timeout: 15_000 });
      else await expect(chart(page, title).locator('.point-stage-nogpu')).toContainText('needs WebGPU', { timeout: 15_000 });
    }
  });
  test('photos by age counts the people with a birthday, and asks for one without', async ({ page }) => {
    await openPeople(page);
    await expect(chart(page, 'Photos by age')).toContainText('Nobody here has a birthday yet');
    const ps = fakePeopleStats(lib.id);
    await page.unrouteAll({ behavior: 'ignoreErrors' });
    await openPeople(page, { [ps.people[0].name.toLowerCase()]: '2015-03-01' });
    const ages = chart(page, 'Photos by age');
    await expect(ages.locator('svg path[stroke-width="2"]')).toHaveCount(1);
    await expect(ages).toContainText(ps.people[0].name);
    // Its axis says what the numbers are: "8 y", not "8".
    await expect(ages.locator('.tick text[transform]').first()).toHaveText(/^\d+\u00a0y$/);
  });

  // Compare: nobody chosen is the people with the most photos; chosen, the
  // charts show those, in that order. The choice outlives a reload.
  test('compare shows the people chosen, and only them', async ({ page }) => {
    const { ps } = await openPeople(page);
    const bars = chart(page, 'Who appears most').locator('rect.stats-pickable');
    await expect(bars).toHaveCount(10);
    const add = page.getByRole('combobox', { name: 'Add a person to compare' });
    await expect(add.locator('option').first()).toHaveText('The most photos');
    const [third, first] = [ps.people[2].name, ps.people[0].name];
    await add.selectOption(third);
    await page.getByRole('combobox', { name: 'Add a person to compare' }).selectOption(first);
    await expect(page.locator('.stats-compare-chip')).toHaveText([`${third}×`, `${first}×`]);
    await expect(bars).toHaveCount(2);
    await expect(chart(page, 'Who appears most')).toContainText(third);
    await expect(chart(page, 'People over time')).toContainText(third);

    await page.reload();
    await waitForAppReady(page);
    await expect(page.locator('.stats-compare-chip')).toHaveCount(2);
    await page.locator('.stats-compare-chip').first().click();
    await page.locator('.stats-compare-chip').first().click();
    await expect(chart(page, 'Who appears most').locator('rect.stats-pickable')).toHaveCount(10);
  });
});
