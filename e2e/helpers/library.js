import { expect } from '@playwright/test';

// Drives the reindex SSE stream to completion for the given library.
// Playwright buffers the full SSE response body before returning, so this
// works correctly for small fixture directories.
export async function reindexLibrary(request, libID) {
    const res = await request.post(`/api/library/${libID}/reindex`, { timeout: 30_000 });
    expect(res.status()).toBe(200);
    const text = await res.text();
    if (!text.includes('"finished":true')) {
        throw new Error('Reindex did not finish: ' + text.slice(0, 300));
    }
}
