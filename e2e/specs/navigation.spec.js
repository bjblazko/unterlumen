import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// The sidebar of places replaced the chevron stepper (ADR-0028). What matters
// here is that places behave like places: they have addresses, the current one
// is marked, and nothing pretends to be "done".

test.describe('Navigation — places, not steps', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await waitForAppReady(page);
  });

  test('nav entries are links with an address, not buttons', async ({ page }) => {
    for (const [id, hash] of [['mode-browse', '#folders'], ['mode-wastebin', '#marked'],
                              ['mode-commander', '#organize'], ['mode-library', '#libraries'],
                              ['mode-published', '#published']]) {
      const el = page.locator(`#${id}`);
      await expect(el).toHaveJSProperty('tagName', 'A');
      expect(await el.getAttribute('href')).toBe(hash);
    }
  });

  test('the current place is marked and it is the only one', async ({ page }) => {
    await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
    await page.locator('#mode-published').click();
    await expect(page.locator('#mode-published')).toHaveAttribute('aria-current', 'page');
    await expect(page.locator('#mode-browse')).not.toHaveAttribute('aria-current', 'page');
    await expect(page.locator('.nav-item[aria-current="page"]')).toHaveCount(1);
  });

  test('no step is shown as completed or upcoming', async ({ page }) => {
    await page.locator('#mode-commander').click();
    await expect(page.locator('.workflow-step, .completed')).toHaveCount(0);
  });

  test('switching places updates the address', async ({ page }) => {
    await page.locator('#mode-wastebin').click();
    await expect(page).toHaveURL(/#marked$/);
  });

  test('the browser back button returns to the previous place', async ({ page }) => {
    await page.locator('#mode-wastebin').click();
    await expect(page).toHaveURL(/#marked$/);
    await page.locator('#mode-published').click();
    await expect(page).toHaveURL(/#published$/);
    await page.goBack();
    await expect(page).toHaveURL(/#marked$/);
    await expect(page.locator('#mode-wastebin')).toHaveAttribute('aria-current', 'page');
  });

  test('a deep link opens that place directly', async ({ page }) => {
    await page.goto('/#organize');
    await waitForAppReady(page);
    await expect(page.locator('#mode-commander')).toHaveAttribute('aria-current', 'page');
  });

  test('number shortcuts still select places', async ({ page }) => {
    await page.keyboard.press('2');
    await expect(page.locator('#mode-wastebin')).toHaveAttribute('aria-current', 'page');
    await page.keyboard.press('1');
    await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
  });

  test('backslash collapses the sidebar and the state is remembered', async ({ page }) => {
    await expect(page.locator('.shell')).not.toHaveClass(/sidebar-collapsed/);
    await page.keyboard.press('\\');
    await expect(page.locator('.shell')).toHaveClass(/sidebar-collapsed/);
    await expect(page.locator('#sidebar-collapse')).toHaveAttribute('aria-expanded', 'false');

    await page.reload();
    await waitForAppReady(page);
    await expect(page.locator('.shell')).toHaveClass(/sidebar-collapsed/);

    await page.keyboard.press('\\');
    await expect(page.locator('.shell')).not.toHaveClass(/sidebar-collapsed/);
  });
});
