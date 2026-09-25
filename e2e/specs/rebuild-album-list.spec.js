import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { reindexLibrary } from '../helpers/library.js';
import { waitForAppReady } from '../helpers/wait.js';

// "Rebuild album list" (Destinations → Advanced) restores albums that are
// missing from the shared register, from what the photos' own sidecars record.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..');
const SHARED_DIR = path.join(ROOT, 'fixtures', '.unterlumen-test');
const FOLDER = path.join(ROOT, 'fixtures', 'photos', 'folder-b');
const SLUG = 'e2e-rebuild-list';
const REGISTER = path.join(SHARED_DIR, 'albums', SLUG);

function parseSseComplete(text) {
    return text.split('\n')
        .filter(l => l.startsWith('data:'))
        .map(l => { try { return JSON.parse(l.slice('data:'.length).trim()); } catch { return null; } })
        .filter(Boolean)
        .find(e => e.complete) ?? null;
}

// The sidecars this destination has written into the fixture folder.
function sidecarsOfDestination() {
    return fs.readdirSync(FOLDER).filter(f => f.endsWith('.xmp'))
        .map(f => path.join(FOLDER, f))
        .filter(f => fs.readFileSync(f, 'utf8').includes(`<ul:Channel>${SLUG}</ul:Channel>`));
}

// Publishing writes into the fixture folder's sidecars and they stay there, so
// a run has to remove what it wrote — before as well as after, or an earlier
// run's albums are found again.
function purgeOwnPublications() {
    for (const f of sidecarsOfDestination()) {
        const cleaned = fs.readFileSync(f, 'utf8').replace(
            /\s*<rdf:li[^>]*>(?:(?!<\/rdf:li>)[\s\S])*<\/rdf:li>/g,
            li => (li.includes(`<ul:Channel>${SLUG}</ul:Channel>`) ? '' : li),
        );
        fs.writeFileSync(f, cleaned);
    }
}

