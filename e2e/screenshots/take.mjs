// Takes the README and website screenshots: `npm run screenshots` in e2e/.
//
// Starts a throwaway Unterlumen on a fresh copy of src/examples (the folders
// renamed so they read like a photo collection), builds a library, a few
// destinations and galleries through the API, then drives the app with
// Playwright. Light theme except the dark twin. Writes WebP files to
// doc/screenshots/ (needs cwebp: `brew install webp`, `apt install webp`).
//
// The binary must be built first: cd src && go build -o ../unterlumen .

import { chromium } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { shots } from './shots.mjs';
import { REPO, BINARY, BASE, preparePhotos, startServer, seed, showHomePath } from './stage.mjs';

const OUT = path.join(REPO, 'doc/screenshots');

function checkTools() {
    if (!existsSync(BINARY)) throw new Error(`No binary at ${BINARY}. Build it first: cd src && go build -o ../unterlumen .`);
    try { execFileSync('cwebp', ['-version'], { stdio: 'ignore' }); } catch {
        throw new Error('cwebp is missing. Install it: brew install webp (macOS) or apt install webp (Linux).');
    }
}

function toWebP(png, name) {
    execFileSync('cwebp', ['-quiet', '-q', '86', '-m', '6', png, '-o', path.join(OUT, `${name}.webp`)]);
}

async function main() {
    checkTools();
    const tmp = mkdtempSync(path.join(tmpdir(), 'unterlumen-shots-'));
    const root = preparePhotos(tmp);
    const server = await startServer(tmp, root);
    const browser = await chromium.launch();
    // The 3D views need WebGPU, which only Chrome itself has headless; it is
    // started for the shots that ask for it (gpu: true).
    let gpuBrowser = null;
    const browserFor = async shot => {
        if (!shot.gpu) return browser;
        gpuBrowser ??= await chromium.launch({ channel: 'chrome', args: ['--enable-unsafe-webgpu'] });
        return gpuBrowser;
    };
    try {
        const lib = await seed();
        const site = path.join(tmp, 'lib', 'channels');
        mkdirSync(OUT, { recursive: true });
        const only = process.argv.slice(2);
        for (const shot of shots) {
            if (only.length && !only.includes(shot.name)) continue;
            const context = await (await browserFor(shot)).newContext({
                viewport: shot.viewport || { width: 1440, height: 900 },
                deviceScaleFactor: shot.scale || 2,
                isMobile: !!shot.phone,
                hasTouch: !!shot.phone,
                colorScheme: shot.dark ? 'dark' : 'light',
                locale: 'en-GB',
            });
            await context.addInitScript(([theme, extra]) => {
                localStorage.setItem('theme', theme);
                localStorage.setItem('warranty-notice-seen', '1'); // a first start's notice is not the subject
                for (const [k, v] of Object.entries(extra)) localStorage.setItem(k, v);
            }, [shot.dark ? 'dark' : 'light', shot.storage || {}]);
            const page = await context.newPage();
            await shot.take(page, { base: BASE, lib, site });
            await showHomePath(page, root);
            const png = path.join(tmp, `${shot.name}.png`);
            await page.screenshot({ path: png });
            toWebP(png, shot.name);
            console.log(`  ${shot.name}.webp`);
            await context.close();
        }
    } finally {
        await browser.close();
        await gpuBrowser?.close();
        server.kill();
        rmSync(tmp, { recursive: true, force: true });
    }
}

main().catch((err) => { console.error(err.message); process.exit(1); });
