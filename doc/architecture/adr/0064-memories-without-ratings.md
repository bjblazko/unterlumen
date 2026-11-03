# ADR-0064: Memories chosen by what photos show, not by stars

*Last modified: 2026-11-03*

## Status

Accepted.

## Context

The owner wants a place that brings old photos back: on this day in earlier
years, a trip, a person, this month back then, something rediscovered. Most
of their photos have no stars, and stars say what was picked for a gallery,
not what is worth seeing again. The choice must therefore come from what the
index already knows about every photo:

- date and place;
- the named faces (ADR-0051);
- the things (ADR-0054);
- the appearance and the dHash (ADR-0044).

Nothing records which photos were opened. Recording it would be new state,
held per installation, that the other installation could not see.

## Decision

- **A place, Memories** (`#memories`, `internal/memories`, `/api/memories`
  and `/api/memories/detail`). The browser sends its local date as `day`,
  and the server answers an overview and a page per memory.
- **One read per library.** `Store.MemoryPhotos` reads every dated photo
  that is not a video, together with what it shows, in five set-based
  queries inside the shared filter. `Manager.MemoryPhotos` merges the
  libraries, keeping each photo once. The result is cached under a version
  made of the content stamps, the change counters of people and things, and
  the number of measured appearances.
- **The selection** (`Pick`), in order:
  1. Leave out what is not a memory: screenshots, documents, and photos
     whose only things are blurred, underexposed or overexposed. A
     failure found beside other things is not the photo's best match, so
     the photo stays.
  2. Split the photos into moments at gaps of more than 30 minutes, and
     after two hours at the latest, so that a day photographed without a
     pause still gives several photos. A moment gives at most one pick.
  3. Score each photo as 100 per named person, plus its rarity in the
     filtered libraries, plus a small jitter from a hash of the day, the
     memory and the photo.
     - Rarity is the maximum of −ln(share) over the photo's things, its
       main hue bin, a low or high tone key, and being more than 100 km
       from home.
     - The maximum, not the sum, so that a photo is not rewarded for
       simply showing many things.
  4. Take the best three to seven — as many as a hash of the day and the
     memory chooses, so the memories weigh differently. Skip one that
     looks like a pick already taken (dHash distance ≤ 10). A person or
     a month takes the best photo of some of its years, chosen by the same
     hash, not of every year in a row.
- **Deterministic per day.** The day is part of every seed. The same day
  gives the same page, and the next day may give another.
- **One sheet, laid out by chance** (amended 2026-11-03). The overview is
  no list of years and cards but one sheet (`memories-sheet.js`):
  - each memory is a block of its photos, a collage without gaps made
    from a random tree of splits (side by side, one above the other);
  - each photo takes a format it can carry without losing what it shows:
    its own shape, a poster (2:3) when upright, a square, wide (16:9) or
    a panorama (21:9) when wide enough;
  - of some sixty trees, the one nearest a shape drawn at random between
    upright and wide is taken, none with a photo too small to see;
  - blocks stand side by side in bands of one height, each as wide as its
    shape asks; a block alone in its band is built again wide enough to
    fill it;
  - the order of the memories is shuffled on every visit, and Shuffle
    lays the sheet out anew. The photos themselves stay the day's.

  No photo appears twice on the sheet: a card shows only what no memory
  before it shows.
- **Stars and months never count.** The handler clears `Rating`, `From` and
  `Until` of the shared filter.
- **Nothing is stored.** There is no hiding, no exclusion list and no record
  of what was opened. "Rediscovered" rotates through the rarest 5 % by the
  day instead.
- **Trips** use the rule of the Statistics' journeys (`findJourneys`):
  - home is the 0.1° cell with the most photos;
  - a day is away more than 100 km from it;
  - away days at most 2 days apart form one trip.

  `memories.FindTrips` ports it to Go; Statistics keeps its own in the
  browser for now.

## Consequences

- The sheet looks different on every visit, the photos only from day to
  day. A layout cannot be linked to; a memory's page can.
- People who are named dominate the choice. A library without named faces
  falls back to rarity alone.
- A library without appearance or things yet still has memories, only
  chosen less well. A pass that adds them changes the version, and the
  choice with it.
- Two implementations of the trip rule exist, one in Go and one in
  JavaScript. A change to one must change the other.
