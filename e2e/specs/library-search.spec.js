import path from 'path';
import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

const FIXTURES_PATH = path.resolve(new URL('../fixtures', import.meta.url).pathname);

// ─── Search panel — no EXIF data ─────────────────────────────────────────────

test.describe('Search panel — no EXIF data', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const res = await request.post('/api/library/', {
            data: { name: 'E2E Empty Library', description: '', sourcePath: '/tmp' },
        });
        expect(res.status()).toBe(201);
        const body = await res.json();
        libID = body.id;
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        await page.locator('#mode-library').click();
        await page.waitForSelector('.library-list-view', { timeout: 8_000 });
        await page.locator('#lib-search-btn').click();
        await page.waitForSelector('#lib-search-panel.visible', { timeout: 5_000 });
        // Panel builds asynchronously; wait for the library selector to appear
        await page.waitForSelector('.lib-search-select', { timeout: 20_000 });
        // Scope to the empty test library to isolate from production libraries
        await page.locator('.lib-search-select').first().selectOption(String(libID));
        // Wait for the panel to rebuild with scoped results
        await page.waitForFunction(
            () => {
                const el = document.querySelector('.lib-search-status');
                return el && el.textContent.includes('0 photo');
            },
            { timeout: 15_000 },
        );
    });

    test('search panel opens with library selector and reset button', async ({ page }) => {
        await expect(page.locator('.lib-search-select').first()).toBeVisible();
        await expect(page.locator('.lib-search-reset')).toBeVisible();
    });

    test('shows "No numeric EXIF data" for unindexed library', async ({ page }) => {
        await expect(page.locator('.lib-filter-groups').first()).toContainText(
            'No numeric EXIF data — re-index the library to populate.',
        );
    });

    test('status shows 0 photos match for empty library', async ({ page }) => {
        await expect(page.locator('.lib-search-status')).toContainText('0');
    });

    test('closing search panel hides it', async ({ page }) => {
        await page.locator('#lib-search-btn').click();
        await expect(page.locator('#lib-search-panel')).not.toHaveClass(/visible/, { timeout: 3_000 });
        await expect(page.locator('#lib-search-btn')).not.toHaveClass(/active/);
    });
});

// ─── All tests that require an indexed library ────────────────────────────────
// Single outer describe so the indexed library is created once and shared across
// the search panel, detail view filter, and API contract sub-groups.

