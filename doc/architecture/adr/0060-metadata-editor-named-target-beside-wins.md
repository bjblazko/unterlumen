# ADR-0060: Metadata is written to a named target, and what lies beside a file wins

*Last modified: 2026-11-01*

## Status

Accepted. Adds to ADR-0048 (notes in the sidecar), ADR-0052 (the index
follows the folder) and ADR-0058 (videos are never edited).

## Context

The owner wanted one editor for what a photo or video says about itself,
and one requirement above all: **it must always be clear whether the EXIF of
the file is changed, or a sidecar beside it** — for a video, an NFO file. The
second need was a correction of many dates at once that **keeps each file's
time of day**, as Set location does for many places at once.

Before this, writing was scattered. GPS went into the EXIF only, through
exiftool. Title, fields and keywords went into the XMP sidecar only.
Nothing could write a date taken, and nothing knew NFO files.

When asked, the owner chose:
- **Fields:** date taken; title, description, keywords; creator, copyright
  and rating. GPS stays with Set location.
- **The target is chosen with each change**, the sidecar by default.
- **Date correction:** both "set the day, keep each file's time" and "shift
  everything by an amount, keep the time between files", with a preview of
  before → after for each file.
- **NFO in Kodi/Jellyfin form**, and **sidecar or NFO wins over the file**
  when reading, with the source shown beside the value.
- **A place of its own** (`#metadata`), reached from any selection.
- **"Several values"** for fields the files disagree on; only what is touched
  is written. Keywords are added and removed, never replaced.
- **exiftool overwrites the file**, as Set location does.

## Decision

1. **`internal/metadata`** owns the editor's reading and writing. A change is
   `Change{Set, KeywordsAdd, KeywordsRemove, Date}` and goes to a
   `Target`, `file` or `sidecar`. `PlanFor` works out each file's own values,
   its date from its own date, so the preview (`/api/metadata/read` with a
   change) and the write (`/api/metadata/write`, one file per request) never
   disagree.
2. **Into the file:** exiftool writes each field where programs look for it.
   - Date: `EXIF:DateTimeOriginal` and `CreateDate`, plus the offset tags when the date has an offset.
   - Title: `XMP-dc:Title` and `IPTC:ObjectName`.
   - Description: `ImageDescription`, `XMP-dc:Description` and `Caption-Abstract`.
   - Creator: `Artist`, `XMP-dc:Creator` and `By-line`.
   - Rights: `Copyright`, `XMP-dc:Rights` and `CopyrightNotice`.
   - Rating: `XMP-xmp:Rating`.
   - Keywords: `XMP-dc:Subject` and `IPTC:Keywords`.

   The file's content changes, so in a library its ID changes and the old
   row is retired (ADR-0052). The editor says so before writing.
3. **Into the sidecar of a photo** (`media/xmp_props.go`), always through
   `changeSidecar`.
   - Names used:
     - Date: `exif:DateTimeOriginal` and `photoshop:DateCreated`.
     - Rating: `xmp:Rating`.
     - Description, creator and rights: `dc:description`, `dc:creator` and `dc:rights`.
     - Title and keywords go where they already went: `dc:title` and `dc:subject`.
   - A property that another program keeps is changed where it is, whether it is an element or an attribute (Lightroom writes `xmp:Rating="3"`).
   - Any other property goes into a description of Unterlumen's own that holds only these. The `ul:` block is untouched.
4. **Into the NFO of a video:** a video is still never changed (ADR-0058).
   Its NFO file beside it, `<movie>` as Kodi and Jellyfin read it, takes:
   - `title`, `plot` and `credits`;
   - `tag` for the keywords;
   - `userrating`, two points per star;
   - `premiered` for the day.

   Kodi keeps only a day, so the time taken goes into `ul:datetaken`, and the
   rights into `ul:rights`, in the namespace
   `https://huepattl.de/unterlumen/nfo/1.0/`; both programs ignore elements
   they do not know. Other elements stay. A `musicvideo` or `episodedetails`
   NFO is changed the same way; any other is refused and left as it is. For a
   video, "into the file" also means its NFO, and the editor says so. The NFO
   travels with its video on a move or rename, and goes when the video is
   deleted.
