# Statistics: Colour

*Last modified: 2026-10-01*

## Summary

A fifth topic in the Statistics place ([ADR-0043](../../architecture/adr/0043-statistics-place-with-topics.md)), **Colour**, built from the measurements in `photo_appearance` and `photo_palette` ([ADR-0044](../../architecture/adr/0044-photo-appearance-from-thumbnails.md)). It has four charts. Clicking a value shows its photos in the photo column, as in the other topics. Marks filled with a photo's own colour are recorded as a deviation in [ADR-0030](../../architecture/adr/0030-rams-design-tokens.md).

## Details

Decided with the owner on 2026-10-01:

| Chart | Value | A click shows |
|---|---|---|
| Black and white | Share of mono, tinted ("toned") and colour photos per period, as three lines on the chart ramp. Follows the month/year setting. | that class in that period |
| Colour of each period | A strip of swatches. Each is the mean of the colour photos' **main colours** (palette swatches with a hue, weighted by share) in OKLab for lightness and hue, with the chroma at the **90th percentile**, so the swatch shows the colour as it appears at its most colourful in the photos and does not grey out to brown. Mono and toned photos are left out. A period whose colour photos have only greys is a gap. Follows the month/year setting. | colour photos of that period |
| Main colours | A hue wheel of twelve 30° sectors (`hue_bin`). Sector length grows with the root of the summed swatch share, so its area grows with the share. The fill is the share-weighted mean of the sector's swatches, at the 90th percentile of their chroma. The sectors are named after the colours whose OKLCh hue lies in them: red, orange, amber, yellow, green, emerald, teal, sky blue, blue, violet, purple, pink. Neutral swatches are a grey centre with their share and cannot be clicked. Only colour photos count. | colour photos with a swatch of that sector covering at least 20 % of the frame. A sector of which no photo has such a swatch cannot be clicked. |
| Warm and cool through the year | Twelve months, all years together, or one year chosen in a select above the chart. Bars above the axis show the share of warm colour photos (warmth > 0.15); bars below show cool ones (< −0.15). A warm bar is filled with its photos' main colours that lean warm (within 60° of 60°: red to yellow), and a cool bar with those that lean cool (teal to violet), mixed as in the strip. A warm photo's blue sky does not turn its bar grey. A bar whose photos have no such colour is the interface grey. | warm (or cool) colour photos of that month, in any year or the year chosen |

Server: `GET /api/library/colour?ids=&pathPrefix=&granularity=` (`Manager.Colour`, `BuildColour` in `internal/library/colour_stats.go`). It reads every library's measured photos and aggregates them together, so medians span all of them. It is cached per scope; a scan clears the cache, and so does the end of an analysis pass.

Search: `/api/library/search` takes `mono`, `hue_bin`, `warmth` and `month`, with the same thresholds as the charts (`WarmthThreshold`, `HueShareMin`), so a click shows exactly the photos counted.

Why main colours and not each photo's average colour: a photo's average colour is greyed out before any mean is taken. Red cherries on green leaves average to a brownish grey. On the e2e fixtures the colour photos' average colours have a mean chroma of 0.024, below the 0.03 at which the index calls a colour neutral, while their swatches with a hue have 0.065. The first version used the average colours, and the owner found the strip muddy. With main colours, the periods' chroma rose from 0.003–0.03 to 0.035–0.094.

Second look, 2026-10-01, on the owner's libraries. "Orange" did not look orange, for three reasons. (1) The data is dull: in "Chronologisch" the swatches of the 60–90° sector have a median chroma of 0.048, and only 1 % reach 0.125, while pure orange has 0.186. Wood, skin, brick and sand are low-chroma oranges, that is browns and tans. (2) The first hue names were shifted by a sector: they put red at 30–60°, but in OKLCh red is at 29° and orange at 53°, so the sector called "orange" held amber, which is khaki when dull. (3) The median chroma showed the typical swatch, and a warm bar mixed in its photos' skies and grass. Fixed by renaming the sectors, drawing at the 90th percentile of chroma, and filling warm and cool bars with warm and cool colours only. No design rule muted anything: the marks are drawn exactly as `oklch(L C h)`.

The warmth threshold, checked on the e2e fixtures: of the 76 colour photos, 43 are warm, 25 cool and 8 neutral at ±0.15. The median warmth is +0.28, with quartiles at −0.26 and +0.60.

Photos not yet analysed are named above the charts ("312 photos have not been analysed yet, so the colours are not complete."). With none analysed, each chart says so.

## Acceptance Criteria

- [x] Colour topic in the sidebar, the overview (strip as preview) and its own address `#statistics/colour`
- [x] Four charts as in the table, each showing its photos when clicked, by mouse and with the keyboard
- [x] `GET /api/library/colour`, aggregated across libraries and cached; the cache is cleared after analysis
- [x] Search criteria `mono`, `hue_bin`, `warmth`, `month`
- [x] ADR-0030 records a photo's own colour as a mark; ADR-0034 points to it
- [x] Go tests (`colour_stats_test.go`, `store_colour_test.go`, `filter_params_test.go`) and e2e `statistics-colour.spec.js`
- [x] The filter's head says on the desk that Statistics finds photos by colour, and links there
- [x] Searches by colour are fast on the owner's libraries (ADR-0045)
- [ ] Checked against the owner's libraries once their analysis has finished: the strip, wheel and seasons look plausible
