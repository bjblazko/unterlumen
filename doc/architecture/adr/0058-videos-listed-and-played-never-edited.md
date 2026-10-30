# ADR-0058: Videos are listed and played, never edited

*Last modified: 2026-10-30*

## Status

Accepted.

## Context

The owner removed Immich from the home server on 2026-10-29. Unterlumen is
now the only tool for the photo collection, and the collection holds videos:
clips from the cameras and iPhones, and the clips of Live Photos. Until now
Unterlumen saw only images; a folder of videos looked empty.

The owner asked for videos to be listed, filterable (shown or hidden),
watchable, with a small moving preview, and explicitly not editable or
renamable. Whatever could be done with the tools Unterlumen already uses was
preferred.

## Decision

1. **The browser plays; nothing is transcoded.** MP4, M4V, MOV and WebM are
   served with their type through `http.ServeFile`, which answers range
   requests, so `<video>` seeks. Every browser plays H.264; Safari, and Chrome
   and Firefox on a Mac, play the HEVC of an iPhone. MKV, AVI, MTS, M2TS and
   3GP are listed and shown with their still frame, but only handed over as a
   download (the rule of ADR-0057 that only known media types are served
   inline). The viewer says in a sentence that the browser cannot play one,
   with the download beside it.
2. **ffmpeg makes the pictures.** ffmpeg is already a helper program
   (HEIF). A still frame one second in (`media.VideoPoster`) is a video's
   thumbnail, in the cache for Folders and in the library's `thumbs` like a
   photo's. ffmpeg turns frames upright on its own; no orientation is read.
3. **A moving preview is a three-second clip, made on the first hover.**
   320 pixels wide, 15 frames a second, no sound, H.264 with `+faststart`,
   from a tenth of the way in (`media.VideoClip`). It is made the first time
   the pointer rests on the tile, through the same limit as thumbnail work,
   and kept in the cache's thumbnails folder under the video's path, for
   Folders and libraries alike (`/api/clip`, `/api/library/{id}/photo/{photo}/clip`).
   It plays muted and looped while the pointer stays on the tile and fades in
   only once it plays. A phone has no hover and shows the still frame. The
   clip is encoded by the user's own ffmpeg; Unterlumen carries no codec.
4. **Metadata comes from `ffmpeg -i`.** `media.ExtractVideoMeta` reads
   length, date, place (ISO 6709), the iPhone's make and model, and the shown
   size into the same `ExifData` a photo has, so the Timeline, the Map and the
   filter treat both alike. An iPhone's `com.apple.quicktime.creationdate`
   (local time with offset) wins; a camera's `creation_time` is UTC and is
   given in this machine's local time. ffprobe is not used: the macOS
   installer fetches ffmpeg alone.
5. **A Live Photo is its photo.** A `.mov` with the same name as a HEIC or
   JPEG in the same folder is its clip: not listed in Folders, not indexed in
   a library (`withoutLiveClips`). The photo carries "Live"; the viewer plays
   the clip over it once (`/api/library/{id}/photo/{photo}/live`). The pair
   is found by reading the folder when it is listed, so nothing new is stored
   in the index.
6. **A video is a row of `photos`.** Its ID is its file's SHA-256, as any
   photo's (ADR-0052); `ext` tells the kind. The shared filter has a kind
   (`Filter.Kind`, `kind=photo|video`), a set through `photos_ext_idx` and
   never a correlated subquery (ADR-0045). The library filter offers it as a
   row of its own at its top, Kind: Photos and videos · Photos · Videos
   (0.21.1; a chip under More filters in 0.21.0 was not found); the shared
   filter of Map, Statistics and the Timeline as the same three choices.
7. **Nothing changes a video.** Crop, set location, rename, export, publish
   and Add to gallery are not offered for a video, and refuse one if it comes
   anyway (`media.ErrVideoNotExported`, `handleToolInvoke`). Faces and Things
   do not look at videos (`notVideo` in their queues). Mark for deletion,
   Download and Show in Organize work as for a photo. The slideshow shows
   photos only.
8. **Videos shown or hidden** is one switch, Videos, in the ⋯ of Folders and
   of a library, remembered per browser. Hidden, they leave the grids, the
   counts and the library filter's results (which then ask for `kind=photo`)
   — unless the filter's Kind asks for videos, which overrules it.

## Consequences

- The first hover on a long 4K HEVC video waits for its clip: 6 to 16 seconds
  on a Mac, longer on the NAS. The still frame stays meanwhile, and later
  hovers come from the cache.
- A large video is read once in full when a library first meets it, for its
  ID — as large photos are.
- A library's card and a folder's head count photos and videos apart
  ("1 721 photos · 12 videos"); `photoCount` stays the count of both, with
  `videoCount` beside it.
- A video's place is in its row (`exif_json.latitude`) for the Map, not in
  `exif_index`; the tile's GPS badge reads it from there.
- **Patents.** H.264 is a patented codec. Unterlumen carries no encoder: on
  a Mac or Windows the user's own FFmpeg encodes the clips. The container
  image ships Debian's FFmpeg with libx264, as it already did for HEIF; the
  owner chose on 2026-10-30 to keep H.264, which every browser plays, and to
  record this rather than switch to VP9/WebM. The risk is low for a
  non-commercial project (patent pools charge on sales); VP9 stays the way
  out should that change.
- The tile clip loops while it is hovered. Recorded in ADR-0030 as a
  deviation: it is not ambient motion, it starts and ends with the pointer.
