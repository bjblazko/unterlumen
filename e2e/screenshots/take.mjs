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
import { spawn, execFileSync } from 'node:child_process';
import { cpSync, mkdtempSync, mkdirSync, rmSync, existsSync, realpathSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { shots } from './shots.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, '../..');
const BINARY = path.join(REPO, 'unterlumen');
const OUT = path.join(REPO, 'doc/screenshots');
const PORT = 8097;
const BASE = `http://127.0.0.1:${PORT}`;

// The example folders, as a person would name them.
const FOLDERS = {
    'folder-b': 'Travel',
    'folder-a/a2': '2017',
    'folder-a/a3': '2025',
    'folder-a/a1': 'Odds and ends',
};

function checkTools() {
    if (!existsSync(BINARY)) throw new Error(`No binary at ${BINARY}. Build it first: cd src && go build -o ../unterlumen .`);
    try { execFileSync('cwebp', ['-version'], { stdio: 'ignore' }); } catch {
        throw new Error('cwebp is missing. Install it: brew install webp (macOS) or apt install webp (Linux).');
    }
}

function preparePhotos(tmp) {
    const root = path.join(tmp, 'Pictures');
    for (const [from, to] of Object.entries(FOLDERS)) {
        cpSync(path.join(REPO, 'src/examples', from), path.join(root, to), { recursive: true });
    }
    return root;
}

async function startServer(tmp, root) {
    const lib = path.join(tmp, 'lib');
    mkdirSync(lib);
    const server = spawn(BINARY, ['--port', String(PORT), '--bind', '127.0.0.1', root], {
        env: { ...process.env, UNTERLUMEN_LIB_DIR: lib, UNTERLUMEN_CHANNELS_DIR: lib },
        stdio: 'ignore',
    });
    for (let i = 0; i < 50; i++) {
        try { if ((await fetch(`${BASE}/api/config`)).ok) return server; } catch { /* not yet */ }
        await new Promise(r => setTimeout(r, 200));
    }
    server.kill();
    throw new Error('The server did not start.');
}

async function api(method, url, body) {
    const r = await fetch(BASE + url, {
        method,
        headers: body ? { 'Content-Type': 'application/json' } : {},
        body: body ? JSON.stringify(body) : undefined,
    });
    const text = await r.text();
    if (!r.ok) throw new Error(`${method} ${url}: ${r.status} ${text}`);
    try { return JSON.parse(text); } catch { return text; }
}

// A library over everything, and three destinations with galleries in
// different states, so Galleries has something to say.
async function seed() {
    const lib = await api('POST', '/api/library/', { name: 'Pictures', description: 'Everything', sourcePath: '.' });
    await api('POST', `/api/library/${lib.id}/reindex`);
    const geo = await api('GET', '/api/library/geo');
    const points = geo.libraries[0].points.map(([id, lat, lon, taken, name]) => ({ id, lat, lon, taken, name }));
    const near = (lat, lon) => points.filter(p => Math.abs(p.lat - lat) < 1 && Math.abs(p.lon - lon) < 1).map(p => p.id);

    // A fresh installation already has Instagram, Mastodon and Website;
    // those are set up the same way rather than created.
    const existing = new Set((await api('GET', '/api/channels/')).map(c => c.slug));
    const channel = (slug, name, extra) => {
        const body = {
            slug, name, format: 'jpeg', quality: 85, exifMode: 'strip',
            scale: { mode: 'max_dim', maxDimension: 'width', maxValue: 1600 }, outputMode: 'save', ...extra,
        };
        return existing.has(slug) ? api('PUT', `/api/channels/${slug}`, body) : api('POST', '/api/channels/', body);
    };
    await channel('website', 'Website', { siteExport: true, siteTitle: 'Photographs' });
    await channel('friends', 'Share links', { galleryExport: true });
    await channel('instagram', 'Instagram', {});

    const collect = (slug, title, ids) => api('POST', `/api/library/${lib.id}/channels/${slug}/drafts`, { photoIDs: ids, title });
    const rhodes = await collect('website', 'Rhodes', near(36.4, 28.2));
    await api('POST', `/api/channels/website/drafts/${rhodes.id}/generate`, { publishedAt: '2025-06-01T12:00:00Z' });
    const rome = await collect('website', 'Rome and the Vatican', near(41.9, 12.47));
    await api('POST', `/api/channels/website/drafts/${rome.id}/generate`, { publishedAt: '2025-09-14T12:00:00Z' });
    await collect('website', 'Hamburg harbour', near(53.5, 9.95));
    const share = await collect('friends', 'Mallorca, for the family', near(39.5, 3.3));
    await api('POST', `/api/channels/friends/drafts/${share.id}/generate`, { publishedAt: '2025-07-20T12:00:00Z' });
    await collect('instagram', 'Summer', near(36.4, 28.2).slice(0, 3));
    return lib;
}

// The photos live in a temporary folder; the pictures show where a person
// keeps theirs.
async function showHomePath(page, root) {
    await page.evaluate((prefixes) => {
        const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
        for (let node = walker.nextNode(); node; node = walker.nextNode()) {
            for (const p of prefixes) {
                if (node.nodeValue.includes(p)) node.nodeValue = node.nodeValue.split(p).join('~/Pictures');
            }
        }
    }, [realpathSync(root), root]);
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
    try {
        const lib = await seed();
        const site = path.join(tmp, 'lib', 'channels');
        mkdirSync(OUT, { recursive: true });
        const only = process.argv.slice(2);
        for (const shot of shots) {
            if (only.length && !only.includes(shot.name)) continue;
            const context = await browser.newContext({
                viewport: shot.viewport || { width: 1440, height: 900 },
                deviceScaleFactor: shot.scale || 2,
                isMobile: !!shot.phone,
                hasTouch: !!shot.phone,
                colorScheme: shot.dark ? 'dark' : 'light',
                locale: 'en-GB',
            });
            await context.addInitScript(([theme, extra]) => {
                localStorage.setItem('theme', theme);
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
        server.kill();
        rmSync(tmp, { recursive: true, force: true });
    }
}

main().catch((err) => { console.error(err.message); process.exit(1); });
