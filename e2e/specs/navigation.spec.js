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
                              ['mode-organize', '#organize'], ['mode-library', '#libraries'],
                              ['mode-published', '#galleries']]) {
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
    await page.locator('#mode-organize').click();
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
    await expect(page).toHaveURL(/#galleries$/);
    await page.goBack();
    await expect(page).toHaveURL(/#marked$/);
    await expect(page.locator('#mode-wastebin')).toHaveAttribute('aria-current', 'page');
  });

  test('a deep link opens that place directly', async ({ page }) => {
    await page.goto('/#organize');
    await waitForAppReady(page);
    await expect(page.locator('#mode-organize')).toHaveAttribute('aria-current', 'page');
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

  // The rule that hides the words once lost its own braces, so the collapsed
  // rail kept every label and clipped them to "F…", "Paula…" instead.
  test('the collapsed sidebar is a rail of icons, with no words left over', async ({ page }) => {
    await page.keyboard.press('\\');
    await expect(page.locator('.shell')).toHaveClass(/sidebar-collapsed/);

    await expect(page.locator('#mode-browse .nav-text')).toBeHidden();
    await expect(page.locator('.sidebar .nav-label').first()).toBeHidden();
    await expect(page.locator('.brand-name')).toBeHidden();
    await expect(page.locator('.sidebar .nav-item.nav-sub').first()).toBeHidden();

    // The icons stay, and the rail stays narrow enough to be a rail.
    await expect(page.locator('#mode-browse svg')).toBeVisible();
    const width = await page.locator('.sidebar').evaluate(el => el.getBoundingClientRect().width);
    expect(width).toBeLessThan(80);
  });
});

test.describe('Leaving Settings', () => {
  test('Done returns to the place Settings was opened from', async ({ page }) => {
    await page.goto('/#libraries');
    await waitForAppReady(page);
    await page.locator('#mode-settings').click();
    await expect(page.locator('#mode-settings')).toHaveAttribute('aria-current', 'page');
    await page.locator('#settings-done').click();
    await expect(page.locator('#mode-library')).toHaveAttribute('aria-current', 'page');
    await expect(page).toHaveURL(/#libraries$/);
  });

  test('Escape does the same as Done', async ({ page }) => {
    await page.goto('/#galleries');
    await waitForAppReady(page);
    await page.locator('#mode-settings').click();
    await expect(page.locator('#settings-done')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('#mode-published')).toHaveAttribute('aria-current', 'page');
  });

  test('opened directly, Done leads to Folders', async ({ page }) => {
    await page.goto('/#settings');
    await waitForAppReady(page);
    await page.locator('#settings-done').click();
    await expect(page.locator('#mode-browse')).toHaveAttribute('aria-current', 'page');
  });
});
