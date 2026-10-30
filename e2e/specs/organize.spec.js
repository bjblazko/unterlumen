import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { GPS_PATH, NO_GPS_PATH, A1_GPS_IMAGE, A1_NO_GPS_IMAGE } from '../helpers/fixtures.js';
import { reindexLibrary } from '../helpers/library.js';

// Organize is one source folder and a list of targets (ADR-0032). Everything
// here happens in throwaway folders, so the fixtures other specs rely on are
// never moved out from under them.

const SRC = 'e2e-organize-src';
const DST = 'e2e-organize-dst';
const SUB = `${SRC}/a-subfolder`;

// The targets are remembered client-side; seeding the store is how a test
// gets a target without driving the folder picker.
async function openOrganize(page, targets = [{ path: DST }]) {
    await page.addInitScript(([key, value]) => {
        window.localStorage.setItem(key, value);
    }, ['organize.targets', JSON.stringify(targets)]);
    await page.goto('/#organize');
    await waitForAppReady(page);
    await page.waitForSelector('.organize-source .browse-header', { timeout: 10_000 });
}

// The source folder is reached the way a person reaches it: by clicking it.
async function loadSource(page) {
    await page.locator(`.organize-source .folder-tile[data-name="${SRC}"]`).dblclick();
    await page.waitForSelector('.organize-source [data-type="image"]', { timeout: 10_000 });
}

