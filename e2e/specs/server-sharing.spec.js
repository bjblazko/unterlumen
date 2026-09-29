import { test, expect } from '@playwright/test';
import { spawn } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

// A server — the container on a NAS — has a fixed photo folder, so sharing
// is all there is to set up there (ADR-0042). Sharing makes
// .unterlumen-shared in the photo folder with the server's destinations,
// where the Mac's setup then finds it.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const BINARY = path.resolve(__dirname, '..', '..', 'unterlumen');
const PORT = 8085;
const URL = `http://127.0.0.1:${PORT}`;

async function waitForServer() {
    for (let i = 0; i < 60; i++) {
        try { if ((await fetch(URL + '/api/config')).ok) return; } catch { /* not up yet */ }
        await new Promise(r => setTimeout(r, 250));
    }
    throw new Error('the server did not start');
}

test.describe('Sharing on a server', () => {
    let app, dir;

    test.beforeAll(async () => {
        dir = fs.mkdtempSync(path.join(os.tmpdir(), 'unterlumen-server-'));
        fs.mkdirSync(path.join(dir, 'photos'));
        fs.mkdirSync(path.join(dir, 'data'));
        fs.writeFileSync(path.join(dir, 'data', 'channels.json'),
            JSON.stringify([{ slug: 'e2e-nas', name: 'On the NAS', format: 'jpeg', quality: 85, scale: { mode: 'original' }, exifMode: 'strip' }]));
        const env = { ...process.env, UNTERLUMEN_ROOT_PATH: path.join(dir, 'photos'), UNTERLUMEN_LIB_DIR: path.join(dir, 'data') };
        delete env.UNTERLUMEN_CHANNELS_DIR;
        app = spawn(BINARY, ['-port', String(PORT), '-bind', '127.0.0.1'], { env, stdio: 'ignore' });
        await waitForServer();
    });

    test.afterAll(() => {
        app?.kill();
        fs.rmSync(dir, { recursive: true, force: true });
    });

    test('Settings shares the destinations through the photo folder', async ({ page }) => {
        await page.goto(URL + '/#settings');
        await expect(page.locator('#settings-sharing')).toContainText('both can use the same destinations and galleries');
        await expect(page.locator('#setup-choose')).toHaveCount(0);
        await page.locator('#settings-share').click();

        const shared = path.join(dir, 'photos', '.unterlumen-shared');
        await expect(page.locator('#settings-sharing')).toContainText(`shared through ${shared}`);
        const copied = JSON.parse(fs.readFileSync(path.join(shared, 'channels.json'), 'utf8'));
        expect(copied.map(c => c.slug)).toContain('e2e-nas');
        const channels = await (await fetch(URL + '/api/channels/')).json();
        expect(channels.map(c => c.slug)).toContain('e2e-nas');
    });
});
