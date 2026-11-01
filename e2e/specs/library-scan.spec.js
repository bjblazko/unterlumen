import { test, expect } from '@playwright/test';
import { libraryMenu, reindexLibrary } from '../helpers/library.js';
import { waitForAppReady } from '../helpers/wait.js';
import { GPS_PATH, GPS_IMAGE, NO_GPS_PATH } from '../helpers/fixtures.js';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const PHOTOS = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'fixtures', 'photos');

// Tests for the five library scan operations:
//   scan-new, reindex, regen-previews-missing, regen-previews-all, cleanup

async function driveSSE(request, url, timeout = 60_000) {
    const res = await request.post(url, { timeout });
    expect(res.status(), `SSE POST ${url}`).toBe(200);
    const ct = res.headers()['content-type'];
    expect(ct, 'content-type should be SSE').toContain('text/event-stream');
    const text = await res.text();
    expect(text, 'SSE stream should signal completion').toContain('"finished":true');
    return text;
}

test.describe('Library scan operations', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(
            existing
                .filter(l => l.name === 'E2E Scan Ops')
                .map(l => request.delete(`/api/library/${l.id}`))
        );
        const res = await request.post('/api/library/', {
            data: { name: 'E2E Scan Ops', description: '', sourcePath: 'folder-a' },
        });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    // ── API contracts ────────────────────────────────────────────────────────

    test('POST /api/library/{id}/scan-new returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/scan-new`);
    });

    test('POST /api/library/{id}/reindex returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/reindex`);
    });

    test('POST /api/library/{id}/regen-previews-missing returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/regen-previews-missing`);
    });

    test('POST /api/library/{id}/regen-previews-all returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/regen-previews-all`);
    });

    test('POST /api/library/{id}/cleanup returns SSE stream that finishes', async ({ request }) => {
        await driveSSE(request, `/api/library/${libID}/cleanup`);
    });

    test('POST /api/library/{id}/scan-new with subfolder scopes scan', async ({ request }) => {
        const text = await driveSSE(
            request,
            `/api/library/${libID}/scan-new?subfolder=a1`
        );
        expect(text).toContain('"finished":true');
    });

    // ── Library card UI ──────────────────────────────────────────────────────

    test('library row offers Match with the folder and the indexed date', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Scan Ops' });
        await expect(card).toBeVisible({ timeout: 8_000 });

        await expect(card.locator('.lib-match')).toContainText('Match with the folder');
        await expect(card.locator('.library-card-indexed')).toContainText('Indexed');

        // The rarer maintenance runs are not on the row any more.
        await expect(card.locator('.lib-scan-toggle')).toHaveCount(0);
        await expect(card.locator('.lib-reindex')).toHaveCount(0);
    });

    // ── Maintenance in "Edit library…" ───────────────────────────────────────

    test('Edit library holds the maintenance runs', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Scan Ops' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('#lib-pane', { timeout: 8_000 });

        await libraryMenu(page, 'edit-library');
        const dialog = page.locator('.library-dialog');
        await expect(dialog).toBeVisible({ timeout: 5_000 });

        const actions = dialog.locator('#lib-edit-maint-actions');
        // Two runs cover what six buttons did (ADR-0053).
        await expect(actions.locator('[data-maint]')).toHaveText(['Match with the folder', 'Read everything again…']);

        await dialog.locator('#lib-edit-cancel').click();
    });

    // ── Scanning the folder you have open ────────────────────────────────────

    test('a library folder offers Match with the folder in its ⋯ menu', async ({ page }) => {
        await page.goto('/#folders');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });

        const card = page.locator('.library-card', { hasText: 'E2E Scan Ops' });
        await card.locator('.lib-open').click();
        await page.waitForSelector('#lib-pane', { timeout: 8_000 });

        await page.locator('#lib-pane .menu-btn').click();
        await expect(page.locator('.menu [data-id="match"]')).toHaveText('Match with the folder');
        // A library is already a library.
        await expect(page.locator('.menu [data-id="make-library"]')).toHaveCount(0);
    });
});

// Match with the folder, on a library's row, does what it does in Edit
// library: a photo deleted outside the app (Finder, a shell) leaves the
// index, and one renamed outside keeps its place in it. The row's button
// once only added new photos, so a deleted photo stayed.
test.describe('Match with the folder', () => {
    const FOLDER = 'e2e-match-folder';
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === 'E2E Match').map(l => request.delete(`/api/library/${l.id}`)));
        fs.rmSync(path.join(PHOTOS, FOLDER), { recursive: true, force: true });
        expect((await request.post('/api/mkdir', { data: { path: FOLDER } })).status()).toBe(200);
        expect((await request.post('/api/copy', { data: { files: [GPS_PATH, NO_GPS_PATH], destination: FOLDER } })).status()).toBe(200);
        const res = await request.post('/api/library/', { data: { name: 'E2E Match', description: '', sourcePath: FOLDER } });
        expect(res.status()).toBe(201);
        libID = (await res.json()).id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        fs.rmSync(path.join(PHOTOS, FOLDER), { recursive: true, force: true });
    });

    test('the row drops a photo deleted outside and follows one renamed outside', async ({ page, request }) => {
        const before = await (await request.get(`/api/library/${libID}/photos?limit=10`)).json();
        expect(before.photos).toHaveLength(2);
        const dir = path.join(PHOTOS, FOLDER);
        fs.rmSync(path.join(dir, GPS_IMAGE));
        const kept = before.photos.find(p => p.filename !== GPS_IMAGE);
        fs.renameSync(path.join(dir, kept.filename), path.join(dir, `renamed-${kept.filename}`));

        await page.goto('/#library');
        await waitForAppReady(page);
        const card = page.locator('.library-card', { hasText: 'E2E Match' });
        await card.locator('.lib-match').click();
        await expect(card.locator('.library-card-count')).toContainText('1 photo', { timeout: 30_000 });

        await expect.poll(async () => {
            const after = await (await request.get(`/api/library/${libID}/photos?limit=10`)).json();
            return after.photos.map(p => [p.id, p.filename]);
        }, { timeout: 30_000 }).toEqual([[kept.id, `renamed-${kept.filename}`]]);
    });
});
