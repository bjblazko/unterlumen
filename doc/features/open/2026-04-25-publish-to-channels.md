*Last modified: 2026-04-25*

# Feature: Publish to Channels

## Summary

From library mode, select one or more photos and **publish** them to a named channel (Instagram, Mastodon, a personal website, etc.). Publishing:

1. Records where and when in an **XMP sidecar** next to the original (primary, portable truth).
2. Caches the record in the **library DB** for fast search.
3. Writes a **platform-optimised export copy** into a per-library channel output folder.

Users manage channels — name, format, quality, target dimensions — in a dedicated **Channel Settings UI**. Channels are global (shared across all libraries); output folders are per-library.

---

## Motivation

Photographers need to track *where* and *when* work was shared — for rights management, for avoiding double-posts, and for building a searchable history. This information must live with the photo, not locked in a proprietary database. The XMP sidecar is the source of truth: drop or rebuild the library DB and nothing is lost.

---

## Storage Design

### XMP sidecar (primary truth)

XMP sidecars are the industry-standard non-destructive metadata layer. They sit next to the original — no file is ever modified.

**Sidecar location:**
```
/photos/IMG_1234.heic
/photos/IMG_1234.xmp    ← created/updated by Unterlumen
```

**Custom namespace:**
```
xmlns:ul="https://unterlumen.app/xmp/1.0/"
```

**Schema** — each publish event is one entry in `ul:Publications`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
        xmlns:ul="https://unterlumen.app/xmp/1.0/">
      <ul:Publications>
        <rdf:Bag>
          <rdf:li rdf:parseType="Resource">
            <ul:Channel>instagram</ul:Channel>
            <ul:PublishedAt>2026-04-25T14:30:00Z</ul:PublishedAt>
          </rdf:li>
        </rdf:Bag>
      </ul:Publications>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
```

Fields per entry:

| Field | Type | Description |
|---|---|---|
| `ul:Channel` | string | Channel slug matching the global channel definition |
| `ul:PublishedAt` | ISO 8601 UTC | When the publish action was recorded (default: now; user can back-date) |

Multiple events for the same channel are allowed (re-posts). Write is always merge-safe: existing sidecar namespaces (darktable, Lightroom, etc.) are preserved.

### Library DB cache (secondary, searchable)

After writing the sidecar, upsert into the existing `photo_meta` table:

| key | value |
|---|---|
| `published:instagram` | `2026-04-25T14:30:00Z` (most recent) |
| `published:mastodon` | `2026-04-26T09:00:00Z` |

Enables fast filter queries without XML parsing. Rebuilt automatically during re-index.

### Re-index integration

During re-index (`src/internal/library/index.go`), after EXIF extraction:

1. Check for a `.xmp` sidecar alongside the photo.
2. If found, parse `ul:Publications` from the `https://unterlumen.app/xmp/1.0/` namespace.
3. For each entry, upsert `published:<channel>` → latest `PublishedAt` into `photo_meta`.

Drop the DB, run re-index, publication history is fully restored.

---

## Channel Configuration

### Global channel registry

Channels are defined once and shared across all libraries:

```
~/.unterlumen/
  channels.json          ← channel definitions (global)
  libraries/
    <uuid>/
      library.db
      channels/
        instagram/         ← export output (per library)
          instagram_2026-04-25T143000_IMG_1234.jpg
        mastodon/
```

### Channel schema (`channels.json`)

```json
[
  {
    "slug": "instagram",
    "name": "Instagram",
    "format": "jpeg",
    "quality": 90,
    "scale": { "mode": "max_dim", "maxDimension": "width", "maxValue": 1080 },
    "exifMode": "keep_no_gps"
  },
  {
    "slug": "mastodon",
    "name": "Mastodon",
    "format": "jpeg",
    "quality": 85,
    "scale": { "mode": "max_dim", "maxDimension": "width", "maxValue": 1920 },
    "exifMode": "keep_no_gps"
  },
  {
    "slug": "website",
    "name": "Website",
    "format": "jpeg",
    "quality": 85,
    "scale": { "mode": "max_dim", "maxDimension": "width", "maxValue": 2400 },
    "exifMode": "strip"
  }
]
```

Fields:

| Field | Type | Description |
|---|---|---|
| `slug` | string | Machine identifier, used in XMP and output filenames. Immutable after creation. |
| `name` | string | Display name in the UI |
| `format` | `jpeg` \| `png` \| `webp` | Output format |
| `quality` | 1–100 | Compression quality (ignored for PNG) |
| `scale` | `ScaleOptions` | Reuses existing `media.ScaleOptions` struct |
| `exifMode` | `strip` \| `keep` \| `keep_no_gps` | Reuses existing `ExportOptions.ExifMode` |

The three entries above ship as built-in defaults. They can be edited or deleted by the user. New channels can be added freely.

### Output filename

```
<channel-slug>_<ISO-datetime-compact>_<original-basename>.<ext>
```

Examples:
```
instagram_20260425T143000Z_IMG_1234.jpg
mastodon_20260425T143000Z_paris.jpg
```

Compact ISO 8601 (`YYYYMMDDTHHmmssZ`) keeps filenames sortable and avoids colons on all platforms.

### Export destination

```
~/.unterlumen/libraries/<uuid>/channels/<slug>/
```

Created on first publish to that channel. No user configuration needed — the path is derived from the library and channel slug.

