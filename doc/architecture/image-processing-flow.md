# Image Processing Flow: Thumbnails, Full Images, ffmpeg, and Caching

*Last modified: 2026-03-25*

This document describes the exact runtime behavior for every code path that
produces image data for the browser — from the HTTP request to the bytes on
the wire. It covers thumbnail generation, full-image serving, when and how
ffmpeg is called, and how the disk cache works.

---

## 1. Thumbnail requests (`GET /thumbnail?path=...`)

The handler lives in `internal/api/thumbnail.go`. `thumbnailMaxDim` is 300 px.

### 1a. Non-HEIF files (JPEG, PNG, GIF, WebP)

```mermaid
flowchart TD
    A[Request] --> B["ExtractOrientation()<br/>read EXIF orientation tag (1–8)<br/><i>opens file, calls goexif — no ffmpeg</i>"]
    B --> C["ExtractThumbnail() — fast path"]
    C --> C1["opens file, calls goexif"]
    C1 --> C2["reads JpegThumbnail() from EXIF"]
    C2 --> C3["validates aspect ratio<br/>(rejects if >10% mismatch)"]
    C3 --> C4["applies orientation rotation if needed (in-memory)"]
    C4 --> C5["returns embedded JPEG bytes<br/><i>no ffmpeg, no disk cache</i>"]
    C5 --> SERVE1([Serve, done])

    C3 -- "no EXIF thumbnail" --> D["GenerateThumbnail() — fallback"]
    D --> D1["opens file, calls image.Decode()"]
    D1 --> D2["applies orientation rotation if needed (in-memory)"]
    D2 --> D3{"image fits<br/>within 300 px?"}
    D3 -- "yes" --> D4["return raw file bytes (no resize)"]
    D3 -- "no" --> D5["nearest-neighbor resize,<br/>re-encode as JPEG/PNG"]
    D4 --> SERVE2([Serve, done])
    D5 --> SERVE2
```

**No ffmpeg is ever called for non-HEIF thumbnails.**
**Nothing is written to disk** (no cache, consistent with ADR-0002).

### 1b. HEIF/HEIC/HIF files

```mermaid
flowchart TD
    A[Request] --> B["ExtractHEIFPreview()"]
    B --> C["compute cache key:<br/>SHA-256( path + '|' + mtime + '|preview-v3' )[:12].hex + '.jpg'"]
    C --> D{"readCache()<br/>hit?"}
    D -- "hit" --> RESIZE

    D -- "miss" --> E["extractBestJPEG()"]
    E --> F["ffmpegProbe()<br/><code>ffmpeg -i &lt;path&gt;</code><br/><i>reads stderr only — discovers embedded JPEG streams</i>"]
    F --> G{"large MJPEG<br/>stream found?<br/>(not 160×120)"}

    G -- "yes" --> H["ffmpegRun() stream copy<br/><code>ffmpeg -i &lt;path&gt; -map 0:&lt;idx&gt; -c copy -f image2pipe pipe:1</code><br/><i>no re-encoding, very fast</i>"]
    G -- "no" --> I["sipsConvert()<br/><code>sips -s format jpeg -s formatOptions 92 &lt;path&gt; --out &lt;tmp&gt;</code><br/><i>Apple native HEIF decoder; temp file deleted after read</i>"]
    I --> J{"sips<br/>succeeded?"}
    J -- "yes" --> ORIENT
    J -- "no" --> K["ffmpegRun() HEVC decode<br/><code>ffmpeg -i &lt;path&gt; -f image2pipe -vcodec mjpeg -q:v 2 -frames:v 1 pipe:1</code><br/><i>slowest path — decodes HEVC to JPEG</i>"]
    H --> ORIENT
    K --> ORIENT

    ORIENT["ExtractHEIFOrientation()<br/><i>reads irot box from ISOBMFF container (no ffmpeg)</i>"]
    ORIENT --> ORI2["applyOrientationJPEG()<br/><i>rotate in-memory, re-encode at quality 80</i>"]
    ORI2 --> CACHE["writeCache()<br/><i>write result to disk cache</i>"]
    CACHE --> RESIZE

    RESIZE["ResizeJPEGBytes()<br/><i>resize to ≤300 px if needed (in-memory)</i>"]
    RESIZE --> SERVE([Serve])
```

---

## 2. Full-image requests (`GET /image?path=...`)

The handler lives in `internal/api/image.go`.

### 2a. Non-HEIF files

```mermaid
flowchart TD
    A[Request] --> B["http.ServeFile()<br/><i>raw file served directly from disk<br/>no ffmpeg, no decode, no cache</i>"]
```

### 2b. HEIF/HEIC/HIF files

