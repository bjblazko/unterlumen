# About as a place: your data and no warranty

*Last modified: 2026-10-04*

## Summary

What Unterlumen sends off the computer, and that it comes without warranty,
was written nowhere in the app. The About dialog becomes a place with topics —
How it works, Your data, No warranty, Licenses — reached from the logo and a
sidebar entry, and a first start says once that there is no warranty.

## Details

- `#about` (`about-place.js`): name, version, the topics with one line each,
  who makes it. Replaces `AboutModal` (ADR-0033 updated).
- The sidebar has About under Settings; its topics show under it while About
  or a topic is open, like Statistics' topics.
- `#privacy` (`privacy-place.js`): no account, no usage data, no update
  check; what stays; what leaves and when — map tiles (OpenFreeMap, seeing
  the IP and the area shown), Publish (rsync/SSH to the destination, the
  reachability check against your own site), Install the missing ones,
  links; no login on a server; published websites load nothing from others.
  CLAUDE.md asks for a line here for every new outbound request.
- `#warranty` (`warranty-place.js`): what it does to files, keep a backup,
  liability as far as the law allows.
- `WarrantyNotice`: once per browser (localStorage), at the foot of the
  sidebar, not blocking; Understood hides it for good. Screenshots and the
  supercut preset it.
- Delete permanently says that Unterlumen keeps no copy.

## Acceptance Criteria

- [x] Logo and sidebar entry open `#about`; topics show under it while open.
- [x] Your data names every outbound request in the code, and that there is no update check.
- [x] No warranty, and the first-start notice that leads to it and goes with Understood.
- [x] The permanent delete question says only a backup restores what is deleted.
- [x] e2e (`about-place`).
