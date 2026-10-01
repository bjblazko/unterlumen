import { test, expect } from '@playwright/test';
import { spawn } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

// The installed app, started for the first time beside an installation made
// by an older -desktop-install, takes over that one's settings from its
// launcher (ADR-0042). The photo folder of that time no longer fences Folders
// (ADR-0047): it is forgotten, and its shared destinations are kept.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const BINARY = path.resolve(__dirname, '..', '..', 'unterlumen');
const PORT = 8087;
const URL = `http://127.0.0.1:${PORT}`;

// Where the older installer put its launcher, per system.
function launcherPath(home) {
    if (process.platform === 'darwin') return path.join(home, 'Applications', 'Unterlumen.app', 'Contents', 'MacOS', 'launch');
    return path.join(home, '.local', 'share', 'unterlumen', 'launch.sh');
}

async function waitForServer() {
    for (let i = 0; i < 60; i++) {
        try { if ((await fetch(URL + '/api/config')).ok) return; } catch { /* not up yet */ }
        await new Promise(r => setTimeout(r, 250));
    }
    throw new Error('the app did not start');
}

test.describe('An older installation', () => {
    let app, home;

    test.beforeAll(async () => {
        home = fs.mkdtempSync(path.join(os.tmpdir(), 'unterlumen-adopt-'));
        const photos = path.join(home, 'Bilder');
        fs.mkdirSync(path.join(photos, 'Fotos'), { recursive: true });
        fs.mkdirSync(path.join(home, 'shared'));
        fs.writeFileSync(path.join(home, 'shared', 'channels.json'),
            JSON.stringify([{ slug: 'e2e-old-install', name: 'From the old install', format: 'jpeg', quality: 85, scale: { mode: 'original' }, exifMode: 'strip' }]));
        const launcher = launcherPath(home);
        fs.mkdirSync(path.dirname(launcher), { recursive: true });
        fs.writeFileSync(launcher, `#!/bin/bash\nexec "$DIR/unterlumen" -desktop -port 8090 -lib-dir '${path.join(home, 'data')}' -channels-dir '${path.join(home, 'shared')}' '${photos}'\n`);

        const env = { ...process.env, HOME: home, XDG_CONFIG_HOME: path.join(home, '.config'), XDG_DATA_HOME: path.join(home, '.local', 'share') };
        for (const k of ['UNTERLUMEN_LIB_DIR', 'UNTERLUMEN_CHANNELS_DIR', 'UNTERLUMEN_ROOT_PATH', 'UNTERLUMEN_PORT']) delete env[k];
        app = spawn(BINARY, ['-port', String(PORT), '-bind', '127.0.0.1'], { env, stdio: 'ignore' });
        await waitForServer();
    });

    test.afterAll(() => {
        app?.kill();
        fs.rmSync(home, { recursive: true, force: true });
    });

    test('its shared destinations are taken over, and Folders is not fenced by its photo folder', async ({ page }) => {
        const cfg = await (await fetch(URL + '/api/config')).json();
        expect(cfg.boundary).toBe('/');
        const channels = await (await fetch(URL + '/api/channels/')).json();
        expect(channels.map(c => c.slug)).toContain('e2e-old-install');

        await page.goto(URL + '/');
        await expect(page).not.toHaveURL(/#setup$/);
        await expect(page.getByRole('button', { name: 'Bilder' })).toBeVisible();
    });

    test('the setup shows the taken-over shared folder', async ({ page }) => {
        await page.goto(URL + '/#setup');
        await expect(page.locator('#setup-shared')).toHaveText(path.join(home, 'shared'));
        await expect(page.locator('#setup-shared-hint')).toContainText('shared through this folder');
    });
});
