import { test, expect } from '@playwright/test';

// Added to a home screen, Unterlumen opens as an app without the browser's
// bars. That needs a manifest the page links to, and icons that load.

test('the page links a manifest that opens standalone, with icons that load', async ({ page, request }) => {
    await page.goto('/');
    const href = await page.locator('link[rel="manifest"]').getAttribute('href');
    expect(href).toBe('/manifest.json');

    const res = await request.get(href);
    expect(res.status()).toBe(200);
    const manifest = await res.json();
    expect(manifest.name).toBe('Unterlumen');
    expect(manifest.start_url).toBe('/');
    expect(manifest.display).toBe('standalone');

    const sizes = manifest.icons.map(i => i.sizes);
    expect(sizes).toContain('192x192');
    expect(sizes).toContain('512x512');
    for (const icon of manifest.icons) {
        const img = await request.get(icon.src);
        expect(img.status(), icon.src).toBe(200);
        expect(img.headers()['content-type']).toBe('image/png');
    }
});
