# Image Processing Flow: Thumbnails, Full Images, ffmpeg, and Caching

*Last modified: 2026-02-27*

This document describes the exact runtime behavior for every code path that
produces image data for the browser — from the HTTP request to the bytes on
the wire. It covers thumbnail generation, full-image serving, when and how
ffmpeg is called, and how the disk cache works.

---

## 1. Thumbnail requests (`GET /thumbnail?path=...`)

The handler lives in `internal/api/thumbnail.go`. `thumbnailMaxDim` is 300 px.

### 1a. Non-HEIF files (JPEG, PNG, GIF, WebP)

```
Request
  │
  ├─ ExtractOrientation()            read EXIF orientation tag (1–8)
  │    └─ opens file, calls goexif   no ffmpeg
  │
  ├─ ExtractThumbnail()              [fast path]
  │    ├─ opens file, calls goexif
  │    ├─ reads JpegThumbnail() from EXIF
  │    ├─ validates aspect ratio (rejects if >10% mismatch)
  │    ├─ applies orientation rotation if needed (in-memory)
  │    └─ returns embedded JPEG bytes  ──► serve, done
  │         (no ffmpeg, no disk cache)
  │
  └─ GenerateThumbnail()             [fallback — no EXIF thumbnail]
       ├─ opens file, calls image.Decode()
       ├─ applies orientation rotation if needed (in-memory)
       ├─ if image fits within 300 px: return raw file bytes (no resize)
       └─ else: nearest-neighbor resize, re-encode as JPEG/PNG
            ──► serve, done
            (no ffmpeg, no disk cache)
```

**No ffmpeg is ever called for non-HEIF thumbnails.**
**Nothing is written to disk** (no cache, consistent with ADR-0002).

### 1b. HEIF/HEIC/HIF files

```
Request
  │
  ├─ ExtractHEIFPreview()
  │    ├─ compute cache key: SHA-256( path + "|" + mtime + "|preview-v3" )[:12].hex + ".jpg"
  │    ├─ readCache()  →  hit? ──► return cached bytes, skip to ResizeJPEGBytes
  │    │
  │    └─ cache miss → extractBestJPEG()
  │         ├─ ffmpegProbe()           runs: ffmpeg -i <path>
  │         │    └─ reads stderr only  (ffmpeg always exits non-zero without output file)
  │         │       result used to discover embedded JPEG streams
  │         │
  │         ├─ if large MJPEG stream found (not 160×120):
  │         │    ffmpegRun()           runs: ffmpeg -i <path> -map 0:<idx> -c copy -f image2pipe pipe:1
  │         │    └─ stream-copy: no re-encoding, very fast  ──► use this JPEG
  │         │
  │         ├─ else: sipsConvert()     runs: sips -s format jpeg -s formatOptions 92 <path> --out <tmp>
  │         │    ├─ writes to a unique OS temp file (immediately deleted after read)
  │         │    └─ uses Apple's native HEIF decoder (correct for multi-tile HEIF)
  │         │         ──► use this JPEG if successful
  │         │
  │         └─ else: ffmpegRun()       runs: ffmpeg -i <path> -f image2pipe -vcodec mjpeg -q:v 2 -frames:v 1 pipe:1
  │              └─ decodes HEVC to JPEG (slowest, for HEIF without embedded previews)
  │
  │    ├─ ExtractHEIFOrientation()    reads irot box from ISOBMFF container (no ffmpeg)
  │    ├─ applyOrientationJPEG()      rotate in-memory if needed, re-encode at quality 80
  │    └─ writeCache()                write result to disk cache
  │
  └─ ResizeJPEGBytes()                resize to ≤300 px if needed (in-memory)
       ──► serve
```

---

## 2. Full-image requests (`GET /image?path=...`)

The handler lives in `internal/api/image.go`.

### 2a. Non-HEIF files

```
Request
  └─ http.ServeFile()    raw file served directly from disk
       (no ffmpeg, no decode, no cache)
```

### 2b. HEIF/HEIC/HIF files

```
Request
  └─ ConvertHEIFToJPEG()
       ├─ compute cache key: SHA-256( path + "|" + mtime + "|full-v3" )[:12].hex + ".jpg"
       ├─ readCache()  →  hit? ──► return cached bytes
       │
       └─ cache miss → extractBestJPEG()    (identical flow to §1b above)
            ├─ ffmpegProbe()
            ├─ ffmpegRun() stream copy  OR
            ├─ sipsConvert()            OR
            └─ ffmpegRun() HEVC decode
       ├─ ExtractHEIFOrientation()
       ├─ applyOrientationJPEG()  (re-encode at quality 92 for full image)
       └─ writeCache()
            ──► http.ServeContent()
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
