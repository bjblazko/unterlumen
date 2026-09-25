import { test, expect, request as pwRequest } from '@playwright/test';
import { spawn } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';
import { reindexLibrary } from '../helpers/library.js';

// A website destination is published from two installations that share only
// the channel directory (-channels-dir). Each has its own -lib-dir and its own
// generated output. Before the album register, the second installation wrote an
// index and a sitemap that listed only its own albums.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..');
const SHARED_DIR = path.join(ROOT, 'fixtures', '.unterlumen-test'); // channels dir of the main server (8082)
const PHOTOS = path.join(ROOT, 'fixtures', 'photos');
const SLUG = 'e2e-register-site';
const SECOND_PORT = 8083;
const SECOND_URL = `http://127.0.0.1:${SECOND_PORT}`;

function parseSseComplete(text) {
    return text.split('\n')
        .filter(l => l.startsWith('data:'))
        .map(l => { try { return JSON.parse(l.slice('data:'.length).trim()); } catch { return null; } })
        .filter(Boolean)
        .find(e => e.complete) ?? null;
}

async function waitForServer(url) {
    for (let i = 0; i < 60; i++) {
        try { if ((await fetch(url + '/api/browse?path=')).ok) return; } catch { /* not up yet */ }
        await new Promise(r => setTimeout(r, 250));
    }
    throw new Error('second installation did not start');
}

async function publishAlbum(api, libID, photoID, title) {
    const draftRes = await api.post(`/api/library/${libID}/channels/${SLUG}/drafts`, {
        data: { photoIDs: [photoID], title },
        timeout: 30_000,
    });
    expect(draftRes.status()).toBe(200);
    const draft = await draftRes.json();
    const res = await api.post(`/api/channels/${SLUG}/drafts/${draft.id}/generate`, {
        data: { publishedAt: '2026-02-01T12:00:00Z' },
        timeout: 90_000,
    });
    expect(res.status()).toBe(200);
    const evt = parseSseComplete(await res.text());
    expect(evt).toBeTruthy();
    return evt;
}

// The directory this installation writes the channel's files to; it differs per installation.
async function outputDirOf(api) {
    const list = await (await api.get('/api/channels/')).json();
    return list.find(c => c.slug === SLUG).outputDir;
}

async function createLibrary(api, name) {
    const res = await api.post('/api/library/', { data: { name, description: '', sourcePath: 'folder-b' } });
    expect(res.status()).toBe(201);
    const id = (await res.json()).id;
    await reindexLibrary(api, id);
    return id;
}

