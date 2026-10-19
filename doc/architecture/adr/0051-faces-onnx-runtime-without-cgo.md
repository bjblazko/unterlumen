# ADR-0051: Faces found on this computer, with ONNX Runtime loaded without cgo

*Last modified: 2026-10-19*

## Status

Accepted. Builds on [ADR-0044](0044-photo-appearance-from-thumbnails.md)
(a pass over the thumbnails beside the scans), [ADR-0048](0048-notes-live-in-the-sidecar.md)
(what the owner writes lives in the sidecar) and
[ADR-0042](0042-installation-one-line-setup-in-the-browser.md) (helper programs
installed from inside the app).

## Context

On 2026-10-18 the owner asked whether local AI could find the faces in the
photos and let them be named — later animals and things, and searches in
plain words — and what that would mean for installing Unterlumen: is it the
end of the single binary, would a container or cgo be needed?

Unterlumen builds with `CGO_ENABLED=0` everywhere (modernc SQLite, the
Dockerfile, the Windows packages), so it cross-compiles to every system from
one machine. Every serious way to run a neural network is native code:
ONNX Runtime, TensorFlow, XLA. The Go bindings of ONNX Runtime
(`yalue/onnxruntime_go` and its forks) need cgo. ONNX runtimes in pure Go
(gonnx, onnx-go, GoMLX's Go backend) are about eight times slower and cover
only part of the operators. A Python service beside the app, as Immich runs,
would make the Mac app two programs.

Decided with the owner: ONNX Runtime; no cgo; the names go into the XMP
sidecar as Metadata Working Group regions; an architecture that is not
"verbastelt".

## Decision

1. **ONNX Runtime is loaded at run time through purego.** `internal/onnx`
   opens the shared library with `dlopen` (`LoadLibrary` on Windows), asks
   `OrtGetApiBase` for the function table of C API version 17 and calls the
   about 25 functions it needs through `purego.RegisterFunc`. The table only
   ever grows, so the indexes (counted in `onnxruntime_c_api.h`) hold for every
   later library. Without the library the binary runs as before; nothing else
   imports `internal/onnx`. Telemetry events are turned off on the
   environment, which matters for Windows builds.

2. **Downloaded on request, at fixed versions, checked.** People offers
   *Download and turn on*: `toolinstall.InstallRecognition` fetches ONNX
   Runtime 1.31.0 from Microsoft's GitHub releases (36.5 MB on macOS arm64,
   11.4 MB on Linux x64, 10.3 MB on Linux arm64, 82.9 MB on Windows x64) and
   YuNet and SFace from OpenCV's model zoo on Hugging Face, pinned to one
   commit (39 MB). Every file is checked against its SHA-256 before it is
   written, into `recognition.download` beside the target, which takes the
   place of `<lib-dir>/recognition` only when all are there. Intel Macs get no
   build: ONNX Runtime 1.31 ships none. The container downloads into `/data`
   like any installation; nothing is added to the image.

3. **Two models from OpenCV's zoo.** YuNet (MIT, 0.2 MB) finds faces and their
   five landmarks at a fixed 640×640 input; the thumbnail (1200 px, already
   upright) is scaled to fit. Faces under 3 % of the longer side are dropped:
   too blurred to recognise. SFace (Apache-2.0) describes a face, turned and
   scaled onto its five-point template, by 128 numbers of unit length; the
   pre- and post-processing follow OpenCV's `FaceDetectorYN` and
   `FaceRecognizerSF`. InsightFace's models were not taken: their weights are
   for non-commercial research only.

4. **A pass beside the scans, like the appearance pass.** After a scan, after
   the appearance pass, and at start, `Manager.FindFacesMissing` looks at the
   photos with a thumbnail and no row in `photo_faces_found` (or one of an
   older `faces.Version`), two at a time, and stores the faces in `faces`.
   It holds no index lock and shows as "Finding faces in …" in the status
   line. At its end the unnamed faces are grouped again.

5. **Groups, then names.** Grouping is one round of joining the nearest
   group average (cosine ≥ 0.42) in order of face size, then one round of
   moving every face to the group whose average it is closest to. A group
   whose average is ≥ 0.45 close to a named person's average asks "This is
   …". Naming a group, or the faces left switched on in it, confirms them as
   that person; a name that exists joins them to that person; hiding marks
   faces nobody is to name. `faces.person_id` with `confirmed` holds this:
   unnamed (NULL, 0), named (id, 1), hidden (NULL, 1).

6. **Names live in the sidecar, and only names.** Confirmed names are
   `mwg-rs:Regions` in an `rdf:Description` of Unterlumen's own — told from
   another program's by its whole opening tag —, relative to the stored photo
   before its EXIF orientation, as the MWG asks. Following
   [ADR-0052](0052-the-index-follows-the-folder.md), naming, taking a name
   away, renaming and hiding a named face write the sidecar first and the
   index after; a photo whose sidecar cannot be written keeps its faces as
   they were. A write changes only the faces it is about, in the sidecar as
   it is on disk (`media.ChangeFaceRegions`), so a name the other installation
   wrote in the meantime stays. A sidecar whose regions sit in another
   program's description (Lightroom keeps everything in one) is left alone;
   its names stay in this index and the page says so. Every scan makes the
   index's names what the sidecar says (`Store.ApplyFaceNames`): a face takes
   the name of the region it overlaps most (IoU ≥ 0.3), and a named face no
   region covers loses its name — taken away on the other installation —
   unless another program keeps regions there. The sidecar holds only who is
   in a photo, never a decision of one installation: hidden faces stay
   hidden on the installation that hid them, and the other may ask again.

7. **People is a place** (`#people`, key 0) under Explore: one library at a
   time, the named people as cards, the groups waiting for a name below with
   a field each, and pages for a group (switch faces off before naming) and a
   person (switch faces on to take them away, Show photos, Rename). Resting on
   a face, or focusing it with the keyboard, shows it in its whole photo with
   the face framed and the rest dimmed (`FacePeek`), since a small face is
   often recognised by where it is. Show photos opens Libraries filtered to the
   person: the library filter's Person chip (`person=` in the photo query,
   matched without regard to case) works across libraries.

## Consequences

- The single binary stays: cgo-free, cross-compiled as before. Face
  recognition is an optional download of 50–120 MB per installation.
- **On Linux the binary is now dynamically linked.** purego's `dlopen`
  without cgo makes the Go linker import `dlopen`, `malloc` and friends
  from glibc, so the executable names `/lib/ld-linux-*.so.1` as its
  interpreter. It runs on every glibc distribution and in the container
  (Debian); it no longer runs on musl (Alpine) or on NixOS without nix-ld.
  macOS binaries were always linked against libSystem, Windows loads with
  `LoadLibrary`: no change there. Accepted by the owner on 2026-10-18:
  on musl or bare NixOS, run the container. A separate helper process
  would keep the binary static, at the price of a second program and an
  interface between the two — not worth it for those systems.
- The `onnx` package is general: animals, things and plain-word search
  (CLIP-like models) can follow on the same runtime with other models.
- A face's box is in the thumbnail's upright frame. A thumbnail rebuilt the
  other way up would misplace boxes; the names come back from the sidecar
  when the faces are found again.
- Faces found by an older `faces.Version` are found again and their names
  read back from the sidecar; names kept in the index only (another program's
  sidecar) would be lost then.
- A name taken away or changed on one installation leaves or changes on the
  other at its next scan or check of the library.
- The engine stays loaded once used; the download cannot be undone from the
  app. Deleting `<lib-dir>/recognition` and restarting turns it off.
- Biometric data stays on the machine. "Your data" says what is downloaded and
  that nothing is sent.
