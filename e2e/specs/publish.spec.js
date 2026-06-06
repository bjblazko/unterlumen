import { test, expect } from '@playwright/test';
import { reindexLibrary } from '../helpers/library.js';

const GALLERY_SLUG = 'e2e-gallery';
const SITE_SLUG = 'e2e-site';

// Parse the SSE response text and return the complete event payload.
function parseSseComplete(text) {
    return text.split('\n')
        .filter(l => l.startsWith('data:'))
        .map(l => {
            try { return JSON.parse(l.slice('data:'.length).trim()); } catch { return null; }
        })
        .filter(Boolean)
        .find(e => e.complete) ?? null;
}

// Publish one photo to a gallery/album channel and return the complete SSE event.
async function publishGallery(request, libID, photoID, channelSlug, galleryTitle, opts = {}) {
    const body = {
        photoIDs: [photoID],
        channel: channelSlug,
        recordXMP: false,
        publishedAt: opts.publishedAt ?? '2026-01-15T12:00:00Z',
    };
    if (galleryTitle) body.galleryTitle = galleryTitle;
    if (opts.targetPostID) body.targetPostID = opts.targetPostID;

    const res = await request.post(`/api/library/${libID}/publish`, {
        data: body,
        timeout: 90_000,
    });
    expect(res.status()).toBe(200);
    const evt = parseSseComplete(await res.text());
    expect(evt).toBeTruthy();
    return evt;
}

