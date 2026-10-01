# Independent libraries, sharing per library

*Last modified: 2026-10-01*

## Summary

The installed app no longer asks for one photo folder. Each library names its
own folder, anywhere, and Folders browses the whole disk (camera cards, NAS
mounts). Sharing with a second installation is decided per library: a shared
library carries a small marker in its folder, which gives it one identity on
every installation that adds the folder. Destinations and galleries are shared
through one shared folder chosen in Settings. See
[ADR-0047](../../architecture/adr/0047-independent-libraries-shared-per-library.md).

## Details

- Setup (`#setup`): shared folder (Choose… / Do not share), data folder, helper
  programs. No photo folder, and no setup on a first start.
- Settings shows the shared folder and links to the setup.
- Edit library → Share with other installations (Shared / This installation
  only) writes or removes `.unterlumen-library.json` in the library's folder.
- New library on a folder with a marker takes the shared library's ID, name and
  description.
- A library whose folder another installation shares under another ID offers
  *Join* in the library; joining keeps the index and thumbnails and points this
  installation's drafts at the new ID.
- The library overview says "shared", "shared by another installation" or "not
  connected" after a library's folder; the library itself says what a missing
  folder means.
- Migration: `photosDir` in config.json is dropped at start; its
  `.unterlumen-shared` becomes the shared folder when none was chosen.
- Server (`UNTERLUMEN_ROOT_PATH`) and folder-argument runs are unchanged.

## Acceptance Criteria

- [x] The installed app starts without a setup and Folders reaches the whole disk.
- [x] The setup chooses a shared folder, refuses a disk root, and joins a folder another installation shares through.
- [x] config.json with `photosDir` is migrated: the photo folder's shared folder is kept, `photosDir` removed; a photo folder that is not mounted is left for later.
- [x] Sharing a library writes the marker; unsharing removes only its own marker.
- [x] A second installation adding the folder gets the same ID and name.
- [x] A library made before sharing offers to join; joining keeps index and thumbnails and rewrites drafts.
- [x] A rename of a shared library shows on the other installation.
- [x] A library whose folder is missing says so instead of failing.
- [x] Go tests (`installation`, `library`, `channels`) and e2e (`setup`, `adopt-older-install`, `library-sharing`).
- [ ] Owner joins the NAS and Mac libraries of the same folders on the real installations.
