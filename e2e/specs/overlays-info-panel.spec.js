import { test, expect } from '@playwright/test';
import { waitForAppReady, waitForThumbnailsLoaded } from '../helpers/wait.js';
import { GPS_IMAGE, navigateToFolder } from '../helpers/fixtures.js';

// Phase 7 of the Rams redesign: thumbnail chips lose their per-format and
// per-film-simulation colours (ADR-0030's recorded deviation), the view
// switches become visible toolbar controls, and the info panel reads short
// list → map → all metadata.

test.describe('Thumbnail overlays', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);
    });

    test('every chip is the same dark chip, with no colour of its own', async ({ page }) => {
        const badges = page.locator('.overlay-badge');
        await expect(badges.first()).toBeVisible({ timeout: 10_000 });

        const styles = await badges.evaluateAll(els => els.map(el => ({
            inline: el.getAttribute('style'),
            background: getComputedStyle(el).backgroundColor,
            family: getComputedStyle(el).fontFamily,
        })));
        expect(styles.length).toBeGreaterThan(0);
        for (const s of styles) {
            // A colour per format used to be set inline on each chip.
            expect(s.inline).toBeNull();
            expect(s.family).toContain('IBM Plex Mono');
        }
        // One treatment for all of them, whatever they say.
        expect(new Set(styles.map(s => s.background)).size).toBe(1);
    });

    test('the switches for names and details sit in the ⋯ menu', async ({ page }) => {
        await page.locator('.browse-more .menu-btn').click();
        const details = page.locator('.menu [data-id="details"]');
        await expect(page.locator('.menu [data-id="names"] .menu-label')).toHaveText('Names');
        await expect(details.locator('.menu-label')).toHaveText('Details');

        // Three visible labels per switch (ADR-0019): purpose and both states.
        await expect(details.locator('.toggle-label-on')).toHaveText('Shown');
        await expect(details.locator('.toggle-label-off')).toHaveText('Hidden');
        await expect(details).toHaveAttribute('aria-checked', 'true');

        await details.click();
        await expect(page.locator('.overlay-badge')).toHaveCount(0);
        await expect(details).toHaveAttribute('aria-checked', 'false');
        await details.click();
        await expect(page.locator('.overlay-badge').first()).toBeVisible();
    });

    test('nothing on the grid animates forever', async ({ page }) => {
        const animated = await page.evaluate(() =>
            [...document.querySelectorAll('.grid-item, .justified-item, .grid-item::after')]
                .map(el => getComputedStyle(el, '::after').animationIterationCount)
                .filter(v => v === 'infinite').length);
        expect(animated).toBe(0);
    });
});

test.describe('Info panel', () => {
    // Note: no addInitScript for the remembered state — it would re-run on
    // every navigation and quietly undo what the persistence test asserts.
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await page.evaluate(() => localStorage.setItem('info-all-metadata-open', '0'));
        await page.goto('/');
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);
        await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
        await page.keyboard.press('i');
        await expect(page.locator('.info-panel.expanded')).toBeVisible({ timeout: 8_000 });
        // The panel renders empty until the photo's info arrives; allInnerTexts
        // and friends do not retry, so wait for content before reading it.
        await expect(page.locator('.info-all-meta')).toBeAttached({ timeout: 10_000 });
    });

    test('reads short list, then the map, then all metadata', async ({ page }) => {
        const titles = await page.locator('.info-section-title, .info-all-meta summary').allInnerTexts();
        // Section titles are uppercased by CSS; compare what they say.
        const order = titles.map(t => t.replace(/[▸▾]/g, '').trim().toLowerCase());
        expect(order[0]).toBe('photo');
        expect(order[1]).toBe('location');
        expect(order).toContain('all metadata');

        // The map is right there for a photo that has coordinates.
        await expect(page.locator('#info-map')).toBeVisible();
    });

    test('all metadata is folded away, grouped, and remembers being open', async ({ page }) => {
        const details = page.locator('.info-all-meta');
        await expect(details).not.toHaveAttribute('open', '');

        await details.locator('summary').click();
        const groups = (await page.locator('.info-meta-group-title').allInnerTexts())
            .map(g => g.trim().toLowerCase());
        expect(groups).toContain('camera');
        expect(groups).toContain('exposure');
        expect(groups).toContain('file');

        // Opening it once keeps it open for the next photo, and after a reload.
        // <details> fires its toggle event asynchronously, and that event is
        // what persists the state — so wait for it rather than reloading into
        // a race the app would never lose in real use.
        await expect.poll(() => page.evaluate(() => localStorage.getItem('info-all-metadata-open')))
            .toBe('1');

        await page.reload();
        await waitForAppReady(page);
        await navigateToFolder(page, 'folder-b');
        await waitForThumbnailsLoaded(page, 1);
        await page.locator(`[data-name="${GPS_IMAGE}"]`).click();
        await page.keyboard.press('i');
        await expect(page.locator('.info-all-meta')).toHaveAttribute('open', '');
    });

    // The map was loaded from an unversioned CDN URL that a major release of
    // MapLibre emptied out; the panel then showed a blank box that looked like
    // a photo without a location (ADR-0031).
    test('the map library is served by the app itself, and the map renders', async ({ page }) => {
        expect(await page.evaluate(() => typeof maplibregl)).toBe('object');

        // MapLibre 6 arrives through a module import (ADR-0038), so check what
        // the browser actually fetched: every MapLibre file, from this app.
        const maplibreFiles = await page.evaluate(() => performance.getEntriesByType('resource')
            .map(e => e.name).filter(n => n.includes('maplibre')));
        expect(maplibreFiles.some(n => n.endsWith('/maplibre-gl.mjs'))).toBe(true);
        const origin = await page.evaluate(() => location.origin);
        expect(maplibreFiles.every(n => n.startsWith(origin + '/'))).toBe(true);

        await expect(page.locator('#info-map canvas')).toBeVisible({ timeout: 15_000 });
        await expect(page.locator('#info-map')).not.toHaveClass(/info-map-unavailable/);
        // Tiles are drawn in a worker, which the entry module loads from next to itself.
        await expect.poll(() => page.evaluate(() => performance.getEntriesByType('resource')
            .some(e => e.name.endsWith('/maplibre-gl-worker.mjs'))), { timeout: 10_000 }).toBe(true);
    });

    test('the short list carries what you cull by', async ({ page }) => {
        const photo = page.locator('.info-section').first();
        await expect(photo).toContainText('Name');
        await expect(photo).toContainText('Format');
        // Exposure is decoded, not the raw EXIF rational.
        const text = await photo.innerText();
        expect(text).not.toContain('"');
    });
});
