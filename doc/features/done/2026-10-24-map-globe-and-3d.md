# Map: Globe, Mercator and 3D

*Last modified: 2026-10-24*

## Summary

The owner asked for the Map in the Equal Earth projection, or a switch
between Mercator and Equal Earth with Equal Earth as the default. MapLibre
has no Equal Earth and OpenFreeMap's tiles exist only in Mercator, so the
owner chose MapLibre's Globe instead. The owner also asked for 3D maps,
"especially in the metadata panel". Recorded in the addendum to
[ADR-0039](../../architecture/adr/0039-map-place.md).

## Details

- **The Map's head has View: Globe · Mercator · 3D**, beside Map: Grey ·
  Colour.
  - Globe is the default; the choice is kept per browser.
  - On a phone both settings are small selects in the head's one row.
- **Globe** shows the world as a sphere, countries at their true size, and
  turns into the flat map close up.
- **3D** tilts the flat map to 60°, with buildings standing in grey or in
  colour.
- **The info panel** keeps its own map and its 3D · 2D switch. 3D zooms in
  close enough that the buildings stand.

## Acceptance criteria

- [x] View: Globe · Mercator · 3D in the Map's head, Globe by default, remembered per browser
- [x] The view survives a change of theme and of Grey · Colour
- [x] 3D buildings on the grey styles as well as in colour
- [x] Info panel: 3D shows standing buildings
- [x] e2e: `map.spec.js` "shows a globe, the flat map or the map tilted in 3D"
- [x] German words for the new switch
