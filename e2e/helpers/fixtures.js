// Canonical test images — paths relative to UNTERLUMEN_ROOT_PATH (fixtures/photos/).
// All images confirmed via exiftool against src/examples originals.

// folder-b images (flat, 50 files, no subdirs)
export const GPS_IMAGE    = '2024-07-04_14-09-43_X-T50_DSCF3258.jpeg'; // GPS, ISO 500
export const NO_GPS_IMAGE = '2018-10-20_17-46-50_Canon EOS 500D_IMG_3826.jpeg'; // no GPS, ISO 400
export const GPS_PATH     = `folder-b/${GPS_IMAGE}`;
export const NO_GPS_PATH  = `folder-b/${NO_GPS_IMAGE}`;

// folder-a/a1 images (7 files: JPEG + 1 HIF)
export const A1_GPS_IMAGE    = '2024-07-04_14-25-18_X-T50_DSCF3276.jpeg'; // GPS confirmed
export const A1_NO_GPS_IMAGE = '2025-03-28_18-21-25_X-T50_DSCF1138.jpeg'; // no GPS confirmed
export const HIF_IMAGE       = '2026-04-24_X-T50-XF23mmF2-R-WR-DSCF6850.hif';
export const HIF_PATH        = `folder-a/a1/${HIF_IMAGE}`;

// GPS editing test images — separate from reference images to avoid cross-test pollution.
// These are modified in-place by gps-editing.spec.js and must not be used elsewhere.
export const GPS_EDIT_IMAGE    = '2024-07-08_10-59-25_X-T50_DSCF3500.jpeg';    // has GPS, ISO 200
export const NO_GPS_EDIT_IMAGE = '2019-06-08_14-30-48_Canon EOS 200D_IMG_9637.jpeg'; // no GPS
export const GPS_EDIT_PATH     = `folder-b/${GPS_EDIT_IMAGE}`;
export const NO_GPS_EDIT_PATH  = `folder-b/${NO_GPS_EDIT_IMAGE}`;

// folder-b counts and camera info
export const FOLDER_B_IMAGE_COUNT = 50;
export const FOLDER_A_A1_IMAGE_COUNT = 7; // includes HIF

/**
 * Navigate the page into a named directory from the current browse view.
 * Waits for the breadcrumb to update before returning.
 */
export async function navigateToFolder(page, dirName) {
    const dir = page.locator(`.grid-item.dir-item[data-name="${dirName}"]`);
    await dir.waitFor({ state: 'visible', timeout: 15_000 });
    await dir.dblclick();
    await page.waitForSelector(`.crumb[data-path="${dirName}"]`, { timeout: 10_000 });
}

/**
 * Navigate the page into a named directory within a specific pane (for commander mode).
 * Works for both grid view (dir items) and list view (table rows).
 */
export async function navigatePaneToFolder(page, paneSelector, dirName, expectedCrumbPath) {
    const dir = page.locator(`${paneSelector} [data-name="${dirName}"]`).first();
    await dir.waitFor({ state: 'visible', timeout: 15_000 });
    await dir.dblclick();
    await page.waitForSelector(`${paneSelector} .crumb[data-path="${expectedCrumbPath}"]`, { timeout: 10_000 });
}
