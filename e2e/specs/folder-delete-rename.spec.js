import { test, expect } from '@playwright/test';
import { GPS_PATH, NO_GPS_PATH, navigateToFolder } from '../helpers/fixtures.js';
import { waitForThumbnailsLoaded } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// Folders can be renamed and marked for deletion from the selection bar,
// and a delete that empties a folder asks whether to remove it too. All of
// it happens in a throwaway folder, so other specs keep their fixtures.

const ROOT = 'e2e-folder-ops';

async function exists(request, rel) {
    const res = await request.get(`/api/browse?path=${encodeURIComponent(rel)}`);
    return res.status() === 200;
}

async function makeFolder(request, rel, photos = []) {
    expect((await request.post('/api/mkdir', { data: { path: rel } })).status()).toBe(200);
    if (photos.length) {
        expect((await request.post('/api/copy', { data: { files: photos, destination: rel } })).status()).toBe(200);
    }
}

async function openSub(page, name) {
    await page.locator(`.dir-item[data-name="${name}"]`).dblclick();
    await page.waitForSelector(`.crumb[data-path="${ROOT}/${name}"]`, { timeout: 10_000 });
}

async function openRoot(page) {
    await page.goto('/#folders');
    await page.waitForSelector('.breadcrumb', { timeout: 10_000 });
    await navigateToFolder(page, ROOT);
}

