// A silent supercut of Unterlumen for social media: `npm run supercut` in e2e/.
//
// Runs on the same throwaway stage as the README pictures (stage.mjs), drives
// the app scene by scene and records what Chrome paints (the DevTools
// screencast, with timestamps), only while a scene moves: its preparation is
// not in the film. ffmpeg then lays the frames out at 30 fps into a square
// H.264 MP4 without sound. Captions are drawn into the page in the app's own
// type. Needs ffmpeg and Chrome (the 3D view uses WebGPU).
//
// The binary must be built first: cd src && go build -o ../unterlumen .

import { chromium } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { REPO, BASE, preparePhotos, startServer, seed, showHomePath } from '../screenshots/stage.mjs';
import { ready, settle, openFolder, openLibrary } from '../screenshots/shots.mjs';

const OUT = path.join(REPO, 'e2e/video/out');
const SIZE = 1080;          // the film is SIZE × SIZE
const VIEW = 900;           // CSS pixels of the page; scaled to SIZE
const FPS = 30;
// Every cut falls on a bar of 4/4 at BPM, so music of that tempo laid on from
// 0 s cuts with it. A scene lasts one bar unless it says more.
const BPM = 120;
const BAR = 4 * 60 / BPM;   // seconds

const wait = ms => new Promise(r => setTimeout(r, ms));

// A caption at the foot of the frame, in the app's own type and colours.
async function caption(page, text) {
    await page.evaluate((t) => {
        document.getElementById('supercut-caption')?.remove();
        const el = document.createElement('div');
        el.id = 'supercut-caption';
        el.textContent = t;
        Object.assign(el.style, {
            position: 'fixed', left: '50%', bottom: '28px', transform: 'translateX(-50%)', zIndex: 9999,
            background: 'var(--fg)', color: 'var(--bg)', font: '600 24px/1.25 var(--font-sans)',
            padding: '12px 20px', borderRadius: '4px', whiteSpace: 'nowrap',
        });
        document.body.appendChild(el);
    }, text);
}

// Moves the pointer from a to b in steps over ms.
async function glide(page, [x0, y0], [x1, y1], ms, { down = false } = {}) {
    const steps = Math.max(2, Math.round(ms / 33));
    await page.mouse.move(x0, y0);
    if (down) await page.mouse.down();
    for (let i = 1; i <= steps; i++) {
        await page.mouse.move(x0 + (x1 - x0) * i / steps, y0 + (y1 - y0) * i / steps);
        await wait(ms / steps);
    }
    if (down) await page.mouse.up();
}

const chart = (page, title) => page.locator('.stats-chart')
    .filter({ has: page.locator('.stats-chart-title', { hasText: new RegExp(`^${title}$`) }) });

// A 3D view of a statistics topic in its Full view, turned by a drag.
function stage3D(topic, title, text) {
    return {
        caption: text,
        bars: 1,
        prepare: async (page) => {
            await ready(page, BASE, `statistics/${topic}`);
            const stage = chart(page, title);
            await stage.locator('.point-stage-canvas').waitFor();
            await stage.locator('.stats-stage-btn').click();
            await settle(page, 2000);
        },
        act: async (page) => {
            const box = await page.locator('.point-stage-canvas:visible').first().boundingBox();
            const cx = box.x + box.width / 2, cy = box.y + box.height / 2;
            await glide(page, [cx - 180, cy], [cx + 220, cy - 30], BAR * 1000 - 100, { down: true });
        },
    };
}

