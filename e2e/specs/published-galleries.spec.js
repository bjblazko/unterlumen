import { test, expect } from '@playwright/test';
import { reindexLibrary } from '../helpers/library.js';
import { waitForAppReady } from '../helpers/wait.js';

const GALLERY_SLUG = 'e2e-published-gallery';
const SITE_SLUG = 'e2e-published-site';

function parseSseComplete(text) {
    return text.split('\n')
        .filter(l => l.startsWith('data:'))
        .map(l => {
            try { return JSON.parse(l.slice('data:'.length).trim()); } catch { return null; }
        })
        .filter(Boolean)
        .find(e => e.complete) ?? null;
}

async function buildGallery(request, libID, photoID, channelSlug, galleryTitle, opts = {}) {
    const body = {
        photoIDs: [photoID],
        channel: channelSlug,
        recordXMP: false,
        publishedAt: opts.publishedAt ?? '2026-01-15T12:00:00Z',
    };
    if (galleryTitle) body.galleryTitle = galleryTitle;
    const res = await request.post(`/api/library/${libID}/build`, { data: body, timeout: 90_000 });
    expect(res.status()).toBe(200);
    const evt = parseSseComplete(await res.text());
    expect(evt).toBeTruthy();
    return evt;
}

// Coverage for the cross-channel "Published Galleries" overview: the
// GET /api/channels/galleries aggregate endpoint merges SiteExport (site.json)
// and GalleryExport (gallery.json) channels into one unified list, and the
// #mode-published tab renders it with a live reachability check for rows
// that have a resolvable URL.
test.describe('Published Galleries overview', () => {
    let libID;
    let photoID;
    let galleryPostID;
    let sitePostID;

    test.beforeAll(async ({ request }) => {
        test.setTimeout(180_000);

        const libs = await (await request.get('/api/library/')).json();
        await Promise.all(
            libs.filter(l => l.name === 'E2E Published Galleries Library')
                .map(l => request.delete(`/api/library/${l.id}`)),
        );
        await request.delete(`/api/channels/${GALLERY_SLUG}`).catch(() => {});
        await request.delete(`/api/channels/${SITE_SLUG}`).catch(() => {});

        const libRes = await request.post('/api/library/', {
            data: { name: 'E2E Published Galleries Library', description: '', sourcePath: 'folder-b' },
        });
        expect(libRes.status()).toBe(201);
        libID = (await libRes.json()).id;
        await reindexLibrary(request, libID);

        const { results } = await (await request.get(`/api/library/search?ids=${libID}&limit=1`)).json();
        expect(results.length).toBeGreaterThanOrEqual(1);
        photoID = results[0].id;

        // GalleryExport channel with no SiteURL and no rsync host — no
        // resolvable URL, must show up without a live-status check.
        const galleryRes = await request.post('/api/channels/', {
            data: {
                slug: GALLERY_SLUG,
                name: 'E2E Published Gallery',
                format: 'jpeg',
                quality: 75,
                exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                galleryExport: true,
                outputMode: 'save',
            },
        });
        expect(galleryRes.status()).toBe(201);

        // SiteExport channel with a deliberately unreachable SiteURL, so the
        // live-status check has a deterministic "unreachable" outcome to assert on.
        const siteRes = await request.post('/api/channels/', {
            data: {
                slug: SITE_SLUG,
                name: 'E2E Published Site',
                format: 'jpeg',
                quality: 75,
                exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                siteExport: true,
                siteTitle: 'E2E Published Test Site',
                siteURL: 'http://127.0.0.1:1',
                outputMode: 'save',
            },
        });
        expect(siteRes.status()).toBe(201);

        const galEvt = await buildGallery(request, libID, photoID, GALLERY_SLUG, 'E2E Published Solo Gallery');
        galleryPostID = galEvt.postID;
        const siteEvt = await buildGallery(request, libID, photoID, SITE_SLUG, 'E2E Published Site Album');
        sitePostID = siteEvt.postID;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.delete(`/api/channels/${GALLERY_SLUG}`).catch(() => {});
        await request.delete(`/api/channels/${SITE_SLUG}`).catch(() => {});
    });

    test('GET /api/channels/galleries returns both channel types with expected fields', async ({ request }) => {
        const res = await request.get('/api/channels/galleries');
        expect(res.status()).toBe(200);
        const rows = await res.json();

        const galRow = rows.find(r => r.postID === galleryPostID);
        expect(galRow).toBeTruthy();
        expect(galRow.channelSlug).toBe(GALLERY_SLUG);
        expect(galRow.url ?? '').toBe(''); // no SiteURL, no rsync host — unresolvable

        const siteRow = rows.find(r => r.postID === sitePostID);
        expect(siteRow).toBeTruthy();
        expect(siteRow.channelSlug).toBe(SITE_SLUG);
        expect(siteRow.url).toContain('127.0.0.1:1');
    });

    test('tab renders a unified table with both gallery types and resolves live status', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');

        const galRow = page.locator(`tr[data-postid="${galleryPostID}"]`);
        const siteRow = page.locator(`tr[data-postid="${sitePostID}"]`);
        await expect(galRow).toBeVisible();
        await expect(siteRow).toBeVisible();

        // No resolvable URL — must never show a checking/live/unreachable status.
        await expect(galRow.locator('.pub-gal-status')).toHaveClass(/pub-gal-status--na/);

        // Deliberately unreachable SiteURL — must resolve to "Unreachable" within the check's timeout budget.
        await expect(siteRow.locator('.pub-gal-status')).toHaveClass(/pub-gal-status--down/, { timeout: 10_000 });
    });

    test('Edit renames a gallery via the prompt dialog', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');

        const galRow = page.locator(`tr[data-postid="${galleryPostID}"]`);
        await expect(galRow).toBeVisible();

        page.once('dialog', dialog => dialog.accept('Renamed Via E2E'));
        await galRow.locator('.pub-gal-edit').click();

        const renamedRow = page.locator(`tr[data-postid="${galleryPostID}"]`);
        await expect(renamedRow).toContainText('Renamed Via E2E');
    });

    test('Delete removes a gallery after confirmation', async ({ page, request }) => {
        // Build a disposable second gallery on the gallery-export channel so
        // this test's delete doesn't interfere with other tests' fixtures.
        const evt = await buildGallery(request, libID, photoID, GALLERY_SLUG, 'E2E Disposable Gallery');
        const disposablePostID = evt.postID;

        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');

        const row = page.locator(`tr[data-postid="${disposablePostID}"]`);
        await expect(row).toBeVisible();

        page.once('dialog', dialog => dialog.accept()); // confirm() for the delete itself
        await row.locator('.pub-gal-delete').click();

        await expect(page.locator(`tr[data-postid="${disposablePostID}"]`)).toHaveCount(0);

        const res = await request.get('/api/channels/galleries');
        const rows = await res.json();
        expect(rows.find(r => r.postID === disposablePostID)).toBeFalsy();
    });
});