test.describe('Publish — gallery listing and add-to-existing', () => {
    let libID;
    let photoID1;
    let photoID2;
    let galleryPostID; // postID from the first gallery publish (= album folder name)
    let sitePostID;    // postID from the first site publish (= album folder name)

    test.beforeAll(async ({ request }) => {
        test.setTimeout(240_000);

        // Clean up stale test libraries and channels from prior runs.
        const libs = await (await request.get('/api/library/')).json();
        await Promise.all(
            libs.filter(l => l.name === 'E2E Publish Library')
                .map(l => request.delete(`/api/library/${l.id}`)),
        );
        await request.delete(`/api/channels/${GALLERY_SLUG}`).catch(() => {});
        await request.delete(`/api/channels/${SITE_SLUG}`).catch(() => {});

        // Create a library scoped to folder-b (50 photos, fast to index).
        const libRes = await request.post('/api/library/', {
            data: { name: 'E2E Publish Library', description: '', sourcePath: 'folder-b' },
        });
        expect(libRes.status()).toBe(201);
        libID = (await libRes.json()).id;
        await reindexLibrary(request, libID);

        // Grab two photo IDs.
        const { results } = await (await request.get(`/api/library/search?ids=${libID}&limit=2`)).json();
        expect(results.length).toBeGreaterThanOrEqual(2);
        photoID1 = results[0].id;
        photoID2 = results[1].id;

        // Create gallery channel (galleryExport: true).
        const galleryRes = await request.post('/api/channels/', {
            data: {
                slug: GALLERY_SLUG,
                name: 'E2E Gallery',
                format: 'jpeg',
                quality: 75,
                exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                galleryExport: true,
                outputMode: 'save',
            },
        });
        expect(galleryRes.status()).toBe(201);

        // Create site channel (siteExport: true).
        const siteRes = await request.post('/api/channels/', {
            data: {
                slug: SITE_SLUG,
                name: 'E2E Site',
                format: 'jpeg',
                quality: 75,
                exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                siteExport: true,
                siteTitle: 'E2E Test Site',
                siteTheme: 'dark',
                outputMode: 'save',
            },
        });
        expect(siteRes.status()).toBe(201);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.delete(`/api/channels/${GALLERY_SLUG}`).catch(() => {});
        await request.delete(`/api/channels/${SITE_SLUG}`).catch(() => {});
    });

    // ── Gallery channel ───────────────────────────────────────────────────────

    test.describe('Gallery channel API', () => {
        test('GET galleries returns 404 for unknown channel', async ({ request }) => {
            const res = await request.get('/api/channels/no-such-channel-xyz/galleries');
            expect(res.status()).toBe(404);
        });

        test('GET galleries returns JSON array for channel with no output yet', async ({ request }) => {
            const res = await request.get(`/api/channels/${GALLERY_SLUG}/galleries`);
            expect(res.status()).toBe(200);
            expect(Array.isArray(await res.json())).toBe(true);
        });

        test('first gallery publish creates gallery and appears in listing', async ({ request }) => {
            test.setTimeout(120_000);
            const baseline = await (await request.get(`/api/channels/${GALLERY_SLUG}/galleries`)).json();

            const evt = await publishGallery(request, libID, photoID1, GALLERY_SLUG, 'Summer E2E');
            galleryPostID = evt.postID;

            const after = await (await request.get(`/api/channels/${GALLERY_SLUG}/galleries`)).json();
            expect(after.length).toBe(baseline.length + 1);

            const mine = after.find(g => g.postID === galleryPostID);
            expect(mine).toBeTruthy();
            expect(mine.title).toBe('Summer E2E');
            expect(mine.photoCount).toBe(1);
        });

        test('add-to-existing merges photos and updates photoCount', async ({ request }) => {
            test.setTimeout(120_000);
            expect(galleryPostID).toBeTruthy(); // depends on previous test

            const before = await (await request.get(`/api/channels/${GALLERY_SLUG}/galleries`)).json();
            const countBefore = before.length;

            await publishGallery(request, libID, photoID2, GALLERY_SLUG, '', {
                targetPostID: galleryPostID,
            });

            const after = await (await request.get(`/api/channels/${GALLERY_SLUG}/galleries`)).json();
            expect(after.length).toBe(countBefore); // no new gallery created

            const mine = after.find(g => g.postID === galleryPostID);
            expect(mine).toBeTruthy();
            expect(mine.photoCount).toBe(2);
        });

        test('add-to-existing with unknown targetPostID returns 400', async ({ request }) => {
            const res = await request.post(`/api/library/${libID}/publish`, {
                data: {
                    photoIDs: [photoID1],
                    channel: GALLERY_SLUG,
                    galleryTitle: '',
                    targetPostID: 'doesnotexist0000000000000',
                    recordXMP: false,
                },
                timeout: 10_000,
            });
            expect(res.status()).toBe(400);
        });
    });

    // ── Site channel ──────────────────────────────────────────────────────────

    test.describe('Site channel API', () => {
        test('first site publish creates album and appears in listing', async ({ request }) => {
            test.setTimeout(120_000);

            const baseline = await (await request.get(`/api/channels/${SITE_SLUG}/galleries`)).json();

            const evt = await publishGallery(request, libID, photoID1, SITE_SLUG, 'Winter E2E', {
                publishedAt: '2026-02-01T12:00:00Z',
            });
            sitePostID = evt.postID;

            const after = await (await request.get(`/api/channels/${SITE_SLUG}/galleries`)).json();
            expect(after.length).toBe(baseline.length + 1);

            const mine = after.find(g => g.postID === sitePostID);
            expect(mine).toBeTruthy();
            expect(mine.title).toBe('Winter E2E');
            expect(mine.photoCount).toBe(1);
        });

        test('add-to-existing site album preserves publishedAt and updates photoCount', async ({ request }) => {
            test.setTimeout(120_000);
            expect(sitePostID).toBeTruthy(); // depends on previous test

            const before = await (await request.get(`/api/channels/${SITE_SLUG}/galleries`)).json();
            const original = before.find(g => g.postID === sitePostID);
            expect(original).toBeTruthy();
            const originalDate = original.publishedAt;

            await publishGallery(request, libID, photoID2, SITE_SLUG, '', {
                targetPostID: sitePostID,
                publishedAt: '2026-03-01T12:00:00Z', // different date — must NOT update album
            });

            const after = await (await request.get(`/api/channels/${SITE_SLUG}/galleries`)).json();
            const mine = after.find(g => g.postID === sitePostID);
            expect(mine).toBeTruthy();
            expect(mine.photoCount).toBe(2);
            expect(mine.publishedAt).toBe(originalDate); // sort order preserved
        });
    });
});
