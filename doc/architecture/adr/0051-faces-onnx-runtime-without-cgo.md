# ADR-0051: Faces found on this computer, with ONNX Runtime loaded without cgo

*Last modified: 2026-10-23*

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

7. **People is a place** (`#people`, key 0) under Explore, with three kinds
   of page since 2026-10-20 (feature doc 2026-10-20-people-one-place):
   everyone named in any library, with a field to find them and a line of
   the libraries with groups waiting for a name (`GET /api/people/to-name`);
   the groups waiting in one library (`#people/name/<library>`), one row
   each with the field for a name beside the faces and a suggested name as a
   button of its own, and a group's page (`#people/name/<library>/group/<n>`)
   where nothing is selected at first: Name all, or Name the faces selected;
   and a person's page by name (`#people/person/<name>`). The explanation of
   People shows only while nobody is named. Older addresses (`#people/all…`,
   `#people/<library>…`) lead to their new place. Resting on a face, or
   focusing it with the keyboard, shows it in its whole photo with the face
   framed and the rest dimmed (`FacePeek`, after 450 ms, clear of the bar at
   the bottom), since a small face is often recognised by where it is; a
   face's picture is cut from inside its photo, off centre at an edge rather
   than padded. Show photos opens Libraries filtered to the person: the
   library filter's Person chip (`person=` in the photo query, repeated for
   several people, who must all be in the photo; matched without regard to
   case) works across libraries. A photo's info panel, wherever it shows a
   library photo, lists its faces (`InfoPeople`, `GET …/photo/{id}/faces`): a
   name, or a field to give one, and a × to take it away. Every name field
   offers the names of all libraries.

8. **A person is one page, linked by name across libraries.** Each library
   keeps its own people, faces and groups; the same name, without regard to
   case, is the same person in every library — the only thing the sidecars
   carry, so it needs nothing stored and holds on every installation. A
   person's page (`GET /api/people/someone?name=`) has all their faces of
   every library with the name in one grid, by the date of their photos,
   newest first and undated last, whichever library they are in. Faces
   selected in one library at a time — a face's number means something only
   in its own — can become the cover (`PUT …/people/{id}/cover`,
   `people.cover`; this installation's choice, out of the sidecar, falling
   back to their largest face). Across libraries — the overview, the
   person's page, Often with, the statistics — the cover chosen last shows
   the person (`people.cover_chosen`, only while the face is still theirs);
   where nobody chose one, the cover of the library with most of their
   faces. They can also be given to someone else in one step (*Is
   instead*, `POST …/faces/name`), or go back to the groups (`POST
   …/faces/unname`). Rename… and Take the name away… (`PATCH`/`DELETE
   …/people/{id}`) list the libraries with the name, all ticked; an untick
   leaves a library as it is, which is how two different people of one name
   are kept apart — by giving one of them another name, which is what
   Lightroom sees too. For each library without the name, the person's faces
   are compared, on this computer, with the people and the groups waiting
   for a name there (`SuggestThreshold`): someone named otherwise gets
   *Rename to Anna in …*, a group *Name them Anna*, both in place, writing
   that library's sidecars; libraries without a suggestion are one line.
   Explicit links between differently named people were rejected (again on
   2026-10-20, by the owner): they would need a list of links stored
   somewhere, local (the other installation would not know them) or shared
   (a second file two installations can overwrite), the library filter
   would have to follow them, and Lightroom would still see two names.

9. **People in the statistics.** A topic of its own (ADR-0043), read from
   `GET /api/library/people-stats` through `aliasCond`, so the folder and
   the shared filter apply (ADR-0050). It is not cached: names change
   outside scans. Only named people count, put together by name; the
   numbers of faces per photo count every face found, named or not, and
   only photos that were looked at. The search takes `faces_min` and
   `faces_max` for those. Its 3D views (ADR-0046): the Face space places
   every face by its print, reduced to the three principal axes of the
   named people's average faces (of the faces themselves while fewer than
   four people are named), so people stand apart and one person's faces
   together; computed on this computer, at most 40,000 faces, every named
   one kept. The Circle of people lays people out by the photos they share,
   the same way on every visit; Life lines give each of the eight people
   with the most photos a lane through time.

10. **Birthdays, for ages.** A person's ⋯ menu, on a library's person page
    and in All libraries, gives them a birthday. It is kept by name in
    `people.json` beside `channels.json` (`internal/birthdays`), the folder
    installations share, so the other installation and every library know
    it; written whole with `atomicfile.Change` like a sidecar. It stays out
    of the sidecars: it is about a person, not a photo. With it, the info
    panel shows each named face's age on the day the photo was taken
    (`PhotoFace.Age`), the person pages say when they were born, Statistics
    counts photos per year of life (Photos by age), and Life lines name the
    age in a photo. No model guesses an age from a face: that would be a
    further download, often years off, and a new inference from faces.

