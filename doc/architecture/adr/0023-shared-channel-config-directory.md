# ADR-0023: Shared Channel Config Directory

*Last modified: 2026-07-03*

## Status

Accepted

## Context

Some users run more than one Unterlumen installation against the same photo folders — for example, a Docker instance on a NAS that is also the file server, plus a native install on a Mac that mounts the same folders over the network. This is a deliberate setup: it gives the Mac fast local thumbnails and a search index even when the NAS is unreachable, since `-lib-dir` (SQLite database, thumbnails, EXIF index) is per-machine by design.

Publishing to a channel writes two things:
- A `ul:Publications` entry in the photo's `.xmp` sidecar (`AppendPublication`, `src/internal/media/xmp.go`) — this lives next to the photo file, so it is already visible to every installation sharing that photo folder, and each install's re-indexer reads it back into its own local `photo_meta` cache.
- The channel *definition* itself (slug, name, format, credentials, output settings) in `channels.json`, historically stored under `{-lib-dir}/channels.json` (ADR-0016).

Because `channels.json` lived under the per-machine `-lib-dir`, two installations pointed at the same photo folders ended up with unrelated channel lists. The photo-level "published" fact rendered fine (the info panel's Publications card only needs the slug, not the channel definition), but anything that needs the actual channel list — notably the channel filter in library search (`src/web/js/library-filter.js`) — showed the wrong, install-local set of channels instead of the ones actually used.

## Decision

Decouple where `channels.json` is read from, from `-lib-dir`. `channels.NewStore` now takes two directories: `configDir` (where `channels.json` lives) and `outputBaseDir` (the default base for generated export output, `<outputBaseDir>/channels/<slug>/`, unchanged from before). A new `-channels-dir` / `UNTERLUMEN_CHANNELS_DIR` flag lets a deployment point `configDir` anywhere; when unset it defaults to `-lib-dir`, reproducing prior behavior exactly.

`outputBaseDir` always stays `-lib-dir`, regardless of where `channels.json` lives. Generated site/gallery HTML, avatars, and logos are per-installation working output (e.g. the file a user SCPs elsewhere), not shared config, so they should not follow the config file to a shared location.

A user with the Mac+NAS setup above sets `-channels-dir`/`UNTERLUMEN_CHANNELS_DIR` to the same directory (reachable by both machines, e.g. a folder on the NAS-hosted share) on both installations, while leaving `-lib-dir` independent on each.

## Consequences

**Positive:**
- Multiple installations sharing a photo library can also share channel identity (name, format, credentials) without sharing anything else — `library.db`, thumbnails, and the search index remain fully independent and local, preserving the offline-capable benefit of running a second installation.
- The change is purely additive: with `-channels-dir` unset, behavior is byte-for-byte identical to before (config and output both under `-lib-dir`).
- The XMP-sidecar-based publish record — already the cross-install source of truth — is unaffected either way.

**Negative / Trade-offs:**
- `channels.json` can contain plaintext credentials for some channel handlers (e.g. a Mastodon token). Pointing `-channels-dir` at a shared directory means those credentials now live wherever that directory is, and any installation with access to it can read them. This is an explicit, opt-in choice by the operator, not a default.
- The shared directory must be writable by every installation that publishes from it (a read-only mount, e.g. the `:ro` pattern in the Docker Compose example in the README, does not work for `-channels-dir`).
- This does not address the separate, pre-existing risk that a Docker deployment without an explicit `UNTERLUMEN_LIB_DIR` volume stores its library database in the container's ephemeral filesystem — documented as a caveat in the README, not fixed by this ADR.