5. **Reading, what lies beside wins:** `metadata.Read` gives each field's
   value with its source: `sidecar`/`nfo`, else `file`, else (date only)
   `file name`.
   - In the editor, the file is read with exiftool when it is there, in one run for all files; otherwise from what the app reads itself.
   - For the index and the info panel, `metadata.Overlay` lays the sidecar's or NFO's date, description, creator, rights and rating over `ExtractMeta`. It marks `DateTakenFrom` and `MetadataFrom` in the tags, so the Timeline, Map and Statistics follow without a change of their own.
   - The info panel says "from the sidecar" or "from the NFO file" beside such a date.
6. **The index follows a sidecar that changed alone:**
   - When a scan finds a changed sidecar stamp, it now also reads the file's metadata again (`refreshOverlay`).
   - It does so only if the sidecar has such a field, or the photo was overlaid before, so ordinary sidecar changes stay cheap.
   - A video's stamp includes its NFO file.
   - `sidecarReading` is now "2", so every sidecar is read once again after the update. Dates Lightroom or darktable keep in sidecars come in then; their formats, such as fractions or `2024:05:01 12:30:00`, are normalised.
7. **After writing**, `Manager.MetadataWritten` indexes the file in every library that covers it, with a new ID where the content changed. A title written to the sidecar becomes the title note directly, because a scan of a library whose notes are not yet in the sidecars only adds notes and would not remove an emptied title (ADR-0048).
8. **The place** shows the target at the top, always:
   - **Write to: Sidecar (XMP · NFO) · File (EXIF)**, with a sentence on what exactly happens. "Changes 12 photos themselves. Their content changes, and with it their ID in a library…"
   - If exiftool is missing, File is disabled, and the sentence says why.
   - If a changed field has a value of its own in a file's sidecar, the sentence says that this value stays and is still the one shown.
   - Writing more than five files runs in the progress dialog (ADR-0036).

9. **Stars.**
   - Owner's choices:
     - The keys **1–5** rate the selection in Folders or a library, or the photo in the viewer, and **0** takes the rating away. These keys no longer open places; only `,` (Settings) is left of those keys.
     - Organize keeps its number keys for its targets.
     - A star in the info panel or the viewer's toolbar can be clicked, and the star of the current rating takes it away.
   - The rating is always written to the sidecar (`POST /api/metadata/rating`): to `xmp:Rating`, or a video's `userrating`. That is quick, needs no exiftool, and leaves the file and its ID alone.
   - `0` is written, not removed, so it also hides a rating the camera or Lightroom put into the file.
   - The file's own rating is read from the XMP packet near its start (`media.embeddedRating`). The sidecar's rating wins over it.
   - Shown:
     - On the tiles, with Details on.
     - In a Rating column in the list.
     - In the info panel and the viewer.
   - The library filter has a **Rating** choice: Doesn’t matter (the default) · ★ or more … ★★★★ or more · ★★★★★ · Not rated. `Filter.Rating` narrows to a set from `exif_index (field='Rating', value IN …)`, never correlated (ADR-0045).

## Consequences

- A date can be corrected for thousands of files without exiftool. The
  default writes only sidecars, and the photos stay byte-identical, so a
  library keeps their IDs.
- A sidecar written by another program now sets the date taken in the
  index. That is the rule the owner chose. Where Lightroom's sidecar
  disagrees with the camera, Lightroom is shown, with its source.
- The first scan after the update reads every sidecar again, and the EXIF
  of each photo whose sidecar holds a date or one of the other fields.
- A title written into the file is shown in the editor ("from the file").
  The info panel's title stays the sidecar's note, as before (ADR-0048).
- An NFO file changed by another program is noticed by the next scan of
  the video's folder (stamp). A full rescan is not needed.