11. **People on a phone is for finding.** The phone's tab bar has People in
    place of Galleries (publishing is desktop work, ADR-0028's phone rule).
    `PeoplePhone` shows everyone named in any library with a search over
    the names, and a person by name: their photos across libraries, newest
    first by year (the library search's `person=`), when they were born,
    and who is most often with them (`SomeoneEverywhere.With`, from each
    library's `Store.Companions`); choosing those narrows the photos to all
    of them together. No groups, naming, hiding, covers or birthdays; the
    info panel lists named faces only. A library's person address on a
    phone leads to the person by name.

12. **A face that was not found is marked by hand.** A face in profile or
    half hidden is often missed. In the viewer, the info panel's People
    section has Mark a face: a frame drawn on the photo as for a crop
    (`FaceMarkTool`, `CropTool` without the thirds), then the name in a
    field below it; `POST /api/library/{id}/photo/{photoID}/faces` takes
    the box in fractions of the upright photo and the name. Decided with
    the owner on 2026-10-23: frame first, then name; no frame without a
    name. The face is a row of `faces` with `manual = 1`, confirmed, named,
    with an empty embedding: it counts everywhere a named face counts —
    People, covers, filters, statistics, ages — but takes no part in
    groups, suggestions or the Face space, as a profile would only spoil
    them. It is never unnamed: × or hiding it removes it. A frame over a
    known face is refused (409).
    Its region goes into the sidecar like any name, and that is how the
    other installation learns of it, whose detector misses it too: a scan
    takes the regions no found face covers as the photo's faces marked by
    hand — adding, renaming and removing them as the sidecar says, removing
    none while another program keeps regions there (`applyMarkedFaces`). So
    a face Lightroom or digiKam named where Unterlumen found none is a
    person in the photo too. A pass keeps faces marked by hand; one it now
    finds itself takes over the name and the frame drawn by hand goes
    (`settleMarkedFaces`). Moving or resizing a saved frame and marking on
    a phone are left out.

13. **Suggestions learn from every name and every No.** Decided with the
    owner on 2026-10-23; the model itself is never trained. A face, or a
    group, is compared with each named person's own faces: the mean
    similarity of the nearest three (`faces.Resemblance`), not with the
    average of all of them, which lies between frontal and profile and
    looks like neither. Measured on a copy of a library of 12 554 named
    faces of 143 people, every face left out in turn: the nearest three
    suggested the right person for 98.1 % and a wrong one for 1.2 %; the
    average managed 92.3 % and 4.8 %. `SuggestThreshold` stays 0.45. To
    stay fast, the averages pick the five people who come into question
    and only they are compared face by face; the prints are cached per
    library until its confirmed faces change (`lookalikes`). A No is a row
    of `face_not` (face, person), kept in this index like hiding: the
    suggestion's No, "This is not Anna" and *Is instead* all write one,
    and naming the face that person after all takes it back. Suggestions
    are shown per group (Waiting for a name), per face in a photo's info
    panel, and on a person's page as *Maybe Anna*: the unnamed faces
    that look most like them of everyone named, at most 60, to take or
    to refuse in one go (`GET /api/people/maybe`, `POST
    …/faces/not`). Nothing is named without the owner. Left out, by the
    owner's choice: listing named faces that may be wrong, prints for
    faces marked by hand, and looking again for faces the detector missed.
    A group is often mixed, so its own page starts from its suggestion
    (`GET …/people/groups/{grp}/suggestion`), names or refuses the
    selected faces or all, and stays while faces are left. Deselected
    faces get no No of their own: they may only be named in a second go.
    Hidden faces are listed per library (`GET …/people/hidden`) and shown
    again (`POST …/faces/show`, then the unnamed faces are grouped anew);
    a frame drawn by hand over a hidden face names that face. People's
    first page lists the hidden faces of all libraries (`GET
    /api/people/hidden`).

14. **Compare in the People topic.** Up to eight people chosen in the
    topic's head are put first, in that order (`peopleCompared`), and the
    charts that show several people — Who appears most, People over time,
    Photos by age, Who with whom, Life lines — show those and only those,
    in the first colours of the ramp. Nobody chosen is the people with the
    most photos, as before. The choice is the browser's (localStorage, a
    per-viewer convenience); redrawing asks the server nothing. It is not
    the shared filter's People (ADR-0050), which keeps the photos with all
    of them together.

15. **Hiding, No's and covers are shared through the shared folder.** They
    were this installation's only, while the NAS and the Mac index the same
    photos. The owner asked on 2026-10-23 for them to follow; they still stay
    out of the sidecars, which say only who is in a photo (decided
    2026-10-19). `internal/facedecisions` keeps `face-decisions.json` beside
    `channels.json` and `people.json`: hidden faces, No's by name, covers by
    name with when they were chosen (the last chosen wins). A face is its
    photo's SHA-256 — the same on every installation — and its box,
    matched by overlap like a sidecar's region, since each installation
    finds its own faces. As with sidecars (ADR-0052), the file is written
    first (`atomicfile.Change`, the decisions of the other installation made
    meanwhile kept) and the index follows: `Store.ApplyDecisions` after a
    scan, after a face pass, and with every check of the shared libraries
    (at start, then every 30 minutes) makes the index's hidden faces, No's
    and covers what the file says — a face shown again there is shown again
    here; a face the sidecar names here is not hidden. The first time, each
    library puts its own earlier decisions into the file (`exportDecisions`,
    the library property `face_decisions_shared` holding the file's path,
    so a new shared folder gets them too), so sharing loses nothing; measured on a copy of the largest library: 190 hidden faces, 46
    No's and 11 covers, unchanged, 65 ms, 60 KB. Once shared, a missing file
    is an error (`facedecisions.ErrMissing`), never "no decisions": an
    unmounted shared folder must not show every hidden face again.

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
- Renaming a person in one library splits them from the same name in the
  others until they are renamed there too; the dialog says so.
- The engine stays loaded once used; the download cannot be undone from the
  app. Deleting `<lib-dir>/recognition` and restarting turns it off.
- Biometric data stays on the machine. "Your data" says what is downloaded and
  that nothing is sent.
