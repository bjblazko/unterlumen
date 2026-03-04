# Location Map in Info Panel

*Last modified: 2026-03-04*

## Summary

Display an interactive map (OpenFreeMap + MapLibre GL JS) in the Info panel's Location section for photos with GPS EXIF data, replacing the raw coordinates-only view.

## Details

- A map is rendered at the top of the Location section using MapLibre GL JS with OpenFreeMap tiles (liberty style).
- The map is centered on the photo's GPS coordinates at zoom level 14, with a marker at the exact location.
- Scroll zoom is disabled to prevent accidental zooming while scrolling the info panel.
- Three buttons below the map allow switching between 2D (flat) and 3D (60° pitch) views, plus opening the location on OpenFreeMap in a new tab.
- The map instance is cleaned up on re-render to prevent memory leaks.
- Attribution is handled automatically by MapLibre's built-in attribution control.

## Acceptance Criteria

- [x] Map renders in the Location section for photos with GPS data
- [x] Marker appears at the photo's coordinates
- [x] 2D button shows flat top-down view (default)
- [x] 3D button tilts the map to 60° perspective
- [x] Open button opens OpenFreeMap in a new tab at the correct coordinates
- [x] No map or Location section shown for photos without GPS data
- [x] Map is properly cleaned up on panel close or image change
- [x] Attribution text appears on the map (MapLibre built-in)
