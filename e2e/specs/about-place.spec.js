import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// About is a place with topics (How it works, Your data, No warranty,
// Licenses), reached from the logo and the sidebar; a first start says once
// that Unterlumen comes without warranty (feature doc 2026-10-04-about-place).

test.describe('About', () => {
  test('the logo and the sidebar entry open About, whose page lists its topics', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);

    await page.locator('#about-trigger').click();
    await expect(page).toHaveURL(/#about$/);
    await expect(page.locator('#mode-about')).toHaveAttribute('aria-current', 'page');
    await expect(page.locator('.guide-pane h1')).toHaveText('About Unterlumen');

    // The topics are on the page, not in the sidebar; About stands for them.
    await page.locator('.guide-pane a[href="#privacy"]').first().click();
    await expect(page).toHaveURL(/#privacy$/);
    await expect(page.locator('#mode-about')).toHaveAttribute('aria-current', 'page');
  });

  test('Your data says what leaves and that nothing checks for updates', async ({ page }) => {
    await page.goto('/#privacy');
    const body = page.locator('.reading-body');
    await expect(body).toContainText('does not check for updates');
    await expect(body).toContainText('tiles.openfreemap.org');
    await expect(body).toContainText('no login');
  });

  test('No warranty says to keep a backup', async ({ page }) => {
    await page.goto('/#warranty');
    await expect(page.locator('.reading-body')).toContainText('without warranty of any kind');
    await expect(page.locator('.reading-body h2', { hasText: 'Keep a backup' })).toBeVisible();
  });

  test('a first start says once that there is no warranty', async ({ page }) => {
    await page.goto('/#folders');
    await waitForAppReady(page);
    const notice = page.locator('.sidebar .warranty-notice');
    await expect(notice).toContainText('without any warranty');

    await notice.locator('a[data-mode="warranty"]').click();
    await expect(page).toHaveURL(/#warranty$/);
    await expect(notice).toBeVisible();

    await notice.getByRole('button', { name: 'Understood' }).click();
    await expect(notice).toHaveCount(0);
    await page.reload();
    await waitForAppReady(page);
    await expect(page.locator('.warranty-notice')).toHaveCount(0);
  });
});
