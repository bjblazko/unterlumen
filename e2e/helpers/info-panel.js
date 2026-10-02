// The info panel starts open on a desk; `i` toggles it. Opening it must not
// close it when it is already open.
export async function openInfoPanel(page, scope = '') {
    // Another place's panel may be open too, hidden with its place.
    const panel = page.locator(`${scope} .info-panel.expanded:visible`.trim());
    if (await panel.count() === 0) await page.keyboard.press('i');
    await panel.first().waitFor({ timeout: 10_000 });
}
