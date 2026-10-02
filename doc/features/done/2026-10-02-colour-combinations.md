# Colour combinations

*Last modified: 2026-10-02*

## Summary

Count and find photos by combinations of colours — orange and teal, blue and
yellow, or any two or three colours a person picks — against the hue circle, in
the Colour topic of Statistics and in the library filter. See
[ADR-0049](../../architecture/adr/0049-colour-combinations.md).

## Details

- A photo has a colour when its swatches of that hue cover at least 10 % of the frame.
- `/api/library/colour` carries `hueSets` (photos per set of hues); the browser counts any combination from them.
- Colour combinations: eight well-known pairs and triads as chords across the hue ring, with a list of counts; a row shows its photos.
- Your combination: a wheel of twelve sectors, up to three chosen, with the count and Show photos.
- Search takes `hues=a,b[,c]`; the library filter has a Colours group.

## Acceptance Criteria

- [x] Photo counts per set of hues, with a photo in two libraries counted once.
- [x] Search by two or three hues, each at least 10 %, as fast as a search without it on 36,000 photos.
- [x] Colour combinations next to Main colours; a row shows its photos.
- [x] Your combination counts the chosen hues and shows their photos.
- [x] The library filter's Colours narrow the results to the combination.
- [x] Go tests (`library`) and e2e (`statistics-colour`).