// Coverage for the channels list's new links into the Published overview:
// a "Visit site" link to the resolved public URL, and a "Published" link
// that switches to #mode-published pre-filtered to that one channel.
test.describe('Channels list — links into Published overview', () => {
    const LINK_SITE_SLUG = 'e2e-published-links-site';

    test.beforeAll(async ({ request }) => {
        await request.delete(`/api/channels/${LINK_SITE_SLUG}`).catch(() => {});
        const res = await request.post('/api/channels/', {
            data: {
                slug: LINK_SITE_SLUG,
                name: 'E2E Published Links Site',
                format: 'jpeg',
                quality: 75,
                exifMode: 'strip',
                siteExport: true,
                siteURL: 'https://example.com',
                outputMode: 'save',
            },
        });
        expect(res.status()).toBe(201);
    });

    test.afterAll(async ({ request }) => {
        await request.delete(`/api/channels/${LINK_SITE_SLUG}`).catch(() => {});
    });

    test('channel row shows a Visit site link and a Published link that filters the overview', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-library');
        await page.click('#lib-channels-btn');

        const row = page.locator('.channel-row', { hasText: 'E2E Published Links Site' });
        await expect(row).toBeVisible();

        const visitLink = row.locator('.ch-visit-site');
        await expect(visitLink).toHaveAttribute('href', 'https://example.com');

        await row.locator('.ch-published').click();

        // No galleries have been published to this channel, so the filter
        // bar falls back to the channel slug (it has no row to read a name from).
        await expect(page.locator('#pub-gal-filter')).toBeVisible();
        await expect(page.locator('#pub-gal-filter')).toContainText(LINK_SITE_SLUG);
    });
});
