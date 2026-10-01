# Statistics: Exposure space, Daylight, Character, Space and time

*Last modified: 2026-10-01*

## Summary

Four more 3D topics in Statistics, on the stage the Colour space introduced ([ADR-0046](../../architecture/adr/0046-colour-space-webgpu-glow-stage.md)). **Exposure space** shows every photo by focal length, aperture and ISO, coloured by camera, so how one photographs shows as glowing clusters (idea 3 of `doc/ideas/statistics.md`). **Daylight** shows every photo by day of the year, hour and brightness. **Character** shows every photo by brightness, contrast and colourfulness. **Space and time** shows every located photo by where (a floor of the world seen from the middle of the photos, distance on a log scale) and when (up), with a trail of days that reaches out on journeys (idea 1). Daylight and Character are new ideas, built from what the index already held.

## Details

- `GET /api/library/exposure-space` (`ids`, `pathPrefix`, `granularity`) returns, by column, every photo with all three settings, the cameras (the five most used and "Other", quotes stripped from the Model tag), and each period's median settings. It takes 0.5 s on 32,000 photos.
- The Colour space's points carry each photo's mean brightness (`lum`). Daylight reads them and needs no endpoint of its own.
- The colour space's engine became a generic point stage. `point-scene.js` is the GPU side: lights, trail, lines and a tone-mapping pass. `point-stage.js` is the view: input, hover, click, labels, legend, preview and Full view. Each topic is one file that maps its data: `colour-space-view.js`, `exposure-space-view.js`, `daylight-view.js`.
- Tone mapping: light adds up in a float texture and is mapped keeping its hue, so a crowd of one camera's photos stays in that camera's colour.
- Exposure: logarithmic axes with ticks (14…800 mm, f/1.4…f/22, ISO 100…25600). Camera colours are the chart ramp's first slots in the dark theme, and "Other" is grey. Each light is offset within about a third of a stop so piles read as clouds.
- Daylight: a ring of months, rings at 0:00, 6:00, 12:00 and 18:00, and a brightness scale from dark to bright. The trail goes round the twelve months, all years together. It has no Periods select.
- Character: brightness 0–1 across, contrast 0–0.4 in depth, colourfulness 0–120 up, with the ends named in words (Dark, Bright, Flat, Contrasty, Colourful). The colour space points carry `contrast` and `colourful`.
- Space and time: `GET /api/library/space-time` (`ids`, `pathPrefix`) returns located, dated photos with their main colour, read from `exif_index` (0.17 s on 32,000 photos). The floor is an azimuthal projection around the median location with log distance, rings at 10, 100, 1,000 and 10,000 km, and Natural Earth coastlines and borders (public domain, vendored, 371 KB). Days within 30 km of the middle sit on the time axis, so the trail climbs the home column and reaches out on journeys. A click on a day shows its photos.
- A click on a light shows that photo. A click on an Exposure trail point shows the period's photos, and on a Daylight month, that month's photos from all years.

- **Grouped by subject:** each 3D view is a stage (full width) after the 2D charts of its subject's topic, and an overview card shows up to four of its topic's charts small: Exposure space in Exposure, Daylight in Time, Colour space and Character in Colour, Space and time in Places. Six topics instead of ten. The sidebar's Statistics sub-entries show only while Statistics is open. The old addresses lead to the topics.

## Acceptance Criteria

- [x] Each 3D view comes after the charts of its subject's topic (Exposure, Time, Colour, Places); an overview card previews up to four charts with their titles. Six sidebar entries, shown while Statistics is open, and old addresses redirect.
- [x] Exposure space: focal length, aperture and ISO on log axes with labelled ticks; lights coloured by camera with a legend; a trail of median settings per period.
- [x] Daylight: day of the year around, hour outward, brightness up; lights in their main colour; a closed trail round the months; labelled months, hours and brightness.
- [x] Dense clusters keep their colour (tone mapping) instead of turning white.
- [x] Character: brightness, contrast and colourfulness with named ends and ticks; a trail of each period's mean.
- [x] Space and time: the world seen from the middle of the photos on a log scale of distance, with coastlines, borders and distance rings; years up the axis; a trail of days that climbs the home column and reaches out on journeys.
- [x] Hover shows the photo and its values; a click shows the photo, a period or a month in the photo column.
- [x] Full view, keyboard, reduced motion and the no-WebGPU sentence work as in the Colour space.
- [x] The exposure query stays fast on a real library (0.5 s on 32,000 photos).
- [x] Go tests for the points, cameras, medians, labels and located photos; e2e specs for the APIs, the world outline and the topics.
