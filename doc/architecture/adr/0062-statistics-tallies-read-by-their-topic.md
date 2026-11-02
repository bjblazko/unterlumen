# ADR-0062: The newer Statistics views count in tallies, read only by their topic

*Last modified: 2026-11-02*

## Status

Accepted. Builds on ADR-0043 (Statistics by topic), ADR-0045 (sets, never
correlated subqueries), ADR-0046 (3D views on one point stage) and ADR-0050
(the shared filter).

## Context

The owner asked for new views across every topic of Statistics — about
twenty-five, three of them 3D — and for them to be arranged so each topic
reads by theme and fills its rows. Each view needs counts the snapshot
(`/api/library/statistics`) does not carry. The snapshot is read by every
visit to Statistics, the overview included; on a NAS each library's queries
run one after another on its one connection, so every query added there
slows every visit.

## Decision

1. **Tallies.** The new views' counts are tallies: for each chart, key →
   photos, the key's dimensions joined by `\x1f` ("XF23\x1f23" — a lens at
   23 mm). Tallies of several libraries are summed, so one shape serves every
   view, its merging and its cache (`library.Insights`, `insightsCache`).
   What is not additive — a median, the last month a person was seen, a
   journey — is worked out in the browser from additive tallies (bins, months,
   days by cell).
2. **Groups, read by their topic.** The tallies come in groups — time, gear,
   frame, catalogue, places, colour, people, things —
   `GET /api/library/insights?group=…`, each read only when a topic whose
   charts name it (`source: 'in:gear'`) is shown. The overview reads none:
   its cards draw only charts from the snapshot and the older sources
   (`statsPreviewable`).
3. **One definition for a count and its click.** What a view calls a phone,
   a photo at risk of shake (longer than 1/focal length, 35 mm), its scene's
   light (EV100), a flash photo, a portrait (turned by Orientation 5–8), a
   cropped photo (under nine tenths of its camera's largest frame), a ratio,
   a session (photos less than two hours apart), a column of the map — is SQL
   in one place (`library/insight_terms.go`, `insights_time.go`), used by the
   tally and by the search parameter its click sets (`device`, `slow`,
   `ev_from`/`ev_to`, `flash`, `orient`, `cropped`, `ratio_from`/`ratio_to`,
   `taken_from`/`taken_to`, `lat_from`…`lon_to`, `nolens`). A click shows
   exactly the photos counted. Every condition is a set (ADR-0045). Ranges
   are not named `_min`/`_max`, which the search reads as EXIF fields.
4. **The 3D views.** The palette space (each photo's three largest colours in
   OKLab, joined into a triangle; a sample of 6,000) and the meaning space
   (the vision model's vectors reduced to three axes by principal components,
   power iteration, `library/pca.go`) are point clouds,
   `GET /api/library/space?view=…`, cached with the colours. The density
   landscape is drawn from the places tallies. None has a preview on the
   overview. UMAP stays open should the principal components not separate
   the owner's photos well enough.
5. **Arrangement.** Within a topic the charts follow its theme; two
   half-width charts stand side by side; the 3D views follow at the page's
   width (ADR-0046).

## Consequences

- Opening a topic reads its groups once per scope and filter; the first
  read of a large library on a NAS takes seconds (the meaning space most),
  later ones come from the cache until the next scan.
- A new view is a tally query, a renderer and a line in `STATS_TOPICS`;
  a new kind of click is one more definition in `insight_terms.go`.
