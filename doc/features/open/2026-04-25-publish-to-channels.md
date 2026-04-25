*Last modified: 2026-04-25 (rev 2)*

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
            <ul:Account>personal</ul:Account>
            <ul:PostID>a3f9c12e8b04</ul:PostID>
            <ul:PublishedAt>2026-04-25T14:30:00Z</ul:PublishedAt>
          </rdf:li>
        </rdf:Bag>
      </ul:Publications>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
```

Fields per entry:

| Field | Type | Required | Description |
|---|---|---|---|
| `ul:Channel` | string | yes | Channel slug matching the global channel definition |
| `ul:Account` | string | no | Account ID within the channel (empty when channel has no sub-accounts) |
| `ul:PostID` | string | no | Shared 24-hex-char ID linking all photos published in the same action |
| `ul:PublishedAt` | ISO 8601 UTC | yes | When the publish action was recorded (default: now; user can back-date) |

Multiple events for the same channel are allowed (re-posts). `Account` and `PostID` are omitted from the sidecar when not applicable. Write is always merge-safe: existing sidecar namespaces (darktable, Lightroom, etc.) are preserved.

### Library DB cache (secondary, searchable)

After writing the sidecar, upsert into the existing `photo_meta` table:

| key | value |
|---|---|
| `published:instagram` | `2026-04-25T14:30:00Z` (most recent timestamp) |
| `published:instagram:account` | `personal` (most recent account used) |
| `published:instagram:postid` | `a3f9c12e8b04` (most recent post ID) |

Enables fast filter queries without XML parsing. Rebuilt automatically during re-index. The `account` and `postid` sub-keys are only written when present.

### Re-index integration

During re-index (`src/internal/library/index.go`), after EXIF extraction:

1. Check for a `.xmp` sidecar alongside the photo.
2. If found, parse `ul:Publications` from the `https://unterlumen.app/xmp/1.0/` namespace.
3. For each channel seen in the sidecar, upsert the latest `published:<channel>`, `published:<channel>:account`, and `published:<channel>:postid` keys into `photo_meta`.

Drop the DB, run re-index, full publication history (including accounts and post groupings) is restored.

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
    "exifMode": "keep_no_gps",
    "accounts": [
      { "id": "personal", "label": "Personal",    "config": { "note": "manual upload" } },
      { "id": "work",     "label": "Work account", "config": { "note": "manual upload" } }
    ]
  },
  {
    "slug": "mastodon",
    "name": "Mastodon",
    "format": "jpeg",
    "quality": 85,
    "scale": { "mode": "max_dim", "maxDimension": "width", "maxValue": 1920 },
    "exifMode": "keep_no_gps",
    "accounts": [
      { "id": "home", "label": "mastodon.social", "config": { "instance": "https://mastodon.social", "token": "…" } }
    ]
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

| Field | Type | Required | Description |
|---|---|---|---|
| `slug` | string | yes | Machine identifier — used in XMP, meta keys, and output filenames. Immutable after first publish. |
| `name` | string | yes | Display name in the UI |
| `format` | `jpeg` \| `png` \| `webp` | yes | Output format |
| `quality` | 1–100 | yes | Compression quality (ignored for PNG) |
| `scale` | `ScaleOptions` | yes | Reuses existing `media.ScaleOptions` struct |
| `exifMode` | `strip` \| `keep` \| `keep_no_gps` | yes | Reuses existing `ExportOptions.ExifMode` |
| `handler` | string | no | Handler identifier for upload automation (e.g. `mastodon`, `scp`). Empty = export-only default. |
| `handlerConfig` | `map[string]string` | no | Free-form key→value config for the handler (instance URL, SSH key path, etc.) |
| `accounts` | `[]Account` | no | Named sub-accounts (e.g. two Mastodon logins). Empty = single anonymous destination. |

**Account fields:**

| Field | Type | Description |
|---|---|---|
| `id` | string | Identifier used in XMP and meta keys. Immutable after first publish. |
| `label` | string | Display name in dropdowns |
| `config` | `map[string]string` | Account-specific handler config (tokens, credentials, notes) |

The three built-in channels (`instagram`, `mastodon`, `website`) are pre-populated on first run as normal user data — fully editable and deletable.

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

A **Channels** settings modal accessible via the "Channels" button in the library header. Shows all configured channels in a list. Actions:

- **Edit** any channel: name, format, quality, scale, exif mode, handler, handlerConfig, accounts.
- **Delete** a channel.
- **Add channel**: slug auto-derived from name (lowercased, spaces → hyphens), editable before save.

The slug is shown read-only once saved. The **handlerConfig** and each **account's config** are edited via a dynamic key→value editor (add/remove rows). Accounts have an ID (immutable), a display label, and their own key→value config map.

When a channel has multiple accounts, the publish modal shows an account dropdown so the user picks which account to publish to. One publish action always targets one account.

---

## Publish Flow

### UI

1. Select one or more photos in the library grid.
2. Click **Publish…** in the library header toolbar (enabled when ≥1 photo selected).
3. A modal opens with:
   - **Channel** — dropdown of configured channels
   - **Account** — dropdown of the channel's accounts (hidden when channel has no sub-accounts)
   - **Date/time** — defaults to now; editable (for recording past manual uploads)
   - Export preview line (format, quality, scale)
   - When multiple photos: note that they will be grouped as one post (shared PostID)
4. Confirm → resolves filesystem paths to photo IDs, calls publish endpoint.
5. Toast: "Published 3 photos to instagram."

### What happens on confirm

The backend generates one shared **PostID** (24-char random hex) for the entire batch.

For each selected photo:

1. Write/merge XMP sidecar: `ul:Channel`, `ul:Account` (if set), `ul:PostID`, `ul:PublishedAt`.
2. Upsert `photo_meta`:
   - `published:<slug>` → timestamp
   - `published:<slug>:account` → account ID (if set)
   - `published:<slug>:postid` → post ID
3. Run `media.ExportImage()` with the channel's preset options.
4. Write output to `~/.unterlumen/libraries/<uuid>/channels/<slug>/<filename>`.

The grouped-post semantics: all photos in the batch share the same `PostID`, making them linkable (e.g. for Instagram carousel). Each photo's sidecar is independent — the grouping is visible by comparing `PostID` values across photos.

### Handler extension point (Phase 2+)

A channel's `handler` field names a registered handler (e.g. `"mastodon"`, `"scp"`). When present, Unterlumen calls it after the export step, passing the channel config, account config, and the exported file paths. The default (empty handler) is always: write sidecar + write export file. Phase 1 implements only the default.

---

## API Routes

```
GET    /api/channels/              — list all channels
POST   /api/channels/              — create channel
PUT    /api/channels/{slug}        — update channel (all fields incl. accounts, handlerConfig)
DELETE /api/channels/{slug}        — delete channel

POST   /api/library/{id}/publish   — publish selected photos
       Body: {
         photoIDs:    ["sha256..."],
         channel:     "instagram",
         account:     "personal",          // optional
         publishedAt: "2026-04-25T14:30:00Z"  // optional, defaults to now
       }
       → writes sidecars + DB cache + export files
       → returns { postID: "a3f9c12e8b04", results: [{ photoID, outputPath, error? }] }
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
