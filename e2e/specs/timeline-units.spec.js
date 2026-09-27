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
});
