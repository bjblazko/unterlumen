# Slideshow

*Last modified: 2026-04-10*

## Summary

A timed full-screen slideshow for reviewing or presenting photos, triggered from the browse toolbar. Operates on the current selection or, if nothing is selected, all images in the current grid.

## Details

A **Slideshow** button (play-bar icon, labelled "Slideshow") appears in the browse controls bar to the right of the Tools dropdown. Clicking it opens an options dialog before playback begins.

### Options dialog

| Option | Values | Default |
|--------|--------|---------|
| Delay | 1–60 s (slider) | 5 s |
| Transition | Fade, Slide, Zoom, Instant | Fade |
| Display | Single, Ken Burns, 2-up, 4-up | Single |
| Audio | None, local file, local folder | None |

**Display modes:**
- **Single** — one image per frame, centered, `object-fit: contain`
- **Ken Burns** — single image slowly pans and zooms over its display duration; alternates direction on consecutive frames
- **2-up** — two images side by side; advances cursor by 2 each frame
- **4-up** — four images in a 2×2 grid; advances cursor by 4 each frame; pads with the last image if fewer remain

**Audio:**
- **File…** — opens a file picker for a single audio track; plays on start
- **Folder…** — opens a directory picker for a folder of audio files; only audio MIME types are used; shuffled and played in sequence, looping back after the last track
- No online/remote music library (requires API key, network reliability, and a backend proxy — outside scope of the local-first design)

**Transitions** are implemented as CSS keyframe animations applied to the incoming and outgoing `.ss-frame` elements. Ken Burns frames always fade in regardless of the selected transition option (the pan/zoom movement is the visual transition).

### Player controls

A semi-transparent HUD strip at the bottom provides: Prev, Pause/Play, Next, frame counter (`N / total`), and Close buttons. The HUD autohides after 3 seconds of inactivity and reappears on mouse movement.

**Keyboard shortcuts in the player:**
- `Space` — pause / resume
- `←` — previous frame
- `→` — skip to next frame immediately
- `Esc` — close player, return to browse grid

Closing the player restores the browse grid with the same scroll position.

## Acceptance Criteria

- [ ] "Slideshow" button appears between Tools dropdown and status bar in browse controls
- [ ] Button does nothing (no dialog) when current folder contains no images
- [ ] Options dialog shows the count of images to be played (selected or all)
- [ ] All four transitions animate correctly with no ghost frames
- [ ] Ken Burns pan/zoom fills the full delay duration; odd/even frames pan in opposite directions
- [ ] 2-up advances two images per frame; 4-up advances four; cursor wraps at end
- [ ] 4-up pads gracefully when fewer than 4 images remain at end
- [ ] Single local audio file plays from start of slideshow
- [ ] Audio folder: files are shuffled, loop continuously through all tracks
- [ ] HUD autohides after 3 s, reappears on mouse movement
- [ ] Esc key and Close button both close the player and restore browse state
- [ ] Dark mode: options modal respects theme variables; player overlay is always black
