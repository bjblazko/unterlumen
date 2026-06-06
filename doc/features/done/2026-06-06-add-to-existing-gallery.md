# Add Photos to Existing Gallery / Album

*Last modified: 2026-06-06*

## Summary

Allow users to add more photos to an already-published gallery (single-gallery mode) or album (multi-album site mode) without creating a new one.

## Details

### Gallery listing endpoint

`GET /api/channels/{slug}/galleries` returns existing published galleries/albums for a channel:
- **siteExport channels**: reads `site.json` and returns album list sorted newest first
- **galleryExport channels**: scans the channel output directory for `gallery.json` statefiles

Response shape: `[{postID, title, publishedAt, updatedAt, photoCount}]`. `updatedAt` is omitted (zero) for galleries that have never had photos added.

### gallery.json statefile (galleryExport channels)

Each gallery folder contains a `gallery.json` alongside `index.html`. Fields: `postID`, `title`, `publishedAt`, `updatedAt` (omitempty), `photoCount`, `hasZip`, `photos`. Written on every gallery publish (new or add-to-existing). Enables listing existing galleries and merging photo lists without scanning HTML.

### Publish flow changes (TargetPostID)

When the request body includes `targetPostID`:
- `outDir` resolves to the existing folder instead of a new `<postID>/` folder
- Existing photos are prepended to the items list (preserving original order)
- ZIP is regenerated from all photos (old + new)
- HTML is regenerated with the merged photo list and the original title
- Statefile is updated (Photos, PhotoCount, HasZip, UpdatedAt); `PublishedAt` is **not** updated (preserves album sort order)
- Cover photo is not changed on add-to-existing
- A fresh `postID` is generated for the XMP sidecar (clean audit trail)

### Date range tracking (updatedAt)

`GalleryState` and `SiteAlbum` both carry an `UpdatedAt time.Time` field (zero / omitted for first-publish galleries). On add-to-existing, `UpdatedAt` is set to the `publishedAt` from the request.

The generated site index displays a human-readable date range:
- Same month/year as original → `"January 2026"`
- Different month, same year → `"January – March 2026"`
- Different year → `"December 2025 – January 2026"`

### Date/time field in publish dialog

The `datetime-local` input was replaced with a two-part control:
- **Date picker** (always visible, defaults to today in UTC)
- **`+ Time` button** that reveals a time input (hidden by default, defaults to 12:00 UTC)

Using noon UTC (12:00Z) as the default avoids day-boundary issues across timezones.

When "New gallery" is selected, the date resets to today (it becomes the album's `PublishedAt` and controls sort order). When an existing gallery is selected, the date also resets to today — it becomes `UpdatedAt` on that album; `PublishedAt` is preserved by the backend regardless. The helper text changes to reflect this difference.

### UI changes (publish modal)

When the selected channel has `galleryExport` or `siteExport`:
- If existing galleries are present, a new **"Add to"** dropdown appears above the title input
- First option is always "New gallery / album" (creates a new folder, shows title input)
- Remaining options list existing galleries: `{title} ({count}) · {dateRange}`
- Selecting an existing gallery hides the title input and pre-fills the date with today

## Acceptance Criteria

- [x] First publish to a galleryExport channel creates `<postID>/gallery.json`
- [x] Second publish with an existing gallery selected adds photos to the same folder, regenerates HTML and ZIP
- [x] `gallery.json` PhotoCount reflects the merged total
- [x] First publish to a siteExport channel creates a new album entry in `site.json`
- [x] Adding to existing site album updates Photos/PhotoCount/HasZip/UpdatedAt in `site.json` without changing `PublishedAt`
- [x] Root `site/index.html` is regenerated after adding to existing album
- [x] Date field shows a plain date picker; `+ Time` reveals the time input
- [x] Selecting existing gallery pre-fills date with today; helper note explains it sets the updated date
- [x] Selecting "New gallery / album" resets the date to today
- [x] `GET /api/channels/{slug}/galleries` returns `[]` for channels with no galleries yet
- [x] Site index shows a date range ("January – March 2026") when updatedAt differs from publishedAt
