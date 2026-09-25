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

// Collect one photo into a draft for a gallery/album channel, then generate
// it. Returns the complete SSE event from generate — same shape as the old
// single-step /build endpoint's SSE.
async function buildGallery(request, libID, photoID, channelSlug, galleryTitle, opts = {}) {
    const collectBody = { photoIDs: [photoID] };
    if (galleryTitle) collectBody.title = galleryTitle;

    const collectRes = await request.post(`/api/library/${libID}/channels/${channelSlug}/drafts`, {
        data: collectBody,
        timeout: 30_000,
    });
    expect(collectRes.status()).toBe(200);
    const draft = await collectRes.json();

    const res = await request.post(`/api/channels/${channelSlug}/drafts/${draft.id}/generate`, {
        data: { publishedAt: opts.publishedAt ?? '2026-01-15T12:00:00Z' },
        timeout: 90_000,
    });
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
        // A website keeps its album pages under albums/<slug>/.
        expect(siteRow.url).toContain('/albums/');
    });

    test('Galleries groups rows by destination and layers the link check on the state', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');

        const galRow = page.locator(`.gal-row[data-postid="${galleryPostID}"]`);
        const siteRow = page.locator(`.gal-row[data-postid="${sitePostID}"]`);
        await expect(galRow).toBeVisible();
        await expect(siteRow).toBeVisible();

        // Each destination is its own group, with the row under its heading.
        await expect(page.locator('.gal-group', { hasText: 'E2E Published Solo Gallery' })
            .locator('.gal-group-name')).toHaveText('E2E Published Gallery');

        // Neither test channel has an upload configured, so both galleries are
        // built and not uploaded — the state says so instead of claiming they
        // are online (ADR-0029).
        await expect(galRow.locator('.gal-state')).toHaveText('Built, not uploaded');
        await expect(siteRow.locator('.gal-state')).toHaveText('Built, not uploaded');

        // A deliberately dead SiteURL: the failed check is reported next to
        // the state rather than replacing it (ADR-0029).
        await expect(siteRow.locator('.gal-check--down')).toContainText('Not reachable', { timeout: 10_000 });
        await expect(siteRow.locator('.gal-row-action')).toHaveText('Check again');
    });

    test('an Online gallery with nothing pending offers no action in its row', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');

        const galRow = page.locator(`.gal-row[data-postid="${galleryPostID}"]`);
        await expect(galRow).toBeVisible();
        // "Publish" used to sit on every row, including rows with nothing to
        // publish — and re-dated the gallery when pressed (ADR-0029).
        await expect(galRow.locator('.gal-row-action')).toHaveCount(0);
    });

    test('the detail view renames a gallery and toggles its visibility', async ({ page, request }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');

        await page.locator(`.gal-row[data-postid="${galleryPostID}"]`).click();
        await expect(page.locator('.gal-detail')).toBeVisible();

        await page.locator('#gal-title-input').fill('Renamed Via E2E');
        await page.locator('#gal-title-save').click();
        await expect(page.locator('.gal-crumb-here')).toHaveText('Renamed Via E2E');

        // Unlisted is editable here because a single-gallery album's folder is
        // the random postID either way — only the noindex tag changes. Three
        // visible labels, as every toggle carries (ADR-0019).
        const toggle = page.locator('#gal-visibility .toggle');
        await expect(toggle.locator('.toggle-label-on')).toHaveText('Hidden');
        await expect(toggle.locator('.toggle-label-off')).toHaveText('Allowed');
        await toggle.click();
        await expect(toggle).toHaveAttribute('aria-checked', 'true');

        await expect.poll(async () => {
            const rows = await (await request.get('/api/channels/galleries')).json();
            return rows.find(r => r.postID === galleryPostID)?.unlisted;
        }).toBe(true);
    });

    test('Unpublish confirms in place and removes the gallery', async ({ page, request }) => {
        // A disposable second gallery, so this test's delete leaves the other
        // tests' fixtures alone.
        const evt = await buildGallery(request, libID, photoID, GALLERY_SLUG, 'E2E Disposable Gallery');
        const disposablePostID = evt.postID;

        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');

        await page.locator(`.gal-row[data-postid="${disposablePostID}"]`).click();
        await page.locator('#gal-remove').click();

        // Inline confirmation naming the object, no browser confirm() dialog.
        await expect(page.locator('.gal-danger-question')).toContainText('E2E Disposable Gallery');
        await page.locator('#gal-remove-confirm').click();

        await expect(page.locator(`.gal-row[data-postid="${disposablePostID}"]`)).toHaveCount(0);

        const rows = await (await request.get('/api/channels/galleries')).json();
        expect(rows.find(r => r.postID === disposablePostID)).toBeFalsy();
    });

    // Unpublishing takes a moment (the sidecars of every photo are cleaned), so
    // the confirmation says it is working (like a library scan does) and cannot be
    // triggered a second time.
    test('Unpublish shows that it is working and cannot be clicked twice', async ({ page, request }) => {
        const evt = await buildGallery(request, libID, photoID, GALLERY_SLUG, 'E2E Busy Gallery');

        // Hold the request until the assertions are done.
        let release;
        const gate = new Promise(r => { release = r; });
        let deleteCalls = 0;
        await page.route('**/api/channels/*/galleries/*', async route => {
            if (route.request().method() !== 'DELETE') return route.continue();
            deleteCalls++;
            await gate;
            await route.continue();
        });

        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-published');
        await page.locator(`.gal-row[data-postid="${evt.postID}"]`).click();
        await page.locator('#gal-remove').click();
        await page.locator('#gal-remove-confirm').click();

        // Same quiet status line as a library scan, and the controls are locked.
        const status = page.locator('#gal-danger-status');
        await expect(status).toBeVisible();
        await expect(status).toContainText('Unpublishing');
        await expect(page.locator('#gal-remove-confirm')).toBeDisabled();
        await expect(page.locator('#gal-remove-cancel')).toBeDisabled();

        // A second click does nothing.
        await page.locator('#gal-remove-confirm').click({ force: true });
        release();

        await expect(page.locator(`.gal-row[data-postid="${evt.postID}"]`)).toHaveCount(0);
        expect(deleteCalls).toBe(1);
    });

    // The destinations list links into Galleries: the address opens the public
    // site, and the gallery count opens Galleries filtered to that destination.
    test('the destinations list links to the public site and into Galleries', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-destinations');

        const siteRow = page.locator('.dest-row', { hasText: 'E2E Published Site' });
        await expect(siteRow).toBeVisible({ timeout: 8_000 });
        await expect(siteRow.locator('.dest-visit')).toHaveAttribute('href', 'http://127.0.0.1:1');

        const galRow = page.locator('.dest-row', { hasText: 'E2E Published Gallery' });
        await galRow.locator('.dest-galleries-link').click();
        await expect(page.locator('#gal-filter')).toContainText('E2E Published Gallery');
    });

    test('opening a destination shows its type first, and the type is fixed', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.click('#mode-destinations');

        await page.locator('.dest-row', { hasText: 'E2E Published Site' }).click();
        const form = page.locator('#dest-form');
        await expect(form).toBeVisible();
        await expect(form).toHaveAttribute('data-type', 'site');
        // A site's type decides its layout and its links, so it cannot change.
        await expect(form.locator('input[name="dest-type"]').first()).toBeDisabled();
        // Only the sections this type needs: a website has no "Output" section.
        await expect(form.locator('.dest-when-files')).toBeHidden();
        await expect(form.locator('.dest-when-site')).toBeVisible();
    });
});
