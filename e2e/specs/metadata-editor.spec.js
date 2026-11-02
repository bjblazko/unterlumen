import { test, expect } from '@playwright/test';
import { spawn, execFileSync } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

// Metadata (ADR-0060): what the chosen files say, written where the owner
// chooses — beside them (XMP sidecar, a video's NFO file) or into them. The
// photos are copies on an instance of their own, so no other spec's files
// change.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..');
const BINARY = path.resolve(ROOT, '..', 'unterlumen');
const PORT = 8096;
const base = `http://127.0.0.1:${PORT}`;
const SOURCE = path.join(ROOT, 'fixtures', 'photos', 'folder-b');
const PHOTOS = [
    '2004-05-01_10-07-27_QV-R3_cimg0021.jpeg',
    '2004-05-01_10-09-55_QV-R3_cimg0024.jpeg',
    '2005-05-19_12-35-33_EX-S500_CIMG0298.jpeg',
];

function has(cmd) {
    try { execFileSync(cmd, ['-ver']); return true; } catch { return false; }
}

async function waitForServer() {
    for (let i = 0; i < 60; i++) {
        try { if ((await fetch(base + '/api/config')).ok) return; } catch { /* not up yet */ }
        await new Promise(r => setTimeout(r, 250));
    }
    throw new Error('the app did not start');
}

