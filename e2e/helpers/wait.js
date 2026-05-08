import { expect } from '@playwright/test';

// Waits for the App initialization Promise.all (API.config + API.toolsCheck) to
// complete.  The init sequence ends with setMode('browse'), which creates the
// .browse-layout div.  Without this guard, clicking #mode-library in a beforeEach
// can race with the deferred setMode('browse') call and switch back to browse mode.
export async function waitForAppReady(page) {
  await page.waitForFunction(
    () => document.querySelector('.browse-layout') !== null,
    { timeout: 15_000 },
  );
}

// Waits for at least `count` image items to be rendered in the DOM.
// This does NOT wait for thumbnail images to download — just for the
// grid/justified items to exist so selectors like [data-type="image"] work.
export async function waitForThumbnailsLoaded(page, count = 1) {
  await page.waitForFunction(
    (n) => document.querySelectorAll('[data-type="image"]').length >= n,
    count,
    { timeout: 10_000 },
  );
}

export async function waitForOverlayBadge(page, imageName, badgeSelector) {
  const item = page.locator(`[data-name="${imageName}"]`);
  await expect(item.locator(badgeSelector)).toBeVisible({ timeout: 10_000 });
}
