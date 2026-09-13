# Streamlined Publish Workflow

*Last modified: 2026-09-06*

## Summary

Replaces the scattered Build / Channels-actions / Deploy / Published surfaces
with a two-phase collect → publish model: collecting photos into a channel is
lightweight and deferred (no export, no HTML, until you're ready); publishing
is one dialog that generates the artifact, lets you review it, and deploys it.

## Details

- "Add to channel…" (library toolbar) replaces "Build…" — adds selected
  photos to a channel's pending draft. Nothing is written to disk.
- The Published tab shows Draft / Generated / Live / Live · N pending status
  per gallery/album, with a Publish action that opens the new 4-step dialog:
  review pending photos → Generate (export + build HTML) → review the
  artifact (copy path / open folder / open in browser) → Deploy (rsync
  channels only).
- The Channels dialog is settings-only now, with a status line linking into
  the Published tab instead of per-row action buttons.
- The Info Panel's Publications card shows both pending and published state
  per channel.

## Acceptance Criteria

- [x] Selecting photos and clicking "Add to channel…" creates/updates a draft
      with no files written to the channel's output directory.
- [x] The Published tab shows a Draft-status row for a channel with only
      pending photos, and a "Live · N pending" row for a channel with both
      generated and newly-collected photos.
- [x] The Publish dialog's Generate step produces the same gallery/site
      output as the old Build action did, for both single-gallery and
      multi-album site channels.
- [x] The Publish dialog's review step offers copy-path / open-folder for
      plain-export channels, plus open-in-browser for gallery/site channels.
- [x] Deploy only appears for channels with an rsync handler, and only after
      Generate has run.
- [x] The Channels dialog has no Rebuild/Albums/Deploy/Published buttons left
      on channel rows.
- [x] The Info Panel shows a "pending" Publications card for a collected but
      not-yet-generated photo, and a "published" card after Generate.
