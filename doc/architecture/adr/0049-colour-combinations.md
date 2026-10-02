# ADR-0049: Colour combinations are counted from sets of hues

*Last modified: 2026-10-02*

## Status

Accepted. Builds on [ADR-0044](0044-photo-appearance-from-thumbnails.md)
(main colours measured per photo) and
[ADR-0043](0043-statistics-place-with-topics.md) (the Colour topic).

## Context

The Colour topic shows the main colours of a library as a hue wheel. The owner
asked on 2026-10-02 for colour combinations against that wheel: how many photos
are orange and teal, blue and yellow, or any two or three colours a person
names, and to find those photos — in Statistics and in the library filter —
by colour ranges such as "blue", not by exact colours.

Every analysed photo already has up to five swatches with a share of the frame
and a hue sector of OKLCh (`photo_palette.hue_bin`, twelve named sectors of 30°).

## Decision

1. **A photo has a hue when its swatches of that hue cover at least 10 % of the
   frame** (`library.ComboShareMin`), added up over its swatches of the hue. A
   combination's photos have every one of its hues. 10 %, not the main colour's
   20 %: two or three colours share one frame.

2. **The server counts photos per set of hues.** The Colour answer
   (`/api/library/colour`) carries `hueSets`: for each set of hues that photos
   have, how many have exactly that set. It is built from the swatches the
   Colour topic reads anyway, with no query of its own (19 ms for 297 sets on
   36,000 photos). A combination's count is the sum over the sets that contain
   it, so the browser answers any choice at once.

3. **Two charts beside Main colours.**
   - *Colour combinations*: the hue ring with eight combinations photographers
     name — orange & teal, blue & orange, blue & yellow, violet & yellow,
     red & green, pink & green, and the triads red, yellow & blue and orange,
     green & violet — as chords (a pair) or triangles (a triad) as thick as
     their photos are many, in the interface's ink, with a list of swatches,
     names and counts. A row shows its photos.
   - *Your combination*: a wheel of the twelve sectors; up to three chosen
     ones carry a frame in the interface's ink (orange stays the page's one
     action), the middle and a sentence give the count, and *Show photos*
     opens them.

4. **Search takes `hues=a,b[,c]`**, each a sector 0–11, at most three. Each is a
   set read through the `(hue_bin, share)` index, grouped by photo. On a copy of
   a 36,000-photo library it costs no more than a search without it.

5. **The library filter has Colours**: the twelve named sectors as chips, up to
   three, with an active-filter chip "Colours: orange & teal".

## Consequences

- The combinations follow OKLCh's hue circle, not a painter's: "blue & orange"
  and "orange & teal" are different pairs, as photographers use them.
- A photo can count for several combinations; the counts do not add up to the
  photos.
- Changing the threshold changes every count and filter at once, from one
  constant.
