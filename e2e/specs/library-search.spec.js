import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// ─── Filter panel — no EXIF data ─────────────────────────────────────────────

test.describe('Filter panel — no EXIF data', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === 'E2E Empty Library').map(l => request.delete(`/api/library/${l.id}`)));

        const res = await request.post('/api/library/', {
            data: { name: 'E2E Empty Library', description: '', sourcePath: 'folder-a' },
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
        await page.waitForSelector('.lib-search-select', { timeout: 20_000 });
        await page.locator('.lib-search-select').first().selectOption(String(libID));
        await page.waitForFunction(
            () => {
                const el = document.querySelector('.lib-search-status');
                return el && el.textContent.includes('0 photo');
            },
            { timeout: 15_000 },
        );
    });

    test('filter panel opens with library selector and reset button', async ({ page }) => {
        await expect(page.locator('.lib-search-select').first()).toBeVisible();
        await expect(page.locator('.lib-search-reset')).toBeVisible();
    });

    test('shows "No numeric EXIF data" for unindexed library', async ({ page }) => {
        await expect(page.locator('.lib-filter-groups', { hasText: 'No numeric EXIF data' })).toContainText(
            'No numeric EXIF data — re-index the library to populate.',
        );
    });

    test('status shows 0 photos match for empty library', async ({ page }) => {
        await expect(page.locator('.lib-search-status')).toContainText('0');
    });

    test('closing filter panel hides it', async ({ page }) => {
        await page.locator('#lib-search-btn').click();
        await expect(page.locator('#lib-search-panel')).not.toHaveClass(/visible/, { timeout: 3_000 });
        await expect(page.locator('#lib-search-btn')).toHaveAttribute('data-state', 'off');
    });
});

// ─── All tests that require an indexed library ────────────────────────────────

