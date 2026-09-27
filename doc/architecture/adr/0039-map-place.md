# ADR-0039: The Map is a place, clustered in the browser

*Last modified: 2026-09-27*

## Status

Accepted.

## Context

The owner asked on 2026-09-27 for a way to see photos on a map: one large map as a place of its own, only photos with coordinates, the newest photo of a spot shown round, and groups that fan out into ever finer places as one zooms in. Asked about the details, they chose: photos from all libraries; a click on a group zooms in; a round thumbnail with a count; one filter, the time a photo was taken, as a slider; and a new sidebar group "Explore".

What was there already: MapLibre GL JS, vendored as an ES module ([ADR-0038](0038-maplibre-6-as-es-module.md)) and used in the info panel with OpenFreeMap tiles; and every library's index, which keeps each photo's EXIF as JSON with the parsed `latitude`/`longitude` (libraries indexed before those fields existed still have the raw GPS tags).

## Decision

- **Source: the libraries' index.** `GET /api/library/geo` returns every located photo of every library, per library, as compact arrays `[photoID, lat, lon, taken, filename]` — about 150 bytes a photo, so tens of thousands stay a few megabytes and one request. It reads `exif_json` (parsed coordinates first, the GPS tags as the fallback, through `library.ParseGPSCoord`, which moved there from the photo-info handler) and leaves out photos marked missing, coordinates outside the globe, and the 0,0 that cameras write without a fix. Folders outside a library do not appear: scanning a tree on every visit would be slow and would know only that tree.
- **Clustering in the browser, by MapLibre.** A GeoJSON source with `cluster: true` (radius 56px, up to zoom 18) groups the points; no server tiles, no clustering library. The groups are drawn as HTML markers (a `<button>` with a round thumbnail and a count), updated from `querySourceFeatures` on every render; an invisible circle layer only makes MapLibre tile the source.
- **The newest photo without asking for leaves.** The points are sorted oldest first and numbered; each point carries its rank, and `clusterProperties: { newest: ['max', ['get', 'rank']] }` gives every group the rank of its newest photo. Undated photos count as the oldest.
- **Click.** A group zooms to `getClusterExpansionZoom`, the first zoom at which it splits. A group that never splits — its photos share one spot — opens in the viewer, newest first, as does a single photo.
- **The viewer opens read-only.** From the map a photo is looked at, not culled: marking for deletion needs the folder the photo was seen in, and cropping a photo that is out of view on the map is not what the place is for. `Viewer` gained `readOnly` (no Crop, no Delete, the Delete key does nothing). `App.showViewer` is the part of `openViewer` that shows and hides the viewer; `openViewer` keeps the pane contract, the map passes its own list.
- **The photos in view, in a column that is closed by default.** The full-width map is the place; the photos are asked for. A "Photos" button in the head — the same panel toggle as the library filter, with the number of photos in view — opens a column beside the map (below it on a phone) with those photos as square tiles, newest first, rendered in chunks as the column scrolls. It follows the map on every `moveend` and the time range; zooming into a group therefore shows that group. Done or Escape closes it; whether it is open is remembered per browser. A tile opens the viewer on all photos in the column. The owner chose this over a count bubble that opens one group: the bubble is a 20px target, and the view already says which photos one means.
- **The time filter is client-side.** The slider spans the months from the oldest to the newest dated photo; narrowing it filters the loaded points and replaces the source's data, with no request. Undated photos are shown only while the full span is chosen. Months are read from the ISO date text, so they do not shift with the browser's time zone.
- **One range slider.** The library filter's two-handle slider was mouse-only. It became `RangeSlider` (`range-slider.js`), used by both: pointer events (mouse, pen, touch), `role="slider"` handles with the arrow, Page and Home/End keys, and the handle being moved in `--accent` — the current value of something adjusted right now. The keys a handle owns stop there, so the place's own shortcuts do not act on them too.
- **Grey tiles.** The map is ground for the photos: OpenFreeMap's `positron` in the light theme and `fiord` in the dark one, switched with the theme. `dark` came first and was too dark to read; `fiord` is a lighter blue-grey. The info panel keeps `liberty`, where the map is the subject. A Style switch in the head (Colour / Grey, grey by default, remembered per browser) shows `liberty` on the Map as well, asked for by the owner after using it; OpenFreeMap has no dark colour style, so colour stays light in the dark theme.
- **Navigation.** A new group "Explore" in the sidebar with one entry, Map (`#mode-map`, `#map`). Its number key is 7: the existing number keys stay as they are ([ADR-0028](0028-places-not-workflow-steps.md)), so the keys no longer follow the sidebar's order. It is also a tab on a phone, where the place works as it does on a desk.

## Consequences

- A photo appears on the map only once its library is indexed; a location set later in Folders appears after the next scan.
- The map needs a connection to OpenFreeMap. Without one the place says so; without MapLibre it says that instead.
- All located photos are held in the browser. At 50 000 photos that is a few megabytes and a clustering index MapLibre builds in a worker; far beyond that, a bounding-box query on the server would be the next step.
- The e2e spec (`map.spec.js`) replaces the tile style with an empty one, so it runs offline and asserts markers, not tiles.