test.describe('Rebuild album list', () => {
    let libID;
    let photos = [];

    test.beforeAll(async ({ request }) => {
        test.setTimeout(180_000);
        purgeOwnPublications();
        fs.rmSync(REGISTER, { recursive: true, force: true });
        await request.delete(`/api/channels/${SLUG}`).catch(() => {});
        fs.rmSync(path.join(SHARED_DIR, 'channels', SLUG), { recursive: true, force: true });

        const libs = await (await request.get('/api/library/')).json();
        await Promise.all(libs.filter(l => l.name === 'E2E Rebuild List').map(l => request.delete(`/api/library/${l.id}`)));
        const lib = await request.post('/api/library/', { data: { name: 'E2E Rebuild List', description: '', sourcePath: 'folder-b' } });
        expect(lib.status()).toBe(201);
        libID = (await lib.json()).id;
        await reindexLibrary(request, libID);
        const { results } = await (await request.get(`/api/library/search?ids=${libID}&limit=3`)).json();
        photos = results.map(r => r.id);

        const ch = await request.post('/api/channels/', {
            data: {
                slug: SLUG, name: 'E2E Rebuild List', format: 'jpeg', quality: 75, exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                siteExport: true, siteTitle: 'E2E Rebuild List', outputMode: 'save',
            },
        });
        expect(ch.status()).toBe(201);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.delete(`/api/channels/${SLUG}`).catch(() => {});
        fs.rmSync(REGISTER, { recursive: true, force: true });
        purgeOwnPublications();
    });

    async function publish(request, photoID, title) {
        const d = await request.post(`/api/library/${libID}/channels/${SLUG}/drafts`, { data: { photoIDs: [photoID], title }, timeout: 30_000 });
        expect(d.status()).toBe(200);
        const res = await request.post(`/api/channels/${SLUG}/drafts/${(await d.json()).id}/generate`, {
            data: { publishedAt: '2026-03-01T12:00:00Z' }, timeout: 90_000,
        });
        expect(res.status()).toBe(200);
        return parseSseComplete(await res.text());
    }

    async function openRebuild(page) {
        await page.goto('/#destinations');
        await waitForAppReady(page);
        await page.locator('.dest-row', { hasText: 'E2E Rebuild List' }).click();
        const advanced = page.locator('.dest-advanced');
        await advanced.locator('summary').click();
        return advanced;
    }

    test('an album lost from the register comes back with the same address', async ({ page, request }) => {
        await publish(request, photos[0], 'Rebuild Alpha');
        const [file] = fs.readdirSync(REGISTER).filter(f => f.endsWith('.json'));
        const before = JSON.parse(fs.readFileSync(path.join(REGISTER, file), 'utf8'));
        fs.rmSync(path.join(REGISTER, file));
        expect(await (await request.get(`/api/channels/${SLUG}/galleries`)).json()).toHaveLength(0);

        const advanced = await openRebuild(page);
        await advanced.locator('#dest-rebuild-albums').click();
        const report = advanced.locator('#dest-rebuild-albums-report');
        await expect(report).toContainText('Found 1 album in the photos. Added 1; 0 already listed.', { timeout: 15_000 });
        await expect(report).toContainText('Rebuild Alpha');

        const after = JSON.parse(fs.readFileSync(path.join(REGISTER, file), 'utf8'));
        expect(after.slug).toBe(before.slug);       // an address that was shared must not change
        expect(after.postID).toBe(before.postID);
        expect(await (await request.get(`/api/channels/${SLUG}/galleries`)).json()).toHaveLength(1);

        // Running it again changes nothing.
        await advanced.locator('#dest-rebuild-albums').click();
        await expect(report).toContainText('Added 0; 1 already listed.', { timeout: 15_000 });
    });

    test('an album whose sidecars have no address is listed, not restored under a made-up one', async ({ page, request }) => {
        // Sidecars written before the address was recorded carry no ul:Slug.
        for (const f of sidecarsOfDestination()) {
            fs.writeFileSync(f, fs.readFileSync(f, 'utf8').replace(/\s*<ul:Slug>[^<]*<\/ul:Slug>/g, ''));
        }
        for (const f of fs.readdirSync(REGISTER)) fs.rmSync(path.join(REGISTER, f));

        const advanced = await openRebuild(page);
        await advanced.locator('#dest-rebuild-albums').click();
        const report = advanced.locator('#dest-rebuild-albums-report');
        await expect(report).toContainText('1 album could not be restored', { timeout: 15_000 });
        await expect(report).toContainText('Rebuild Alpha');
        await expect(report).toContainText('Added 0');

        expect(fs.readdirSync(REGISTER).filter(f => f.endsWith('.json'))).toHaveLength(0);
    });

    test('a deleted album stays deleted when the list is rebuilt', async ({ page, request }) => {
        purgeOwnPublications();
        for (const f of fs.readdirSync(REGISTER)) fs.rmSync(path.join(REGISTER, f));

        const evt = await publish(request, photos[1], 'Rebuild Gamma');
        expect(sidecarsOfDestination().some(f => fs.readFileSync(f, 'utf8').includes('Rebuild Gamma'))).toBe(true);

        const del = await request.delete(`/api/channels/${SLUG}/galleries/${evt.postID}`, { data: { deleteRemote: false } });
        expect(del.status()).toBe(200);

        // Deleting takes the album out of its photos and leaves a tombstone.
        expect(sidecarsOfDestination().some(f => fs.readFileSync(f, 'utf8').includes('Rebuild Gamma'))).toBe(false);
        expect(fs.existsSync(path.join(REGISTER, `${evt.postID}.deleted`))).toBe(true);

        const advanced = await openRebuild(page);
        await advanced.locator('#dest-rebuild-albums').click();
        await expect(advanced.locator('#dest-rebuild-albums-report')).toContainText('No albums found', { timeout: 15_000 });
        expect(await (await request.get(`/api/channels/${SLUG}/galleries`)).json()).toHaveLength(0);
    });

    test('an album published before addresses were recorded gets its address written into its photos', async ({ page, request }) => {
        purgeOwnPublications();
        for (const f of fs.readdirSync(REGISTER)) fs.rmSync(path.join(REGISTER, f));
        await publish(request, photos[2], 'Rebuild Delta');
        const [file] = fs.readdirSync(REGISTER).filter(f => f.endsWith('.json'));
        const slug = JSON.parse(fs.readFileSync(path.join(REGISTER, file), 'utf8')).slug;

        // The register knows the album; its photo's sidecar predates the address.
        for (const f of sidecarsOfDestination()) {
            fs.writeFileSync(f, fs.readFileSync(f, 'utf8').replace(/\s*<ul:Slug>[^<]*<\/ul:Slug>/g, ''));
        }
        expect(sidecarsOfDestination().some(f => fs.readFileSync(f, 'utf8').includes('<ul:Slug>'))).toBe(false);

        const advanced = await openRebuild(page);
        await advanced.locator('#dest-rebuild-albums').click();
        await expect(advanced.locator('#dest-rebuild-albums-report')).toContainText('Completed 1 photo sidecar', { timeout: 15_000 });
        expect(sidecarsOfDestination().some(f => fs.readFileSync(f, 'utf8').includes(`<ul:Slug>${slug}</ul:Slug>`))).toBe(true);

        // Now the album survives losing its register entry.
        fs.rmSync(path.join(REGISTER, file));
        await advanced.locator('#dest-rebuild-albums').click();
        await expect(advanced.locator('#dest-rebuild-albums-report')).toContainText('Added 1', { timeout: 15_000 });
        expect(JSON.parse(fs.readFileSync(path.join(REGISTER, file), 'utf8')).slug).toBe(slug);
    });
});
