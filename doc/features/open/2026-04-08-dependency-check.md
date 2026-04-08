# Dependency Check

*Last modified: 2026-04-08*

## Summary

Add a "Check dependencies" entry to the Settings menu that opens a modal listing the status of all required external tools. Missing or misconfigured tools are shown with platform-specific install instructions.

## Details

The app relies on three external tools whose absence causes silent failures:

- **ffmpeg** — HEIF/HEIC display and WebP export
- **exiftool** — GPS metadata editing and EXIF stripping on export
- **sips** — macOS fallback for HEIF conversion (should always be present on macOS)

The modal fetches tool status from the existing `/api/tools/check` endpoint (extended to include all three tools and the current platform). Each dependency row shows:

- A checkmark (green) or warning triangle (orange) status icon
- Tool name and one-line description
- If unavailable: a plain-language explanation of the impact plus a platform-specific install command

The `/api/tools/check` response is extended to:
```json
{
  "platform": "darwin",
  "ffmpeg": { "available": true, "heifSupport": true },
  "exiftool": { "available": false },
  "sips": { "available": true }
}
```

## Acceptance Criteria

- [ ] Settings menu contains a "Check dependencies" button
- [ ] Clicking it opens a modal listing ffmpeg, exiftool, and (on macOS) sips
- [ ] Available tools show a green checkmark
- [ ] Missing or broken tools show an orange warning with explanation and install command
- [ ] ffmpeg installed without HEVC support shows a specific warning (not just "not installed")
- [ ] Install commands are correct for the current platform (macOS, Linux, Windows)
- [ ] sips row is only shown on macOS
- [ ] Modal closes on Escape, overlay click, or Close button
