---
name: unterlumen-supercut
description: Make the silent social-media supercut of Unterlumen — a square MP4 of about 30 s, cut on the bars of 120 BPM so music can be laid on, showing the main features and the statistics (2D and 3D) with short captions. Use when the user asks for "the supercut", "a promo video", "a video for social media / Instagram / Mastodon", or to "redo the video" after a release or a visible change.
allowed-tools: Bash, Read, Edit, Write
---

# Supercut video

A square (1080 × 1080, 30 fps, H.264, no sound) film of Unterlumen for social
media, made by a script — never by screen recording by hand, never on the real
installation.

- `e2e/video/supercut.mjs` — runs on the same throwaway stage as the README
  pictures (`e2e/screenshots/stage.mjs`: a copy of `src/examples` on port 8097
  with a seeded library and galleries). It drives the app scene by scene in
  Chrome (WebGPU, for the 3D view) and records what Chrome paints through the
  DevTools screencast, **only while a scene moves**: each scene's `prepare` is
  not filmed, its `act` is, for `bars` bars of music (one by default).
- **Every cut falls on a bar** of 4/4 at `BPM` (120 → one bar = 2.0 s = 60
  frames). The owner lays music of that tempo on from 0 s. Change `BPM` in
  the script for another tempo; scenes then last a bar of that tempo.
- Captions are drawn into the page in the app's own type (`caption()`); the
  end card is a layer over the app (`endCard()`); the phone scene shows two
  iframes of the app at phone width side by side.
- The frames are written one by one — each scene exactly bars × bar length ×
  30 frames, each the last frame Chrome painted by then — and ffmpeg encodes
  them at 30 fps. Counting frames, not durations, keeps the cuts on the bars;
  ffmpeg's concat with durations drifted by over a second.
- Output: `e2e/video/out/unterlumen-supercut.mp4` (git-ignored).

## Make it

```bash
cd src && go build -o ../unterlumen . && cd ../e2e && npm run supercut
```

About 2 minutes. Needs ffmpeg, Google Chrome (the `chrome` channel) and a
network connection (map tiles). The script prints the scenes and "N cuts on
the bars of 120 BPM, F frames, S s"; the MP4 has exactly F frames.

## Check it — cheaply

Do not watch the film frame by frame. One contact sheet and the last frame
tell enough:

```bash
S=<scratchpad>; V=e2e/video/out/unterlumen-supercut.mp4
ffprobe -v error -show_entries format=duration -of csv=p=0 $V
# the middle frame of every bar: one picture per scene, the end card last
ffmpeg -v error -y -i $V -vf "select='eq(mod(n-30\,60)\,0)',scale=300:300,tile=5x3" -frames:v 1 -vsync vfr $S/contact.png
ffmpeg -v error -y -sseof -0.5 -i $V -frames:v 1 -vf scale=540:540 $S/end.png
```

Read both images. Check: every scene has its caption and real content
(thumbnails and map tiles loaded); no temporary path (`/private/var/…`,
`unterlumen-shots-`) — `showHomePath` runs on every frame, iframes included;
each bar shows its own scene (a scene bleeding into the next bar means a cut
is off the beat); the last frame is the end card; length about 30 s.

## Change it

- **Scenes** are the `scenes` array: `caption`, `bars`, `prepare(page)`,
  `act(page)`; a 3D view of a topic is `stage3D(topic, title, caption)`.
  Keep `act` within its bars (a longer motion is cut off; a shorter one
  leaves the caption standing, which is fine). Reuse the shot
  helpers (`ready`, `settle`, `openFolder`, `openLibrary` from
  `e2e/screenshots/shots.mjs`).
- **Selectors** that also exist in the info panel's mini map or in hidden
  places need scoping (`.map-pane .maplibregl-ctrl-zoom-in`).
- **A scene that does not move** paints no frame; `rec.start` flips a 1-pixel
  element's colour so Chrome paints one.
- **Length**: the owner wants about 30 s, with the statistics shown at length
  (2D and 3D); captions short, English, sentence case, no exclamation marks
  (CLAUDE.md copy rules).

## After

Tell the owner the path, the length and what each scene shows. The film is a
build product: do not commit it. The script and the skill are code: commit
those like any change.
