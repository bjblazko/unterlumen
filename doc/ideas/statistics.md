# Ideas for Statistics

*Last modified: 2026-10-01*

A collection of ideas for the Statistics place ([ADR-0043](../architecture/adr/0043-statistics-place-with-topics.md)), not a plan. Nothing here is decided. When one of them is taken up, it gets its own feature document in `doc/features/open/`, with details and acceptance criteria, and an ADR where it makes a design decision.

A new idea usually becomes a topic in `STATS_TOPICS` (`src/web/js/stats-topics.js`), or a chart in an existing topic. It should say which photos a click shows, so it keeps the photo column working.

## Already named

Mentioned while Statistics became a place:

- **Colour over time.** How the average or dominant colour of the photos changes by month or year, as a topic of its own ("Colour"). Needs a colour value per photo in the index (see "What the ideas need").
- **More metadata.** Statistics from fields the index already holds but no chart shows yet: white balance, flash, exposure time, exposure compensation.
- **Weekday and season** in the Time topic.
- **Charts drawn to the width of their card** instead of at a fixed size and scaled. That matters most on a phone.
- **Photos without a lens tag** as a search criterion, so the "(no lens)" cell of Camera and lens shows exactly its photos.

## Three-dimensional views

Six ideas for views in three dimensions, collected on 2026-09-30.

### 1. Space-time cube

- **Axes:** longitude, latitude, and time taken as the height (a year or a day per unit).
- **Encoding:** the colour of a point is the photo's dominant colour; its size is the focal length or the file size.
- **What it shows:** journeys and periods of life as paths in three dimensions. Connecting the points in the order they were taken with 3D splines gives a trail above a base map. Where and when photos were taken, and how many, can be seen at once.

### 2. Colour space cloud with mood paths

- **Axes:** each photo's average or main colour in CIELAB or HSL: hue, saturation and lightness.
- **Encoding:** lines connecting photos in the order they were taken.
- **What it shows:** the library as a cloud of colours. The path through it shows colour periods, such as dark winter months against saturated summer holidays.

### 3. Exposure parameter cube (photographic style space)

- **Axes:** focal length, aperture, and exposure time or ISO.
- **Encoding:** the colour of a point is the camera model or the lens.
- **What it shows:** how one photographs, as dense clusters in the space of settings. Portraits gather around 85 mm, f/1.8, ISO 100; night and astro photos form a separate cloud around 14 mm, f/2.8, ISO 3200. A section through the cube shows which lenses are used at the extremes.

### 4. Semantic space (image embeddings reduced to 3D)

- **Data:** each photo goes through a vision model (for example CLIP), which gives a vector of many dimensions. UMAP or t-SNE reduces it to three.
- **Encoding:** the three coordinates place the photo; colour can show the time taken, and paths through the space can show how content changed over time.
- **What it shows:** photos arranged by what they show and their mood alone: dogs near dogs, mountains near mountains, sunsets near sunsets, without any tags.

### 5. Relationship graph (force-directed network)

- **Nodes:** photos, places, lenses, people or tags, weekdays.
- **Edges:** weighted by closeness in time, distance on the map or visual similarity.
- **Layout:** a force-directed graph in 3D.
- **What it shows:** holidays form dense clusters; lenses connect different periods of life. Zooming into a node shows the photo's thumbnail beside it.

### 6. Density landscape (voxel topography)

- **Axes:** a geographic grid, and the number of photos per cell as the height.
- **Encoding:** the colour of a column is the mean hue of the photos taken there; its surface texture can show image entropy (visual complexity).
- **What it shows:** a landscape of columns over the world map. Places photographed often, such as holiday spots, rise high; places seen rarely stay low.

## What the ideas need

Colour per photo, image entropy and a perceptual hash are collected since 2026-10-01 ([feature](../features/open/2026-10-01-photo-appearance-index.md), [ADR-0044](../architecture/adr/0044-photo-appearance-from-thumbnails.md)); what follows is how the need was described before.

- **Colour per photo** (ideas 1, 2 and 6, and colour over time): a dominant or average colour computed when a photo is indexed, stored in the library index, and computed for photos indexed before. The thumbnails are there to compute it from.
- **Image entropy** (idea 6): one more value per photo at indexing.
- **Embeddings** (idea 4): a vision model has to run somewhere. Running it locally keeps the photos on the machine but needs a model runtime next to the Go binary. Sending photos to a service would be a change in what Unterlumen does with photos and needs a decision of its own. The reduction to 3D (UMAP) is computed once per library and redone when photos change.
- **Graph edges** (idea 5): similarity between photos, from colours or embeddings; closeness in time and on the map can be computed from what the index holds.
- **WebGPU for the 3D views.** D3 draws in two dimensions; the 3D views are meant to draw with WebGPU in the browser, which handles tens of thousands of points and columns. Whether directly or through a library with a WebGPU renderer (three.js has one) is open; a library would be vendored like D3 ([ADR-0017](../architecture/adr/0017-d3-vendored-bundle.md)) and MapLibre ([ADR-0031](../architecture/adr/0031-vendor-maplibre.md)). To check: which browsers the owner and the NAS's visitors use support WebGPU, and what a browser without it shows instead (the 2D charts, or WebGL). The base maps of ideas 1 and 6 could come from MapLibre, which can draw extruded columns itself.

## Design questions

These ideas come from what 3D graphics make possible, while the app follows huepattl-rams-design ([ADR-0030](../architecture/adr/0030-rams-design-tokens.md)). Before any of them is built:

- Glow, trails and free-floating clouds are ruled out by the design ("no gradients, glows"). A 3D view has to be flat and legible as well: hairlines, the chart ramp (ADR-0034), labels. A deviation must be recorded in ADR-0030.
- Colour that encodes a photo's own colour (ideas 1, 2, 6) is data, not an interface signal, like the film simulation colours. It would be another recorded exception to the chart ramp.
- A 3D view needs a way to read exact values (hover, a table or a 2D section) and has to work with the keyboard and when motion is reduced.
- Every view should say which photos a click shows, so the photo column stays the way from a chart to the photos.

## Storage: DuckDB instead of SQLite?

An idea, not planned: SQLite is fine for now. Should the statistics grow slow with the ideas above, this is where to look, and it would be a topic of its own with its own ADR.

Statistics are analytical queries: counts, percentiles and distributions over all photos of several libraries, grouped by period. SQLite answers them row by row, so percentiles are computed in Go (`computePercentiles`), and the answers are cached per scope. DuckDB is built for such queries: it stores by column and has percentiles, window functions and grouping sets in SQL.

Worth knowing if it comes up:

- **Replace or add.** DuckDB can read the libraries' SQLite files in place (its `sqlite` extension) and answer only the statistics, while SQLite stays the store the scanner writes to.
- **Build.** The project builds without cgo (`modernc.org/sqlite`, `CGO_ENABLED=0` in `.goreleaser.yml`); the DuckDB driver for Go needs cgo, which changes the cross-compiled releases, the Docker image and the binary's size.
- **Two installations.** Each installation keeps its own index in `-lib-dir` (ADR-0035); a DuckDB file would follow the same rule.
- **Measure first.** How long the statistics take on the NAS library with a cold cache decides whether it is worth it.
