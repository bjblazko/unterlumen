# IPTC fields and own fields

*Last modified: 2026-11-02*

## Summary

Photos get the fields of the IPTC Photo Metadata Standard that Lightroom,
digiKam, Capture One and Photo Mechanic show:

- Where it was taken: Sublocation, City, Province/State, Country, Country code.
- Headline and Event.
- Origin and rights: Credit Line, Source, Usage Terms, Web Statement of Rights, Instructions, Job ID.
- Who and what is shown: Person Shown, Organisation Shown.

They are edited in Metadata and in the info panel, for photos in Folders
and in libraries alike. They are indexed for the library filter and offered
by the search above every place.

The free fields Unterlumen already has (`ul:Fields`, ADR-0048) become
**Own fields** for everything the standard has no name for. They can be
edited in Metadata and in Folders too, and can be mirrored as a keyword
(a machine tag), so other programs find them.

Country and country code can be filled from a photo's GPS position,
offline, from Natural Earth's country borders built into the app.

Builds on ADR-0060. The decisions are in [ADR-0061](../../architecture/adr/0061-iptc-fields-own-fields-and-country-from-gps.md).

## Details

### Fields and where they are written

Each field is one entry of a single table in `internal/metadata`
(`fieldSpec`). An entry holds:

- the sidecar property (namespace, element, kind: simple · Alt · Bag);
- the exiftool tags in the file, XMP and IPTC-IIM;
- the NFO element of a video;
- its group and label in the interface.

Today's scattered maps (`sidecarProps`, `fileTags`, `nfoElements`) become
that table. A new field is then one line, not five edits.

| Field | Sidecar (XMP) | Into the file (exiftool) | Video NFO |
|---|---|---|---|
| Sublocation | `Iptc4xmpCore:Location` | `XMP-iptcCore:Location`, `IPTC:Sub-location` | `ul:sublocation` |
| City | `photoshop:City` | `XMP-photoshop:City`, `IPTC:City` | `ul:city` |
| Province/State | `photoshop:State` | `XMP-photoshop:State`, `IPTC:Province-State` | `ul:state` |
| Country | `photoshop:Country` | `XMP-photoshop:Country`, `IPTC:Country-PrimaryLocationName` | `country` |
| Country code | `Iptc4xmpCore:CountryCode` | `XMP-iptcCore:CountryCode`, `IPTC:Country-PrimaryLocationCode` | `ul:countrycode` |
| Headline | `photoshop:Headline` | `XMP-photoshop:Headline`, `IPTC:Headline` | `tagline` |
| Event | `Iptc4xmpExt:Event` (Alt) | `XMP-iptcExt:Event` | `ul:event` |
| Credit Line | `photoshop:Credit` | `XMP-photoshop:Credit`, `IPTC:Credit` | `ul:credit` |
| Source | `photoshop:Source` | `XMP-photoshop:Source`, `IPTC:Source` | `ul:source` |
| Usage Terms | `xmpRights:UsageTerms` (Alt) | `XMP-xmpRights:UsageTerms` | `ul:usageterms` |
| Web Statement | `xmpRights:WebStatement` | `XMP-xmpRights:WebStatement` | `ul:webstatement` |
| Instructions | `photoshop:Instructions` | `XMP-photoshop:Instructions`, `IPTC:SpecialInstructions` | `ul:instructions` |
| Job ID | `photoshop:TransmissionReference` | `XMP-photoshop:TransmissionReference`, `IPTC:OriginalTransmissionReference` | `ul:jobid` |
| Person Shown | `Iptc4xmpExt:PersonInImage` (Bag) | `XMP-iptcExt:PersonInImage` | `actor` (name only) |
| Organisation Shown | `Iptc4xmpExt:OrganisationInImageName` (Bag) | `XMP-iptcExt:OrganisationInImageName` | `ul:organisation` |

