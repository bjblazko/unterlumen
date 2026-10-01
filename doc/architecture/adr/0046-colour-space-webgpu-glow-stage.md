# ADR-0046: The Colour space draws with WebGPU, on a glowing stage

*Last modified: 2026-10-01*

## Status

Accepted.

## Context

`doc/ideas/statistics.md` collects six views in three dimensions. The colour space cloud (idea 2) was taken up first because its data already exists: every analysed photo has an average colour and up to five main colours in OKLCh (ADR-0044). The idea left two questions open. Should it draw with WebGPU directly or through a library such as three.js, and what does a browser without WebGPU show? And how does a cloud of glowing lights fit a design that rules out glows (ADR-0030)?

The owner decided both for this view. It may glow, and it uses WebGPU with no WebGL fallback.

## Decision

- **A topic of its own.** "Colour space" (`#statistics/colour-space`) is the sixth topic of Statistics (ADR-0043), with one large stage. Its overview card shows the same cloud, smaller and turning by itself.
- **The data.** `GET /api/library/colour-space` returns every analysed photo as a point at its main colour, which is the largest swatch with a hue, or its average colour when it has none. The points come in columns (`id`, `lib`, `date`, `mono`, `l`, `a`, `b`, in OKLab), and the path is the mean OKLab colour of each period's colour photos. The query leaves out the file name: reading it reaches each photo's row with its EXIF, which took 1.2 s instead of 0.2 s on 32,000 photos (ADR-0045). The answer is cached like the Colour topic's and dropped with it.
- **Raw WebGPU, no library, no fallback.** `gpu-stage.js` holds one device for the page, a canvas that follows its size and the device pixel ratio, an orbit camera, and a loop that draws only while something moves. `colour-space-gpu.js` holds the WGSL and three pipelines: instanced quads for the lights, a ribbon for the trail, and lines for the floor. Converting OKLab to sRGB is done in the shader. A point cloud does not justify a vendored 3D engine. Without `navigator.gpu`, the topic says in one sentence that it needs WebGPU and which browsers have it.
- **The stage.** The canvas is near black in both themes. Lights are added on top of each other (additive blending), so where many photos share a colour it glows. The more photos, the smaller and fainter each light, so a crowd does not burn out at once. Dark colours are lifted so that they still glow; their position still says how dark they are. The trail and its period dots are laid over the cloud rather than added to it, so they stay visible over a white core, and they are drawn more colourful than their mean (×2.5), since a mean of colours is greyer than any of them. Chroma is stretched (a and b ×5 against L ×2), because photos rarely reach a chroma of 0.2.
- **Reading it.** Hovering over a light shows its thumbnail, date, class and L, C and h values. A light's own colour always carries words, as ADR-0030 asks. Clicking a photo shows that photo in the photo column (`photo_id` in the search), and clicking a period shows that period's colour photos. Hue names, lightness and the first and last period are HTML labels in IBM Plex Mono over the canvas.
- **Full view.** A chart marked `stage` in `STATS_TOPICS` has a Full view button. It lays the chart and the photo column over the whole window and hides the head, the sidebar and everything else. It is dark in both themes, because the place takes `data-theme="dark"`, and the theme's tokens apply to any element that carries it, not only to `:root`. Escape leaves the full view before it closes the photos, and so does the button, now labelled Leave full view. Opening a photo still brings up the viewer, and closing it returns to the full view. The topic is `wide`: its grid has no reading-width cap, so the stage follows the window and the sidebar.
- **Input.** Drag to turn, wheel or pinch to zoom. While selected, the canvas takes the `keyboard-owner` class, the arrow keys turn it and + and − zoom it, and Escape gives the keys back. It turns by itself until it is touched, and never under `prefers-reduced-motion`. It draws nothing while off-screen and stops once it leaves the page.

## Consequences

- The glow, the dark stage and additive blending are a recorded deviation in ADR-0030, limited to this canvas. Everything around it (card, head, tooltip, help line) follows the tokens.
- Browsers without WebGPU (Firefox on some systems, older Safari) see a sentence instead of the view. The other topics are unaffected.
- Headless Chromium in the e2e run has no WebGPU, so the e2e spec checks the API, the `photo_id` search and the sentence, and checks the canvas only where WebGPU exists. The drawing itself is checked by hand.
- `gpu-stage.js` is ready for the next 3D view, such as the exposure cube or the space-time cube.