test.describe('Organize — one source, many targets', () => {
    test.beforeEach(async ({ request }) => {
        for (const folder of [SRC, DST]) {
            await request.post('/api/delete', { data: { files: [folder] } }).catch(() => {});
            expect((await request.post('/api/mkdir', { data: { path: folder } })).status()).toBe(200);
        }
        expect((await request.post('/api/mkdir', { data: { path: SUB } })).status()).toBe(200);
        const copy = await request.post('/api/copy', {
            data: { files: [GPS_PATH, NO_GPS_PATH], destination: SRC },
        });
        expect(copy.status()).toBe(200);
    });

    test.afterAll(async ({ request }) => {
        for (const folder of [SRC, DST]) {
            await request.post('/api/delete', { data: { files: [folder] } }).catch(() => {});
        }
    });

    test('the screen shows one source pane and the target list', async ({ page }) => {
        await openOrganize(page);
        await expect(page.locator('.organize-source .browse-header')).toHaveCount(1);
        await expect(page.locator('.org-target')).toHaveCount(2); // the folder target and "Mark for deletion"
        await expect(page.locator('.org-target--mark')).toContainText('Mark for deletion');
        // The dual pane is gone with its middle column.
        await expect(page.locator('#left-pane, #right-pane, #cmd-copy')).toHaveCount(0);
    });

    test('a number key moves the selection to that target, and U brings it back', async ({ page, request }) => {
        await openOrganize(page);
        await loadSource(page);

        const photos = page.locator('.organize-source [data-type="image"]');
        await expect(photos).toHaveCount(2);
        await photos.first().click();

        await page.keyboard.press('1');
        await expect(page.locator('#org-result')).toContainText('moved to', { timeout: 10_000 });
        await expect(photos).toHaveCount(1);

        const afterMove = await (await request.get(`/api/browse?path=${DST}`)).json();
        expect(afterMove.entries.filter((e) => e.type === 'image')).toHaveLength(1);

        await page.keyboard.press('u');
        await expect(page.locator('#org-result')).toContainText('moved back', { timeout: 10_000 });
        await expect(photos).toHaveCount(2);

        const afterUndo = await (await request.get(`/api/browse?path=${DST}`)).json();
        expect(afterUndo.entries.filter((e) => e.type === 'image')).toHaveLength(0);
    });

    // A second press while files are on their way, or a held key, sent the
    // same photos again: one run moved them, the other found them gone.
    test('a second press while a move runs sends nothing more', async ({ page, request }) => {
        await openOrganize(page);
        await loadSource(page);
        const photos = page.locator('.organize-source [data-type="image"]');
        await photos.first().click();
        await photos.nth(1).click({ modifiers: ['ControlOrMeta'] });

        // The server answers slowly, as a network drive does.
        await page.route('**/api/move', async (route) => {
            await new Promise((r) => setTimeout(r, 1200));
            await route.continue();
        });
        await page.keyboard.press('1');
        await page.keyboard.press('1');
        await page.keyboard.down('1'); // a held key repeats
        await page.keyboard.up('1');
        await expect(page.locator('#org-result')).toContainText('2 items moved to', { timeout: 10_000 });
        await expect(page.locator('#org-result')).not.toContainText('did not');
        await expect(page.locator('#org-result .activity-errors')).toHaveCount(0);
        const moved = await (await request.get(`/api/browse?path=${DST}`)).json();
        expect(moved.entries.filter((e) => e.type === 'image')).toHaveLength(2);
    });

    // More than a few files move in the progress dialog that location and
    // deleting use: it counts them as they go and reports at the end.
    test('a move of many photos runs in the progress dialog', async ({ page, request }) => {
        const folder = await (await request.get('/api/browse?path=folder-b')).json();
        const more = folder.entries.filter((e) => e.type === 'image').map((e) => `folder-b/${e.name}`)
            .filter((p) => !p.includes('DSCF3500') && !p.includes('IMG_9637') && p !== GPS_PATH && p !== NO_GPS_PATH)
            .slice(0, 10);
        expect((await request.post('/api/copy', { data: { files: more, destination: SRC } })).status()).toBe(200);
        await openOrganize(page);
        await loadSource(page);
        await page.locator('.organize-source').getByRole('button', { name: 'Select all' }).click();

        await page.route('**/api/move', async (route) => {
            await new Promise((r) => setTimeout(r, 150));
            await route.continue();
        });
        await page.keyboard.press('1');
        const dialog = page.locator('.progress-dialog');
        await expect(dialog).toContainText(`Moving to ${DST}`);
        await expect(dialog).toContainText(/\d+ of 12/, { timeout: 10_000 });
        await expect(dialog).toContainText('Moved: 12 of 12 files.', { timeout: 15_000 });
        await page.keyboard.press('1'); // the dialog holds the keys: nothing more is sent
        await dialog.getByRole('button', { name: 'Close' }).click();
        await expect(page.locator('#org-result')).toContainText('12 items moved to');
        const moved = await (await request.get(`/api/browse?path=${DST}`)).json();
        expect(moved.entries.filter((e) => e.type === 'image')).toHaveLength(12);
    });

    // Enter opens, as everywhere else: it never moves a file.
    test('Enter with photos selected moves nothing', async ({ page, request }) => {
        await openOrganize(page);
        await loadSource(page);
        const photos = page.locator('.organize-source [data-type="image"]');
        await photos.first().click();
        await page.keyboard.press('Enter');
        await page.keyboard.press('Escape');
        const dst = await (await request.get(`/api/browse?path=${DST}`)).json();
        expect(dst.entries.filter((e) => e.type === 'image')).toHaveLength(0);
        await expect(page.locator('.org-keys')).not.toContainText('Enter');
    });

    test('exactly one target is current, and clicking another makes it current', async ({ page }) => {
        await openOrganize(page, [{ path: DST }, { path: `${SRC}/a-subfolder` }]);
        await loadSource(page);

        await expect(page.locator('.org-target[aria-current="true"]')).toHaveCount(1);
        await page.locator('.org-target').nth(1).click();
        await expect(page.locator('.org-target').nth(1)).toHaveAttribute('aria-current', 'true');
        await expect(page.locator('.org-target[aria-current="true"]')).toHaveCount(1);
    });

    // The number keys are the place shortcuts everywhere else; in Organize they
    // only take over while something is selected.
    test('with nothing selected, a number key still switches places', async ({ page }) => {
        await openOrganize(page);
        await loadSource(page);
        await page.keyboard.press('1');
        await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
    });

    test('a folder can go to Mark for deletion', async ({ page }) => {
        await openOrganize(page);
        await loadSource(page);

        const chip = page.locator('.organize-source .folder-tile[data-name="a-subfolder"]');
        await chip.click({ modifiers: ['ControlOrMeta'] });
        // "Mark for deletion" is always key 0, however many targets there are.
        await page.keyboard.press('0');

        await expect(page.locator('#wastebin-count')).toHaveText('1', { timeout: 5_000 });
        await expect(page.locator('.organize-source .folder-tile[data-name="a-subfolder"]'))
            .toHaveClass(/marked-for-deletion/);

        await page.locator('#mode-wastebin').click();
        await expect(page.locator('.wastebin-dir')).toHaveCount(1);
        await expect(page.locator('#wb-delete')).toContainText('Delete 1 permanently');
    });

    test('Mark for deletion stands first and keeps key 0 when targets are added', async ({ page }) => {
        await openOrganize(page, [{ path: DST }, { path: SUB }]);
        await loadSource(page);
        const first = page.locator('.org-target').first();
        await expect(first).toHaveClass(/org-target--mark/);
        await expect(first.locator('.key')).toHaveText('0');
        await expect(page.locator('.org-target').nth(1).locator('.key')).toHaveText('1');
        await expect(page.locator('.org-target').nth(2).locator('.key')).toHaveText('2');
    });

    test('a photo dragged onto a target moves there', async ({ page, request }) => {
        await openOrganize(page);
        await loadSource(page);
        const photo = page.locator('.organize-source [data-type="image"] img').first();
        await photo.dragTo(page.locator('.org-target[data-target="0"]'));
        await expect(page.locator('#org-result')).toContainText('moved to', { timeout: 10_000 });
        const dst = await (await request.get(`/api/browse?path=${DST}`)).json();
        expect(dst.entries.filter((e) => e.type === 'image')).toHaveLength(1);
    });

    test('Add target opens the folder picker on the libraries, and Select waits for one', async ({ page }) => {
        await openOrganize(page);
        await expect(page.locator('.org-target-actions .btn')).toHaveCount(1);
        await page.locator('[data-org="add"]').click();
        await expect(page.locator('.fp-dialog [data-source="libs"]')).toHaveAttribute('aria-pressed', 'true');
        await expect(page.locator('#fp-select')).toBeDisabled();
        await page.locator('.fp-dialog [data-source="fs"]').click();
        await expect(page.locator('#fp-select')).toBeEnabled();
        await page.locator('#fp-cancel').click();
    });

    test('photos cannot be sent to a folder target while a folder is selected', async ({ page }) => {
        await openOrganize(page);
        await loadSource(page);

        await page.locator('.organize-source .folder-tile[data-name="a-subfolder"]')
            .click({ modifiers: ['ControlOrMeta'] });
        await page.keyboard.press('1');
        await expect(page.locator('#org-result')).toContainText('Mark for deletion');
    });
});

