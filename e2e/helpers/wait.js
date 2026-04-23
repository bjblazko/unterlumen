import { expect } from '@playwright/test';

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
