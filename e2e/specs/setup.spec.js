import { test, expect } from '@playwright/test';
import { spawn } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

// The installed app needs no photo folder (ADR-0047): it starts on the whole
// disk, each library names its own folder, and the setup (#setup, ADR-0042)
// chooses only the shared folder and the data folder. A folder another
// installation shares through — it has .unterlumen-shared in it — is joined,
// and its destinations appear.
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

async function chooseSharedFolder(page, crumbOrRow) {
    await page.click('#setup-choose');
    await crumbOrRow(page);
    await page.click('#fp-select');
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

    test('a first start opens no setup, and Folders reaches the whole disk', async ({ page }) => {
        const cfg = await (await fetch(URL + '/api/config')).json();
        expect(cfg.boundary).toBe('/');
        expect(cfg.needsSetup).toBeUndefined();
        await page.goto(URL + '/');
        await expect(page).not.toHaveURL(/#setup$/);
        await expect(page.getByRole('button', { name: 'Photos' })).toBeVisible();
        await expect(page.getByRole('button', { name: 'NAS' })).toBeVisible();
    });

    test('the top of the disk cannot be shared, and the setup says why', async ({ page }) => {
        await page.goto(URL + '/#setup');
        await expect(page.locator('#setup-shared')).toHaveText('Not shared');
        await chooseSharedFolder(page, async (p) => {
            await p.locator('.fp-crumb[data-crumb=""]').click();
            await expect(p.locator('.fp-crumb-here')).toHaveCount(0);
        });
        await expect(page.locator('#setup-shared-hint')).toContainText('The top of a disk cannot be shared');
        await expect(page.locator('#setup-shared')).toHaveText('Not shared');
    });

    test('a folder another installation shares through is joined', async ({ page }) => {
        await page.goto(URL + '/#settings');
        await expect(page.locator('#settings-shared-dir')).toHaveText('Not shared');
        await page.getByRole('link', { name: 'Change the shared folder or the data folder' }).click();
        await expect(page).toHaveURL(/#setup$/);

        await chooseSharedFolder(page, async (p) => {
            await p.locator('.fp-row', { hasText: 'NAS' }).click();
        });
        const shared = path.join(home, 'NAS', '.unterlumen-shared');
        await expect(page.locator('#setup-shared')).toHaveText(shared);
        await expect(page.locator('#setup-shared-hint')).toContainText('Another installation shares through this folder');
        await page.click('#setup-save');

        await expect(page).toHaveURL(/#settings$/);
        await expect(page.locator('#settings-shared-dir')).toHaveText(shared);
        const channels = await (await fetch(URL + '/api/channels/')).json();
        expect(channels.map(c => c.slug)).toContain('e2e-shared');
        const saved = JSON.parse(fs.readFileSync(configFile(home), 'utf8'));
        expect(saved.channelsDir).toBe(shared);
        expect(saved.photosDir).toBeUndefined();
    });

    test('a library is added from any folder, with no folder in common', async ({ page }) => {
        const created = await (await fetch(URL + '/api/library/', {
            method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: 'Anywhere', sourcePath: path.join(home, 'Photos', 'folder-a') }),
        })).json();
        expect(created.sourcePath).toBe(fs.realpathSync(path.join(home, 'Photos', 'folder-a')));
        await page.goto(URL + '/#libraries');
        await expect(page.locator('.library-card-name', { hasText: 'Anywhere' })).toBeVisible();
    });

    test('missing helper programs are installed from the setup', async ({ page }) => {
        const missing = { platform: 'darwin', exiftool: { available: false }, ffmpeg: { available: false }, sips: { available: true }, heifConvert: { available: false },
            install: { missing: ['ffmpeg', 'exiftool', 'cwebp'], canInstall: true, how: "the makers' downloads" } };
        const found = { platform: 'darwin', exiftool: { available: true }, ffmpeg: { available: true, heifSupport: true, webpSupport: true }, sips: { available: true }, heifConvert: { available: false },
            install: { missing: [], canInstall: false } };
        let installed = false;
        await page.route('**/api/tools/check', (route) => route.fulfill({ json: installed ? found : missing }));
        await page.route('**/api/tools/install', (route) => { installed = true; return route.fulfill({ status: 204 }); });

        await page.goto(URL + '/#setup');
        await expect(page.locator('#setup-tools')).toContainText('missing: exiftool');
        await expect(page.locator('#setup-tools-install')).toContainText("From the makers' downloads");
        await page.locator('#setup-tools-install button').click();
        await expect(page.locator('#setup-tools')).toHaveText(/^exiftool, ffmpeg, sips found/);
        await expect(page.locator('#setup-tools-install')).toBeHidden();
    });

    test('on Linux the setup shows the command instead', async ({ page }) => {
        await page.route('**/api/tools/check', (route) => route.fulfill({ json: {
            platform: 'linux', exiftool: { available: false }, ffmpeg: { available: true }, sips: { available: false }, heifConvert: { available: false },
            install: { missing: ['exiftool', 'heif-convert'], canInstall: false, command: 'sudo apt-get install -y ffmpeg libimage-exiftool-perl libheif-examples webp' } } }));
        await page.goto(URL + '/#setup');
        await expect(page.locator('#setup-tools-install .tools-command')).toHaveText(/^sudo apt-get install/);
        await expect(page.locator('#setup-tools-install button')).toHaveCount(0);
    });
});
