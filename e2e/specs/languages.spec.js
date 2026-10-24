import { test, expect } from '@playwright/test';

// ADR-0056: the interface speaks the language chosen in Settings, by default
// the browser's; the choice is this browser's and reloads the page.
test.describe('Languages', () => {
  test('Deutsch in Settings shows the sidebar in German, English brings it back', async ({ page }) => {
    await page.goto('/#settings');
    await page.waitForSelector('#settings-language', { timeout: 10_000 });
    await expect(page.locator('#settings-language [data-language="auto"]')).toHaveAttribute('aria-pressed', 'true');

    await Promise.all([
      page.waitForEvent('load'),
      page.locator('#settings-language [data-language="de"]').click(),
    ]);
    await page.waitForSelector('#settings-language', { timeout: 10_000 });
    await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    await expect(page.locator('#mode-library .nav-text')).toHaveText('Bibliotheken');
    await expect(page.locator('#settings-language [data-language="de"]')).toHaveAttribute('aria-pressed', 'true');

    await Promise.all([
      page.waitForEvent('load'),
      page.locator('#settings-language [data-language="en"]').click(),
    ]);
    await page.waitForSelector('#settings-language', { timeout: 10_000 });
    await expect(page.locator('#mode-library .nav-text')).toHaveText('Libraries');
  });

  test('a German browser gets German by default', async ({ browser }) => {
    const ctx = await browser.newContext({ locale: 'de-DE' });
    const page = await ctx.newPage();
    await page.goto('/#libraries');
    await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    await expect(page.locator('.place-title').first()).toHaveText('Bibliotheken', { timeout: 10_000 });
    await ctx.close();
  });

  test('a new library asks for the language of its keywords', async ({ page }) => {
    await page.goto('/#libraries');
    await page.locator('#lib-new-btn').click();
    const select = page.locator('#lib-dlg-lang');
    await expect(select).toBeVisible();
    await expect(select).toHaveValue('en');
    await expect(select.locator('option')).toHaveText(['English', 'Deutsch']);
    await page.getByRole('button', { name: 'Cancel' }).click();
  });
});
