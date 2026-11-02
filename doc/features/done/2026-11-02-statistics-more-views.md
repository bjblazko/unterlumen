# Statistics: more views, 2D where it reads better, 3D where it shows more

*Last modified: 2026-11-02*

## Summary

New charts for every topic of Statistics, chosen with one rule: three
dimensions only when each photo has three continuous values and the point
is the clusters in their cloud, or when a map is the floor. Counts and
matrices read better flat; a heatmap as 3D columns hides its own values.
No new chart repeats one there is (Exposure space, Daylight, Colour space,
Character, Space and time, Face space).

A new topic, **Cataloguing**, counts what the metadata editor (ADR-0060,
ADR-0061) fills: how complete the photos' descriptions are, their stars,
their keywords and own fields.

Every chart is clickable like the others: a click shows its photos in
Libraries, where they can be selected and taken to Metadata.

## Details

### The views

| Topic | View | Form |
|---|---|---|
| Equipment | Phone or camera — share of phone photos per period | 2D |
| | How a zoom is used — focal lengths per lens | 2D |
| | Life lines of the gear — first to last photo per camera and lens | 2D |
| Exposure | Risk of shake — share longer than 1/focal length, per lens | 2D |
| | How bright the scenes were — EV100 from aperture, time and ISO | 2D |
| | Exposure compensation and flash per period | 2D |
| Time | **Weekday and hour** — 7 × 24 heatmap | 2D |
| | Year against year — photos per month, a line per year | 2D |
| | Sessions — photos close in time, by length and size | 2D |
| Frame | Portrait or landscape per camera | 2D |
| | Cropped or not — smaller than the camera's full size, per period | 2D |
| | Aspect ratio as a continuous histogram | 2D |
| Colour | Colourfulness per film simulation or camera | 2D |
| | Hue across the hours of the day | 2D |
| | **Palette space** — each photo's three main colours as a triangle in OKLab | 3D |
| Places | **Countries and cities** — from the IPTC fields | 2D |
| | Journeys — days far from home, as a time line | 2D |
| | **Density landscape** — columns on the map, height the photos, colour their mean hue | 3D |
| People | Not seen for a while — months since each person's last photo | 2D |
| | Who where — people × countries or cities | 2D |
| | Who with what — people × things | 2D |
| Things | Things through the year — thing × month | 2D |
| | Things by place — thing × country or city | 2D |
| | **Meaning space** — photos placed by what they show, from the SigLIP vectors | 3D |
| Cataloguing (new) | **Completeness** — share with title, description, keywords, location, country, people, stars | 2D |
| | Stars and the unrated, per period | 2D |
| | Keywords and own fields — the most used | 2D |

Weighed and left out: weekday × hour as 3D columns (they hide each other),
an "exposure triangle" of EV, time and ISO (too close to Exposure space), the
calendar as a mountain range (an effect, not a reading), a relationship graph
in 3D (Circle of people reads better flat).

### Phase 1 (done 2026-11-02)

- **Weekday and hour** (Time): photos per weekday (Monday first) and hour,
  in the sequential ramp (ADR-0034). A click shows the photos of that hour
  on that weekday: search parameter `weekday` (0 = Monday … 6 = Sunday) with
  `hour`.
- **Countries** and **Cities** (Places): photos per value of the IPTC
  fields Country and City, as bars. A click sets that field in the filter,
  as the search does.
- **Completeness** (Cataloguing): for each of title, description, keywords,
  location (GPS), country, named people and stars, the photos that have it
  and those that lack it. A click on the missing part shows exactly the
  photos without it: search parameter `lacks` (repeatable). The sets a
  field is counted from are defined once (`library/completeness.go`) and
  serve the count and the filter alike; each is a set, never correlated
  (ADR-0045).
- **Keywords in the index.** Keywords were not indexed; Completeness and the
  later keyword chart need them. The overlay lays the keywords beside the
  file (sidecar `dc:subject`, a video's NFO `tag`) — or the file's own XMP —
  over the tags as `Keywords`, a list tag kept apart in `exif_entries`
  (ADR-0061). `sidecarReading` becomes 5.

### Phases 2–6 (done 2026-11-02)

All views of the table, arranged by theme at the owner's word ("ordne stats
ruhig neu an"): half-width charts in pairs, 3D views last. They read tallies
by group (`/api/library/insights`, ADR-0062) and two point clouds
(`/api/library/space`); the overview reads neither. Each click sets a
parameter whose SQL is the count's own (`library/insight_terms.go`). Added
on the way from the ideas: exposure time and white balance, the "(no lens)"
criterion. The meaning space uses principal components; UMAP stays open if
they do not separate the owner's photos.

## Acceptance Criteria

- [x] Time shows Weekday and hour; a click shows the photos of that weekday and hour.
- [x] Places shows Countries and Cities from the IPTC fields; a click filters by the value; with none, the chart says where they come from.
- [x] Cataloguing is a topic with Completeness; a click on a missing part shows the photos that lack it.
- [x] Keywords are indexed (sidecar, NFO, the file's XMP) as a list tag.
- [x] The new counts follow the shared filter (ADR-0050) and the path scope; their filters are sets with a plan test.
- [x] Go tests for the counts and the filters; e2e for the three charts and their clicks.
- [x] Later phases as listed: every view of the table, each with a click that shows exactly what it counted.
