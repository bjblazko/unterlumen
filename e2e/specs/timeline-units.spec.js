import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// Browser-side checks of the timeline's arithmetic (ADR-0040). The classes are
// globals of the app page, so the page is loaded once and they are called in it.

test.describe('Timeline units', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
    });

    test('the calendar counts days in UTC and knows months', async ({ page }) => {
        const r = await page.evaluate(() => {
            const c = new TimelineCalendar('2020-02-28', 400);
            return {
                leap: c.formatDay(1), march: c.formatMonth(2), long: c.formatMonthLong(2),
                marchStart: c.monthStart(2020 * 12 + 2), marchEnd: c.monthEnd(2020 * 12 + 2),
                first: c.monthStart(c.firstMonth), y2021: c.yearStart(2021),
                back: c.dayAt(c.msOf(123)),
            };
        });
        expect(r).toEqual({ leap: 'Sat 29 Feb 2020', march: 'Mar 2020', long: 'March 2020',
            marchStart: 2, marchEnd: 33, first: 0, y2021: 308, back: 123 });
    });

    test('a one-day stream has a usable calendar', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-07-14', days: [0, 0, 0], ratios: [1.5, 0.667, 1], undated: 0 });
            return { span: s.span, prefix: [...s.prefix], month: s.monthCount(s.calendar.firstMonth), label: s.calendar.formatMonth(0) };
        });
        expect(r).toEqual({ span: 1, prefix: [0, 3], month: 3, label: 'Jul 2024' });
    });

    test('an empty stream has no calendar', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '', days: [], ratios: [], undated: 5 });
            return { count: s.count, span: s.span, calendar: s.calendar, undated: s.undated };
        });
        expect(r).toEqual({ count: 0, span: 0, calendar: null, undated: 5 });
    });

    test('details load by page and a stale version is reported', async ({ page }) => {
        await page.route('**/api/timeline/photos?*', (route) => {
            const u = new URL(route.request().url());
            if (u.searchParams.get('v') === 'old') return route.fulfill({ status: 409, body: 'changed' });
            const from = Number(u.searchParams.get('from'));
            route.fulfill({ contentType: 'application/json',
                body: JSON.stringify({ photos: [[`L`, `id${from}`, `f${from}.jpg`, '2024-01-01T00:00:00']] }) });
        });
        const r = await page.evaluate(async () => {
            const s = new TimelineStream();
            s.adopt({ version: 'v1', start: '2024-01-01', days: new Array(600).fill(0), ratios: new Array(600).fill(1.5), undated: 0 });
            let details = 0; s.onDetails = () => details++;
            await s.ensure(0, 1);
            const first = s.detail(0), missing = s.detail(501);
            s.version = 'old';
            let stale = 0; s.onStale = () => stale++;
            await s.ensure(500, 501);
            return { first, missing, details, stale };
        });
        expect(r).toEqual({ first: { lib: 'L', id: 'id0', name: 'f0.jpg', taken: '2024-01-01T00:00:00' }, missing: null, details: 1, stale: 1 });
    });

    test('the axis maps days to pixels and back, also for a one-day domain', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-01-01', days: [0, 10, 99], ratios: [1.5, 1.5, 1.5], undated: 0 });
            const axis = new TimelineAxis(s, { height: 92, onSeek() {} });
            axis.el.style.width = '1000px';
            document.body.appendChild(axis.el);
            axis.redraw();
            const wide = { x10: axis.x(10), back: axis.dayAt(axis.x(10) + 1) };
            axis.setDomain(10, 10);
            const one = { x: axis.x(10), end: axis.x(11), back: axis.dayAt(500) };
            axis.el.remove();
            return { wide, one };
        });
        expect(r.wide).toEqual({ x10: 100, back: 10 });
        expect(r.one).toEqual({ x: 0, end: 1000, back: 10 });
    });

    test('the range snaps to whole months', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-01-01', days: [0, 400], ratios: [1.5, 1.5], undated: 0 });
            let limit = null;
            const range = new TimelineRange(s, { height: 40, onLimit: (a, b) => { limit = [a, b]; } });
            range.el.style.width = '800px';
            document.body.appendChild(range.el);
            range.redraw();
            range.set(40, 70);
            range.el.remove();
            return { limit, from: s.calendar.formatDay(limit[0]), until: s.calendar.formatDay(limit[1]) };
        });
        expect(r.from).toBe('Thu 1 Feb 2024');
        expect(r.until).toBe('Sun 31 Mar 2024');
    });

    test('tiles are created near the screen and released when they leave', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-01-01', days: [0, 0, 1], ratios: [1.5, 1.5, 1.5], undated: 0 });
            const layer = document.createElement('div');
            const tiles = new TimelineTiles(layer, s);
            tiles.show([{ i: 0, x: 0, y: 0, w: 10, h: 10 }, { i: 1, x: 12, y: 0, w: 10, h: 10 }]);
            const two = layer.children.length;
            tiles.select(1);
            tiles.show([{ i: 1, x: 0, y: 0, w: 10, h: 10 }, { i: 2, x: 12, y: 0, w: 10, h: 10 }]);
            return { two, after: [...layer.children].map(c => c.dataset.i), selected: layer.querySelector('.is-selected')?.dataset.i, size: tiles.size };
        });
        expect(r).toEqual({ two: 2, after: ['1', '2'], selected: '1', size: 2 });
    });

    test('the band puts each photo in the row that ends furthest left, a month per column', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-01-30', days: [0, 0, 0, 3], ratios: [1, 2, 1, 1], undated: 0 });
            s.ensure = () => Promise.resolve();
            const band = new TimelineBand(s, { rows: 2, onView() {}, onSelect() {}, onOpen() {}, onCount() {} });
            Object.assign(band.el.style, { width: '600px', height: '314px', position: 'absolute' });
            document.body.appendChild(band.el);
            band.setRange(0, s.span - 1);
            const out = { rows: [...band.rows_], xs: [...band.xs], labels: band.labels.map(l => l.x) };
            band.el.remove();
            return out;
        });
        // row height = (314 - 14 scrollbar - 16 pad - 28 labels - 4 gap) / 2 = 126
        expect(r.rows).toEqual([0, 1, 0, 0]);
        expect(r.xs[0]).toBe(16);
        expect(r.xs[1]).toBe(16);
        expect(r.xs[2]).toBe(16 + 126 + 4);
        expect(r.labels.length).toBe(2);
        expect(r.xs[3]).toBe(r.labels[1]);
    });

    test('the phone list starts with the newest month and the scrubber has it on top', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2023-12-30', days: [0, 1, 40, 41], ratios: [1.5, 1.5, 1.5, 0.667], undated: 0 });
            s.ensure = () => Promise.resolve();
            const list = new TimelineList(s, { onView() {}, onOpen() {}, onCount() {} });
            Object.assign(list.el.style, { width: '390px', height: '700px', position: 'absolute' });
            document.body.appendChild(list.el);
            list.relayout();
            const heads = list.blocks.filter(b => b.t === 'h').map(b => b.label);
            const scrub = new TimelineScrubber(s, { onSeek() {} });
            Object.assign(scrub.el.style, { height: '700px', position: 'absolute' });
            document.body.appendChild(scrub.el);
            scrub.redraw();
            const order = scrub.y(41) < scrub.y(0);
            list.el.remove(); scrub.el.remove();
            return { heads, order, top: list.blocks[0].d };
        });
        expect(r.heads).toEqual(['February 2024', 'December 2023']);
        expect(r.order).toBe(true);
        expect(r.top).toBe(41);
    });
});
