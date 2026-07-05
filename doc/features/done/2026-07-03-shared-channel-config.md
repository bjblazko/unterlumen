# Shared Channel Config Across Installations

*Last modified: 2026-07-03*

## Summary

Users who run more than one Unterlumen installation against the same photo folders
(e.g. Docker on a NAS plus a native Mac install mounting the same folders over the
network) previously ended up with unrelated `channels.json` files on each machine,
since channel config lived under the per-machine `-lib-dir`. The "photo was published"
fact was already portable via the XMP sidecar, but the channel *definitions*
themselves (name, format, credentials) were not, so features that list channels
(notably the channel filter in library search) showed the wrong set on the
non-publishing installation.

## Details

`channels.NewStore` now takes two directories instead of one: `configDir` (where
`channels.json` is read/written) and `outputBaseDir` (the default base for generated
export output — site/gallery HTML, avatars, logos — unchanged in location). A new
`-channels-dir` / `UNTERLUMEN_CHANNELS_DIR` flag sets `configDir`; when unset it
defaults to `-lib-dir`, so existing single-installation setups are unaffected.

Users with a multi-installation setup point `-channels-dir` at the same directory
(reachable by every installation, e.g. a folder on a NAS-hosted share) on all of
them, while `-lib-dir` — and therefore the SQLite database, thumbnails, and search
index — stays independent and local per machine.

Export output deliberately stays anchored to `-lib-dir` rather than following the
config file, since generated site/gallery output is per-installation working output
(e.g. what a user then SCPs to a destination), not shared config.

See [ADR-0023](../../architecture/adr/0023-shared-channel-config-directory.md) and
the README section "Sharing channel config across installations".

## Acceptance Criteria

- [x] `-channels-dir` / `UNTERLUMEN_CHANNELS_DIR` overrides where `channels.json` is
      read/written, independent of `-lib-dir`
- [x] When unset, behavior is identical to before (config and output both under
      `-lib-dir`)
- [x] A channel's explicit `OutputPath` override still takes precedence over the
      computed output directory
- [x] `go test ./internal/channels/...` covers the config/output directory split
