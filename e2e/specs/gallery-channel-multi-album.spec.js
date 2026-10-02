import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { reindexLibrary } from '../helpers/library.js';
import { waitForAppReady } from '../helpers/wait.js';

// A single-gallery channel is one host holding many unrelated, unlisted
// albums. These specs guard the paths that used to assume one gallery per
// channel, which forced users into one channel per album.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FOLDER = path.resolve(__dirname, '..', 'fixtures', 'photos', 'folder-b');
const SLUG = 'e2e-multi-album';

function parseSseComplete(text) {
    return text.split('\n')
        .filter(l => l.startsWith('data:'))
        .map(l => {
            try { return JSON.parse(l.slice('data:'.length).trim()); } catch { return null; }
        })
        .filter(Boolean)
        .find(e => e.complete) ?? null;
}

async function collect(request, libID, photoIDs, body) {
    const res = await request.post(`/api/library/${libID}/channels/${SLUG}/drafts`, {
        data: { photoIDs, ...body },
        timeout: 30_000,
    });
    expect(res.status()).toBe(200);
    return res.json();
}

async function generate(request, draftID) {
    const res = await request.post(`/api/channels/${SLUG}/drafts/${draftID}/generate`, {
        data: { publishedAt: '2026-01-15T12:00:00Z' },
        timeout: 90_000,
    });
    expect(res.status()).toBe(200);
    const evt = parseSseComplete(await res.text());
    expect(evt).toBeTruthy();
    return evt;
}

