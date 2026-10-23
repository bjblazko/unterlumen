// stage.mjs — the throwaway Unterlumen the README pictures and the
// supercut video are made on: a fresh copy of src/examples with the folders
// renamed like a photo collection, a library over it, destinations and
// galleries through the API, on port 8097.

import { spawn } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, realpathSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const HERE = path.dirname(fileURLToPath(import.meta.url));
export const REPO = path.resolve(HERE, '../..');
export const BINARY = path.join(REPO, 'unterlumen');
export const PORT = 8097;
export const BASE = `http://127.0.0.1:${PORT}`;

// The example folders, as a person would name them.
const FOLDERS = {
    'folder-b': 'Travel',
    'folder-a/a2': '2017',
    'folder-a/a3': '2025',
    'folder-a/a1': 'Odds and ends',
};

export function preparePhotos(tmp) {
    const root = path.join(tmp, 'Pictures');
    for (const [from, to] of Object.entries(FOLDERS)) {
        cpSync(path.join(REPO, 'src/examples', from), path.join(root, to), { recursive: true });
    }
    // Invented people for People (e2e/faces/ABOUT.md): no real person is shown.
    cpSync(path.join(REPO, 'e2e/faces'), path.join(root, 'Friends'), { recursive: true });
    for (const note of ['ABOUT.md', 'LICENSE']) rmSync(path.join(root, 'Friends', note));
    return root;
}

// ONNX Runtime and the face models: the ones the e2e run keeps, or the
// app's own download.
const RECOGNITION = path.join(REPO, 'e2e/fixtures/.unterlumen-faces/recognition');

export async function startServer(tmp, root) {
    const lib = path.join(tmp, 'lib');
    mkdirSync(lib);
    if (existsSync(RECOGNITION)) cpSync(RECOGNITION, path.join(lib, 'recognition'), { recursive: true });
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

export async function api(method, url, body) {
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
export async function seed() {
    const lib = await api('POST', '/api/library/', { name: 'Pictures', description: 'Everything', sourcePath: '.' });
    await api('POST', `/api/library/${lib.id}/reindex`);
    // The measurements the colour charts and 3D views read; returns when done.
    await api('POST', `/api/library/${lib.id}/analyse`);
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
    await seedPeople(lib);
    return lib;
}

// The people of Friends, named after a portrait of each; the faces the
// detector did not put with them stay waiting, some with a suggestion.
const PEOPLE = {
    'portraits-12': 'Mei', 'portraits-11': 'Amara', 'portraits-13': 'Ravi', 'portraits-14': 'Klaus',
    'portraits-31': 'Leila', 'portraits-32': 'Kofi', 'portraits-33': 'Grace', 'portraits-34': 'Jun',
    'portraits-24': 'Klaus', 'portraits-42': 'Sofia',
};

async function seedPeople(lib) {
    if (!(await api('GET', '/api/faces')).installed) await api('POST', '/api/faces/install');
    for (let i = 0; i < 300; i++) {
        const o = await api('GET', `/api/library/${lib.id}/people`);
        if (!o.pending && !o.finding) break;
        await new Promise(r => setTimeout(r, 1000));
    }
    const tile = f => f.file.replace(/^.*_/, '').replace(/\.jpg$/, '');
    const { groups } = await api('GET', `/api/library/${lib.id}/people`);
    for (const g of groups) {
        const faces = await api('GET', `/api/library/${lib.id}/people/groups/${g.id}`);
        const name = faces.map(f => PEOPLE[tile(f)]).find(Boolean);
        if (name) await api('POST', `/api/library/${lib.id}/faces/name`, { faces: faces.map(f => f.id), name });
    }
    await api('PUT', '/api/people/birthdays', { name: 'Mei', birthday: '1994-03-08' });
}

// The photos live in a temporary folder; the pictures show where a person
// keeps theirs.
export async function showHomePath(page, root) {
    await page.evaluate((prefixes) => {
        const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
        for (let node = walker.nextNode(); node; node = walker.nextNode()) {
            for (const p of prefixes) {
                if (node.nodeValue.includes(p)) node.nodeValue = node.nodeValue.split(p).join('~/Pictures');
            }
        }
    }, [realpathSync(root), root]);
}