// Each scene: its caption, a preparation that is not filmed, and a motion
// that is, for `bars` bars of music.
const scenes = [
    {
        caption: 'Cull your photos where they are',
        bars: 1,
        prepare: async (page) => {
            await ready(page, BASE, 'folders');
            await openFolder(page, 'Travel');
            await page.locator('[data-type="image"]').nth(4).click();
            if (!await page.locator('.info-panel.expanded').count()) await page.keyboard.press('i');
            await page.waitForSelector('.info-panel.expanded .maplibregl-canvas');
            await settle(page, 1500);
        },
        act: async (page) => {
            await page.mouse.move(300, 500);
            for (let i = 0; i < 7; i++) { await page.mouse.wheel(0, 50); await wait(200); }
        },
    },
    {
        caption: 'Catalog any folder as a library',
        bars: 1,
        // The library's folders with the four-photo preview of each — not a
        // photo grid, which the scene before already shows.
        prepare: async (page) => {
            await openLibrary(page, BASE);
            await page.waitForFunction(() => document.querySelectorAll('.folder-tile-mosaic img').length >= 8);
            if (!await page.locator('.info-panel.expanded').count()) await page.keyboard.press('i');
            await page.locator('.folder-tile').first().click();
            await settle(page, 1500);
        },
        act: async (page) => {
            const tiles = page.locator('.folder-tile');
            for (let i = 1; i < Math.min(4, await tiles.count()); i++) { await wait(420); await tiles.nth(i).click(); }
        },
    },
    {
        caption: 'Every photo on one map',
        bars: 1,
        prepare: async (page) => {
            await ready(page, BASE, 'map');
            await page.waitForSelector('.map-marker');
            await settle(page, 2500);
        },
        act: async (page) => {
            await page.locator('.map-pane .maplibregl-ctrl-zoom-in').click();
            await wait(750);
            await page.locator('.map-pane .maplibregl-ctrl-zoom-in').click();
        },
    },
    {
        caption: '…and on one timeline',
        bars: 1,
        prepare: async (page) => {
            await ready(page, BASE, 'timeline');
            await page.waitForSelector('.timeline-body img');
            await settle(page, 1500);
        },
        act: async (page) => {
            await page.mouse.move(VIEW / 2 + 100, 300);
            for (let i = 0; i < 7; i++) { await page.mouse.wheel(0, -130); await wait(190); }
        },
    },
    {
        caption: 'Statistics in six topics',
        bars: 1,
        prepare: async (page) => {
            await ready(page, BASE, 'statistics');
            await page.waitForSelector('.stats-card svg');
            await settle(page, 2000);
        },
        act: async (page) => {
            const cards = page.locator('.stats-card');
            for (let i = 0; i < Math.min(5, await cards.count()); i++) { await cards.nth(i).hover(); await wait(380); }
        },
    },
    {
        caption: 'Cameras and lenses, and their photos',
        bars: 1,
        prepare: async (page) => {
            await ready(page, BASE, 'statistics/equipment');
            await page.locator('.lens-cell .stats-pickable').first().waitFor();
            await settle(page, 1500);
        },
        act: async (page) => {
            const cells = page.locator('.lens-cell .stats-pickable');
            await cells.nth(1).hover(); await wait(500);
            await cells.first().click(); await wait(900);
            await cells.nth(2).click();
        },
    },
    {
        caption: 'When you shoot',
        bars: 1,
        prepare: async (page) => {
            await ready(page, BASE, 'statistics/time');
            await chart(page, 'Time of day').locator('svg').waitFor();
            await chart(page, 'Time of day').evaluate(el => el.scrollIntoView({ block: 'start' }));
            await settle(page, 1500);
        },
        act: async (page) => {
            const marks = chart(page, 'Shooting calendar').locator('.stats-pickable');
            const n = await marks.count();
            for (let i = 0; i < 5 && n; i++) { await marks.nth(Math.floor(i * n / 5)).hover(); await wait(380); }
        },
    },
    {
        caption: 'Colours and their combinations',
        bars: 1,
        prepare: async (page) => {
            await ready(page, BASE, 'statistics/colour');
            await chart(page, 'Colour combinations').locator('.combo-row').first().waitFor();
            await chart(page, 'Main colours').evaluate(el => el.scrollIntoView({ block: 'start' }));
            await settle(page, 1200);
        },
        act: async (page) => {
            const rows = chart(page, 'Colour combinations').locator('.combo-row');
            for (let i = 0; i < 4; i++) { await rows.nth(i).hover(); await wait(300); }
            await chart(page, 'Colour combinations').locator('button.combo-item').first().click();
        },
    },
    stage3D('colour', 'Colour space', 'Your library as a cloud of light'),
    stage3D('exposure', 'Exposure space', 'How you set your camera, in 3D'),
    stage3D('places', 'Space and time', 'Home and journeys over the years'),
    {
        // Two phones side by side: the app as a phone shows it, each the real
        // page in a frame of a phone's width.
        caption: 'On your phone, too',
        bars: 1,
        prepare: async (page) => {
            await page.evaluate(() => {
                const el = document.createElement('div');
                el.id = 'supercut-phones';
                Object.assign(el.style, {
                    position: 'fixed', inset: '0', zIndex: 9998, background: 'var(--bg-2)',
                    display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '36px', paddingBottom: '70px',
                });
                el.innerHTML = ['#folders', '#map'].map(hash => `<iframe src="/${hash}" style="width:390px;height:760px;border:10px solid var(--fg);border-radius:36px;background:var(--bg)"></iframe>`).join('');
                document.body.appendChild(el);
            });
            const frames = page.locator('#supercut-phones iframe');
            // The left phone opens a folder, whose photos fill its screen.
            const folders = frames.nth(0).contentFrame();
            await folders.locator('.dir-item[data-name="Travel"]').first().dblclick();
            await folders.locator('[data-type="image"] img').nth(8).waitFor();
            await frames.nth(1).contentFrame().locator('.map-marker').first().waitFor();
            await settle(page, 2500);
        },
        act: async (page) => {
            await page.mouse.move(255, 420);
            for (let i = 0; i < 6; i++) { await page.mouse.wheel(0, 40); await wait(250); }
        },
    },
];

