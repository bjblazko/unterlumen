# Statistics: Colour space

*Last modified: 2026-10-01*

## Summary

A sixth topic of Statistics shows the library as a cloud of light in three dimensions. Every analysed photo is a glowing point at its main colour in OKLab: lightness up, hue around, chroma outward. A trail joins the mean colour of each period's colour photos. It is the first 3D view from `doc/ideas/statistics.md` (idea 2), drawn with WebGPU on a dark stage that is allowed to glow ([ADR-0046](../../architecture/adr/0046-colour-space-webgpu-glow-stage.md)).

## Details

- `GET /api/library/colour-space` (`ids`, `pathPrefix`, `granularity`, as for `/api/library/colour`) returns the points in columns, the library ids they refer to, the path per period and the analysed and unanalysed counts. A point sits at the photo's largest swatch with a hue, or at its average colour.
- `/api/library/search` takes `photo_id`, so a click on a light shows exactly that photo.
- Front end: `gpu-stage.js` (device, canvas, orbit camera, loop), `colour-space-gpu.js` (WGSL and pipelines) and `colour-space-view.js` (labels, input, tooltip, picking). The topic is one entry in `STATS_TOPICS`.
- The Periods select changes the trail between months and years. A library or folder scope works as in every other topic.
- Without WebGPU the topic and its card say so in one sentence.
- **Full view**: a button on the chart shows only the stage and the photo column, over the whole window and dark in any theme. Escape or Leave full view returns to the place. Outside the full view, the stage takes the full width of the page and follows the sidebar when it collapses.

## Acceptance Criteria

- [x] A "Colour space" entry in the sidebar under Statistics and a card on the overview with a smaller, self-turning cloud.
- [x] Every analysed photo is a light in its own colour; a crowd of similar colours glows.
- [x] A trail through each period's mean colour, following the Periods select, with the first and last period labelled.
- [x] Hue names around the floor and lightness on the axis.
- [x] A legend at the foot of the stage tells a photo's light from a period's larger light on the line.
- [x] Drag, wheel and pinch turn and zoom the view; arrow keys and + / − do so while it is selected; Escape gives the keys back.
- [x] No turning under `prefers-reduced-motion`; nothing is drawn while off-screen.
- [x] Hovering shows the thumbnail, date, class and L/C/h; clicking a photo shows it in the photo column, and clicking a period shows its colour photos.
- [x] The stage is dark in both themes; everything around it follows the tokens.
- [x] A browser without WebGPU gets a sentence instead.
- [x] Full view: the stage and the photos fill the window, dark in both themes, and Escape leaves it before it closes the photos.
- [x] The stage grows and shrinks with the page, for example when the sidebar is collapsed.
- [x] The points query stays fast on a real library (0.2 s on 32,000 photos, measured with the probe test).
- [x] Go tests for the points, the path and the `photo_id` filter; an e2e spec for the API, the search and the topic.
