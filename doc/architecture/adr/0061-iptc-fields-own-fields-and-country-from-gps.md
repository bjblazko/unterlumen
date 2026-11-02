# ADR-0061: IPTC fields from one table, own fields in the sidecar, the country from GPS offline

*Last modified: 2026-11-02*

## Status

Accepted. Builds on ADR-0060 (metadata written to a named target, what lies
beside a file wins) and ADR-0048 (notes live in the sidecar).

## Context

After the metadata editor, the owner asked about free metadata fields:
key and value, and the standards and nomenclatures for them. The answer
was the IPTC Photo Metadata Standard for what has a name, and
Unterlumen's own fields (`ul:Fields`) for the rest. When asked, the owner
chose:
- **Fields:** where (sublocation, city, province or state, country,
  country code), headline and event, origin and rights (credit line,
  source, usage terms, web statement, instructions, job ID), and who and
  what is shown (people and organisations).
- **Where they are edited:** in Metadata and in the info panel, for photos
  in Folders too, not only in libraries.
- **The country from the GPS position**, offline, country only.
- **Own fields** in Metadata and in Folders, **mirrored as a machine tag**
  among the keywords when asked; the IPTC fields in the **library filter**
  and in the **search** above every place.

## Decision

1. **One field table.** `internal/metadata/table.go` names, for each field,
   its sidecar property, its exiftool tags (XMP and IPTC-IIM), its element in
   a video's NFO and the tag the index and the info panel show it under.
   Reading, writing, the overlay for the index and the NFO all go through
   it; a new field is one line. The date taken and the rating keep rules of
   their own (EXIF date tags with offset, `premiered`, stars in tens).
2. **IPTC Core's legacy location fields**, not the Extension's Location
   Shown structure: `Iptc4xmpCore:Location`, `photoshop:City`, `State`,
   `Country`, `Iptc4xmpCore:CountryCode`. That is what Lightroom, digiKam
   and Capture One show as Sublocation, City, State and Country.
3. **Lists** (Person Shown, Organisation Shown, and Creator) are read as
   "Ann; Bob" and written as entries: `rdf:Bag` (`rdf:Seq` for Creator) in
   XMP, `-sep "; "` with exiftool, one element each in the NFO — Kodi's
   `<actor><name>`, keeping an actor's role and picture. The editor adds and
   removes entries per file, like keywords (`Change.Add`/`Remove`). The
   index keeps each entry in `exif_entries`, so a filter finds a photo by
   one name; the condition is a set, never correlated (ADR-0045).
4. **Unterlumen's own description** in a sidecar now declares the IPTC
   namespaces too. The earlier one, with fewer, is found and taken over, so
   no sidecar gets two.
5. **The file's own XMP is read** below what lies beside it and above EXIF,
   so values another program wrote into a file reach the index and the info
   panel without exiftool. IPTC-IIM alone is read only with exiftool.
   Files already indexed get them when they change, not by an update: a
   re-read of every file is too costly on a NAS.
6. **Own fields only in the sidecar.** exiftool writes no free namespace,
   so own fields go to `ul:Fields` whatever the target, and the editor says
   so; a video has none. A library follows at once through
   `Manager.MetadataWritten`, which takes every note written.
7. **Machine tags.** An own field mirrored as a keyword is `key=value`,
   with `ul:` before a key that has no namespace of its own
   (`ul:stock=Portra`, `film:stock=Portra`). Whether it is mirrored is read
   from the keywords themselves; there is no second record to keep in step.
   When the value changes, the old keyword goes.
8. **The country from the GPS position, offline.** `internal/places` embeds
   Natural Earth's Admin 0 countries at 1:10m (public domain), simplified to
   about half a kilometre, about 1 MB: ISO 3166-1 alpha-2 codes
   (`ISO_A2_EH`) and names in English and German. **Disputed areas follow
   the German point of view** (`_deu`): the Crimea in Ukraine, Northern
   Cyprus in Cyprus, Western Sahara its own. A position up to about 20 km
   off a border counts as in that country, so photos from a beach or a pier
   are not lost to the sea. It is worked out per file
   (`Change.CountryFromGPS`) and shown in the preview; no request leaves the
   machine.
9. **The info panel** shows the filled IPTC fields — description, creator
   and copyright among them — in a section of their own, edited in place
   and added from "Add a field…", written to the sidecar like the stars. A
   photo outside a library gets its title and own fields there too, read and
   written through Metadata's API. **Where a value lies is marked, not
   coloured**: the overlay records per tag what lies beside the file
   (`ExifData.Beside`) and what the file says under it (`Shadowed`); such a
   value carries the neutral chip `XMP` (`NFO` for a video) after its label,
   whose hover names the file's own value. A value without the mark is in
   the file. Of three ways shown to the owner — the source as a word after
   every value, a mark on the exception, two sections by place — the owner
   chose the mark: it is compact, keeps related fields together, and colour
   would have given a signal colour a second meaning (ADR-0030).
10. **Filter and search.** The library filter offers Country, City,
    Sublocation, Event, People and Organisations shown as soon as one photo
    has a value, and says where they come from while none has. The search
    suggests their values, "Kiel · City", and opens Libraries with that
    filter.

## Consequences

- Every sidecar is read once again after the update (`sidecarReading` 3,
  then 4 for the marks).
- A value removed beside a file that the file itself holds shows again;
  the info panel says so and names where to remove it for good.
- A new IPTC field costs a line in the table, a line in `META_GROUPS`
  (`metadata-form.js`) and its translations.
- Countries come from a dataset frozen at its version; `gen_countries.go`
  builds it again from a newer one.
