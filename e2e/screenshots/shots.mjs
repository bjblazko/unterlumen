// The screenshots, in README order. Each opens a place and puts it in the
// state worth showing; take.mjs captures it. `name` is the file name.

const IMAGE = '[data-type="image"]';

async function ready(page, base, hash) {
    await page.goto(`${base}/#${hash}`);
    await page.waitForFunction(() => document.querySelector('.nav-item[aria-current="page"]') && document.querySelector('#app > *'));
}

// Images have loaded when every visible thumbnail has a size.
async function settle(page, ms = 800) {
    await page.waitForLoadState('networkidle', { timeout: 20_000 }).catch(() => {});
    await page.waitForFunction(() => [...document.images]
        .filter(img => img.offsetParent && img.loading !== 'lazy')
        .every(img => img.complete), null, { timeout: 20_000 }).catch(() => {});
    await page.waitForTimeout(ms);
}

async function openFolder(page, name) {
    const dir = page.locator(`.dir-item[data-name="${name}"]`).first();
    await dir.waitFor();
    if (await page.evaluate(() => matchMedia('(hover: none)').matches)) await dir.tap();
    else await dir.dblclick();
    await page.waitForSelector(`.crumb[data-path="${name}"]`);
    await page.locator(IMAGE).first().waitFor();
}

async function folderWithInfo(page, base) {
    await ready(page, base, 'folders');
    await openFolder(page, 'Travel');
    const rome = page.locator(`${IMAGE}[data-name="${ROME}"]`);
    await rome.click();
    if (!await page.locator('.info-panel.expanded').count()) await page.keyboard.press('i');
    await page.waitForSelector('.info-panel.expanded .maplibregl-canvas');
    // The open panel narrows the grid; scroll once the rows have settled.
    await page.waitForTimeout(500);
    await rome.evaluate(el => el.scrollIntoView({ block: 'center' }));
    await settle(page, 3000);
}

// Selects `count` photos from `first` on, as a person would with Cmd-click.
async function selectPhotos(page, first, count) {
    const images = page.locator(IMAGE);
    await images.nth(first).click();
    for (let i = 1; i < count; i++) await images.nth(first + i).click({ modifiers: ['ControlOrMeta'] });
}

async function openLibrary(page, base) {
    await ready(page, base, 'libraries');
    await page.locator('.library-card', { hasText: 'Pictures' }).locator('.lib-open').click();
    await page.waitForSelector('#lib-pane [data-type="dir"], #lib-pane [data-type="image"]');
}

async function selectionDialog(page, base, action) {
    await ready(page, base, 'folders');
    await openFolder(page, 'Travel');
    await selectPhotos(page, 3, 4);
    await page.locator(`.selection-bar [data-action="${action}"]`).click();
    await page.waitForSelector('.dialog');
    await settle(page, 2500);
}

const ROME = '2009-09-08_14-41-41_iPhone_IMG_0311.jpeg';
const FUJI = '2024-07-04_14-09-43_X-T50_DSCF3258.jpeg';
const PHONE = { viewport: { width: 390, height: 844 }, scale: 3, phone: true };