---

## Channel Management UI

A **Channels** settings screen accessible from the library header (settings icon or dedicated tab). Shows all configured channels in a list. Actions:

- **Edit** any channel: name, format, quality, scale mode/value, exif mode.
- **Delete** a channel (confirmation required if the channel has XMP records in the current library).
- **Add channel**: a form with the same fields, slug auto-derived from name (lowercased, spaces → hyphens), editable before save.

The slug is shown read-only once saved — it's used in XMP sidecars and output filenames, so it must not change after first publish.

Built-in channels (`instagram`, `mastodon`, `website`) are pre-populated on first run but treated as normal user data — fully editable and deletable.

---

## Publish Flow

### UI

1. Select one or more photos in the library grid.
2. Click **Publish** in the selection toolbar.
3. A modal opens with:
   - **Channel** — dropdown of configured channels
   - **Date/time** — defaults to now; editable (for recording past manual uploads)
   - Preview of output path and filename
4. Confirm → progress indicator for multi-photo publish.
5. Toast: "Published 3 photos to Instagram."

### What happens on confirm

For each selected photo:

1. Write/merge XMP sidecar next to the original.
2. Upsert `photo_meta` in the library DB (`published:<slug>` → ISO timestamp).
3. Run `media.ExportImage()` with the channel's preset options.
4. Write output to `~/.unterlumen/libraries/<uuid>/channels/<slug>/<filename>`.

### Handler extension point (Phase 2+)

A channel can optionally declare a `handler` field (e.g. `"handler": "mastodon_api"`). When a registered handler is present, Unterlumen calls it after the export step. The default (no handler) is always: write sidecar + write export file. Phase 1 implements only the default.

---

## API Routes

```
GET    /api/channels                      — list all channels
POST   /api/channels                      — create channel
PUT    /api/channels/:slug                — update channel (name, format, quality, scale, exifMode)
DELETE /api/channels/:slug                — delete channel

POST   /api/library/:id/publish           — publish selected photos
       Body: { photoIDs: ["sha256..."], channel: "instagram", publishedAt: "2026-04-25T14:30:00Z" }
       → writes sidecars + DB cache + export files
       → returns { exported: [{ photoID, outputPath }] }
```

Publication records are read via the existing `GET /api/library/:id/photo/:photoID/meta`.

---

## Implementation

### New files

| File | Purpose |
|---|---|
| `src/internal/media/xmp.go` | Read/write XMP sidecars (`encoding/xml`, no exiftool) |
| `src/internal/channels/model.go` | `Channel` struct, `ScaleOptions` reuse |
| `src/internal/channels/store.go` | Read/write `~/.unterlumen/channels.json` |
| `src/internal/api/channels/handler.go` | CRUD HTTP handlers |
| `src/web/js/channels.js` | Channel management UI |

### Modified files

| File | Change |
|---|---|
| `src/internal/library/index.go` | Read sidecars during re-index → populate `photo_meta` |
| `src/internal/api/library/handler.go` | Add `/publish` endpoint |
| `src/internal/api/routes.go` | Register channel and publish routes |
| `src/web/js/library-pane.js` | Publish button in selection toolbar |
| `src/web/js/library.js` | Publish modal; link to channel settings |
| `src/web/index.html` | Include channels.js |
| `CHANGELOG.md` | Entry |

### XMP implementation note

Go has no first-class XMP library. `src/internal/media/xmp.go` will use `encoding/xml` directly. The read path parses `ul:Publications` from existing sidecars (returns empty slice if none). The write path merges new entries, then serialises the full file — preserving any unrecognised XML namespaces present in an existing sidecar.

---

## Phase 1 scope

- Global channel CRUD (`channels.json`)
- Channel management UI
- Publish action: XMP sidecar + DB cache + export copy
- Re-index reads sidecars to restore DB cache
- Default channels: instagram, mastodon, website (pre-populated, editable)
- No upload handlers

## Phase 2+ scope

- Upload handlers: Mastodon API (OAuth2), custom webhook
- "Published to" badge on thumbnails in the library grid
- Filter library by publication status

---

## Acceptance Criteria

- [ ] `GET /api/channels` returns the three default channels on first run
- [ ] Channel CRUD: create, edit name/format/quality/scale/exifMode, delete
- [ ] Slug is immutable after creation; auto-derived from name on create
- [ ] `POST /api/library/:id/publish` writes XMP sidecar next to the original
- [ ] Sidecar contains correct `ul:Channel` and `ul:PublishedAt`
- [ ] Publishing a second time to the same channel appends a new entry
- [ ] Publishing to a different channel merges into the same sidecar without losing other namespaces
- [ ] `photo_meta` key `published:<slug>` is set after successful publish
- [ ] Export file written to `~/.unterlumen/libraries/<uuid>/channels/<slug>/` with correct filename
- [ ] Filename format: `<slug>_<YYYYMMDDTHHmmssZ>_<original-basename>.<ext>`
- [ ] Re-index restores `photo_meta` from sidecars after DB deletion
- [ ] Publish button appears in library selection toolbar; modal shows channel + date + output path preview
- [ ] Channel settings screen: list, edit, delete, add channel
- [ ] `go vet ./...` passes
- [ ] E2E: publish photo to instagram → check export file exists → delete DB → re-index → check `photo_meta` restored