test.describe('Library search with indexed fixtures', () => {
    let libID;

    test.beforeAll(async ({ request }) => {
        test.setTimeout(200_000); // indexing 79 images can take up to 3 minutes under load
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === 'E2E Indexed Library').map(l => request.delete(`/api/library/${l.id}`)));

        const res = await request.post('/api/library/', {
            data: { name: 'E2E Indexed Library', description: '', sourcePath: '/' },
        });
        expect(res.status()).toBe(201);
        const body = await res.json();
        libID = body.id;
        await reindexLibrary(request, libID);
    });

    test.afterAll(async ({ request }) => {
        if (libID) await request.delete(`/api/library/${libID}`);
    });

    async function waitForSearchStatus(page) {
        await page.waitForFunction(
            () => {
                const el = document.querySelector('.lib-search-status');
                return el && el.textContent.includes('match');
            },
            { timeout: 10_000 },
        );
    }

    // ── Filter panel in list view ────────────────────────────────────────────

    test.describe('Filter panel in list view', () => {
        test.beforeEach(async ({ page }) => {
            await page.goto('/');
            await waitForAppReady(page);
            await page.locator('#mode-library').click();
            await page.waitForSelector('.library-list-view', { timeout: 8_000 });
            await page.locator('#lib-search-btn').click();
            await page.waitForSelector('#lib-search-panel.visible', { timeout: 5_000 });
            await page.waitForSelector('.lib-search-select', { timeout: 20_000 });
            await waitForSearchStatus(page);
            await page.locator('.lib-search-select').first().selectOption(String(libID));
            // waitForSearchStatus would pass immediately (status already says "match" from the
            // initial global search). Wait for the library-specific response instead so that the
            // count shown in the DOM belongs to this library, not all libraries combined.
            await page.waitForResponse(
                res => res.url().includes('/api/library/search')
                    && new URL(res.url()).searchParams.get('ids') === String(libID)
                    && res.status() === 200,
                { timeout: 15_000 },
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
            // src/examples has 79 images; indexer may skip unsupported formats
            expect(parseInt(countText, 10)).toBeGreaterThan(50);
        });

        test('filter breadcrumb shows photo count in results area', async ({ page }) => {
            await expect(page.locator('.search-breadcrumb')).toContainText('Filter results');
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
            // src/examples: X-T50, iPhone 12 Pro, Canon EOS 200D, Canon EOS 500D, etc. → multiple models → dropdown visible
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

            // Register before clicking so the response is never missed.
            // Match only library-scoped reset queries (ids=libID, no text filter params).
            const resetResponse = page.waitForResponse(
                res => res.url().includes('/api/library/search')
                    && new URL(res.url()).searchParams.get('ids') === String(libID)
                    && !new URL(res.url()).searchParams.has('Model')
                    && !new URL(res.url()).searchParams.has('LensModel')
                    && !new URL(res.url()).searchParams.has('FilmSimulation')
                    && res.status() === 200,
                { timeout: 30_000 },
            );
            await page.locator('.lib-search-reset').click();
            await resetResponse;
            await expect(status).toHaveText(String(total));
        });

        test('focal length 35mm equivalent checkbox is present', async ({ page }) => {
            await expect(
                page.locator('.lib-filter-35mm [role="switch"]'),
            ).toBeVisible({ timeout: 5_000 });
        });

        test('arrow key moves focus through filter results', async ({ page }) => {
            // Results render in justified view; photos use .justified-item
            const results = page.locator('#lib-search-results-area [data-type="image"]');
            await expect(results.first()).toBeVisible({ timeout: 10_000 });
            // loadResults sets focusedIndex=0 so first item is already focused
            await expect(page.locator('#lib-search-results-area .focused')).toHaveCount(1, { timeout: 3_000 });
            // ArrowRight moves focus to second item — still exactly 1 focused
            await page.keyboard.press('ArrowRight');
            await expect(page.locator('#lib-search-results-area .focused')).toHaveCount(1);
        });

        test('i key opens info panel in list-view filter results', async ({ page }) => {
            const results = page.locator('#lib-search-results-area [data-type="image"]');
            await expect(results.first()).toBeVisible({ timeout: 10_000 });
            await page.keyboard.press('i');
            await expect(page.locator('.info-panel.expanded')).toBeVisible({ timeout: 5_000 });
        });

        test('date taken filter section renders in the filter panel', async ({ page }) => {
            await expect(page.locator('.lib-filter-group--date')).toBeVisible({ timeout: 5_000 });
            await expect(page.locator('.lib-date-input').first()).toBeVisible();
        });

        test('setting a far-future From date filters results to zero', async ({ page }) => {
            const status = page.locator('.lib-search-status strong');
            await page.locator('.lib-date-input').first().fill('2099-01-01');
            await page.locator('.lib-date-input').first().dispatchEvent('change');
            await page.waitForFunction(
                () => {
                    const el = document.querySelector('.lib-search-status strong');
                    return el && el.textContent.trim() === '0';
                },
                { timeout: 10_000 },
            );
            expect(await status.textContent()).toBe('0');
        });

        test('Reset clears date inputs and restores photo count', async ({ page }) => {
            const status = page.locator('.lib-search-status strong');
            const total = parseInt(await status.textContent(), 10);

            await page.locator('.lib-date-input').first().fill('2099-01-01');
            await page.locator('.lib-date-input').first().dispatchEvent('change');
            await page.waitForFunction(
                () => {
                    const el = document.querySelector('.lib-search-status strong');
                    return el && el.textContent.trim() === '0';
                },
                { timeout: 10_000 },
            );

            const resetDone = page.waitForResponse(
                res => res.url().includes('/api/library/search')
                    && !new URL(res.url()).searchParams.has('date_taken_min')
                    && res.status() === 200,
                { timeout: 15_000 },
            );
            await page.locator('.lib-search-reset').click();
            await resetDone;
            expect(await status.textContent()).toBe(String(total));

            const fromVal = await page.locator('.lib-date-input').first().inputValue();
            expect(fromVal).toBe('');
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
            await expect(page.locator('#lib-filter-btn')).toHaveAttribute('data-state', 'on');
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
            await expect(page.locator('#lib-filter-btn')).toHaveAttribute('data-state', 'off');
        });

        test('info panel loads EXIF data for a library photo without path errors', async ({ page }) => {
            // Library root only has subdirs; navigate into folder-b to reach images
            await page.waitForSelector('#lib-pane [data-type="dir"]', { timeout: 15_000 });
            await page.locator('#lib-pane [data-name="folder-b"]').dblclick();
            await page.waitForSelector('#lib-pane [data-type="image"]', { timeout: 15_000 });
            await page.locator('#lib-pane [data-type="image"]').first().click();
            await page.keyboard.press('i');
            await page.waitForSelector('.info-panel.expanded', { timeout: 10_000 });
            await page.waitForFunction(
                () => {
                    const panel = document.querySelector('.info-panel.expanded');
                    return panel && !panel.textContent.includes('Loading');
                },
                { timeout: 15_000 },
            );

            const panelText = await page.locator('.info-panel.expanded').textContent();
            expect(panelText).not.toMatch(/error.*invalid path/i);
            expect(panelText).toMatch(/Name/i);
            // Library contains JPEG and HIF images
            expect(panelText).toMatch(/JPEG|HEIF/i);
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
            const res = await request.get(`/api/library/search?ids=${libID}&limit=100`);
            expect(res.status()).toBe(200);
            const body = await res.json();
            // src/examples has 79 images; indexer may skip unsupported formats
            expect(body.total).toBeGreaterThan(50);
        });

        test('ISO range filter scopes results to matching photos', async ({ request }) => {
            const allRes = await request.get(`/api/library/search?ids=${libID}&limit=1`);
            const allTotal = (await allRes.json()).total;

            // iPhone 12 Pro = ISO 25; X-T50 = ISO 500; Canon EOS 500D = ISO 400
            // Filtering ISO 20–30 should return only the low-ISO iPhone shots (fewer than total)
            const res = await request.get(
                `/api/library/search?ids=${libID}&ISOSpeedRatings_min=20&ISOSpeedRatings_max=30`,
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
            // src/examples contains Fujifilm X-T50 and Canon EOS cameras
            expect(models.some((m) => m.includes('X-T50'))).toBe(true);
            expect(models.some((m) => m.includes('Canon'))).toBe(true);
        });

        test('GET /api/library/search with Model filter scopes results', async ({ request }) => {
            const allRes = await request.get(`/api/library/search?ids=${libID}&limit=1`);
            const allTotal = (await allRes.json()).total;

            const res = await request.get(
                `/api/library/search?ids=${libID}&Model=X-T50`,
            );
            expect(res.status()).toBe(200);
            const body = await res.json();
            expect(body.total).toBeGreaterThan(0);
            expect(body.total).toBeLessThan(allTotal);
        });

        test('date_taken_min with far-future date returns 0 photos', async ({ request }) => {
            const res = await request.get(`/api/library/search?ids=${libID}&date_taken_min=2099-01-01`);
            expect(res.status()).toBe(200);
            expect((await res.json()).total).toBe(0);
        });

        test('date_taken_max with far-past date returns 0 photos', async ({ request }) => {
            const res = await request.get(`/api/library/search?ids=${libID}&date_taken_max=1970-01-01`);
            expect(res.status()).toBe(200);
            expect((await res.json()).total).toBe(0);
        });

        test('search results with date_taken are sorted newest first', async ({ request }) => {
            const res = await request.get(`/api/library/search?ids=${libID}&limit=20`);
            expect(res.status()).toBe(200);
            const { results } = await res.json();
            const dated = results.filter(p => p.dateTaken);
            expect(dated.length).toBeGreaterThan(0);
            for (let i = 1; i < dated.length; i++) {
                expect(dated[i].dateTaken <= dated[i - 1].dateTaken).toBe(true);
            }
        });
    });
});