export const shots = [
    {
        name: 'folders',
        take: (page, { base }) => folderWithInfo(page, base),
    },
    {
        name: 'folders-dark',
        dark: true,
        take: (page, { base }) => folderWithInfo(page, base),
    },
    {
        name: 'viewer',
        take: async (page, { base }) => {
            await ready(page, base, 'folders');
            await openFolder(page, 'Travel');
            await page.locator(`${IMAGE}[data-name="${FUJI}"]`).dblclick();
            await page.waitForSelector('.viewer img');
            await page.keyboard.press('f');
            await page.keyboard.press('i');
            await settle(page, 2500);
        },
    },
    {
        name: 'organize',
        storage: { 'organize.targets': JSON.stringify([{ path: '2017' }, { path: '2025' }, { path: 'Odds and ends' }]) },
        take: async (page, { base }) => {
            await ready(page, base, 'organize');
            await page.waitForSelector('.organize-source .browse-header');
            await openFolder(page, 'Travel');
            await selectPhotos(page, 6, 3);
            await settle(page, 1500);
        },
    },
    {
        name: 'marked',
        take: async (page, { base }) => {
            await ready(page, base, 'folders');
            await openFolder(page, 'Travel');
            await selectPhotos(page, 10, 4);
            await page.keyboard.press('Delete');
            await page.locator('#mode-wastebin').click();
            await page.waitForSelector('.wastebin-actions');
            await settle(page, 1500);
        },
    },
    {
        // A folder selected, with the info panel's dashboard of it beside the
        // tiles: what it holds, by type, camera and lens, and hour.
        name: 'library',
        take: async (page, { base }) => {
            await openLibrary(page, base);
            // The previews follow the folder names; wait for all four tiles.
            await page.waitForFunction(() => document.querySelectorAll('.folder-tile-mosaic img').length >= 13);
            if (!await page.locator('.info-panel.expanded').count()) await page.keyboard.press('i');
            await page.waitForSelector('.info-panel.expanded');
            await page.locator('.folder-tile[data-name="Travel"]').click();
            await page.waitForSelector('.info-panel.expanded .folder-type-chart');
            await page.evaluate(() => document.activeElement?.blur());
            await settle(page, 2500);
        },
    },
    {
        name: 'library-filter',
        take: async (page, { base }) => {
            await openLibrary(page, base);
            await page.locator('#lib-filter-btn').click();
            const camera = page.locator('.lib-text-filter-select').first();
            await camera.waitFor();
            await camera.selectOption({ index: 1 });
            await page.waitForSelector('#lib-results-pane [data-type="image"]');
            await settle(page, 2000);
        },
    },
    {
        // Sharing is per library (ADR-0047): the switch in Edit library,
        // turned on, with the sentence saying what it writes where.
        name: 'library-sharing',
        take: async (page, { base }) => {
            await openLibrary(page, base);
            await page.locator('#lib-edit-btn').click();
            await page.waitForSelector('#lib-edit-share .toggle');
            await page.locator('#lib-edit-share .toggle').click();
            await page.waitForSelector('#lib-edit-share .toggle[data-state="on"]');
            await page.waitForFunction(() => !document.querySelector('#lib-edit-share-hint').textContent.endsWith('\u2026'));
            await page.evaluate(() => document.activeElement?.blur());
            await settle(page, 1000);
        },
    },
    {
        // The overview: a card per topic with its charts small, the topics
        // under Statistics in the sidebar.
        name: 'statistics',
        gpu: true,
        take: async (page, { base }) => {
            await page.emulateMedia({ reducedMotion: 'reduce' });
            await ready(page, base, 'statistics');
            await page.waitForSelector('.stats-card svg');
            await page.waitForSelector('.stats-card .point-stage-canvas');
            await settle(page, 2000);
        },
    },
    {
        // A topic with a lens clicked: its photos in the column beside the charts.
        name: 'statistics-photos',
        take: async (page, { base }) => {
            await ready(page, base, 'statistics/equipment');
            await page.waitForSelector('.stats-chart svg');
            await page.locator('.lens-cell .stats-pickable').first().click();
            await page.waitForSelector('#stats-photos .photo-column-tile img');
            await page.mouse.move(1, 1); // no tooltip in the picture
            await page.evaluate(() => document.activeElement?.blur());
            await settle(page, 2000);
        },
    },
    {
        // Colour combinations beside Main colours, and the wheel to choose
        // one's own, with orange and teal's photos in the column.
        name: 'statistics-colour-combos',
        take: async (page, { base }) => {
            await ready(page, base, 'statistics/colour');
            const chart = title => page.locator('.stats-chart').filter({ has: page.locator('.stats-chart-title', { hasText: new RegExp(`^${title}$`) }) });
            await chart('Colour combinations').locator('.combo-row').first().waitFor();
            await chart('Main colours').evaluate(el => el.scrollIntoView({ block: 'start' }));
            await chart('Colour combinations').locator('button.combo-item').first().click();
            await page.waitForSelector('#stats-photos .photo-column-tile img');
            await chart('Main colours').evaluate(el => el.scrollIntoView({ block: 'start' }));
            await page.mouse.move(1, 1);
            await page.evaluate(() => document.activeElement?.blur());
            await settle(page, 2000);
        },
    },
    ...[
        // The 3D views, each in its topic's Full view with the photos beside it.
        ['statistics-colour-space', 'colour', 'Colour space'],
        ['statistics-exposure-space', 'exposure', 'Exposure space'],
        ['statistics-daylight', 'time', 'Daylight'],
        ['statistics-space-time', 'places', 'Space and time'],
    ].map(([name, topic, title]) => ({
        name,
        gpu: true,
        take: async (page, { base }) => {
            await page.emulateMedia({ reducedMotion: 'reduce' });
            await ready(page, base, `statistics/${topic}`);
            const stage = page.locator('.stats-chart').filter({ has: page.locator('.stats-chart-title', { hasText: new RegExp(`^${title}$`) }) });
            await stage.locator('.point-stage-canvas').waitFor();
            await stage.locator('.stats-stage-btn').click();
            await page.mouse.move(1, 1);
            await page.evaluate(() => document.activeElement?.blur());
            await settle(page, 2500);
        },
    })),
    {
        name: 'map',
        storage: { 'map-photos-open': '1' },
        take: async (page, { base }) => {
            await ready(page, base, 'map');
            await page.waitForSelector('.map-marker');
            await page.waitForSelector('#map-photos .photo-column-tile img');
            await settle(page, 4000);
        },
    },
    {
        name: 'timeline',
        take: async (page, { base }) => {
            await ready(page, base, 'timeline');
            await page.waitForSelector('.timeline-tile img');
            // A photo selected, so the info panel shows what a click gives.
            await page.locator('.timeline-tile').nth(3).click();
            await page.evaluate(() => document.activeElement?.blur());
            await settle(page, 2000);
        },
    },
    {
        name: 'rename',
        take: (page, { base }) => selectionDialog(page, base, 'rename'),
    },
    {
        name: 'export',
        take: (page, { base }) => selectionDialog(page, base, 'export'),
    },
    {
        name: 'location',
        take: async (page, { base }) => {
            await selectionDialog(page, base, 'location');
            await page.locator('.dialog input').nth(0).fill('41.8986');
            await page.locator('.dialog input').nth(1).fill('12.4769');
            await page.evaluate(() => document.activeElement.blur());
            await settle(page, 2500);
        },
    },
    {
        name: 'galleries',
        take: async (page, { base }) => {
            await ready(page, base, 'galleries');
            await page.waitForSelector('.gal-row');
            await settle(page, 3000);
        },
    },
    {
        name: 'website',
        take: async (page, { site }) => {
            await page.goto(`file://${site}/website/site/index.html`);
            await settle(page, 1500);
        },
    },
    {
        name: 'website-album',
        take: async (page, { site }) => {
            await page.goto(`file://${site}/website/site/index.html`);
            await page.locator('a[href*="rhodes"]').first().click();
            await settle(page, 1500);
        },
    },
    {
        name: 'share-link',
        take: async (page, { site }) => {
            const { readdirSync } = await import('node:fs');
            const id = readdirSync(`${site}/friends`).find(d => /^[0-9a-f]{24}$/.test(d));
            await page.goto(`file://${site}/friends/${id}/index.html`);
            await settle(page, 1500);
        },
    },
    {
        name: 'phone-folders',
        ...PHONE,
        take: async (page, { base }) => {
            await ready(page, base, 'folders');
            await openFolder(page, 'Travel');
            await settle(page, 1500);
        },
    },
    {
        name: 'phone-map',
        ...PHONE,
        storage: { 'map-photos-open': '1' },
        take: async (page, { base }) => {
            await ready(page, base, 'map');
            await page.waitForSelector('#map-photos .photo-column-tile img');
            await settle(page, 4000);
        },
    },
    {
        name: 'phone-timeline',
        ...PHONE,
        take: async (page, { base }) => {
            await ready(page, base, 'timeline');
            await page.waitForSelector('.timeline-tile img');
            await settle(page, 2000);
        },
    },
];
