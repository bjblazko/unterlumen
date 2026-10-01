import { test, expect } from '@playwright/test';
import { spawn } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

// Sharing is per library (ADR-0047): a shared library carries a marker in its
// folder, and a second installation — its own data folder, its own index —
// gets the same library from it, by adding the folder or by joining.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..');
const BINARY = path.resolve(ROOT, '..', 'unterlumen');
const NAS = { port: 8088 };
const MAC = { port: 8089 };
const url = (inst) => `http://127.0.0.1:${inst.port}`;
const photos_dir = (photo) => path.dirname(photo.pathHint);

async function waitForServer(inst) {
    for (let i = 0; i < 60; i++) {
        try { if ((await fetch(url(inst) + '/api/config')).ok) return; } catch { /* not up yet */ }
        await new Promise(r => setTimeout(r, 250));
    }
    throw new Error('the app did not start');
}

async function createLibrary(inst, name, sourcePath) {
    const r = await fetch(url(inst) + '/api/library/', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, sourcePath }),
    });
    expect(r.ok).toBe(true);
    return r.json();
}

test.describe('Sharing a library between two installations', () => {
    test.describe.configure({ mode: 'serial', retries: 0 });
    let home, photos;

    test.beforeAll(async () => {
        home = fs.mkdtempSync(path.join(os.tmpdir(), 'unterlumen-libshare-'));
        photos = path.join(home, 'photos');
        fs.cpSync(path.join(ROOT, 'fixtures', 'photos', 'folder-a'), path.join(photos, 'Travel'), { recursive: true });
        fs.cpSync(path.join(ROOT, 'fixtures', 'photos', 'folder-b'), path.join(photos, 'Projects'), { recursive: true });
        for (const inst of [NAS, MAC]) {
            const env = { ...process.env, UNTERLUMEN_LIB_DIR: path.join(home, `data-${inst.port}`) };
            for (const k of ['UNTERLUMEN_CHANNELS_DIR', 'UNTERLUMEN_ROOT_PATH', 'UNTERLUMEN_PORT']) delete env[k];
            inst.app = spawn(BINARY, ['-port', String(inst.port), '-bind', '127.0.0.1', photos], { env, stdio: 'ignore' });
        }
        await Promise.all([waitForServer(NAS), waitForServer(MAC)]);
    });

    test.afterAll(() => {
        NAS.app?.kill();
        MAC.app?.kill();
        fs.rmSync(home, { recursive: true, force: true });
    });

    test('sharing in Edit library writes the marker into the folder', async ({ page }) => {
        const lib = await createLibrary(NAS, 'Travel', 'Travel');
        await page.goto(url(NAS) + '/#libraries');
        await page.locator('.library-card-name', { hasText: 'Travel' }).click();
        await page.locator('#lib-edit-btn').click();
        await expect(page.locator('#lib-edit-share .toggle')).toHaveAttribute('aria-checked', 'false');
        await page.locator('#lib-edit-share .toggle').click();
        await expect(page.locator('#lib-edit-share-hint')).toContainText('.unterlumen-library.json');
        const marker = JSON.parse(fs.readFileSync(path.join(photos, 'Travel', '.unterlumen-library.json'), 'utf8'));
        expect(marker).toMatchObject({ id: lib.id, name: 'Travel' });
    });

    test('New library on a shared folder adds that library, under its name', async ({ page }) => {
        await page.goto(url(MAC) + '/#libraries');
        await page.locator('#lib-new-btn').click();
        // Before anything is added, the dialog says what it does to the folder.
        await expect(page.locator('.library-dialog-facts')).toContainText('.xmp');
        await page.locator('#lib-dlg-path').fill('/Travel');
        await page.locator('#lib-dlg-path').dispatchEvent('change');
        await expect(page.locator('#lib-dlg-name')).toHaveValue('Travel');
        await expect(page.locator('#lib-dlg-name')).toHaveJSProperty('readOnly', true);
        await expect(page.locator('.library-dialog-note')).toContainText('Another installation shares this folder');
        await page.locator('#lib-dlg-create').click();
        // The dialog reads the photos before it opens the library.
        await expect(page.locator('.library-detail-name')).toHaveText('Travel', { timeout: 50_000 });

        const nasLibs = await (await fetch(url(NAS) + '/api/library/')).json();
        const macLibs = await (await fetch(url(MAC) + '/api/library/')).json();
        expect(macLibs.map(l => l.id)).toContain(nasLibs.find(l => l.name === 'Travel').id);
        expect(macLibs.find(l => l.name === 'Travel').shared).toBe(true);
    });

    test('a library of the same folder made before sharing offers to join', async ({ page }) => {
        const own = await createLibrary(MAC, 'My projects', 'Projects');
        const nas = await createLibrary(NAS, 'Projects', 'Projects');
        expect((await fetch(url(NAS) + `/api/library/${nas.id}/share`, { method: 'PUT' })).ok).toBe(true);

        await page.goto(url(MAC) + '/#libraries');
        await expect(page.locator('.library-card-meta', { hasText: 'shared by another installation' })).toBeVisible();
        await page.locator('.library-card-name', { hasText: 'My projects' }).click();
        await expect(page.locator('.library-notice')).toContainText('Another installation shares this folder as “Projects”');
        await page.locator('#lib-join-btn').click();
        await expect(page.locator('.library-detail-name')).toHaveText('Projects');
        await expect(page.locator('.library-notice')).toHaveCount(0);

        const macLibs = await (await fetch(url(MAC) + '/api/library/')).json();
        expect(macLibs.map(l => l.id)).toContain(nas.id);
        expect(macLibs.map(l => l.id)).not.toContain(own.id);
    });

    test('a rename on one installation shows on the other', async () => {
        const nasLibs = await (await fetch(url(NAS) + '/api/library/')).json();
        const travel = nasLibs.find(l => l.name === 'Travel');
        await fetch(url(NAS) + `/api/library/${travel.id}`, {
            method: 'PATCH', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: 'Journeys', description: '' }),
        });
        const seen = await (await fetch(url(MAC) + `/api/library/${travel.id}`)).json();
        expect(seen.name).toBe('Journeys');
    });

    // Titles and fields live in the sidecar beside the photo (ADR-0048), so
    // what one installation writes, the other reads — without a scan.
    test('a field written on one installation is read on the other', async () => {
        const travel = (await (await fetch(url(NAS) + '/api/library/')).json()).find(l => l.name === 'Journeys');
        // The NAS's library was made through the API and not read yet; this returns when it is.
        await (await fetch(url(NAS) + `/api/library/${travel.id}/reindex`, { method: 'POST' })).text();
        const { photos } = await (await fetch(url(NAS) + `/api/library/${travel.id}/photos?limit=1`)).json();
        const photo = photos[0];
        const put = await fetch(url(NAS) + `/api/library/${travel.id}/photo/${photo.id}/meta`, {
            method: 'PUT', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'people', value: 'Anna & Ben' }),
        });
        expect(put.ok).toBe(true);
        const sidecar = path.join(photos_dir(photo), photo.filename.replace(/\.[^.]+$/, '.xmp'));
        expect(fs.readFileSync(sidecar, 'utf8')).toContain('Anna &amp; Ben');

        const seen = await (await fetch(url(MAC) + `/api/library/${travel.id}/photo/${photo.id}/meta`)).json();
        expect(seen.find(e => e.key === 'people')?.value).toBe('Anna & Ben');

        await fetch(url(MAC) + `/api/library/${travel.id}/photo/${photo.id}/meta?key=people`, { method: 'DELETE' });
        const after = await (await fetch(url(NAS) + `/api/library/${travel.id}/photo/${photo.id}/meta`)).json();
        expect(after.find(e => e.key === 'people')).toBeUndefined();
    });
});
