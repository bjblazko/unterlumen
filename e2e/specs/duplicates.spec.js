import { test, expect } from '@playwright/test';
import { GPS_PATH, GPS_IMAGE } from '../helpers/fixtures.js';
import { reindexLibrary } from '../helpers/library.js';
import { waitForAppReady } from '../helpers/wait.js';

// Duplicates (ADR-0059): two copies of one photo in one library form a
// group; marking one and deleting it in Marked for deletion removes that
// copy — the one chosen, not the one the index names first — and keeps the
// other in the library.
//
// A throwaway folder and library, so the fixtures other specs use stay.

const DUP_FOLDER = 'e2e-duplicates-test';

test.describe('Duplicates', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        await request.post('/api/delete', { data: { files: [DUP_FOLDER] } }).catch(() => {});
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter((l) => l.name === 'E2E Duplicates')
            .map((l) => request.delete(`/api/library/${l.id}`)));

        for (const dir of [DUP_FOLDER, `${DUP_FOLDER}/first`, `${DUP_FOLDER}/second`]) {
            expect((await request.post('/api/mkdir', { data: { path: dir } })).status()).toBe(200);
        }
        for (const dest of ['first', 'second']) {
            const res = await request.post('/api/copy', { data: { files: [GPS_PATH], destination: `${DUP_FOLDER}/${dest}` } });
            expect(res.status()).toBe(200);
        }
        const libRes = await request.post('/api/library/', {
            data: { name: 'E2E Duplicates', description: '', sourcePath: DUP_FOLDER },
        });
        expect(libRes.status()).toBe(201);
        libID = (await libRes.json()).id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.post('/api/delete', { data: { files: [DUP_FOLDER] } }).catch(() => {});
    });

    test('marks one copy and deletes only that one', async ({ page, request }) => {
        await page.goto('/#duplicates');
        await waitForAppReady(page);
        await expect(page.locator('#mode-duplicates')).toHaveAttribute('aria-current', 'page');

        const group = page.locator('.duplicates-group', { hasText: `${DUP_FOLDER}/second` })
            .filter({ hasText: `${DUP_FOLDER}/first` });
        await expect(group).toBeVisible({ timeout: 30_000 });
        await expect(group.locator('.duplicates-group-title')).toContainText('Identical');
        const second = group.locator('.duplicates-version', { hasText: `${DUP_FOLDER}/second` });
        await expect(second.locator('.duplicates-name')).toHaveText(GPS_IMAGE);

        await second.locator('[data-mark]').click();
        await expect(page.locator('#wastebin-count')).toHaveText('1');
        const marked = group.locator('.duplicates-version', { hasText: `${DUP_FOLDER}/second` });
        await expect(marked).toHaveClass(/is-marked/);
        await expect(marked.locator('[data-mark]')).toHaveText('Unmark');

        await page.locator('#mode-wastebin').click();
        await page.locator('#wb-delete').click();
        await page.locator('#wb-delete-confirm').click();
        await expect(page.locator('#wastebin-count')).toHaveText('');

        const photos = await (await request.get(`/api/library/${libID}/photos?limit=100`)).json();
        expect(photos.photos).toHaveLength(1);
        expect(photos.photos[0].pathHint).toContain(`${DUP_FOLDER}/first/`);
        const listed = await (await request.get(`/api/browse?path=${DUP_FOLDER}/second`)).json();
        expect((listed.entries || []).filter((e) => e.name === GPS_IMAGE)).toHaveLength(0);

        await page.locator('#mode-duplicates').click();
        await expect(page.locator('.duplicates-group', { hasText: `${DUP_FOLDER}/second` })).toHaveCount(0, { timeout: 30_000 });
    });
});
