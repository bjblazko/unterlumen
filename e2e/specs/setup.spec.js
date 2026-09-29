import { test, expect } from '@playwright/test';
import { spawn } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

// The installed app starts without a photo folder and asks for it in the
// browser (#setup, ADR-0042). A folder another installation shares — it has
// .unterlumen-shared in it — is joined, and its destinations appear.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..');
const BINARY = path.resolve(ROOT, '..', 'unterlumen');
const PORT = 8084;
const URL = `http://127.0.0.1:${PORT}`;

function configFile(home) {
    const dir = process.platform === 'darwin'
        ? path.join(home, 'Library', 'Application Support')
        : path.join(home, '.config');
    return path.join(dir, 'Unterlumen', 'config.json');
}

async function waitForServer() {
    for (let i = 0; i < 60; i++) {
        try { if ((await fetch(URL + '/api/config')).ok) return; } catch { /* not up yet */ }
        await new Promise(r => setTimeout(r, 250));
    }
    throw new Error('the app did not start');
}

async function choosePhotoFolder(page, name) {
    await page.click('#setup-choose');
    await page.locator('.fp-row', { hasText: name }).click();
    await expect(page.locator('.fp-crumb-here')).toHaveText(name);
    await page.click('#fp-select');
    await expect(page.locator('#setup-photos')).toContainText(name);
}

test.describe('Setup in the browser', () => {
    // One app, set up step by step: a retry would start from a setup already done.
    test.describe.configure({ mode: 'serial', retries: 0 });
    let app, home;

    test.beforeAll(async () => {
        home = fs.mkdtempSync(path.join(os.tmpdir(), 'unterlumen-setup-'));
        fs.cpSync(path.join(ROOT, 'fixtures', 'photos', 'folder-a'), path.join(home, 'Photos', 'folder-a'), { recursive: true });
        fs.mkdirSync(path.join(home, 'NAS', '.unterlumen-shared'), { recursive: true });
        fs.writeFileSync(path.join(home, 'NAS', '.unterlumen-shared', 'channels.json'),
            JSON.stringify([{ slug: 'e2e-shared', name: 'Shared from the NAS', format: 'jpeg', quality: 85, scale: { mode: 'original' }, exifMode: 'strip' }]));

        const env = { ...process.env, HOME: home, XDG_CONFIG_HOME: path.join(home, '.config'), XDG_DATA_HOME: path.join(home, '.local', 'share') };
        for (const k of ['UNTERLUMEN_LIB_DIR', 'UNTERLUMEN_CHANNELS_DIR', 'UNTERLUMEN_ROOT_PATH', 'UNTERLUMEN_PORT']) delete env[k];
        app = spawn(BINARY, ['-port', String(PORT), '-bind', '127.0.0.1'], { env, stdio: 'ignore' });
        await waitForServer();
    });

    // The installed app opens the system's folder dialog where it can, which
    // a test cannot click; these tests use the dialog in the page.
    test.beforeEach(async ({ page }) => {
        await page.route('**/api/config', async (route) => {
            const response = await route.fetch();
            await route.fulfill({ response, json: { ...(await response.json()), folderDialog: false } });
        });
    });

    test.afterAll(() => {
        app?.kill();
        fs.rmSync(home, { recursive: true, force: true });
    });

    test('a first start opens the setup, and the chosen folder is shown in Folders', async ({ page }) => {
        await page.goto(URL + '/');
        await expect(page).toHaveURL(/#setup$/);
        await expect(page.locator('#setup-photos')).toHaveText('Not chosen yet');
        await expect(page.locator('#setup-share-field')).toBeHidden();

        await choosePhotoFolder(page, 'Photos');
        await expect(page.locator('#setup-share-toggle .toggle')).toHaveAttribute('aria-checked', 'false');
        await page.click('#setup-save');

        await expect(page).toHaveURL(/#folders$/);
        await expect(page.getByRole('button', { name: 'folder-a' })).toBeVisible();
        const saved = JSON.parse(fs.readFileSync(configFile(home), 'utf8'));
        expect(saved.photosDir).toBe(path.join(home, 'Photos'));
    });

    test('a folder another installation shares is joined', async ({ page }) => {
        await page.goto(URL + '/#settings');
        await page.getByRole('link', { name: 'Change the photo folder or sharing' }).click();
        await expect(page).toHaveURL(/#setup$/);

        await page.click('#setup-choose');
        await page.locator('.fp-home').click();
        await page.locator('.fp-row', { hasText: 'NAS' }).click();
        await page.click('#fp-select');
        await expect(page.locator('#setup-share-hint')).toContainText('Another Unterlumen installation works in this folder');
        await expect(page.locator('#setup-share-toggle .toggle')).toHaveAttribute('aria-checked', 'true');
        await page.click('#setup-save');

        await expect(page).toHaveURL(/#folders$/);
        const channels = await (await fetch(URL + '/api/channels/')).json();
        expect(channels.map(c => c.slug)).toContain('e2e-shared');
    });
});
