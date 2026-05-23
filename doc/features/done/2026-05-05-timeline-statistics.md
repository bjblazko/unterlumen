# Timeline Statistics

*Last modified: 2026-05-23*

## Summary

A "Timeline" tab in the Statistics modal showing how shooting habits evolved over time. The tab lazy-loads on first click and offers month/year granularity (auto-detected from library date span, with a manual override toggle).

## Details

### New backend endpoint

`GET /api/library/timeline` — accepts `ids`, `pathPrefix`, and `granularity` (`month`, `year`, or empty for auto-detect). Auto-detect uses month when date span ≤ 4 years, year otherwise.

### Six D3 charts in the Timeline tab

1. **Camera usage** — stacked bar chart showing shots per camera per period. Top 5 cameras by total count are shown; remaining cameras are collapsed into "Other". Clicking a legend item dims/highlights the layer.

2. **Focal length drift** — median 35mm-equivalent focal length per period as a line, with a light IQR band (p25–p75) behind it. Shows how focal preferences shifted over time.

3. **ISO evolution** — area chart of median ISO per period on a log scale, with dashed reference lines at 100/400/1600/6400/25600. Shows how the photographer pushed sensor limits as equipment improved.

4. **Aperture usage** — period × f-stop heatmap with cells normalised to share-within-period. Color scales from neutral (border) to orange (accent). Hover tooltips show absolute count and percentage.

5. **Aspect ratio mix** — 100% stacked area chart showing the proportion of 3:2, 4:3, 16:9+, 1:1 and other frame shapes per period. Reflects camera changes and intentional crop decisions.

6. **Megapixel timeline** — max MP as a step line (accent color), average MP as a dashed smooth line. Circles mark periods where max MP jumps >20% (camera upgrades).

### Implementation notes

- Date grouping uses `SUBSTR(json_extract(exif_json,'$.dateTaken'), 1, N)` (N=7 for month, N=4 for year), matching the existing date-query pattern and avoiding timezone edge cases with ISO 8601 offset strings.
- Median, p25, p75 are computed in Go from sorted value arrays (one SQL query per EXIF field ordered by period + value), not via SQLite window functions.
- Lazy-loading and a generation counter prevent stale fetch results from rendering after a library or granularity change.
- Camera names have surrounding quotes stripped (same artifact handled by `cleanExif` on the existing camera×lens treemap).

## Acceptance Criteria

- [x] Statistics modal shows "Snapshot" and "Timeline" tab buttons
- [x] "Snapshot" tab shows all existing 8 charts unchanged
- [x] "Timeline" tab lazy-loads on first click (shows loading indicator while fetching)
- [x] All 6 timeline charts render with real data
- [x] Auto granularity detects correctly (month for short libraries, year for long ones)
- [x] Month / Year / Auto toggle re-fetches timeline data and re-renders
- [x] Library dropdown change clears cached timeline data and re-fetches on next Timeline click
- [x] Libraries with no date EXIF show "No data" gracefully in each chart without JS errors
- [x] Camera names shown without surrounding quotes in legend