**Notes on the mapping:**
- **Location fields:** IPTC Core's legacy location fields are used, not the Extension's Location Shown structure. That is what Lightroom and digiKam show as Sublocation/City/State/Country.
- **Bag fields:** Person and Organisation Shown are read joined by "; ", the way Creator already is. In the editor they are edited like keywords: added and removed, not replaced.
- **Sidecar:** written through `media.WriteSidecarProps`, which gains a `propBag` kind and the namespaces `Iptc4xmpCore`, `Iptc4xmpExt` and `xmpRights`. As before, a property that another program keeps is changed where it is.
- **Reading the file:** `metadata.Read` asks exiftool for the tags above. Without exiftool, the file's own XMP packet is read (the same scan as `embeddedRating`, generalised to every property). IPTC-IIM alone is read only with exiftool.

### Own fields

Own fields are key and value, written only to `ul:Fields` in the sidecar,
since exiftool knows no free namespace. The editor says so beside the group:
"Own fields are always written to the sidecar."

**In Metadata:**
- The keys of all chosen files are listed, each with its value or "Several values" and "in 4 of 12".
- You can add a field, change a value, or remove a field from all files.
- In a library, notes go through `Manager.MetadataWritten`, so the index follows (ADR-0048).

**In the info panel:** a photo in Folders gets the same Own fields section a
library photo has today. It writes through `/api/metadata/write`.

**Mirroring as a keyword:** a field's switch **As keyword** writes the
keyword `key=value`. If the key has no namespace of its own (`film:stock`),
it becomes `ul:key=value`. When the value changes, the old keyword goes.
Whether a field is mirrored is read from the keywords themselves, so there
is no second record to keep in step.

### Country from the GPS position, offline

- The countries come from Natural Earth's Admin 0 country borders at 1:10m (public domain). Points-in-polygon are found on a simplified grid of polygons embedded in the binary (`internal/places/countries.bin`, target < 1 MB). The ISO 3166-1 alpha-2 code goes into Country code. The name goes into Country, in the library's language where Natural Earth has it (`NAME_DE`, `NAME_EN`, …), otherwise in English.
- In Metadata, the Location group has **Country from the GPS position**. It is worked out for each file, like the date correction (`Change.CountryFromGPS`), and the preview shows it per file. Files without GPS are named and left as they are.
- No network call; Your data stays true.
- **Before it ships:** `credits.json` names the countries dataset beside the coastlines, and the huepattl-legal-check skill runs, since something new is bundled.

### Index, filter, search

- **Overlay:** `metadata.Overlay` lays the IPTC fields over the tags (`City`, `Country`, `CountryCode`, `Sublocation`, `State`, `Headline`, `Event`, `Credit`, `Source`, `PersonInImage`, `OrganisationInImageName`, …), so they reach `exif_index`. `sidecarReading` becomes "3", so every sidecar is read once again.
- **Library filter:** the text filters offer Country, City, Event, Person shown and Organisation shown, from the values in the index. The chip input keeps its "Custom tag" entries for own fields.
- **Search bar:** suggestions also list places (City, Country, Sublocation), events, people shown and organisations whose values hold the letters typed, as "Kiel · City". Choosing one opens Libraries with that filter set. The values come from the endpoint the filter already reads its text values from.
- **New conditions:** each one is a set from `exif_index (field, value)`, never correlated, with a plan test (ADR-0045).

### Interface

- **Metadata:** the fields stand in groups, each of which can be folded:
  1. Date taken
  2. Description (title, description, keywords, creator, copyright, rating)
  3. Location
  4. Event and headline
  5. Origin and rights
  6. Shown
  7. Own fields

  A group whose fields are all empty in every chosen file starts folded and says "Empty". The target switch at the top stays as it is. Own fields carry their note that they always go to the sidecar.
- **Info panel:** a section **IPTC** lists the filled fields, grouped as above. Each value is editable in place, like the title. Empty fields are offered under "Add a field…". Own fields follow in their own section.
- Labels in sentence case; values in mono where they are data (country code, Job ID, URL).

### Documentation

- ADR-0061: the field table as the single source, IPTC Core legacy location fields, own fields sidecar-only, machine tags `ns:key=value`, offline country.
- arc42: building blocks (`internal/places`), ADR index.
- README, CHANGELOG, credits.

## Phases

