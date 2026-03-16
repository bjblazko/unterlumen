# Set Location: Smart Initial Map View

*Last modified: 2026-03-16*

## Summary

The Set Location modal now opens pre-centered on the best available position rather than always showing a world view.

## Details

When the modal opens, it uses the following priority order to determine the initial map position:

1. **Existing GPS (single file only):** Fetches EXIF data for the selected image. If GPS coordinates are present, the map flies to that location at zoom 14, the input fields are pre-filled, and the marker is placed. Geolocation is not requested in this case.
2. **Stored user location:** Reads `user-location` from `localStorage` (JSON `{"lat": ..., "lon": ...}`). If present, the map initializes at zoom 9 (~50 km radius).
3. **Browser geolocation (requested once):** If no stored location exists, `navigator.geolocation.getCurrentPosition()` is called. On success, the result is stored to `user-location` and the map flies to it at zoom 9. On error, no action is taken.
4. **World fallback:** Center `[0, 20]`, zoom 2 — the previous default.

## Acceptance Criteria

- [x] Opening the modal on an image with GPS coordinates centers the map there, pre-fills inputs, and places the marker.
- [x] Opening the modal on an image without GPS, with no stored location, triggers a browser geolocation prompt; on grant, map zooms to user's location.
- [x] After the geolocation prompt, the location is cached; subsequent opens use the cached value without prompting again.
- [x] Opening the modal when geolocation is denied shows the world view fallback.
- [x] Opening the modal with multiple files selected skips the GPS fetch and uses stored location or world view.
