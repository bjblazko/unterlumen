import { test, expect } from '@playwright/test';
import { spawn, execFileSync } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

// Videos are listed and played, never edited (ADR-0058): a tile with its
// length and a clip under the pointer, the video in the viewer, the clip of a
// Live Photo beside its photo, and a switch to hide them all. The videos are
// made here with ffmpeg, on an instance of their own, so no other spec's
// counts change.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..');
const BINARY = path.resolve(ROOT, '..', 'unterlumen');
const PORT = 8094;
const base = `http://127.0.0.1:${PORT}`;
const PHOTO = '2004-05-01_10-07-27_QV-R3_cimg0021';

function makeVideo(file, seconds) {
    execFileSync('ffmpeg', ['-loglevel', 'error', '-f', 'lavfi', '-i', `testsrc=d=${seconds}:s=640x360:r=25`,
        '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-metadata', 'creation_time=2024-07-04T12:25:18Z', '-y', file]);
}

async function waitForServer() {
    for (let i = 0; i < 60; i++) {
        try { if ((await fetch(base + '/api/config')).ok) return; } catch { /* not up yet */ }
        await new Promise(r => setTimeout(r, 250));
    }
    throw new Error('the app did not start');
}

async function openTrip(page) {
    await page.goto(base + '/#folders');
    await page.locator('.dir-item[data-name="trip"]').dblclick();
    await expect(page.locator('.image-item:visible')).toHaveCount(3);
}

