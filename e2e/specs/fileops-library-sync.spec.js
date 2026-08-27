import { test, expect } from '@playwright/test';
import { GPS_PATH } from '../helpers/fixtures.js';
import { reindexLibrary } from '../helpers/library.js';

// Regression test: the generic /api/delete, /api/rename, and /api/mkdir
// file-ops endpoints (fileops.go) used to never touch the library index —
// only /api/copy and /api/move did. Deleting or renaming a library-tracked
// file left a stale "ok" row in the library DB pointing at a path that no
// longer existed (broken thumbnail, dead link) until a manual rescan.
//
// Operates on a dedicated throwaway subfolder + a fresh library scoped to
// it, so the fixtures used by other specs are never touched.

const SYNC_FOLDER = 'e2e-fileops-sync-test';

async function libraryPhotoPaths(request, libID) {
    const res = await request.get(`/api/library/${libID}/photos?limit=1000`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    return (body.photos || []).map((p) => p.pathHint);
}

test.describe('File-ops keep the library index in sync', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        await request.post('/api/delete', { data: { files: [SYNC_FOLDER] } }).catch(() => {});

        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(
            existing
                .filter((l) => l.name === 'E2E Fileops Sync')
                .map((l) => request.delete(`/api/library/${l.id}`)),
        );

        expect((await request.post('/api/mkdir', { data: { path: SYNC_FOLDER } })).status()).toBe(200);

        const libRes = await request.post('/api/library/', {
            data: { name: 'E2E Fileops Sync', description: '', sourcePath: SYNC_FOLDER },
        });
        expect(libRes.status()).toBe(201);
        libID = (await libRes.json()).id;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.post('/api/delete', { data: { files: [SYNC_FOLDER] } }).catch(() => {});
    });

    test('deleting a library-tracked file via /api/delete removes it from the library index', async ({ request }) => {
        const copyRes = await request.post('/api/copy', {
            data: { files: [GPS_PATH], destination: SYNC_FOLDER },
        });
        expect(copyRes.status()).toBe(200);
        const copiedName = GPS_PATH.split('/').pop();
        const relPath = `${SYNC_FOLDER}/${copiedName}`;

        await reindexLibrary(request, libID);
        let paths = await libraryPhotoPaths(request, libID);
        expect(paths.some((p) => p.endsWith(copiedName))).toBe(true);

        const delRes = await request.post('/api/delete', { data: { files: [relPath] } });
        expect(delRes.status()).toBe(200);

        // No manual rescan here — the fix makes /api/delete trigger the sync itself.
        await expect
            .poll(async () => (await libraryPhotoPaths(request, libID)).some((p) => p.endsWith(copiedName)), {
                timeout: 15_000,
                message: 'deleted file should disappear from the library index without a manual rescan',
            })
            .toBe(false);
    });

    test('renaming a library-tracked file via /api/rename updates the library index', async ({ request }) => {
        const copyRes = await request.post('/api/copy', {
            data: { files: [GPS_PATH], destination: SYNC_FOLDER },
        });
        expect(copyRes.status()).toBe(200);
        const copiedName = GPS_PATH.split('/').pop();
        const oldRelPath = `${SYNC_FOLDER}/${copiedName}`;
        const newName = 'renamed-by-e2e-test.jpeg';

        await reindexLibrary(request, libID);
        let paths = await libraryPhotoPaths(request, libID);
        expect(paths.some((p) => p.endsWith(copiedName))).toBe(true);

        const renameRes = await request.post('/api/rename', {
            data: { path: oldRelPath, name: newName },
        });
        expect(renameRes.status()).toBe(200);

        await expect
            .poll(async () => (await libraryPhotoPaths(request, libID)).some((p) => p.endsWith(newName)), {
                timeout: 15_000,
                message: 'renamed file should appear under its new name in the library index without a manual rescan',
            })
            .toBe(true);

        paths = await libraryPhotoPaths(request, libID);
        expect(paths.some((p) => p.endsWith(`/${copiedName}`))).toBe(false);

        // Clean up this test's file so the next test starts from a known state.
        await request.post('/api/delete', { data: { files: [`${SYNC_FOLDER}/${newName}`] } });
    });
});