test.describe('Single-gallery channel — many albums on one host', () => {
    let libID;
    let photos = [];

    test.beforeAll(async ({ request }) => {
        test.setTimeout(180_000);

        const libs = await (await request.get('/api/library/')).json();
        await Promise.all(
            libs.filter(l => l.name === 'E2E Multi Album Library').map(l => request.delete(`/api/library/${l.id}`)),
        );
        await request.delete(`/api/channels/${SLUG}`).catch(() => {});

        const libRes = await request.post('/api/library/', {
            data: { name: 'E2E Multi Album Library', description: '', sourcePath: 'folder-b' },
        });
        expect(libRes.status()).toBe(201);
        libID = (await libRes.json()).id;
        await reindexLibrary(request, libID);

        const { results } = await (await request.get(`/api/library/search?ids=${libID}&limit=3`)).json();
        expect(results.length).toBeGreaterThanOrEqual(2);
        photos = results.map(r => r.id);

        const chRes = await request.post('/api/channels/', {
            data: {
                slug: SLUG,
                name: 'E2E Multi Album',
                format: 'jpeg',
                quality: 75,
                exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                galleryExport: true,
                siteURL: 'https://fotos.e2e.invalid',
                outputMode: 'save',
            },
        });
        expect(chRes.status()).toBe(201);

        // Deleting a channel drops its config but leaves its output directory,
        // so albums and drafts from an earlier run of this spec would still be
        // scanned into the overview and break the counts below.
        await resetChannelContents(request);
    });

    // Remove every gallery and draft this channel currently holds.
    async function resetChannelContents(request) {
        const galleries = await (await request.get(`/api/channels/${SLUG}/galleries`)).json();
        for (const g of galleries) {
            await request.delete(`/api/channels/${SLUG}/galleries/${g.postID}`, { data: { deleteRemote: false } });
        }
        const drafts = await (await request.get(`/api/channels/${SLUG}/drafts`)).json();
        for (const d of drafts) {
            await request.delete(`/api/channels/${SLUG}/drafts/${d.id}`);
        }
    }

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.delete(`/api/channels/${SLUG}`).catch(() => {});
    });

    test('two albums publish independently and both stay listed', async ({ request }) => {
        const a = await collect(request, libID, [photos[0]], { title: 'Album A', unlisted: true });
        const b = await collect(request, libID, [photos[1]], { title: 'Album B', unlisted: true });
        expect(a.id).not.toBe(b.id);

        const evtA = await generate(request, a.id);
        const evtB = await generate(request, b.id);
        expect(evtA.postID).not.toBe(evtB.postID);
        expect(evtA.galleryPath).not.toBe(evtB.galleryPath);

        const rows = await (await request.get('/api/channels/galleries')).json();
        const mine = rows.filter(r => r.channelSlug === SLUG);
        expect(mine).toHaveLength(2);
        expect(new Set(mine.map(r => r.rowKey)).size).toBe(2);
        expect(mine.map(r => r.title).sort()).toEqual(['Album A', 'Album B']);

        // Each album's public URL is its own postID folder under the channel's base URL.
        for (const row of mine) {
            expect(row.url).toBe(`https://fotos.e2e.invalid/${row.postID}/`);
        }
    });

    test('adding photos to an existing album reuses its folder and album id', async ({ request }) => {
        const first = await collect(request, libID, [photos[0]], { title: 'Growing Album', unlisted: true });
        const albumID = (await generate(request, first.id)).postID;

        const second = await collect(request, libID, [photos[1]], { postID: albumID });
        const evt = await generate(request, second.id);

        // The complete event must name the album written to, not a fresh id —
        // that value is what the XMP sidecar and built: meta record.
        expect(evt.postID).toBe(albumID);

        const rows = await (await request.get('/api/channels/galleries')).json();
        const album = rows.find(r => r.postID === albumID);
        expect(album.photoCount).toBe(2);
        expect(rows.filter(r => r.channelSlug === SLUG && r.title === 'Growing Album')).toHaveLength(1);
    });

    test('a photo in two albums of one channel records both memberships', async ({ request }) => {
        const a = await collect(request, libID, [photos[0]], { title: 'Shared A', unlisted: true });
        const albumA = (await generate(request, a.id)).postID;
        const b = await collect(request, libID, [photos[0]], { title: 'Shared B', unlisted: true });
        const albumB = (await generate(request, b.id)).postID;

        const meta = await (await request.get(`/api/library/${libID}/photo/${photos[0]}/meta`)).json();
        const byKey = Object.fromEntries(meta.map(e => [e.key, e.value]));

        expect(byKey[`built:${SLUG}:${albumA}:title`]).toBe('Shared A');
        expect(byKey[`built:${SLUG}:${albumB}:title`]).toBe('Shared B');

        // Both survive a reindex — this used to collapse to the newest album.
        await reindexLibrary(request, libID);
        const after = await (await request.get(`/api/library/${libID}/photo/${photos[0]}/meta`)).json();
        const afterKeys = after.map(e => e.key);
        expect(afterKeys).toContain(`built:${SLUG}:${albumA}:title`);
        expect(afterKeys).toContain(`built:${SLUG}:${albumB}:title`);
    });

    test('unpublishing a gallery takes it out of its photos, and the marker goes with the last one', async ({ request }) => {
        const photo = photos[2] ?? photos[0];
        const gone = await collect(request, libID, [photo], { title: 'Unpublish Gone', unlisted: true });
        const goneID = (await generate(request, gone.id)).postID;
        const stays = await collect(request, libID, [photo], { title: 'Unpublish Stays', unlisted: true });
        const staysID = (await generate(request, stays.id)).postID;

        const keys = async () => (await (await request.get(`/api/library/${libID}/photo/${photo}/meta`)).json()).map(e => e.key);
        const sidecarsNaming = (postID) => fs.readdirSync(FOLDER).filter(f => f.endsWith('.xmp'))
            .filter(f => fs.readFileSync(path.join(FOLDER, f), 'utf8').includes(`<ul:PostID>${postID}</ul:PostID>`));
        expect(sidecarsNaming(goneID)).toHaveLength(1);

        const del = await request.delete(`/api/channels/${SLUG}/galleries/${goneID}`, { data: { deleteRemote: false } });
        expect(del.status()).toBe(200);

        // Only that gallery's record and keys are gone; the other gallery and the channel marker stay.
        expect(sidecarsNaming(goneID)).toHaveLength(0);
        expect(sidecarsNaming(staysID)).toHaveLength(1);
        let now = await keys();
        expect(now).not.toContain(`built:${SLUG}:${goneID}`);
        expect(now).toContain(`built:${SLUG}:${staysID}`);
        expect(now).toContain(`built:${SLUG}`);

        await request.delete(`/api/channels/${SLUG}/galleries/${staysID}`, { data: { deleteRemote: false } });
        expect(sidecarsNaming(staysID)).toHaveLength(0);
        now = await keys();
        expect(now).not.toContain(`built:${SLUG}`);   // the channel filter matches on it
    });

    test('unlisted albums carry a noindex tag, listed ones do not', async ({ request }) => {
        const hidden = await collect(request, libID, [photos[0]], { title: 'Hidden', unlisted: true });
        const hiddenID = (await generate(request, hidden.id)).postID;
        const shown = await collect(request, libID, [photos[1]], { title: 'Shown', unlisted: false });
        const shownID = (await generate(request, shown.id)).postID;

        const read = async (postID) => {
            const res = await request.get(`/api/channels/${SLUG}/galleries`);
            const item = (await res.json()).find(g => g.postID === postID);
            expect(item).toBeTruthy();
            return item;
        };
        expect((await read(hiddenID)).unlisted).toBe(true);
        expect((await read(shownID)).unlisted).toBeFalsy();

        // Toggling after publish is allowed here: the folder name is the
        // random postID either way, so the shared link keeps working.
        const patch = await request.patch(`/api/channels/${SLUG}/galleries/${shownID}`, {
            data: { title: 'Shown', unlisted: true },
        });
        expect(patch.status()).toBe(200);
        expect((await read(shownID)).unlisted).toBe(true);
    });

    test('the Info Panel lists one card per album, and no cards for meta fields', async ({ request, page }) => {
        const a = await collect(request, libID, [photos[0]], { title: 'Card A', unlisted: true });
        const albumA = (await generate(request, a.id)).postID;
        const b = await collect(request, libID, [photos[0]], { title: 'Card B', unlisted: true });
        const albumB = (await generate(request, b.id)).postID;

        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view, .library-detail', { timeout: 8_000 });
        if (await page.locator('.library-list-view').isVisible()) {
            await page.locator('.library-card', { hasText: 'E2E Multi Album Library' }).locator('.lib-open').click();
            await page.waitForSelector('.library-detail', { timeout: 8_000 });
        }
        await page.waitForSelector('#lib-pane [data-type="image"]', { timeout: 15_000 });

        const images = page.locator('#lib-pane [data-type="image"]');
        await images.first().click();
        if (await page.locator('.info-panel.expanded:visible').count() === 0) await page.keyboard.press('i');
        await page.locator('.info-panel.expanded:visible').waitFor({ timeout: 10_000 });

        // One card per album the photo actually belongs to. Earlier tests in
        // this file publish the same photo too, so the expected number comes
        // from its meta rather than a fixed count. The channel's own
        // built:<slug>:title and :postid keys have two segments as well and
        // must not be counted as albums — mistaking them for albums renders
        // extra cards whose "date" is really a title string.
        const meta = await (await request.get(`/api/library/${libID}/photo/${photos[0]}/meta`)).json();
        const albums = meta.filter(e => {
            const parts = e.key.split(':');
            return parts[0] === 'built' && parts[1] === SLUG && parts.length === 3
                && !['title', 'postid', 'account'].includes(parts[2]);
        });
        expect(albums.length).toBeGreaterThanOrEqual(2);

        const cards = page.locator(`.info-pub-card:not(.info-pub-card--pending)[data-key^="built:${SLUG}"]`);
        await expect(cards).toHaveCount(albums.length, { timeout: 10_000 });
        // Match the album ids rather than the titles: a retried run republishes
        // "Card A", and two albums may then legitimately share that title.
        for (const albumID of [albumA, albumB]) {
            await expect(page.locator(`.info-pub-card[data-key="built:${SLUG}:${albumID}"]`)).toHaveCount(1);
        }
        // A published gallery has no × — the info panel cannot take a photo
        // out of a gallery — and its title leads to the gallery's page.
        await expect(page.locator('.info-pub-card:not(.info-pub-card--pending) .info-meta-del')).toHaveCount(0);
        await page.locator(`.info-pub-card[data-key="built:${SLUG}:${albumA}"] .info-pub-link`).click();
        await expect(page.locator('#mode-published')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.gal-detail')).toBeVisible({ timeout: 10_000 });
    });

    test('two pending albums render as two separate rows under their destination', async ({ request, page }) => {
        await collect(request, libID, [photos[0]], { title: 'Pending One' });
        await collect(request, libID, [photos[1]], { title: 'Pending Two' });

        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');

        const rowOne = page.locator('.gal-row', { hasText: 'Pending One' });
        const rowTwo = page.locator('.gal-row', { hasText: 'Pending Two' });
        await expect(rowOne).toBeVisible();
        await expect(rowTwo).toBeVisible();

        const keyOne = await rowOne.getAttribute('data-rowkey');
        const keyTwo = await rowTwo.getAttribute('data-rowkey');
        expect(keyOne).toBeTruthy();
        expect(keyOne).not.toBe(keyTwo);
        await expect(rowOne.locator('.gal-state')).toHaveText('Not online yet');

        // A draft has no address of its own yet; where it is headed is the
        // destination's address, which the group heading carries once for all
        // of its galleries instead of repeating it per row.
        await expect(page.locator('.gal-group', { hasText: 'Pending One' })
            .locator('.gal-group-addr')).toContainText('fotos.e2e.invalid');
    });
});
