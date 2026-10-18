# People: faces found and named

*Last modified: 2026-10-18*

## Summary

Unterlumen finds the faces in a library's photos on the user's own computer,
groups the ones that look alike and lets the owner name them. The names are
written into the photos' XMP sidecars. It is the first step of local
recognition; animals (named pets), things and searches in plain words may
follow on the same runtime. See [ADR-0051](../../architecture/adr/0051-faces-onnx-runtime-without-cgo.md).

## Details

- **Runtime:** ONNX Runtime 1.31.0, loaded at run time through purego — no
  cgo, the binary stays one file. Downloaded on request from Microsoft's
  GitHub releases into `<lib-dir>/recognition`, with YuNet and SFace from
  OpenCV's model zoo on Hugging Face; every file checked by SHA-256.
- **Pass:** after each scan (after the appearance pass) and at start, photos
  with a thumbnail that were not looked at are looked at, two at a time; the
  status line says "Finding faces in …". The unnamed faces are grouped again
  at the end.
- **Place:** People under Explore (`#people`, key 0). One library at a time
  (a select when there are several). Named people as cards; groups waiting
  for a name, the 60 largest, each with a name field, "This is …" when it
  looks like someone named, and Hide for strangers. A group's page switches
  faces off before naming; a person's page switches faces on to take them
  away, shows the photos, renames.
- **Sidecar:** confirmed names as `mwg-rs:Regions` in Unterlumen's own
  description, in the stored photo's frame. Regions of other programs are
  read, never overwritten.
- **Measured** on an M-series Mac: about 30 ms per photo with decoding; 186
  faces in 1 500 photos of a real library. On the NAS (Raspberry Pi 5):
  about 140 ms per photo. Public-domain portraits of three
  people grouped without a mix-up.

## Acceptance Criteria

- [x] The binary builds and runs with `CGO_ENABLED=0`; without the download everything works as before
- [x] People offers the download, says what it fetches from where and how much, and that nothing is sent afterwards
- [x] Downloads are checked against fixed checksums and land complete or not at all
- [x] After a scan, new photos are looked at in the background, shown in the status line
- [x] Faces of the same person form one group; strangers stay apart
- [x] A group can be named in one line, or after switching faces off on its page
- [x] A name that exists joins the faces to that person; renaming to an existing name merges two people
- [x] A new group that looks like a named person asks "This is …"
- [x] Hide takes faces out of the groups; "not this person" puts faces back into a group
- [x] Names are written into the sidecar as MWG regions that exiftool reads; other programs' regions are read and left alone
- [x] A photo deleted from the library takes its faces with it
- [x] "Your data", Licenses and the README name the download
- [x] Go tests for the binding, detection decoding, grouping, storage, sidecar regions, download and API; an e2e spec for the place
- [x] Tried on the NAS (Raspberry Pi 5, aarch64) in a throwaway container from the 0.15.6 image with the branch's binary: 49 MB download, same three groups, about 140 ms per photo
- [x] Owner decided (2026-10-18): the Linux binary needs glibc's loader; musl and bare NixOS use the container (ADR-0051, Consequences)
- [ ] A name removed in one installation is removed in the other when it reads the sidecar (today reading only adds names)
- [ ] People shows its own photo column on a person's page instead of opening the viewer straight away
- [ ] The library filter can narrow to a person