// The end: the logo, what it is, where to get it — a layer over the app,
// in its own type and colours.
async function endCard(page) {
    await page.evaluate(() => {
        document.getElementById('supercut-caption')?.remove();
        const el = document.createElement('div');
        Object.assign(el.style, {
            position: 'fixed', inset: '0', zIndex: 10000, background: 'var(--bg)', color: 'var(--fg)',
            display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '20px',
            fontFamily: 'var(--font-sans)',
        });
        el.innerHTML = `<img src="/favicon.svg" alt="" style="width:112px;height:112px">
            <div style="font-size:52px;font-weight:600">Unterlumen</div>
            <div style="font-size:24px;color:var(--fg-2)">Browse, cull and publish your photos. No cloud.</div>
            <div style="font:400 26px var(--font-mono);margin-top:8px">huepattl.de/unterlumen</div>`;
        document.body.appendChild(el);
    });
    await page.waitForFunction(() => document.querySelector('img[src="/favicon.svg"]')?.complete);
    await wait(300);
}

// The frames Chrome paints, kept only while `on` is true.
async function screencast(page) {
    const cdp = await page.context().newCDPSession(page);
    const frames = [];
    const rec = { on: false, frames, segments: [] };
    cdp.on('Page.screencastFrame', async ({ data, metadata, sessionId }) => {
        if (rec.on) frames.push({ t: metadata.timestamp, data, seg: rec.segments.length - 1 });
        await cdp.send('Page.screencastFrameAck', { sessionId }).catch(() => {});
    });
    await cdp.send('Page.startScreencast', { format: 'jpeg', quality: 92, maxWidth: SIZE, maxHeight: SIZE });
    rec.start = async (seconds) => {
        rec.segments.push({ seconds, t0: Date.now() / 1000 });
        rec.on = true;
        // A static page paints no frame: a pixel that changes its colour
        // at each start makes Chrome paint one.
        await page.evaluate(() => {
            let px = document.getElementById('supercut-nudge');
            if (!px) {
                px = document.createElement('div');
                px.id = 'supercut-nudge';
                Object.assign(px.style, { position: 'fixed', right: '0', bottom: '0', width: '1px', height: '1px', zIndex: 10001, pointerEvents: 'none' });
                document.body.appendChild(px);
            }
            px.style.background = px.style.background === 'rgba(0, 0, 0, 0.01)' ? 'rgba(0, 0, 0, 0.02)' : 'rgba(0, 0, 0, 0.01)';
        });
    };
    rec.stop = () => { rec.on = false; };
    rec.end = () => cdp.send('Page.stopScreencast').catch(() => {});
    return rec;
}

