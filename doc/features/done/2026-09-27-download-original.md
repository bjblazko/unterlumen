# Download originals: from the viewer and from a selection

*Last modified: 2026-09-27*

## Summary

The viewer has a button that downloads the photo on screen as the original file. Asked for by the owner on 2026-09-27 for "all views that show a picture"; the viewer is that view, wherever it is opened from.

## Details

- **Button.** A framed icon button (arrow into a tray) beside the full-screen button, named "Download the original file". It is a link with `download`, so the browser saves the file itself; nothing runs in the page.
- **Everywhere the viewer is.** Folders, a library's folders, filter results, the map's read-only viewer, and the phone's viewer. The address is the viewer's own image address with `download=1`, so every place that opens the viewer gets it without extra wiring.
- **The original.** `GET /api/image?path=…&download=1` and `GET /api/library/{id}/photo/{photoID}?download=1` return the file unchanged with `Content-Disposition: attachment` and its own name (RFC 2231 encoded when it is not plain ASCII). A HEIF comes as HEIF; the viewer shows it as JPEG, the download does not. The browse route checks the path against the root as before; the library route uses the path from the index.
- **From a selection.** Asked for next "where we do not see the photo but can select it". The selection bar (Folders, Organize, the library overview and a library) has Download after Export — the bar is where actions on a selection live, not the ⋯ menu. One photo downloads as its file, as in the viewer. Several go through the export's ZIP stream with the new format `original`: each file is stored unchanged (`zip.Store`, no recompression), names are made unique ("IMG_0001 (2).JPG"), the status line says "Packing N originals into a ZIP", and the browser saves `<folder>.zip` (`Photos.zip` for filter results) straight to disk via `/api/export/zip-download?name=`. A ZIP that would be empty — every file unreadable or, in server mode, outside the root — ends with an error the page shows.
- **Folders.** A selected folder downloads as a ZIP of everything in it, subfolders included, with its own name at the top of the paths inside (`Travel/2024/IMG_1.jpg`); hidden files and anything Unterlumen does not read are left out. One folder alone gives `<folder>.zip`. The selection bar counts folders in every place now; with only folders selected, Download and Show in Organize work and the photo-only actions are disabled with "Works on photos. Select photos to use it."
- **Filter results on a server.** Library photos go to the ZIP export as `{ library, id }`, and the server looks the file up in the index. Filter results carry absolute paths, which server mode refuses, so their ZIPs used to come out empty — Export included. In server mode a library `sourcePath` sent by the page must also lie inside the browse root (`pathguard.Inside`); before, any existing folder was accepted as the root for relative paths.
- **Where the code is.** `internal/api/download` (`Requested`, `Original`), used by both routes; `FormatOriginal`, `addZipEntry`, `uniqueEntryName` and `zipSources.collect` (files, folders, library photos) in `internal/api/export`; `pathguard.Inside`; `originals.js` on the page.

## Acceptance Criteria

- [x] The viewer shows a download button in every context, including the read-only one on the map.
- [x] The download is the original file, byte for byte, under its own name; a HEIF stays HEIF.
- [x] A missing file answers 404; a non-ASCII name survives.
- [x] On a phone the button fits in the viewer's bar.
- [x] The selection bar downloads one photo as its file and several as a ZIP of unchanged originals with unique names, named after the folder.
- [x] An empty ZIP is reported, not downloaded.
- [x] A selected folder downloads with everything in it, in its folders; the bar appears for folders and disables what works on photos only.
- [x] Filter results pack by library and photo ID, so the ZIP works in server mode; a source folder outside the root is refused there.
- [x] Go tests for `internal/api/download`; `viewer-download.spec.js` downloads from Folders, a HEIF, a library and the map; `phone.spec.js` checks the button on a phone; the same spec downloads one and several photos from the selection bar in Folders and in a library; `zip_test.go` covers stored originals, unique names, the download name and the empty ZIP.
