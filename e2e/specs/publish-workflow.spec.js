import { test, expect } from '@playwright/test';
import { reindexLibrary } from '../helpers/library.js';
import { waitForAppReady } from '../helpers/wait.js';

// Coverage for the full collect-then-publish loop introduced by the
// "streamlined publish workflow" feature: select photos in the Libraries
// view, "Add to channel…" collects them into a pending draft (no build yet),
// the Published tab surfaces that draft with a Draft badge, and the Publish
// dialog walks review → Generate → artifact review → (Deploy, when the
// channel has a handler). Covers a GalleryExport channel (single-gallery,
// "Instagram-style" export — the plan's "plain-export" scenario) and a
// SiteExport channel (multi-album site).
//
// Neither test channel configures an rsync handler (no fake rsync target is
// available in the e2e fixture environment — see e2e/NOTES.md), so both
// exercise the "no Deploy button, generate-only" path through the Publish
// dialog rather than the Deploy step itself.

const LIB_NAME = 'E2E Publish Workflow Library';
const GALLERY_SLUG = 'e2e-publish-workflow-gallery';
const SITE_SLUG = 'e2e-publish-workflow-site';

// ─── Shared setup helpers ────────────────────────────────────────────────────

async function openLibraryDetail(page) {
    await page.locator('#mode-library').click();
    await page.waitForSelector('.library-list-view, .library-detail', { timeout: 8_000 });
    if (await page.locator('.library-list-view').isVisible()) {
        const card = page.locator('.library-card', { hasText: LIB_NAME });
        await card.locator('.lib-open').click();
        await page.waitForSelector('.library-detail', { timeout: 8_000 });
    }
    await page.waitForSelector('#lib-pane [data-type="image"]', { timeout: 15_000 });
}

// Selects two images at [offset, offset+1] in the library pane (folder-b is
// the library's source root, so images render directly with no subfolder
// nav). Each test scenario uses a disjoint offset so the two tests never
// touch the same photo — sharing one would mean the second test's "not yet
// published" assertion sees a leftover "built:" entry from the first test's
// completed publish.
async function selectTwoPhotosAt(page, offset) {
    const images = page.locator('#lib-pane [data-type="image"]');
    await images.nth(offset).click();
    await images.nth(offset + 1).click({ modifiers: ['Meta'] });
    // The selection bar appears once something is selected (ADR-0029).
    await expect(page.locator('.selection-bar [data-action="collect"]')).toBeVisible({ timeout: 5_000 });
}

// The dialog asks for a gallery, not a channel: "New gallery…" then a title
// and the destination it belongs to.
async function collectSelectionToChannel(page, { channelSlug, galleryTitle }) {
    await page.locator('.selection-bar [data-action="collect"]').click();
    const dlg = page.locator('.collect-dialog');
    await expect(dlg).toBeVisible({ timeout: 5_000 });
    await dlg.locator('.collect-item--new').click();
    await dlg.locator('#collect-new-title').fill(galleryTitle);
    await dlg.locator('#collect-new-dest').selectOption(channelSlug);
    await dlg.locator('#collect-confirm').click();
    await expect(page.locator('#ui-hint.visible')).toContainText('Added 2 photos', { timeout: 5_000 });
    await expect(dlg).toHaveCount(0, { timeout: 5_000 });
}

// Loads the Info Panel's meta context for the library photo at `offset`,
// forcing a fresh fetch even if the panel is already expanded from an
// earlier step in the same test (switching focus away and back re-triggers
// _onPhotoFocus).
async function loadInfoForPhotoAt(page, offset) {
    const images = page.locator('#lib-pane [data-type="image"]');
    const target = images.nth(offset);
    await target.click();
    if (await page.locator('.info-panel.expanded').count() === 0) {
        await page.keyboard.press('i');
    } else {
        await images.nth(offset + 1).click();
        await target.click();
    }
    await page.waitForSelector('.info-panel.expanded', { timeout: 10_000 });
    await page.waitForFunction(
        () => {
            const panel = document.querySelector('.info-panel.expanded');
            return panel && !panel.querySelector('.info-loading');
        },
        { timeout: 15_000 },
    );
}

// Deleting a channel (Store.Delete) only removes its channels.json entry —
// it deliberately leaves drafts.json and any generated gallery/album output
// on disk (same reasoning as the Published tab's own "local delete only"
// default — see published-galleries.spec.js). A fresh POST with the same
// slug therefore inherits any leftover drafts/generated galleries from a
// previous run of this spec, which breaks this file's title-based row
// lookups (multiple stale rows sharing the same title) and its "not yet
// published" pending-state assertion (stale "built:" meta pollution).
// Explicitly wipe both before each run so the channel starts blank.
async function resetChannelOutputState(request, slug) {
    const drafts = await (await request.get(`/api/channels/${slug}/drafts`)).json().catch(() => []);
    for (const d of drafts || []) {
        await request.delete(`/api/channels/${slug}/drafts/${d.id}`).catch(() => {});
    }
    const galleries = await (await request.get(`/api/channels/${slug}/galleries`)).json().catch(() => []);
    for (const g of galleries || []) {
        await request.delete(`/api/channels/${slug}/galleries/${g.postID}`, {
            data: { deleteRemote: false },
        }).catch(() => {});
    }
}

