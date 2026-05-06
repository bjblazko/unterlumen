# Statistics Modal

*Last modified: 2026-05-06*

## Summary

A full-featured statistics view accessible from the library list header. Shows aggregated photo data across all libraries (or a filtered subset), visualised with D3.js charts embedded in a scrollable modal.

## Details

- **Entry point**: Single "Statistics" button in the library header, visible in both list and detail views. Context-aware: shows stats for all libraries in list view, for the current library at its root, or for the current subfolder when navigated into one.
- **Library filter**: Dropdown in the modal header allows scoping statistics to specific libraries. Disabled and pre-selected when opened from within a library.
- **Backend**: `GET /api/library/statistics?ids=...` aggregates data across the requested libraries (or all if no IDs given). Returns format counts, focal lengths, apertures, ISOs, film simulations, camera×lens counts, shooting-hour distribution, and shooting-day counts. Also returns `indexingPhotos` (photos still being scanned) and `warnings` (libraries that could not be read).
- **Indexing awareness**: If any photos are still in `status='missing'` (active scan in progress), an amber banner is shown: "N photos are still being indexed — statistics are incomplete." Libraries whose DB cannot be opened surface a named warning rather than being silently dropped.
- **EXIF coverage subtitles**: The Focal length, Aperture, and ISO charts show "N of M photos" below the title when not all photos carry that EXIF field.
- **D3.js**: Bundled locally at `src/web/js/vendor/d3.v7.min.js` — no CDN dependency.

### Charts

| Chart | Type | Description |
|---|---|---|
| Format | Donut | Photo file formats (JPEG, HEIF, PNG, …) |
| Film simulation | Horizontal bar | Fujifilm film sim distribution; known sims get signature colours |
| Focal length | Histogram | Native mm values with toggle to 35mm-equivalent (`FocalLengthIn35mmFilm`, falls back to `FocalLength`) |
| Aperture | Histogram | f-stop distribution on a log-spaced x-axis |
| ISO | Histogram | ISO distribution on a log scale |
| Camera × Lens | Treemap | Two-level treemap: camera bodies → lens children, sized by shot count. Cameras without a `LensModel` tag appear as "(no lens)" rather than being excluded. |
| Time of day | Radial clock | 24-h radial bar chart; AM hours subtly shaded |
| Shooting calendar | Heatmap | GitHub-style calendar for the last 3 years of shooting activity |

## Acceptance Criteria

- [ ] "Statistics" button appears in both the library list header and the library detail header
- [ ] In library list view, clicking Statistics shows all-library stats with the library selector active
- [ ] In library detail view (at root or in a subfolder), clicking Statistics shows stats scoped to that library/folder, with the library pre-selected and the selector disabled
- [ ] All 8 charts render for a library with EXIF data
- [ ] Focal length toggle switches between native and 35mm-equivalent values with animated transition
- [ ] Library dropdown filters charts to selected libraries
- [ ] Modal closes on Escape or clicking the overlay
- [ ] All charts use the Dieter Rams colour palette (off-white, warm grays, functional orange)
- [ ] Charts handle empty data gracefully (show "No data" rather than throwing)
- [ ] Amber warning banner appears when a library is actively being indexed
- [ ] Named warning appears when a library DB cannot be opened
- [ ] Camera × Lens treemap shows cameras without a lens tag as "(no lens)"
- [ ] Focal length / Aperture / ISO charts show "N of M photos" subtitle when EXIF coverage is partial
- [ ] Statistics button does not appear in browse (raw filesystem) mode
- [ ] Backend compiles and passes `go vet`