test.describe('Website destination published from two installations', () => {
    let second;        // child process: the second installation
    let secondLibDir;
    let mainLib, secondLib;
    let mainApi, secondApi;
    let photos = [];

    test.beforeAll(async () => {
        test.setTimeout(240_000);
        const request = mainApi = await pwRequest.newContext({ baseURL: 'http://localhost:8082' });

        // A previous run leaves the register and both output folders behind.
        fs.rmSync(path.join(SHARED_DIR, 'albums', SLUG), { recursive: true, force: true });
        await request.delete(`/api/channels/${SLUG}`).catch(() => {});
        fs.rmSync(path.join(SHARED_DIR, 'channels', SLUG), { recursive: true, force: true });

        const libs = await (await request.get('/api/library/')).json();
        await Promise.all(libs.filter(l => l.name === 'E2E Register Main').map(l => request.delete(`/api/library/${l.id}`)));
        mainLib = await createLibrary(request, 'E2E Register Main');
        const { results } = await (await request.get(`/api/library/search?ids=${mainLib}&limit=3`)).json();
        expect(results.length).toBeGreaterThanOrEqual(2);
        photos = results.map(r => r.id);

        const ch = await request.post('/api/channels/', {
            data: {
                slug: SLUG, name: 'E2E Register Site', format: 'jpeg', quality: 75, exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                siteExport: true, siteTitle: 'E2E Register Site', siteURL: 'https://site.e2e.invalid',
                outputMode: 'save',
            },
        });
        expect(ch.status()).toBe(201);

        // Album A is published from the first installation ...
        await publishAlbum(request, mainLib, photos[0], 'Register Alpha');

        // ... then a second installation starts: own -lib-dir, same channels dir.
        secondLibDir = fs.mkdtempSync(path.join(os.tmpdir(), 'unterlumen-second-'));
        second = spawn(path.join(ROOT, '..', 'unterlumen'),
            ['--port', String(SECOND_PORT), '--bind', '127.0.0.1', PHOTOS],
            { env: { ...process.env, UNTERLUMEN_LIB_DIR: secondLibDir, UNTERLUMEN_CHANNELS_DIR: SHARED_DIR }, stdio: 'ignore' });
        await waitForServer(SECOND_URL);
        secondApi = await pwRequest.newContext({ baseURL: SECOND_URL });

        // It indexes after A was published, so it reads A's XMP sidecar.
        secondLib = await createLibrary(secondApi, 'E2E Register Second');
    });

    test.afterAll(async () => {
        const request = mainApi;
        if (secondApi) await secondApi.dispose();
        if (second) second.kill();
        if (mainLib) await request.delete(`/api/library/${mainLib}`);
        await request.delete(`/api/channels/${SLUG}`).catch(() => {});
        fs.rmSync(path.join(SHARED_DIR, 'albums', SLUG), { recursive: true, force: true });
        await mainApi.dispose();
        if (secondLibDir) fs.rmSync(secondLibDir, { recursive: true, force: true });
    });

    test('the second installation sees album A and its build lists both albums', async () => {
        const before = await (await secondApi.get(`/api/channels/${SLUG}/galleries`)).json();
        expect(before.map(g => g.title)).toEqual(['Register Alpha']);

        await publishAlbum(secondApi, secondLib, photos[1], 'Register Beta');

        const outputDir = await outputDirOf(secondApi);
        const siteDir = path.join(outputDir, 'site');
        const index = fs.readFileSync(path.join(siteDir, 'index.html'), 'utf8');
        const sitemap = fs.readFileSync(path.join(siteDir, 'sitemap.xml'), 'utf8');
        for (const title of ['Register Alpha', 'Register Beta']) expect(index).toContain(title);
        for (const slug of ['register-alpha', 'register-beta']) expect(sitemap).toContain(`/${slug}`);

        // The first installation sees B without having published it.
        const seen = await (await mainApi.get(`/api/channels/${SLUG}/galleries`)).json();
        expect(seen.map(g => g.title).sort()).toEqual(['Register Alpha', 'Register Beta']);
    });

    test('the register holds one file per album and no machine-local path', async () => {
        const dir = path.join(SHARED_DIR, 'albums', SLUG);
        const files = fs.readdirSync(dir).filter(f => f.endsWith('.json'));
        expect(files).toHaveLength(2);
        for (const f of files) {
            const text = fs.readFileSync(path.join(dir, f), 'utf8');
            expect(text).not.toContain(PHOTOS);
            expect(text).not.toContain(secondLibDir);
        }
    });

    test('deleting an album from one installation leaves the other installation\'s album', async () => {
        const rows = await (await secondApi.get(`/api/channels/${SLUG}/galleries`)).json();
        const beta = rows.find(g => g.title === 'Register Beta');
        const res = await secondApi.delete(`/api/channels/${SLUG}/galleries/${beta.postID}`, { data: { deleteRemote: false } });
        expect(res.status()).toBeLessThan(300);

        const left = await (await mainApi.get(`/api/channels/${SLUG}/galleries`)).json();
        expect(left.map(g => g.title)).toEqual(['Register Alpha']);
        const index = fs.readFileSync(path.join(await outputDirOf(secondApi), 'site', 'index.html'), 'utf8');
        expect(index).toContain('Register Alpha');
        expect(index).not.toContain('Register Beta');
    });
});