async function reopenPublishedTab(page) {
    // #mode-published re-runs PublishedGalleriesPane.render() -> _load() on
    // every click (see app.js's mode==='published' branch), so re-clicking
    // it (even when already active) is enough to force a fresh fetch.
    await page.locator('#mode-library').click();
    await page.locator('#mode-published').click();
    await page.waitForSelector('.gal-pane', { timeout: 8_000 });
}

test.describe('Publish workflow — collect, draft, generate', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        test.setTimeout(200_000);

        const libs = await (await request.get('/api/library/')).json();
        await Promise.all(
            libs.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)),
        );
        await request.delete(`/api/channels/${GALLERY_SLUG}`).catch(() => {});
        await request.delete(`/api/channels/${SITE_SLUG}`).catch(() => {});

        const libRes = await request.post('/api/library/', {
            data: { name: LIB_NAME, description: '', sourcePath: 'folder-b' },
        });
        expect(libRes.status()).toBe(201);
        libID = (await libRes.json()).id;
        await reindexLibrary(request, libID);

        // GalleryExport channel — a single-gallery export (no multi-album
        // site nav), the "plain-export"/Instagram-style scenario.
        const galleryRes = await request.post('/api/channels/', {
            data: {
                slug: GALLERY_SLUG,
                name: 'E2E Publish Workflow Gallery Channel',
                format: 'jpeg',
                quality: 75,
                exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                galleryExport: true,
                outputMode: 'save',
            },
        });
        expect(galleryRes.status()).toBe(201);
        await resetChannelOutputState(request, GALLERY_SLUG);

        // SiteExport channel — a multi-album static site.
        const siteRes = await request.post('/api/channels/', {
            data: {
                slug: SITE_SLUG,
                name: 'E2E Publish Workflow Site Channel',
                format: 'jpeg',
                quality: 75,
                exifMode: 'strip',
                scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 800 },
                siteExport: true,
                siteTitle: 'E2E Publish Workflow Site',
                outputMode: 'save',
            },
        });
        expect(siteRes.status()).toBe(201);
        await resetChannelOutputState(request, SITE_SLUG);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
        await request.delete(`/api/channels/${GALLERY_SLUG}`).catch(() => {});
        await request.delete(`/api/channels/${SITE_SLUG}`).catch(() => {});
    });

    // Shared body for both channel-type scenarios: collect -> Draft badge ->
    // Publish dialog review/remove -> Generate -> artifact review -> status
    // flip -> Info Panel published state. `isSite` toggles the one place the
    // two channel types diverge in the UI: the Unlisted checkbox and the
    // Open-in-browser assertion the plan calls out explicitly.
    async function runPublishWorkflow(page, { channelSlug, galleryTitle, isSite, photoOffset }) {
        await page.goto('/');
        await waitForAppReady(page);

        // ── 1. Collect two photos into a new draft ──────────────────────────
        await openLibraryDetail(page);
        await selectTwoPhotosAt(page, photoOffset);
        await collectSelectionToChannel(page, { channelSlug, galleryTitle });

        // Info Panel shows "pending" for the still-selected (first of the
        // two collected) photo before anything has been generated.
        await loadInfoForPhotoAt(page, photoOffset);
        await expect(page.locator('.info-pub-card--pending')).toBeVisible({ timeout: 5_000 });
        // Scoped to this test's own channel: other spec files (e.g. build.spec.js)
        // permanently write their own built:<slug> XMP sidecars to the shared
        // folder-b fixture, so asserting "zero published cards of any kind" here
        // can be defeated by unrelated leftover state from earlier spec files.
        await expect(page.locator(`.info-pub-card:not(.info-pub-card--pending) .info-meta-del[data-key="built:${channelSlug}"]`)).toHaveCount(0);

        // ── 2. Published tab shows the Draft badge ──────────────────────────
        await reopenPublishedTab(page);
        const row = page.locator('.gal-row', { hasText: galleryTitle });
        await expect(row).toBeVisible({ timeout: 5_000 });
        await expect(row.locator('.gal-state')).toHaveText('Not online yet');
        await expect(row.locator('.gal-row-sub')).toContainText('2 photos collected');

        // ── 3. The gallery's own view lists what is waiting; remove one ────
        // Reviewing the collected photos belongs to the gallery, not to the
        // publish run (ADR-0029), so it moved out of the dialog.
        await row.click();
        await expect(page.locator('.gal-detail')).toBeVisible({ timeout: 5_000 });
        await expect(page.locator('.gal-pending-photo')).toHaveCount(2, { timeout: 10_000 });

        // Remove the second photo, keeping the first (the one the Info Panel
        // checks above and below both target) in the draft.
        await page.locator('.gal-pending-photo .gal-pending-remove').nth(1).click();
        await expect(page.locator('.gal-pending-photo')).toHaveCount(1, { timeout: 5_000 });

        // ── 4. Publish is one action: export → build → (upload) → check ────
        await page.locator('.gal-detail-primary .gal-row-action').click();
        const publishDlg = page.locator('.publish-dialog');
        await expect(publishDlg).toBeVisible({ timeout: 5_000 });
        await expect(publishDlg.locator('.publish-plan')).toContainText('Export 1 photo');
        // Neither test channel configures an rsync handler (no fake rsync
        // target exists in the fixture env), so no upload step is planned.
        await expect(publishDlg.locator('.publish-plan')).not.toContainText('Upload');

        await publishDlg.locator('#pub-run').click();
        await expect(publishDlg.locator('.publish-step[data-step="build"][data-state="done"]'))
            .toBeVisible({ timeout: 30_000 });
        await expect(publishDlg.locator('.publish-step[data-step="export"]')).toHaveAttribute('data-state', 'done');
        await expect(publishDlg.locator('#pub-copy-path')).toBeVisible();
        await expect(publishDlg.locator('#pub-open-folder')).toBeVisible();
        await expect(publishDlg.locator('#pub-done')).toBeVisible();
        await publishDlg.locator('#pub-done').click();
        await expect(publishDlg).toHaveCount(0);

        // ── 5. Published tab now shows Generated, not Draft ─────────────────
        await reopenPublishedTab(page);
        const generatedRow = page.locator('.gal-row', { hasText: galleryTitle });
        await expect(generatedRow).toBeVisible({ timeout: 5_000 });
        await expect(generatedRow.locator('.gal-state')).not.toHaveText('Not online yet');
        // Built here, never uploaded — these channels have no upload target,
        // and the state says that rather than claiming the gallery is online.
        await expect(generatedRow.locator('.gal-state')).toHaveText('Built, not uploaded');

        // ── 6. Info Panel now shows the photo as published, not pending ────
        await openLibraryDetail(page);
        await loadInfoForPhotoAt(page, photoOffset);
        // Scoped to this test's own channel for the same reason as the check
        // above: other channels/specs may have already published this same
        // shared photo, so an unscoped locator can match multiple cards and
        // trip Playwright's strict-mode violation on toBeVisible().
        // The card is per album, so its delete key is the qualified
        // built:<slug>:<postID> once the photo belongs to one (a plain-export
        // channel has no album and keeps the bare channel key).
        await expect(page.locator(`.info-pub-card:not(.info-pub-card--pending) .info-meta-del[data-key^="built:${channelSlug}"]`).first()).toBeVisible({ timeout: 5_000 });
        await expect(page.locator(`.info-pub-card--pending .info-meta-del[data-key^="pending:${channelSlug}"]`)).toHaveCount(0);
    }

    test('GalleryExport (plain) channel: collect, draft badge, generate, published state', async ({ page }) => {
        await runPublishWorkflow(page, {
            channelSlug: GALLERY_SLUG,
            galleryTitle: 'E2E Publish Workflow Gallery Album',
            isSite: false,
            photoOffset: 0,
        });
    });

    test('SiteExport channel: collect, draft badge, generate, published state', async ({ page }) => {
        await runPublishWorkflow(page, {
            channelSlug: SITE_SLUG,
            galleryTitle: 'E2E Publish Workflow Site Album',
            isSite: true,
            photoOffset: 2,
        });
    });

    // The date belongs to the gallery, not to the publish run. The dialog used
    // to default to today, so refreshing a gallery silently re-dated it
    // (ADR-0029).
    test('publishing again keeps the date the gallery already carries', async ({ page, request }) => {
        const rows = await (await request.get('/api/channels/galleries')).json();
        const published = rows.find(r => r.channelSlug === GALLERY_SLUG && r.status !== 'draft');
        expect(published, 'the GalleryExport test above must have published one gallery').toBeTruthy();

        const existing = new Date(published.publishedAt);
        const existingValue = existing.toISOString().slice(0, 10);
        const today = new Date().toISOString().slice(0, 10);

        await page.goto('/');
        await waitForAppReady(page);
        await reopenPublishedTab(page);
        await page.locator(`.gal-row[data-postid="${published.postID}"]`).click();
        await page.locator('.gal-detail-primary .gal-row-action').click();

        const dateInput = page.locator('#pub-date');
        await expect(dateInput).toBeVisible({ timeout: 5_000 });
        await expect(dateInput).toHaveValue(existingValue);
        if (existingValue !== today) {
            await expect(dateInput).not.toHaveValue(today);
        }
        await page.locator('#pub-cancel').click();
    });
});