// Writes the film frame by frame: each scene exactly seconds × FPS frames,
// each frame the last one Chrome painted by then. Counting frames rather
// than durations keeps every cut on its bar.
function writeFrames(tmp, rec) {
    const dir = path.join(tmp, 'frames');
    mkdirSync(dir);
    let n = 0;
    rec.segments.forEach((seg, s) => {
        const frames = rec.frames.filter(f => f.seg === s);
        if (!frames.length) throw new Error(`Scene ${s + 1} painted nothing.`);
        const start = frames[0].t;
        let at = 0;
        const count = Math.round(seg.seconds * FPS);
        for (let k = 0; k < count; k++) {
            while (at + 1 < frames.length && frames[at + 1].t - start <= k / FPS) at++;
            writeFileSync(path.join(dir, `${String(n++).padStart(5, '0')}.jpg`), Buffer.from(frames[at].data, 'base64'));
        }
    });
    console.log(`  ${rec.segments.length} cuts on the bars of ${BPM} BPM, ${n} frames, ${(n / FPS).toFixed(1)} s`);
    return path.join(dir, '%05d.jpg');
}

async function main() {
    const tmp = mkdtempSync(path.join(tmpdir(), 'unterlumen-supercut-'));
    const root = preparePhotos(tmp);
    const server = await startServer(tmp, root);
    const browser = await chromium.launch({ channel: 'chrome', args: ['--enable-unsafe-webgpu'] });
    try {
        await seed();
        const context = await browser.newContext({
            viewport: { width: VIEW, height: VIEW },
            deviceScaleFactor: SIZE / VIEW,
            colorScheme: 'light',
            locale: 'en-GB',
        });
        await context.addInitScript(() => {
            localStorage.setItem('theme', 'light');
            localStorage.setItem('sidebar-collapsed', '1'); // the photos get the room
        });
        const page = await context.newPage();
        const rec = await screencast(page);
        for (const scene of scenes) {
            await scene.prepare(page);
            for (const frame of page.frames()) await showHomePath(frame, root);
            await caption(page, scene.caption);
            await page.mouse.move(VIEW - 2, 2);
            const seconds = (scene.bars || 1) * BAR;
            await rec.start(seconds);
            const started = Date.now();
            await scene.act(page);
            await wait(Math.max(0, seconds * 1000 - (Date.now() - started)));
            rec.stop();
            console.log(`  ${scene.caption}`);
        }
        await endCard(page);
        await rec.start(2 * BAR);
        await wait(2 * BAR * 1000 + 100);
        rec.stop();
        await rec.end();

        mkdirSync(OUT, { recursive: true });
        const out = path.join(OUT, 'unterlumen-supercut.mp4');
        execFileSync('ffmpeg', ['-y', '-loglevel', 'error', '-framerate', String(FPS), '-i', writeFrames(tmp, rec),
            '-vf', `scale=${SIZE}:${SIZE}:flags=lanczos,format=yuv420p`,
            '-c:v', 'libx264', '-preset', 'slow', '-crf', '18', '-movflags', '+faststart', '-an', out]);
        console.log(`\n  ${path.relative(REPO, out)}`);
    } finally {
        await browser.close();
        server.kill();
        rmSync(tmp, { recursive: true, force: true });
    }
}

main().catch((err) => { console.error(err.message); process.exit(1); });
