# Photos of people for the face recognition tests

*Last modified: 2026-10-23*

These 64 photos show **no real people**. They are the only photos in this
repository made with AI: they were generated with Google Gemini as four
contact sheets — portraits, a city, the sea, a party — of the same invented
people, cut into single photos, and given dates and, but for the party,
places in Berlin and Nice with exiftool. Each carries the IPTC digital
source type `trainedAlgorithmicMedia`, so it says so itself. As far as any
rights in them exist, they are CC0 (`LICENSE`).

They are here only for Unterlumen's end-to-end tests of People
(`e2e/specs/face-recognition.spec.js`) and for its screenshots. Some faces
are in profile or half hidden on purpose: the detector misses them, and the
tests mark one of them by hand.

Every other photo here — `src/examples` and what the README's other
screenshots show — was taken by Timo Böwing (`src/examples/LICENSE`).

`fixtures/setup.sh` copies these to `fixtures/faces-photos`, so the sidecars
the tests write never land here.