test.describe('Videos', () => {
    test.describe.configure({ mode: 'serial', retries: 0 });
    let home, app;

    test.beforeAll(async () => {
        try { execFileSync('ffmpeg', ['-version']); } catch { test.skip(true, 'ffmpeg is not installed'); }
        home = fs.mkdtempSync(path.join(os.tmpdir(), 'unterlumen-videos-'));
        const trip = path.join(home, 'photos', 'trip');
        fs.mkdirSync(trip, { recursive: true });
        makeVideo(path.join(trip, 'clip.mp4'), 4);
        // A Live Photo: a photo and, beside it, its clip of the same name.
        fs.copyFileSync(path.join(ROOT, 'fixtures', 'photos', 'folder-b', `${PHOTO}.jpeg`), path.join(trip, `${PHOTO}.jpeg`));
        makeVideo(path.join(trip, `${PHOTO}.mov`), 2);
        fs.writeFileSync(path.join(trip, 'old.avi'), execFileSync('ffmpeg', ['-loglevel', 'error', '-f', 'lavfi',
            '-i', 'testsrc=d=2:s=320x240', '-c:v', 'mpeg4', '-f', 'avi', 'pipe:1']));
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

    test('a folder lists videos with their length, and a Live Photo once', async ({ page }) => {
        await openTrip(page);
        await expect(page.locator('.image-item[data-name="clip.mp4"] .video-badge')).toHaveText('0:04');
        await expect(page.locator(`.image-item[data-name="${PHOTO}.jpeg"] .video-badge`)).toHaveText('Live');
        await expect(page.locator(`.image-item[data-name="${PHOTO}.mov"]`)).toHaveCount(0);
        await expect(page.locator('.place-header:visible')).toContainText('1 photo');
        await expect(page.locator('.place-header:visible')).toContainText('2 videos');
    });

    test('a video tile plays its clip under the pointer', async ({ page }) => {
        await openTrip(page);
        const clip = page.waitForResponse(r => r.url().includes('/api/clip'));
        await page.locator('.image-item[data-name="clip.mp4"]').hover();
        const res = await clip;
        expect(res.headers()['content-type']).toBe('video/mp4');
        await expect(page.locator('.image-item[data-name="clip.mp4"] .tile-clip')).toHaveClass(/tile-clip-playing/);
    });

    test('a video is served in ranges, one the browser cannot play as a download', async ({ request }) => {
        const part = await request.get(base + '/api/image?path=trip/clip.mp4', { headers: { Range: 'bytes=0-99' } });
        expect(part.status()).toBe(206);
        const avi = await request.get(base + '/api/image?path=trip/old.avi');
        expect(avi.headers()['content-disposition'] || '').toContain('attachment');
    });

    test('the viewer plays a video, offers no crop, and plays a Live Photo', async ({ page }) => {
        await openTrip(page);
        await page.locator('.image-item[data-name="clip.mp4"]').dblclick();
        await expect(page.locator('.viewer-video')).toBeVisible();
        await expect(page.locator('.viewer-crop-btn')).toHaveCount(0);
        await page.locator('.viewer-menu-wrap .menu-btn').click();
        await expect(page.locator('.menu [data-id="location"]')).toHaveCount(0);
        await page.keyboard.press('Escape');
        await page.keyboard.press('Escape');
        await page.locator(`.image-item[data-name="${PHOTO}.jpeg"]`).dblclick();
        await page.locator('.viewer-live-btn').click();
        await expect(page.locator('.viewer-live-clip')).toHaveCount(1);
    });

    test('the controls of a video take clicks, not the Prev and Next strips', async ({ page }) => {
        await openTrip(page);
        await page.locator('.image-item[data-name="clip.mp4"]').dblclick();
        const video = page.locator('.viewer-video');
        await expect(video).toBeVisible();
        const box = await video.boundingBox();
        // The pointer over the photo shows the strips; the play button and the
        // full screen button sit in the bottom corners of the video.
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        const under = await page.evaluate(({ x, y, w }) => [
            document.elementFromPoint(x + 16, y)?.className,
            document.elementFromPoint(x + w - 16, y)?.className,
        ], { x: box.x, y: box.y + box.height - 16, w: box.width });
        expect(under).toEqual(['viewer-video', 'viewer-video']);
    });

    test('a video the browser cannot play says so and offers the file', async ({ page }) => {
        await openTrip(page);
        await page.locator('.image-item[data-name="old.avi"]').dblclick();
        await expect(page.locator('.viewer-video-failed')).toContainText('cannot be played');
        await expect(page.locator('.viewer-video-failed a[download]')).toBeVisible();
    });

    test('Videos hidden in the menu leaves only the photos, until shown again', async ({ page }) => {
        await openTrip(page);
        await page.locator('.place-header:visible .menu-btn').click();
        await page.locator('.menu [data-id="videos"]').click();
        await page.keyboard.press('Escape');
        await expect(page.locator('.image-item:visible')).toHaveCount(1);
        await page.reload();
        await page.locator('.dir-item[data-name="trip"]').dblclick();
        await expect(page.locator('.image-item:visible')).toHaveCount(1);
        await page.locator('.place-header:visible .menu-btn').click();
        await page.locator('.menu [data-id="videos"]').click();
        await page.keyboard.press('Escape');
        await expect(page.locator('.image-item:visible')).toHaveCount(3);
    });

    test('a library holds the videos, and its filter finds them by kind', async ({ page, request }) => {
        const created = await request.post(base + '/api/library/', { data: { name: 'Trip', sourcePath: 'trip' } });
        expect(created.status()).toBe(201);
        const { id } = await created.json();
        const scan = await request.post(`${base}/api/library/${id}/reindex`, { timeout: 120_000 });
        expect(await scan.text()).toContain('"finished":true');

        const videos = await (await request.get(`${base}/api/library/search?ids=${id}&kind=video`)).json();
        expect(videos.results.map(p => p.filename).sort()).toEqual(['clip.mp4', 'old.avi']);
        const photos = await (await request.get(`${base}/api/library/search?ids=${id}&kind=photo`)).json();
        expect(photos.results.map(p => p.filename)).toEqual([`${PHOTO}.jpeg`]);

        // Map, Statistics and the Timeline narrow by kind through the shared filter.
        const stats = await (await request.get(`${base}/api/library/statistics?ids=${id}&kind=video`)).json();
        expect(stats.totalPhotos).toBe(2);

        await page.goto(base + '/#libraries');
        await expect(page.locator('.library-card', { hasText: 'Trip' }).locator('.library-card-count')).toHaveText('1 photo · 2 videos');
        await page.locator('.library-card', { hasText: 'Trip' }).click();
        await expect(page.locator('.image-item[data-name="clip.mp4"]:visible .video-badge')).toHaveText('0:04');
        await expect(page.locator(`.image-item[data-name="${PHOTO}.jpeg"]:visible .video-badge`)).toHaveText('Live');
        const clip = page.waitForResponse(r => r.url().includes('/clip'));
        await page.locator('.image-item[data-name="clip.mp4"]:visible').hover();
        expect((await clip).ok()).toBe(true);
    });

    test('the library filter shows videos only, or photos only, with one press', async ({ page }) => {
        await page.goto(base + '/#libraries');
        await page.locator('.library-card', { hasText: 'Trip' }).click();
        await page.locator('#lib-filter-btn:visible').click();
        await page.locator('.lib-filter-kind:visible [data-kind="video"]').click();
        await expect(page.locator('.image-item:visible')).toHaveCount(2);
        await expect(page.locator('.image-item[data-video]:visible')).toHaveCount(2);
        await page.locator('.lib-filter-kind:visible [data-kind="photo"]').click();
        await expect(page.locator('.image-item:visible')).toHaveCount(1);
        await expect(page.locator('.image-item[data-video]:visible')).toHaveCount(0);
    });

    test('Kind: Videos in the filter overrules videos hidden in the ⋯', async ({ page }) => {
        await page.addInitScript(() => localStorage.setItem('unterlumen.videosShown', 'no'));
        await page.goto(base + '/#libraries');
        await page.locator('.library-card', { hasText: 'Trip' }).click();
        await expect(page.locator('.image-item:visible')).toHaveCount(1);
        await page.locator('#lib-filter-btn:visible').click();
        await page.locator('.lib-filter-kind:visible [data-kind="video"]').click();
        await expect(page.locator('.image-item[data-video]:visible')).toHaveCount(2);
        // Photos and videos again: hidden videos stay hidden.
        await page.locator('.lib-filter-kind:visible [data-kind=""]').click();
        await page.locator('.lib-filter-kind:visible [data-kind="photo"]').click();
        await expect(page.locator('.image-item:visible')).toHaveCount(1);
    });

    test('the filter of Map, Statistics and Timeline offers photos or videos', async ({ page }) => {
        await page.goto(base + '/#statistics');
        await page.locator('.scope-filter-toggle:visible').click();
        await page.locator('.scope-filter-kind:visible').selectOption('video');
        await expect(page.locator('.scope-filter-chips:visible')).toContainText('Kind: Videos');
        await page.locator('.scope-filter-chips:visible .lib-filter-chip', { hasText: 'Kind' }).click();
        await expect(page.locator('.scope-filter-chips .lib-filter-chip', { hasText: 'Kind' })).toHaveCount(0);
    });
});