```mermaid
flowchart TD
    A[Request] --> B["ConvertHEIFToJPEG()"]
    B --> C["compute cache key:<br/>SHA-256( path + '|' + mtime + '|full-v3' )[:12].hex + '.jpg'"]
    C --> D{"readCache()<br/>hit?"}
    D -- "hit" --> SERVE

    D -- "miss" --> E["extractBestJPEG()<br/><i>identical flow to §1b above</i>"]
    E --> F["ffmpegProbe()"]
    F --> G{"embedded JPEG<br/>found?"}
    G -- "yes" --> H["ffmpegRun() stream copy"]
    G -- "no" --> I["sipsConvert()"]
    I --> J{"sips<br/>succeeded?"}
    J -- "yes" --> ORIENT
    J -- "no" --> K["ffmpegRun() HEVC decode"]
    H --> ORIENT
    K --> ORIENT

    ORIENT["ExtractHEIFOrientation()"]
    ORIENT --> ORI2["applyOrientationJPEG()<br/><i>re-encode at quality 92 for full image</i>"]
    ORI2 --> CACHE["writeCache()"]
    CACHE --> SERVE

    SERVE(["http.ServeContent()"])
```

Note: thumbnails and full images use **different cache keys** (`preview-v3` vs
`full-v3`), so the first full-image view of a HEIF file is always a cache miss
even if the thumbnail was already generated.

---

## 3. ffmpeg invocations — complete list

| Call site | Command | When | Notes |
|---|---|---|---|
| `CheckFFmpeg()` | `ffmpeg -decoders` | Once at startup | Checks availability + HEVC decoder support; result cached for process lifetime |
| `ffmpegProbe()` | `ffmpeg -i <path>` | Per HEIF request (cache miss) | Reads stderr; always exits non-zero; used to detect embedded JPEG streams |
| `ffmpegRun()` stream copy | `ffmpeg -i <path> -map 0:<idx> -c copy -f image2pipe pipe:1` | Per HEIF request (cache miss, embedded JPEG found) | Fast; no re-encode |
| `ffmpegRun()` HEVC decode | `ffmpeg -i <path> -f image2pipe -vcodec mjpeg -q:v 2 -frames:v 1 pipe:1` | Per HEIF request (cache miss, no embedded JPEG, sips also failed) | Slowest path |

`sips` (macOS only) is tried between the two `ffmpegRun` calls. It writes to
a unique OS temp file that is deleted immediately after reading.

**No ffmpeg is called on a cache hit.** After the first successful conversion,
subsequent requests for the same file (same mtime) are served entirely from
the disk cache with no subprocess invocation.

---

## 4. Disk cache

### Location

```
os.TempDir() + "/unterlumen-cache/"
```

On macOS this is typically `/var/folders/<user-specific>/T/unterlumen-cache/`.
On Linux: `/tmp/unterlumen-cache/`.

The directory is created on first use with permissions `0700`.

### Cache key

```
SHA-256( absolutePath + "|" + mtime + "|" + purpose )[:12]  →  hex string + ".jpg"
```

- `purpose` is either `"full-v3"` (full-resolution image) or `"preview-v3"` (thumbnail source before resize)
- Including `mtime` means the cache is effectively **content-addressed by modification time**: if the source file changes, a new entry is created automatically (the old entry is orphaned, not deleted)

### Cache scope

Only HEIF conversions are cached to disk. Non-HEIF thumbnails and full images
are never written to disk (see §1a and §2a).

### Cache lifetime and cleanup

| Aspect | Behavior |
|---|---|
| Survives process restart | Yes — files remain in the OS temp directory |
| Invalidated on file change | Yes — mtime change produces a new key; old entry becomes orphaned |
| Explicit cleanup | No — unterlumen does not delete cache files |
| OS-managed cleanup | Yes — macOS and Linux periodically purge stale temp files |
| Maximum size | Unbounded; each HEIF file produces up to two `.jpg` files (`full-v3` + `preview-v3`) |

There is no housekeeping, LRU eviction, or TTL logic within unterlumen.
Cleanup is delegated entirely to the operating system's temp-directory policy.

---

## 5. Summary table

| Format | Thumbnail source | Full image | ffmpeg? | Disk cache? |
|---|---|---|---|---|
| JPEG (with EXIF thumb) | EXIF embedded thumbnail | Raw file served | No | No |
| JPEG (no EXIF thumb) | Server-side decode + resize | Raw file served | No | No |
| PNG / GIF / WebP | Server-side decode + resize | Raw file served | No | No |
| HEIF/HEIC/HIF | ffmpeg / sips conversion (cached) | ffmpeg / sips conversion (cached) | Yes (on cache miss) | Yes (`$TMPDIR/unterlumen-cache/`) |

---

## 6. Related decisions

- **ADR-0002** — No write side-effects in the photo directory; the temp cache satisfies this by writing to `$TMPDIR` instead
- **ADR-0003** — EXIF embedded thumbnails as primary source for JPEG
- **ADR-0004** — HEIF/HEIC support via ffmpeg shell-out (updated: disk cache added)