1. **Field table** (done 2026-11-02): `internal/metadata/table.go` holds every field; `media.NewProp` with `PropBag` and the IPTC namespaces; Unterlumen's own description declares them all and takes over the earlier one; exiftool writes lists with `-sep "; "`; the NFO reads and writes lists (`actor` by name, keeping its role); without exiftool the file's own XMP packet is read; `Change.Add`/`Remove` for list fields. Go tests for every field in the sidecar, the file and the NFO.
2. **Index** (done 2026-11-02): `metadata.Overlay` lays the fields over the tags, the file's own XMP below what lies beside it; `sidecarReading` is 3; list fields (Person and Organisation Shown, Creator) keep each entry in `exif_entries`, which the library's text filter and `/api/library/exif-values` read for those tags (a set, plan test `TestEntryFilterIsNotCorrelated`). The IPTC of files already indexed and without a sidecar is read when the file changes, not by the update.
3. **Metadata place** (done 2026-11-02): groups that fold (`META_GROUPS` in `metadata-form.js`), list fields and keywords as chips (`metadata-chips.js`), own fields with "in n of m" and **As keyword** (`metadata-own.js`); `Change.Own`, `Plan.Own`, the keywords in the plan, `metadata.MachineTag`; `Manager.MetadataWritten` takes the notes written (title and own fields).
4. **Info panel** (done 2026-11-02): section IPTC (`info-iptc.js`) with the filled fields by group, edited in place and added from **Add a field…**, written through `/api/metadata/write` to the sidecar; title and own fields for any photo outside a library through `sidecarNotesContext` (`info-records.js`); the section "Meta" is now "Own fields".
5. **Country from GPS** (done 2026-11-02): `internal/places` with `countries.json.gz` (Natural Earth Admin 0 1:10m 5.2.0-pre in the German point of view `_deu` — the Crimea in Ukraine, Northern Cyprus in Cyprus, Western Sahara its own — simplified to 0.005°, about 1 MB, names in English and German, codes from `ISO_A2_EH`) built by `gen_countries.go`; a position up to about 20 km off a border counts as in that country (coasts); `Change.CountryFromGPS{Lang}`, GPS read with exiftool or from EXIF; the switch in Metadata's Location group, the country per file in the preview; credits updated; legal check done (public domain, no notice duty, no network call).
6. **Search and filter** (done 2026-11-02): the filter's text fields offer Country, City, Sublocation, Event, People and Organisations shown as soon as one photo has a value (`any` in `EXIF_TEXT_FILTER_FIELDS`); the search bar suggests their values under Places, Events and Shown (`SEARCH_IPTC_FIELDS`), and a choice opens Libraries with `{ exif: { City: … } }`.
7. **Docs and tests** (done 2026-11-02): ADR-0061, README, arc42, CLAUDE.md; e2e in `metadata-editor.spec.js` and `library-search.spec.js`; the info panel says when a value removed beside a file still stands in the file itself.

## Acceptance Criteria

- [x] The 15 IPTC fields can be read and written, into the file or the sidecar. A video gets them in its NFO. Lightroom-style sidecars are changed in place.
- [x] One table defines each field. No field is spelled out in more than one place in Go.
- [x] Metadata groups the fields. Empty groups start folded. Person and Organisation Shown are added and removed like keywords.
- [x] Own fields can be edited in Metadata, many files at once, and in the info panel for Folders and library photos. They always go to the sidecar, and the interface says so.
- [x] A field's **As keyword** writes `ns:key=value` and replaces it when the value changes.
- [x] Country from the GPS position fills Country and Country code per file, offline, and names the files without GPS.
- [x] The info panel shows the filled IPTC fields and edits them in place.
- [x] The library filter offers Country, City, Event, Person and Organisation Shown. The search bar suggests their values. No query is correlated.
- [x] The countries data is credited, and the legal check is done before release.
- [x] Go tests (field table round trip per target, NFO, embedded XMP, country lookup, filter plans); e2e (editor groups, own fields with machine tag, info panel IPTC in Folders, country from GPS, filter and search suggestion).