test.describe('Folders: rename, delete, and the folder a delete leaves empty', () => {
    test.beforeEach(async ({ request }) => {
        await request.post('/api/delete', { data: { files: [ROOT] } }).catch(() => {});
        await makeFolder(request, ROOT);
    });

    test.afterAll(async ({ request }) => {
        await request.post('/api/delete', { data: { files: [ROOT] } }).catch(() => {});
    });

    test('a selected folder is renamed from the selection bar', async ({ page, request }) => {
        await makeFolder(request, `${ROOT}/before`, [GPS_PATH]);
        await openRoot(page);

        await page.locator('.dir-item[data-name="before"]').click();
        const rename = page.locator('.selection-bar [data-action="rename"]');
        await expect(rename).toBeEnabled();
        await rename.click();

        const input = page.locator('#folder-rename');
        await expect(input).toHaveValue('before');
        await input.fill('after');
        await input.press('Enter');

        await expect(page.locator('.dir-item[data-name="after"]')).toBeVisible({ timeout: 10_000 });
        await expect(page.locator('.dir-item[data-name="before"]')).toHaveCount(0);
        expect(await exists(request, `${ROOT}/after`)).toBe(true);
    });

    test('a name that is taken is said in the dialog, which stays', async ({ page, request }) => {
        await makeFolder(request, `${ROOT}/one`);
        await makeFolder(request, `${ROOT}/two`);
        await openRoot(page);

        await page.locator('.dir-item[data-name="one"]').click();
        await page.locator('.selection-bar [data-action="rename"]').click();
        await page.locator('#folder-rename').fill('two');
        await page.locator('#folder-rename').press('Enter');
        await expect(page.locator('.dialog-note')).toContainText('already there');
        await expect(page.locator('.dialog')).toBeVisible();
    });

    test('several folders are renamed one at a time', async ({ page, request }) => {
        await makeFolder(request, `${ROOT}/one`);
        await makeFolder(request, `${ROOT}/two`);
        await openRoot(page);

        await page.locator('.dir-item[data-name="one"]').click();
        await page.locator('.dir-item[data-name="two"]').click({ modifiers: [process.platform === 'darwin' ? 'Meta' : 'Control'] });
        await expect(page.locator('.selection-bar [data-action="rename"]')).toBeDisabled();
        await expect(page.locator('.selection-bar [data-action="mark"]')).toBeEnabled();
    });

    test('a selected folder is marked and deleted', async ({ page, request }) => {
        await makeFolder(request, `${ROOT}/gone`, [GPS_PATH]);
        await openRoot(page);

        await page.locator('.dir-item[data-name="gone"]').click();
        await page.locator('.selection-bar [data-action="mark"]').click();
        await expect(page.locator('#wastebin-count')).toHaveText('1');

        await page.locator('#mode-wastebin').click();
        await expect(page.locator('.wastebin-dir')).toHaveCount(1);
        await page.locator('#wb-delete').click();
        await page.locator('#wb-delete-confirm').click();
        await expect(page.locator('.wastebin-empty-box')).toBeVisible();
        expect(await exists(request, `${ROOT}/gone`)).toBe(false);
    });

    test('deleting every photo of a folder asks to remove the folder', async ({ page, request }) => {
        await makeFolder(request, `${ROOT}/trip`, [GPS_PATH, NO_GPS_PATH]);
        await openRoot(page);
        await openSub(page, 'trip');
        await waitForThumbnailsLoaded(page, 2);

        await page.keyboard.press(process.platform === 'darwin' ? 'Meta+a' : 'Control+a');
        await page.keyboard.press('Delete');
        await expect(page.locator('#wastebin-count')).toHaveText('2');

        await page.locator('#mode-wastebin').click();
        await page.locator('#wb-delete').click();
        await page.locator('#wb-delete-confirm').click();

        const question = page.locator('.wastebin-emptied');
        await expect(question).toContainText('Deleting left this folder empty:');
        await expect(question.locator('.wastebin-emptied-path')).toHaveText(`${ROOT}/trip`);
        await page.locator('#wb-remove-folders').click();
        await expect(question).toHaveCount(0);
        expect(await exists(request, `${ROOT}/trip`)).toBe(false);

        // Folders was in the folder; it goes back to the one around it.
        await page.locator('#mode-browse').click();
        await expect(page.locator(`.crumb[data-path="${ROOT}"]`)).toBeVisible();
        await expect(page.locator('.error')).toHaveCount(0);
    });

    test('Keep leaves the empty folder where it is', async ({ page, request }) => {
        await makeFolder(request, `${ROOT}/kept`, [GPS_PATH]);
        await openRoot(page);
        await openSub(page, 'kept');
        await waitForThumbnailsLoaded(page, 1);

        await page.locator('[data-type="image"]').first().click();
        await page.keyboard.press('Delete');
        await page.locator('#mode-wastebin').click();
        await page.locator('#wb-delete').click();
        await page.locator('#wb-delete-confirm').click();

        await page.locator('#wb-keep-folders').click();
        await expect(page.locator('.wastebin-emptied')).toHaveCount(0);
        expect(await exists(request, `${ROOT}/kept`)).toBe(true);
    });

    // In a library a folder's path is relative to the library's folder. It
    // used to reach /api/delete as it was, which names a folder of the same
    // name under the browse root — here a decoy — instead of the one marked.
    test('a library folder that is deleted is the one marked', async ({ page, request }) => {
        const DECOY = 'e2e-folder-ops-decoy';
        await makeFolder(request, `${ROOT}/lib`);
        await makeFolder(request, `${ROOT}/lib/${DECOY}`, [GPS_PATH]);
        await request.post('/api/delete', { data: { files: [DECOY] } }).catch(() => {});
        await makeFolder(request, DECOY);
        const libRes = await request.post('/api/library/', {
            data: { name: 'E2E Folder ops', description: '', sourcePath: `${ROOT}/lib` },
        });
        expect(libRes.status()).toBe(201);
        const libID = (await libRes.json()).id;
        try {
            await reindexLibrary(request, libID);
            await page.goto('/#libraries');
            await page.locator('.library-card', { hasText: 'E2E Folder ops' }).locator('.lib-open').click();
            await page.locator(`#lib-pane .dir-item[data-name="${DECOY}"]`).click();
            await page.locator('.selection-bar:visible [data-action="mark"]').click();

            await page.locator('#mode-wastebin').click();
            await page.locator('#wb-delete').click();
            await page.locator('#wb-delete-confirm').click();
            await expect(page.locator('.wastebin-empty-box')).toBeVisible();
            expect(await exists(request, `${ROOT}/lib/${DECOY}`)).toBe(false);
            expect(await exists(request, DECOY)).toBe(true);
        } finally {
            await request.delete(`/api/library/${libID}`);
            await request.post('/api/delete', { data: { files: [DECOY] } }).catch(() => {});
        }
    });
});