// ── "Show in Organize" from a library ───────────────────────────────────────
//
// Organize sorts one folder, so the selection has to name one. Photos in a
// library are all in the folder that is open, so that folder is the answer and
// they arrive selected. The button used to do nothing at all for a selection
// of photos, and never said why, because the reason was handed to the
// selection bar under the wrong key.

test.describe('Show in Organize — from a library', () => {
    const LIB = 'E2E Organize Library';
    const SUB_B = `${SRC}/b-subfolder`;
    let libID;

    test.beforeEach(async ({ request }) => {
        await request.post('/api/delete', { data: { files: [SRC] } }).catch(() => {});
        expect((await request.post('/api/mkdir', { data: { path: SRC } })).status()).toBe(200);
        for (const sub of [SUB, SUB_B]) {
            expect((await request.post('/api/mkdir', { data: { path: sub } })).status()).toBe(200);
        }
        expect((await request.post('/api/copy', {
            data: { files: [GPS_PATH, NO_GPS_PATH], destination: SRC },
        })).status()).toBe(200);
        // A library only knows folders that hold indexed photos, so an empty
        // subfolder would not appear in it at all — and it indexes by content
        // hash, so a copy of a photo the root already has is the same photo
        // and would leave the subfolder empty again.
        for (const [file, sub] of [[`folder-a/a1/${A1_GPS_IMAGE}`, SUB], [`folder-a/a1/${A1_NO_GPS_IMAGE}`, SUB_B]]) {
            expect((await request.post('/api/copy', {
                data: { files: [file], destination: sub },
            })).status()).toBe(200);
        }

        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === LIB).map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', {
            data: { name: LIB, description: '', sourcePath: SRC },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
        await reindexLibrary(request, libID);
    });

    test.afterEach(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.post('/api/delete', { data: { files: [SRC] } }).catch(() => {});
    });

    async function openLibrary(page) {
        await page.addInitScript(([key, value]) => {
            window.localStorage.setItem(key, value);
        }, ['organize.targets', JSON.stringify([{ path: DST }])]);
        await page.goto('/#libraries');
        await waitForAppReady(page);
        await page.locator('.library-card', { hasText: LIB }).click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });
    }

    test('selected photos open Organize on their folder, already selected', async ({ page }) => {
        await openLibrary(page);
        await page.waitForSelector('#lib-pane [data-type="image"]', { timeout: 20_000 });

        const images = page.locator('#lib-pane [data-type="image"]');
        await expect(images).toHaveCount(2);
        await images.nth(0).click();
        await images.nth(1).click({ modifiers: ['ControlOrMeta'] });

        const organize = page.locator('.selection-bar [data-action="organize"]');
        await expect(organize).toBeEnabled();
        await organize.click();

        await expect(page.locator('#org-path')).toHaveText(SRC, { timeout: 10_000 });
        await expect(page.locator('.organize-source .selected')).toHaveCount(2);
    });

    test('one folder opens Organize on that folder', async ({ page }) => {
        await openLibrary(page);
        await page.waitForSelector('#lib-pane .folder-tile', { timeout: 20_000 });

        await page.locator('#lib-pane .folder-tile[data-name="a-subfolder"]')
            .click({ modifiers: ['ControlOrMeta'] });
        await page.locator('.selection-bar [data-action="organize"]').click();

        await expect(page.locator('#org-path')).toHaveText(SUB, { timeout: 10_000 });
    });

    test('two folders cannot name one source, and the button says so', async ({ page }) => {
        await openLibrary(page);
        await page.waitForSelector('#lib-pane .folder-tile', { timeout: 20_000 });

        for (const name of ['a-subfolder', 'b-subfolder']) {
            await page.locator(`#lib-pane .folder-tile[data-name="${name}"]`)
                .click({ modifiers: ['ControlOrMeta'] });
        }

        const organize = page.locator('.selection-bar [data-action="organize"]');
        await expect(organize).toBeDisabled();
        await expect(organize).toHaveAttribute('title', /one folder/);
    });
});
