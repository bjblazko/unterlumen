import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { GPS_PATH, NO_GPS_PATH } from '../helpers/fixtures.js';

// Organize is one source folder and a list of targets (ADR-0032). Everything
// here happens in throwaway folders, so the fixtures other specs rely on are
// never moved out from under them.

const SRC = 'e2e-organize-src';
const DST = 'e2e-organize-dst';
const SUB = `${SRC}/a-subfolder`;

// The targets are remembered client-side; seeding the store is how a test
// gets a target without driving the folder picker.
async function openOrganize(page, targets = [{ path: DST }]) {
    await page.addInitScript(([key, value]) => {
        window.localStorage.setItem(key, value);
    }, ['organize.targets', JSON.stringify(targets)]);
    await page.goto('/#organize');
    await waitForAppReady(page);
    await page.waitForSelector('.organize-source .browse-header', { timeout: 10_000 });
}

// The source folder is reached the way a person reaches it: by clicking it.
async function loadSource(page) {
    await page.locator(`.organize-source .folder-chip[data-name="${SRC}"]`).click();
    await page.waitForSelector('.organize-source [data-type="image"]', { timeout: 10_000 });
}

test.describe('Organize — one source, many targets', () => {
    test.beforeEach(async ({ request }) => {
        for (const folder of [SRC, DST]) {
            await request.post('/api/delete', { data: { files: [folder] } }).catch(() => {});
            expect((await request.post('/api/mkdir', { data: { path: folder } })).status()).toBe(200);
        }
        expect((await request.post('/api/mkdir', { data: { path: SUB } })).status()).toBe(200);
        const copy = await request.post('/api/copy', {
            data: { files: [GPS_PATH, NO_GPS_PATH], destination: SRC },
        });
        expect(copy.status()).toBe(200);
    });

    test.afterAll(async ({ request }) => {
        for (const folder of [SRC, DST]) {
            await request.post('/api/delete', { data: { files: [folder] } }).catch(() => {});
        }
    });

    test('the screen shows one source pane and the target list', async ({ page }) => {
        await openOrganize(page);
        await expect(page.locator('.organize-source .browse-header')).toHaveCount(1);
        await expect(page.locator('.org-target')).toHaveCount(2); // the folder target and "Mark for deletion"
        await expect(page.locator('.org-target--mark')).toContainText('Mark for deletion');
        // The dual pane is gone with its middle column.
        await expect(page.locator('#left-pane, #right-pane, #cmd-copy')).toHaveCount(0);
    });

    test('a number key moves the selection to that target, and U brings it back', async ({ page, request }) => {
        await openOrganize(page);
        await loadSource(page);

        const photos = page.locator('.organize-source [data-type="image"]');
        await expect(photos).toHaveCount(2);
        await photos.first().click();

        await page.keyboard.press('1');
        await expect(page.locator('#org-result')).toContainText('moved to', { timeout: 10_000 });
        await expect(photos).toHaveCount(1);

        const afterMove = await (await request.get(`/api/browse?path=${DST}`)).json();
        expect(afterMove.entries.filter((e) => e.type === 'image')).toHaveLength(1);

        await page.keyboard.press('u');
        await expect(page.locator('#org-result')).toContainText('moved back', { timeout: 10_000 });
        await expect(photos).toHaveCount(2);

        const afterUndo = await (await request.get(`/api/browse?path=${DST}`)).json();
        expect(afterUndo.entries.filter((e) => e.type === 'image')).toHaveLength(0);
    });

    test('exactly one target is current, and clicking another makes it current', async ({ page }) => {
        await openOrganize(page, [{ path: DST }, { path: `${SRC}/a-subfolder` }]);
        await loadSource(page);

        await expect(page.locator('.org-target[aria-current="true"]')).toHaveCount(1);
        await page.locator('.org-target').nth(1).click();
        await expect(page.locator('.org-target').nth(1)).toHaveAttribute('aria-current', 'true');
        await expect(page.locator('.org-target[aria-current="true"]')).toHaveCount(1);
    });

    // The number keys are the place shortcuts everywhere else; in Organize they
    // only take over while something is selected.
    test('with nothing selected, a number key still switches places', async ({ page }) => {
        await openOrganize(page);
        await loadSource(page);
        await page.keyboard.press('1');
        await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
    });

    test('a folder can go to Mark for deletion', async ({ page }) => {
        await openOrganize(page);
        await loadSource(page);

        const chip = page.locator('.organize-source .folder-chip[data-name="a-subfolder"]');
        await chip.click({ modifiers: ['ControlOrMeta'] });
        // "Mark for deletion" is the last target; with one folder target it is key 2.
        await page.keyboard.press('2');

        await expect(page.locator('#wastebin-count')).toHaveText('1', { timeout: 5_000 });
        await expect(page.locator('.organize-source .folder-chip[data-name="a-subfolder"]'))
            .toHaveClass(/marked-for-deletion/);

        await page.locator('#mode-wastebin').click();
        await expect(page.locator('.wastebin-dir')).toHaveCount(1);
        await expect(page.locator('#wb-delete')).toContainText('Delete 1 permanently');
    });

    test('photos cannot be sent to a folder target while a folder is selected', async ({ page }) => {
        await openOrganize(page);
        await loadSource(page);

        await page.locator('.organize-source .folder-chip[data-name="a-subfolder"]')
            .click({ modifiers: ['ControlOrMeta'] });
        await page.keyboard.press('1');
        await expect(page.locator('#org-result')).toContainText('Mark for deletion');
    });
});