test.describe('Metadata', () => {
    test.describe.configure({ mode: 'serial', retries: 0 });
    let home, app, trip;

    test.beforeAll(async () => {
        home = fs.mkdtempSync(path.join(os.tmpdir(), 'unterlumen-metadata-'));
        trip = path.join(home, 'photos', 'trip');
        fs.mkdirSync(trip, { recursive: true });
        for (const p of PHOTOS) fs.copyFileSync(path.join(SOURCE, p), path.join(trip, p));
        const env = { ...process.env, UNTERLUMEN_ROOT_PATH: path.join(home, 'photos'),
            UNTERLUMEN_LIB_DIR: path.join(home, 'data'), UNTERLUMEN_CACHE_DIR: path.join(home, 'cache') };
        for (const k of ['UNTERLUMEN_CHANNELS_DIR', 'UNTERLUMEN_PORT']) delete env[k];
        app = spawn(BINARY, ['-port', String(PORT), '-bind', '127.0.0.1'], { env, stdio: 'ignore' });
        await waitForServer();
    });

    test.afterAll(() => {
        app?.kill();
        if (home) fs.rmSync(home, { recursive: true, force: true });
    });

    async function chooseAll(page) {
        await page.goto(base + '/#folders');
        await page.locator('.dir-item[data-name="trip"]').dblclick();
        await expect(page.locator('.image-item:visible')).toHaveCount(PHOTOS.length);
        await page.locator('.select-all-all').click();
        await page.locator('.selection-bar [data-action="metadata"]').click();
        await expect(page.locator('#mode-metadata')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.meta-preview')).toBeVisible();
    }

    test('without photos it says how to bring some', async ({ page }) => {
        await page.goto(base + '/#metadata');
        await expect(page.locator('.meta-place')).toContainText('No photos chosen');
    });

    test('the day is set in the sidecars, each photo keeping its time', async ({ page }) => {
        const before = PHOTOS.map(p => fs.readFileSync(path.join(trip, p)));
        await chooseAll(page);
        await expect(page.locator('.meta-target .seg button[data-value="sidecar"]')).toHaveAttribute('aria-pressed', 'true');
        await expect(page.locator('.meta-target-text')).toContainText('The files themselves stay as they are.');

        await page.locator('.meta-date .seg button[data-value="day"]').click();
        await page.locator('#meta-day').fill('2024-05-01');
        const dates = page.locator('.meta-preview-date strong');
        await expect(dates).toHaveCount(3);
        await expect(dates).toHaveText(['2024-05-01 10:07:27', '2024-05-01 10:09:55', '2024-05-01 12:35:33']);

        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toHaveText('Written to 3 files.');
        await expect(page.locator('.meta-date .meta-source')).toHaveText('from the sidecar');

        PHOTOS.forEach((p, i) => expect(fs.readFileSync(path.join(trip, p)).equals(before[i])).toBe(true));
        const xmp = fs.readFileSync(path.join(trip, PHOTOS[0].replace('.jpeg', '.xmp')), 'utf8');
        expect(xmp).toContain('2024-05-01T10:07:27');
    });

    test('a shift moves every photo by the same amount', async ({ page }) => {
        await chooseAll(page);
        await page.locator('.meta-date .seg button[data-value="shift"]').click();
        await page.locator('#meta-shift-hours').fill('1');
        await expect(page.locator('.meta-preview-date strong')).toHaveText(['2024-05-01 11:07:27', '2024-05-01 11:09:55', '2024-05-01 13:35:33']);
    });

    test('a field with different values is left alone until it is changed', async ({ page, request }) => {
        await request.post(base + '/api/metadata/write', { data: { file: `trip/${PHOTOS[0]}`, target: 'sidecar', change: { set: { title: 'One' } } } });
        await chooseAll(page);
        await expect(page.locator('#meta-title')).toHaveAttribute('placeholder', 'Several values');
        await page.locator('#meta-creator').fill('Ann');
        await expect(page.locator('.meta-field[data-field="creator"] .meta-reset')).toBeVisible();
        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toHaveText('Written to 3 files.');
        const res = await (await request.post(base + '/api/metadata/read', { data: { files: PHOTOS.map(p => `trip/${p}`) } })).json();
        expect(res.files.map(f => f.current.fields.title?.value || '')).toEqual(['One', '', '']);
        expect(res.files.map(f => f.current.fields.creator.value)).toEqual(['Ann', 'Ann', 'Ann']);
    });

    test('written into the file, the photo itself changes', async ({ page }) => {
        test.skip(!has('exiftool'), 'exiftool is not installed');
        const before = fs.readFileSync(path.join(trip, PHOTOS[2]));
        await chooseAll(page);
        await page.locator('.meta-target .seg button[data-value="file"]').click();
        await expect(page.locator('.meta-target-text')).toContainText('Changes 3 photos themselves.');
        await page.locator('.meta-field[data-field="rating"] [data-stars="4"]').click();
        await expect(page.locator('.meta-field[data-field="rating"] .meta-reset')).toBeVisible();
        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toHaveText('Written to 3 files.');
        expect(fs.readFileSync(path.join(trip, PHOTOS[2])).equals(before)).toBe(false);
        await expect(page.locator('.meta-field[data-field="rating"] .meta-source')).toHaveText('from the file');
    });

    // Stars (ADR-0060): the number keys rate, always beside the file. The
    // photos carry 4 stars in the files themselves from the test above.
    async function openTrip(page) {
        await page.goto(base + '/#folders');
        await page.locator('.dir-item[data-name="trip"]').dblclick();
        await expect(page.locator('.image-item:visible')).toHaveCount(PHOTOS.length);
    }

    test('the number keys rate the selection, beside the file', async ({ page }) => {
        await openTrip(page);
        const tile = page.locator(`.image-item[data-name="${PHOTOS[0]}"]`);
        await expect(tile.locator('.rating-marks')).toHaveText('★★★★');
        await tile.click();
        await page.keyboard.press('5');
        await expect(tile.locator('.rating-marks')).toHaveText('★★★★★');
        await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
        expect(fs.readFileSync(path.join(trip, PHOTOS[0].replace('.jpeg', '.xmp')), 'utf8')).toContain('5');

        await page.locator(`.image-item[data-name="${PHOTOS[1]}"]`).click();
        await page.keyboard.press('0');
        await expect(page.locator(`.image-item[data-name="${PHOTOS[1]}"] .rating-marks`)).toHaveCount(0);
    });

    test('the info panel and the viewer show the stars and take a click', async ({ page }) => {
        await openTrip(page);
        await page.locator(`.image-item[data-name="${PHOTOS[2]}"]`).click();
        if (await page.locator('.info-panel.expanded:visible').count() === 0) await page.keyboard.press('i');
        const panel = page.locator('.info-panel.expanded:visible .rating-control');
        await expect(panel.locator('.rating-star.is-on')).toHaveCount(4);
        await panel.locator('[data-stars="2"]').click();
        await expect(page.locator(`.image-item[data-name="${PHOTOS[2]}"] .rating-marks`)).toHaveText('★★');

        await page.locator(`.image-item[data-name="${PHOTOS[2]}"]`).dblclick();
        const viewerStars = page.locator('.viewer-rating .rating-star.is-on');
        await expect(viewerStars).toHaveCount(2);
        await page.keyboard.press('3');
        await expect(viewerStars).toHaveCount(3);
        await page.keyboard.press('Escape');
        await expect(page.locator(`.image-item[data-name="${PHOTOS[2]}"] .rating-marks`)).toHaveText('★★★');
    });

    test('the list has a Rating column', async ({ page }) => {
        await openTrip(page);
        await page.evaluate(() => App.browsePane.setView('list'));
        await expect(page.locator(`tr[data-name="${PHOTOS[0]}"] .list-rating`)).toHaveText('★★★★★');
        await page.evaluate(() => App.browsePane.setView('justified'));
    });

    test('the library filter narrows by stars', async ({ page }) => {
        const created = await (await fetch(base + '/api/library/', { method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: 'Rated', description: '', sourcePath: 'trip' }) })).json();
        const scan = await fetch(`${base}/api/library/${created.id}/reindex`, { method: 'POST' });
        expect(await scan.text()).toContain('"finished":true');

        await page.goto(base + '/#libraries');
        await page.locator('.library-card', { hasText: 'Rated' }).locator('.lib-open').click();
        await page.locator('#lib-filter-btn').click();
        const count = page.locator('#lib-results-pane .place-meta');
        for (const [choice, n] of [['5', '1 photo'], ['3', '2 photos'], ['none', '1 photo']]) {
            await page.locator('select.lib-filter-rating').selectOption(choice);
            await expect(count).toHaveText(n);
        }
        await page.locator('select.lib-filter-rating').selectOption('');
        await expect(page.locator('select.lib-filter-rating option:checked')).toHaveText('Doesn’t matter');
    });

    // IPTC and own fields (ADR-0061).
    const group = (page, id) => page.locator(`.meta-group[data-group="${id}"]`);
    const sidecar = (i) => fs.readFileSync(path.join(trip, PHOTOS[i].replace('.jpeg', '.xmp')), 'utf8');

    test('empty groups start folded, and a folded group says it holds a change', async ({ page }) => {
        await chooseAll(page);
        await expect(group(page, 'date')).toHaveAttribute('open', '');
        await expect(group(page, 'description')).toHaveAttribute('open', '');
        for (const id of ['location', 'event', 'origin', 'shown', 'own']) {
            await expect(group(page, id)).not.toHaveAttribute('open', '');
            await expect(group(page, id).locator('.meta-group-state')).toHaveText('Empty');
        }
        await group(page, 'location').locator('summary').click();
        await page.locator('#meta-city').fill('Kiel');
        await page.locator('#meta-countryCode').fill('DE');
        await group(page, 'location').locator('summary').click();
        await expect(group(page, 'location').locator('.meta-group-state')).toHaveText('Changed');
        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toHaveText('Written to 3 files.');
        await expect(group(page, 'location')).toHaveAttribute('open', '');
        await expect(page.locator('#meta-city')).toHaveValue('Kiel');
        expect(sidecar(0)).toContain('<photoshop:City>Kiel</photoshop:City>');
        expect(sidecar(0)).toContain('<Iptc4xmpCore:CountryCode>DE</Iptc4xmpCore:CountryCode>');
    });

    test('people shown are added and removed like keywords', async ({ page }) => {
        await chooseAll(page);
        await group(page, 'shown').locator('summary').click();
        await page.locator('#meta-personShown').fill('Ann');
        await page.keyboard.press('Enter');
        await page.locator('#meta-personShown').fill('Bob');
        await page.keyboard.press('Enter');
        await expect(page.locator('.meta-preview tbody tr').first()).toContainText('People shown');
        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toHaveText('Written to 3 files.');
        expect(sidecar(1)).toContain('<rdf:Bag><rdf:li>Ann</rdf:li><rdf:li>Bob</rdf:li></rdf:Bag>');

        const chips = page.locator('.meta-list[data-field="personShown"]');
        await chips.locator('[data-entry="Ann"] .meta-chip-remove').click();
        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toHaveText('Written to 3 files.');
        await expect(chips.locator('.meta-chip')).toHaveCount(1);
        expect(sidecar(1)).toContain('<Iptc4xmpExt:PersonInImage><rdf:Bag><rdf:li>Bob</rdf:li></rdf:Bag></Iptc4xmpExt:PersonInImage>');
    });

    test('an own field goes to the sidecar and, if asked, among the keywords', async ({ page }) => {
        test.skip(!has('exiftool'), 'exiftool is not installed');
        await chooseAll(page);
        await page.locator('.meta-target .seg button[data-value="file"]').click();
        await group(page, 'own').locator('summary').click();
        await expect(group(page, 'own')).toContainText('Own fields are always written to the sidecar');
        await page.locator('#meta-own-key').fill('stock');
        await page.locator('#meta-own-value').fill('Portra');
        await page.locator('#meta-own-btn').click();
        const row = page.locator('.meta-own-row[data-own="stock"]');
        await row.locator('.toggle').click();
        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toHaveText('Written to 3 files.');
        expect(sidecar(2)).toContain('Portra');
        await expect(page.locator('.meta-chip[data-entry="ul:stock=Portra"]')).toBeVisible();
        await expect(row.locator('.toggle')).toHaveAttribute('aria-checked', 'true');

        await row.locator('input').fill('Ektar');
        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toHaveText('Written to 3 files.');
        await expect(page.locator('.meta-chip[data-entry="ul:stock=Ektar"]')).toBeVisible();
        await expect(page.locator('.meta-chip[data-entry="ul:stock=Portra"]')).toHaveCount(0);
    });

    test('the info panel in Folders shows the IPTC fields and edits them in place', async ({ page }) => {
        await openTrip(page);
        await page.locator(`.image-item[data-name="${PHOTOS[0]}"]`).click();
        if (await page.locator('.info-panel.expanded:visible').count() === 0) await page.keyboard.press('i');
        const panel = page.locator('.info-panel.expanded:visible');
        const city = panel.locator('.info-iptc-val[data-field="city"]');
        await expect(city).toHaveText('Kiel');
        await expect(panel.locator('.info-iptc-val[data-field="personShown"]')).toHaveText('Bob');
        // What lies beside the file is marked; the creator written earlier too.
        await expect(panel.locator('.info-iptc-row[data-iptc="city"] .info-source')).toHaveText('XMP');
        await expect(panel.locator('.info-iptc-val[data-field="creator"]')).toHaveText('Ann');
        await expect(panel.locator('.info-iptc-row[data-iptc="creator"] .info-source')).toHaveText('XMP');
        await city.click();
        await page.keyboard.press('ControlOrMeta+A');
        await page.keyboard.type('Lübeck');
        await page.keyboard.press('Enter');
        await expect.poll(() => sidecar(0)).toContain('<photoshop:City>Lübeck</photoshop:City>');
        await expect(city).toHaveText('Lübeck');

        await panel.locator('.info-iptc-field').selectOption('event');
        await panel.locator('.info-iptc-input').fill('Kieler Woche');
        await panel.locator('.info-iptc-add-btn').click();
        await expect(panel.locator('.info-iptc-val[data-field="event"]')).toHaveText('Kieler Woche');
        expect(sidecar(0)).toContain('Kieler Woche');
    });

    test('a photo in Folders has its own fields in the info panel', async ({ page }) => {
        await openTrip(page);
        await page.locator(`.image-item[data-name="${PHOTOS[1]}"]`).click();
        if (await page.locator('.info-panel.expanded:visible').count() === 0) await page.keyboard.press('i');
        const panel = page.locator('.info-panel.expanded:visible');
        await expect(panel.locator('.info-meta-val[data-key="stock"]')).toHaveText('Ektar');
        await panel.locator('.info-meta-key-input').fill('lab');
        await panel.locator('.info-meta-val-input').fill('Nordfoto');
        await panel.locator('.info-meta-add-btn').click();
        await expect(panel.locator('.info-meta-val[data-key="lab"]')).toHaveText('Nordfoto');
        expect(sidecar(1)).toContain('Nordfoto');
        await panel.locator('.info-title-val').click();
        await page.keyboard.type('Am Kai');
        await page.keyboard.press('Enter');
        await expect.poll(() => sidecar(1)).toContain('Am Kai');
    });

    test('the country comes from the GPS position, per file, offline', async ({ page }) => {
        await chooseAll(page);
        await expect(group(page, 'location')).toHaveAttribute('open', '');
        const box = page.locator('.meta-country-gps');
        await expect(box).toContainText('Every chosen file has one.');
        await box.locator('.toggle').click();
        await expect(page.locator('#meta-country')).toBeDisabled();
        // Only the group that holds a change says so.
        await expect(group(page, 'location').locator('.meta-group-state')).toHaveText('Changed');
        for (const id of ['description', 'event', 'origin', 'shown', 'own']) {
            await expect(group(page, id).locator('.meta-group-state')).not.toHaveText('Changed');
        }
        const rows = page.locator('.meta-preview tbody tr');
        await expect(rows.nth(0)).toContainText('Country: Germany');
        await page.locator('#meta-write').click();
        await expect(page.locator('.meta-result-text')).toContainText('Written to');
        expect(sidecar(0)).toContain('<photoshop:Country>Germany</photoshop:Country>');
    });

    test('the search suggests a city and the library filter narrows by it', async ({ page }) => {
        const libs = await (await fetch(base + '/api/library/')).json();
        const rated = (libs.libraries || libs).find(l => l.name === 'Rated');
        const scan = await fetch(`${base}/api/library/${rated.id}/reindex`, { method: 'POST' });
        expect(await scan.text()).toContain('"finished":true');

        await page.goto(base + '/#folders');
        await page.locator('#search-bar-input').click();
        await page.locator('#search-bar-input').fill('kie');
        await expect(page.locator('.search-bar-option', { hasText: 'Kieler Woche' })).toContainText('Event');
        const option = page.locator('.search-bar-option', { hasText: 'City' });
        await expect(option).toContainText('Kiel');
        await option.click();
        await expect(page.locator('#mode-library')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('#lib-results-pane .place-meta')).toHaveText('2 photos');

        // The filter shows the city chosen, and offers the people shown.
        const city = page.locator('.lib-filter-group', { hasText: 'City' }).locator('select');
        await expect(city).toHaveValue('Kiel');
        const shown = page.locator('.lib-filter-group', { hasText: 'People shown' }).locator('select');
        await city.selectOption('');
        await shown.selectOption('Bob');
        await expect(page.locator('#lib-results-pane .place-meta')).toHaveText('3 photos');
    });
});