test.describe('Library search with indexed fixtures', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const res = await request.post('/api/library/', {
            data: { name: 'E2E Indexed Library', description: '', sourcePath: FIXTURES_PATH },
        });
        expect(res.status()).toBe(201);
        const body = await res.json();
        libID = body.id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    // Helper used by beforeEach hooks below to wait for the panel status to settle
    async function waitForSearchStatus(page) {
        await page.waitForFunction(
            () => {
                const el = document.querySelector('.lib-search-status');
                return el && el.textContent.includes('match');
            },
            { timeout: 10_000 },
        );
    }

    // ── Search panel in list view ────────────────────────────────────────────

    test.describe('Search panel in list view', () => {
        test.beforeEach(async ({ page }) => {
            await page.goto('/');
            await waitForAppReady(page);
            await page.locator('#mode-library').click();
            await page.waitForSelector('.library-list-view', { timeout: 8_000 });
            await page.locator('#lib-search-btn').click();
            await page.waitForSelector('#lib-search-panel.visible', { timeout: 5_000 });
            // Wait for async panel build, then scope to the indexed test library
            await page.waitForSelector('.lib-search-select', { timeout: 20_000 });
            await waitForSearchStatus(page);
            const prevStatus = await page.locator('.lib-search-status').textContent();
            await page.locator('.lib-search-select').first().selectOption(String(libID));
            // Wait for the panel to rebuild with scoped results
            await page.waitForFunction(
                (prev) => document.querySelector('.lib-search-status')?.textContent !== prev,
                prevStatus,
                { timeout: 10_000 },
            );
        });

        test('range sliders render when EXIF data exists', async ({ page }) => {
            await expect(page.locator('.lib-range-slider').first()).toBeVisible();
        });

        test('range sliders show formatted range labels', async ({ page }) => {
            const label = page.locator('.lib-filter-range-display').first();
            await expect(label).toBeVisible();
            const text = await label.textContent();
            expect(text.trim().length).toBeGreaterThan(0);
        });

        test('status shows total photo count', async ({ page }) => {
            const countText = await page.locator('.lib-search-status strong').textContent();
            expect(parseInt(countText, 10)).toBeGreaterThanOrEqual(3);
        });

        test('search breadcrumb shows photo count in results area', async ({ page }) => {
            await expect(page.locator('.search-breadcrumb')).toContainText('Search results');
            await expect(page.locator('.search-breadcrumb')).toContainText('photo');
        });

        test('library selector has "All libraries" option', async ({ page }) => {
            const allOption = page.locator('.lib-search-select option[value=""]').first();
            await expect(allOption).toHaveText('All libraries');
        });

        test('library selector contains the test library name', async ({ page }) => {
            await expect(page.locator('.lib-search-select').first()).toContainText('E2E Indexed Library');
        });

        test('text filter dropdowns appear for libraries with multiple camera models', async ({ page }) => {
            // Fixtures: Nikon COOLPIX P6000 + Canon EOS 40D → 2 distinct models → dropdown visible
            await expect(page.locator('.lib-text-filter-select').first()).toBeVisible({ timeout: 5_000 });
        });

        test('selecting a camera model filters results to a subset', async ({ page }) => {
            const status = page.locator('.lib-search-status strong');
            const total = parseInt(await status.textContent(), 10);

            const sel = page.locator('.lib-text-filter-select').first();
            await sel.selectOption({ index: 1 }); // first real model, not "All"

            await page.waitForFunction(
                (prev) => {
                    const el = document.querySelector('.lib-search-status strong');
                    return el && parseInt(el.textContent, 10) !== prev;
                },
                total,
                { timeout: 5_000 },
            );

            const filtered = parseInt(await status.textContent(), 10);
            expect(filtered).toBeGreaterThan(0);
            expect(filtered).toBeLessThan(total);
        });

        test('Reset button restores full photo count after filtering', async ({ page }) => {
            const status = page.locator('.lib-search-status strong');
            const total = parseInt(await status.textContent(), 10);

            const sel = page.locator('.lib-text-filter-select').first();
            await sel.selectOption({ index: 1 });
            await page.waitForFunction(
                (prev) => {
                    const el = document.querySelector('.lib-search-status strong');
                    return el && parseInt(el.textContent, 10) !== prev;
                },
                total,
                { timeout: 5_000 },
            );

            await page.locator('.lib-search-reset').click();
            await page.waitForFunction(
                (prev) => {
                    const el = document.querySelector('.lib-search-status strong');
                    return el && parseInt(el.textContent, 10) === prev;
                },
                total,
                { timeout: 5_000 },
            );
            await expect(status).toHaveText(String(total));
        });

        test('focal length 35mm equivalent checkbox is present', async ({ page }) => {
            await expect(
                page.locator('.lib-filter-35mm input[type="checkbox"]'),
            ).toBeVisible({ timeout: 5_000 });
        });
    });

    // ── Filter panel in library detail view ─────────────────────────────────

    test.describe('Filter panel in library detail view', () => {
        test.beforeEach(async ({ page }) => {
            await page.goto('/');
            await waitForAppReady(page);
            await page.locator('#mode-library').click();
            await page.waitForSelector('.library-list-view', { timeout: 8_000 });
            const card = page.locator('.library-card', { hasText: 'E2E Indexed Library' });
            await card.locator('.lib-open').click();
            await page.waitForSelector('.library-detail', { timeout: 8_000 });
        });

        test('Filter button opens the filter panel in detail view', async ({ page }) => {
            await page.locator('#lib-filter-btn').click();
            await expect(page.locator('#lib-search-panel')).toHaveClass(/visible/, { timeout: 5_000 });
            await expect(page.locator('#lib-filter-btn')).toHaveClass(/active/);
        });

        test('filter panel in detail view shows sliders', async ({ page }) => {
            await page.locator('#lib-filter-btn').click();
            await page.waitForFunction(
                () => document.querySelector('.lib-range-slider') !== null,
                { timeout: 8_000 },
            );
            await expect(page.locator('.lib-range-slider').first()).toBeVisible();
        });

        test('closing Filter panel restores button to inactive state', async ({ page }) => {
            await page.locator('#lib-filter-btn').click();
            await page.waitForSelector('#lib-search-panel.visible', { timeout: 5_000 });

            await page.locator('#lib-filter-btn').click();
            await expect(page.locator('#lib-search-panel')).not.toHaveClass(/visible/, { timeout: 3_000 });
            await expect(page.locator('#lib-filter-btn')).not.toHaveClass(/active/);
        });
    });

    // ── API contracts ────────────────────────────────────────────────────────

    test.describe('API contracts', () => {
        test('GET /api/library/search returns results array and total', async ({ request }) => {
            const res = await request.get('/api/library/search?limit=10');
            expect(res.status()).toBe(200);
            const body = await res.json();
            expect(Array.isArray(body.results)).toBe(true);
            expect(typeof body.total).toBe('number');
        });

        test('search with ids= scopes to a single library', async ({ request }) => {
            const res = await request.get(`/api/library/search?ids=${libID}&limit=50`);
            expect(res.status()).toBe(200);
            const body = await res.json();
            expect(body.total).toBeGreaterThanOrEqual(3);
        });

        test('ISO range filter scopes results to matching photos', async ({ request }) => {
            const allRes = await request.get(`/api/library/search?ids=${libID}&limit=1`);
            const allTotal = (await allRes.json()).total;

            // Canon EOS 40D = ISO 100, Nikon COOLPIX P6000 = ISO 64
            // Filtering ISO 90–110 should return only Canon photos (fewer than total)
            const res = await request.get(
                `/api/library/search?ids=${libID}&ISOSpeedRatings_min=90&ISOSpeedRatings_max=110`,
            );
            expect(res.status()).toBe(200);
            const body = await res.json();
            expect(body.total).toBeGreaterThanOrEqual(1);
            expect(body.total).toBeLessThan(allTotal);
        });

        test('GET /api/library/{id}/exif-ranges returns numeric ranges', async ({ request }) => {
            const res = await request.get(`/api/library/${libID}/exif-ranges`);
            expect(res.status()).toBe(200);
            const body = await res.json();
            const hasAnyRange = ['FocalLength', 'ISOSpeedRatings', 'ExposureTime', 'FNumber'].some(
                (k) => body[k] && typeof body[k].min === 'number' && typeof body[k].max === 'number',
            );
            expect(hasAnyRange).toBe(true);
        });

        test('GET /api/library/exif-values returns distinct camera models', async ({ request }) => {
            const res = await request.get(`/api/library/exif-values?field=Model&ids=${libID}`);
            expect(res.status()).toBe(200);
            const body = await res.json();
            expect(Array.isArray(body)).toBe(true);
            const models = body.map((m) => (typeof m === 'string' ? m : String(m)));
            expect(models.some((m) => m.includes('COOLPIX'))).toBe(true);
            expect(models.some((m) => m.includes('Canon'))).toBe(true);
        });

        test('GET /api/library/search with Model filter scopes results', async ({ request }) => {
            const allRes = await request.get(`/api/library/search?ids=${libID}&limit=1`);
            const allTotal = (await allRes.json()).total;

            const res = await request.get(
                `/api/library/search?ids=${libID}&Model=COOLPIX+P6000`,
            );
            expect(res.status()).toBe(200);
            const body = await res.json();
            expect(body.total).toBeGreaterThan(0);
            expect(body.total).toBeLessThan(allTotal);
        });
    });
});
